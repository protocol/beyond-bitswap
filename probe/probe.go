package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	ipfslibp2p "github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"golang.org/x/sync/errgroup"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"

	// bsnet "github.com/ipfs/go-bitswap/network"

	ma "github.com/multiformats/go-multiaddr"
)

// IPFSNode structure.
type IPFSNode struct {
	Node *core.IpfsNode
	API  icore.CoreAPI
}

// CreateTempRepo creates a new repo in /tmp/
func createTempRepo(ctx context.Context) (string, error) {
	repoPath, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}

// CreateIPFSNode an IPFS specifying exchange node and returns its coreAPI
func CreateIPFSNode(ctx context.Context) (*IPFSNode, error) {

	repoPath, err := createTempRepo(ctx)
	if err != nil {
		return nil, err
	}
	repo, err := fsrepo.Open(repoPath)
	swarmAddrs := []string{
		"/ip4/0.0.0.0/tcp/0",
		"/ip6/::/tcp/0",
		"/ip4/0.0.0.0/udp/0/quic",
		"/ip6/::/udp/0/quic",
	}
	if err := repo.SetConfigKey("Addresses.Swarm", swarmAddrs); err != nil {
		return nil, err
	}

	// Construct the node
	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: ipfslibp2p.DHTOption,
		Repo:    repo,
	}

	node, err := core.NewNode(ctx, nodeOptions)
	fmt.Println("Listening at: ", node.PeerHost.Addrs())
	fmt.Println("PeerInfo: ", host.InfoFromHost(node.PeerHost))
	if err != nil {
		return nil, fmt.Errorf("Failed starting the node: %s", err)
	}

	api, err := coreapi.NewCoreAPI(node)
	// Attach the Core API to the constructed node
	return &IPFSNode{node, api}, nil
}

// setupPlugins automatically loads plugins.
func setupPlugins(externalPluginsPath string) error {
	// Load any external plugins if available on externalPluginsPath
	plugins, err := loader.NewPluginLoader(filepath.Join(externalPluginsPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	// Load preloaded and external plugins
	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}

// PrintStats for the node.
func printStats(bs *metrics.Stats) {
	fmt.Printf("Bandwidth")
	fmt.Printf("TotalIn: %s\n", humanize.Bytes(uint64(bs.TotalIn)))
	fmt.Printf("TotalOut: %s\n", humanize.Bytes(uint64(bs.TotalOut)))
	fmt.Printf("RateIn: %s/s\n", humanize.Bytes(uint64(bs.RateIn)))
	fmt.Printf("RateOut: %s/s\n", humanize.Bytes(uint64(bs.RateOut)))
}

// conectPeer connects to a peer in the network.
func connectPeer(ctx context.Context, ipfs *IPFSNode, id string) error {
	maddr, err := ma.NewMultiaddr(id)
	if err != nil {
		fmt.Println("Invalid peer ID")
		return err
	}
	addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		fmt.Println("Invalid peer info")
		return err
	}
	err = ipfs.API.Swarm().Connect(ctx, *addrInfo)
	if err != nil {
		fmt.Println("Couldn't connect to peer")
		return err
	}
	fmt.Println("Connected successfully to peer")
	return nil
}

// get file from fs.
func getUnixfsFile(path string) (files.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	st, err := file.Stat()
	if err != nil {
		return nil, err
	}

	f, err := files.NewReaderPathFile(path, file, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func getUnixfsNode(path string) (files.Node, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := files.NewSerialFile(path, false, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}

var randReader *rand.Rand

// RandReader helper to generate random files.
func RandReader(len int) io.Reader {
	if randReader == nil {
		randReader = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}

// getContent gets a file from the network and computes time_to_fetch
func getContent(ctx context.Context, n *IPFSNode, fPath path.Path) error {
	fmt.Println("Searching for: ", fPath)
	start := time.Now()
	f, err := n.API.Unixfs().Get(ctx, fPath)
	if err != nil {
		return err
	}
	timeToFetch := time.Since(start)
	s, _ := f.Size()
	fmt.Printf("[*] Size of the file obtained %d in %s\n", s, timeToFetch)
	fmt.Println("Cleaning datastore")
	n.ClearDatastore(ctx)
	err = n.ClearBlockstore(ctx)
	if err != nil {
		fmt.Println("Error cleaning blockstore", err)
	}
	return nil
}

// adds random content to the network.
func addRandomContent(ctx context.Context, n *IPFSNode, size int) {
	tmpFile := files.NewReaderFile(RandReader(size))

	cidRandom, err := n.API.Unixfs().Add(ctx, tmpFile)
	if err != nil {
		panic(fmt.Errorf("Could not add random: %s", err))
	}
	fmt.Println("Adding a random file to the network:", cidRandom)
}

// adds a file from filesystem to the network.
func addFile(ctx context.Context, n *IPFSNode, inputPathFile string) error {
	someFile, err := getUnixfsNode(inputPathFile)
	if err != nil {
		fmt.Println("Could not get File:", err)
		return err
	}

	cidFile, err := n.API.Unixfs().Add(ctx, someFile)
	if err != nil {
		fmt.Println("Could not add random: ", err)
		return err
	}
	fmt.Println("Adding file to the network:", cidFile)
	return nil
}

// ClearDatastore removes a block from the datastore.
func (n *IPFSNode) ClearDatastore(ctx context.Context) error {
	ds := n.Node.Repo.Datastore()
	// Empty prefix to receive all the keys
	qr, err := ds.Query(dsq.Query{})
	if err != nil {
		return err
	}
	for r := range qr.Next() {
		if r.Error != nil {
			return r.Error
		}
		ds.Delete(datastore.NewKey(r.Entry.Key))
		ds.Sync(datastore.NewKey(r.Entry.Key))
	}
	return nil
}

// ClearBlockstore clears Bitswap blockstore.
func (n *IPFSNode) ClearBlockstore(ctx context.Context) error {
	bstore := n.Node.Blockstore
	ks, err := bstore.AllKeysChan(ctx)
	if err != nil {
		return err
	}
	g := errgroup.Group{}
	for k := range ks {
		c := k
		g.Go(func() error {
			return bstore.DeleteBlock(c)
		})
	}
	return g.Wait()
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("-- Getting an IPFS node running -- ")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := setupPlugins(""); err != nil {
		panic(fmt.Errorf("Failed setting up plugins: %s", err))
	}

	// Spawn a node using a temporary path, creating a temporary repo for the run
	fmt.Println("Spawning node on a temporary repo")
	ipfs, err := CreateIPFSNode(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	// Adding random content for testing.
	addRandomContent(ctx, ipfs, 11111)
	// Adding directory,
	fmt.Println("Adding inputData directory")
	addFile(ctx, ipfs, "../testbed/scripts/inputData")

	ch := make(chan string)
	chSignal := make(chan os.Signal)
	done := make(chan bool)
	signal.Notify(chSignal, os.Interrupt, syscall.SIGTERM)

	// Prompt routine
	go func(ch chan string, done chan bool) {
		for {
			fmt.Print(">> Enter command: ")
			text, _ := reader.ReadString('\n')
			ch <- text
			<-done
		}
	}(ch, done)

	// Processing loop.
	for {
		select {
		case text := <-ch:
			processInput(ctx, ipfs, text, done)

		case <-chSignal:
			fmt.Printf("\nUse exit to close the tool\n")
			fmt.Printf(">> Enter command: ")

		}
	}
}

// Process commands received from prompt
func processInput(ctx context.Context, ipfs *IPFSNode, text string, done chan bool) error {
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.ReplaceAll(text, " ", "")
	words := strings.Split(text, "_")
	// If we use add we can add random content to the network.
	if words[0] == "add" {
		size, err := strconv.Atoi(words[1])
		if err != nil {
			fmt.Println("Not a valid size for random add")
			return err
		}
		addRandomContent(ctx, ipfs, size)
	} else if words[0] == "exit" {
		os.Exit(0)
	} else if words[0] == "connect" {
		connectPeer(ctx, ipfs, words[1])
	} else if words[0] == "addFile" {
		addFile(ctx, ipfs, words[1])
	} else if words[0] == "get" {
		fPath := path.New(words[1])
		ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		err := getContent(ctxTimeout, ipfs, fPath)
		if err != nil {
			fmt.Println("Couldn't find content", err)
			return err
		}
	} else {
		fmt.Println("[!] Wrong command! Only available add, addFile, get, connect, exit")
	}
	done <- true
	// We could show metrics after each command in certain cases.
	// fmt.Println("=== METRICS ===")
	// bw := ipfs1.Node.Reporter.GetBandwidthTotals()
	// printStats(&bw)
	return nil
}
