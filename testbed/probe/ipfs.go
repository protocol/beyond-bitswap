package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/rand"
	mathRand "math/rand"

	"github.com/dustin/go-humanize"
	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	config "github.com/ipfs/go-ipfs-config"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/node"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	ipfslibp2p "github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/jbenet/goprocess"
	"go.uber.org/fx"
	"golang.org/x/sync/errgroup"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	mint "github.com/ipfs/go-metrics-interface"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"

	// bsnet "github.com/ipfs/go-bitswap/network"

	dsync "github.com/ipfs/go-datastore/sync"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	ma "github.com/multiformats/go-multiaddr"
)

// IPFSNode structure.
type IPFSNode struct {
	Node  *core.IpfsNode
	API   icore.CoreAPI
	Close func() error
}

type NodeConfig struct {
	Addrs    []string
	AddrInfo *peer.AddrInfo
	PrivKey  []byte
}

func getFreePort() string {
	mathRand.Seed(time.Now().UnixNano())
	notAvailable := true
	port := 0
	for notAvailable {
		port = 3000 + mathRand.Intn(5000)
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			notAvailable = false
			_ = ln.Close()
		}
	}
	return strconv.Itoa(port)
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
	if err := repo.SetConfigKey("Discovery.MDNS.Enabled", false); err != nil {
		return nil, err
	}

	// Construct the node
	nodeOptions := &core.BuildCfg{
		Online: true,
		// Routing: ipfslibp2p.DHTOption,
		Routing: ipfslibp2p.NilRouterOption,
		Repo:    repo,
	}

	node, err := core.NewNode(ctx, nodeOptions)
	fmt.Println("Listening at: ", node.PeerHost.Addrs())
	for _, i := range node.PeerHost.Addrs() {
		a := strings.Split(i.String(), "/")
		if a[1] == "ip4" && a[2] == "127.0.0.1" && a[3] == "tcp" {
			fmt.Println("Connect from other peers using: ")
			fmt.Printf("connect_/ip4/127.0.0.1/tcp/%v/p2p/%s\n", a[4], node.PeerHost.ID().Pretty())
		}

	}
	fmt.Println("PeerInfo: ", host.InfoFromHost(node.PeerHost))
	if err != nil {
		return nil, fmt.Errorf("Failed starting the node: %s", err)
	}

	api, err := coreapi.NewCoreAPI(node)
	// Attach the Core API to the constructed node
	return &IPFSNode{node, api, node.Close}, nil
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
	fmt.Println("Multiaddr", maddr)
	addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		fmt.Println("Invalid peer info", err)
		return err
	}
	err = ipfs.API.Swarm().Connect(ctx, *addrInfo)
	if err != nil {
		fmt.Println("Couldn't connect to peer", err)
		return err
	}
	fmt.Println("Connected successfully to peer")
	return nil
}

func GenerateAddrInfo(ip string) (*NodeConfig, error) {
	// Use a free port
	port := getFreePort()
	// Generate new KeyPair instead of using existing one.
	priv, pub, err := ci.GenerateKeyPairWithReader(ci.RSA, 2048, rand.Reader)
	if err != nil {
		panic(err)
	}
	// Generate PeerID
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		panic(err)
	}
	// Get PrivKey
	privkeyb, err := priv.Bytes()
	if err != nil {
		panic(err)
	}

	addrs := []string{
		fmt.Sprintf("/ip4/%s/tcp/%s", ip, port),
		"/ip6/::/tcp/" + port,
		fmt.Sprintf("/ip4/%s/udp/%s/quic", ip, port),
		fmt.Sprintf("/ip6/::/udp/%s/quic", port),
	}
	multiAddrs := make([]ma.Multiaddr, 0)

	for _, a := range addrs {
		maddr, err := ma.NewMultiaddr(a)
		if err != nil {
			return nil, err
		}
		multiAddrs = append(multiAddrs, maddr)
	}

	return &NodeConfig{addrs, &peer.AddrInfo{ID: pid, Addrs: multiAddrs}, privkeyb}, nil
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

var randReader *mathRand.Rand

// RandReader helper to generate random files.
func RandReader(len int) io.Reader {
	if randReader == nil {
		randReader = mathRand.New(mathRand.NewSource(time.Now().UnixNano()))
	}
	data := make([]byte, len)
	randReader.Read(data)
	return bytes.NewReader(data)
}

// getContent gets a file from the network and computes time_to_fetch
func getContent(ctx context.Context, n *IPFSNode, fPath path.Path, pin bool) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var (
		timeToFetch time.Duration
		f           files.Node
		err         error
	)

	fmt.Println("Searching for: ", fPath)
	fileName := "/tmp/" + time.Now().String()
	if pin {
		start := time.Now()
		// Pinning also traverses the full graph
		err = n.API.Pin().Add(ctx, fPath)
		if err != nil {
			return err
		}
		timeToFetch = time.Since(start)
		fmt.Printf("[*] Pinned file in %s\n", timeToFetch)
	} else {
		start := time.Now()
		f, err = n.API.Unixfs().Get(ctx, fPath)
		if err != nil {
			return err
		}
		// We need to write the file in order to traverse de DagReader.
		err = files.WriteTo(f, fileName)
		if err != nil {
			return err
		}
		timeToFetch = time.Since(start)
		s, _ := f.Size()
		fmt.Printf("[*] Size of the file obtained %d in %s\n", s, timeToFetch)
		fmt.Println("Wrote in ")
	}

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
	fmt.Println(someFile)
	start := time.Now()
	cidFile, err := n.API.Unixfs().Add(ctx, someFile)
	end := time.Since(start).Milliseconds()
	if err != nil {
		fmt.Println("Could not add file: ", err)
		return err
	}
	fmt.Println("Adding file to the network:", cidFile)
	fmt.Printf("Added in %d (ms)\n", end)
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

// setConfig manually injects dependencies for the IPFS nodes.
func setConfig(ctx context.Context, nConfig *NodeConfig, DHTenabled bool) fx.Option {

	// Create new Datastore
	// TODO: This is in memory we should have some other external DataStore for big files.
	d := datastore.NewMapDatastore()
	// Initialize config.
	cfg := &config.Config{}

	// Use defaultBootstrap
	cfg.Bootstrap = config.DefaultBootstrapAddresses

	//Allow the node to start in any available port. We do not use default ones.
	cfg.Addresses.Swarm = nConfig.Addrs

	cfg.Identity.PeerID = nConfig.AddrInfo.ID.Pretty()
	cfg.Identity.PrivKey = base64.StdEncoding.EncodeToString(nConfig.PrivKey)

	// Repo structure that encapsulate the config and datastore for dependency injection.
	buildRepo := &repo.Mock{
		D: dsync.MutexWrap(d),
		C: *cfg,
	}
	repoOption := fx.Provide(func(lc fx.Lifecycle) repo.Repo {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return buildRepo.Close()
			},
		})
		return buildRepo
	})

	// Enable metrics in the node.
	metricsCtx := fx.Provide(func() helpers.MetricsCtx {
		return helpers.MetricsCtx(ctx)
	})

	// Use DefaultHostOptions
	hostOption := fx.Provide(func() libp2p.HostOption {
		return libp2p.DefaultHostOption
	})

	dhtOption := libp2p.NilRouterOption
	if DHTenabled {
		dhtOption = libp2p.DHTOption // This option sets the node to be a full DHT node (both fetching and storing DHT Records)
		//dhtOption = libp2p.DHTClientOption, // This option sets the node to be a client DHT node (only fetching records)
	}

	// Use libp2p.DHTOption. Could also use DHTClientOption.
	routingOption := fx.Provide(func() libp2p.RoutingOption {
		// return libp2p.DHTClientOption
		//TODO: Reminder. DHTRouter disabled.
		return dhtOption
	})

	// Return repo datastore
	repoDS := func(repo repo.Repo) datastore.Datastore {
		return d
	}

	exch := func(mctx helpers.MetricsCtx, lc fx.Lifecycle,
		host host.Host, rt routing.Routing, bs blockstore.GCBlockstore) exchange.Interface {
		bitswapNetwork := network.NewFromIpfsHost(host, rt)
		exch := bitswap.New(helpers.LifecycleCtx(mctx, lc), bitswapNetwork, bs)

		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return exch.Close()
			},
		})
		return exch
	}

	// Assign some defualt values.
	var repubPeriod, recordLifetime time.Duration
	ipnsCacheSize := cfg.Ipns.ResolveCacheSize
	enableRelay := cfg.Swarm.Transports.Network.Relay.WithDefault(!cfg.Swarm.DisableRelay) //nolint

	// Inject all dependencies for the node.
	// Many of the default dependencies being used. If you want to manually set any of them
	// follow: https://github.com/ipfs/go-ipfs/blob/master/core/node/groups.go
	return fx.Options(
		// RepoConfigurations
		repoOption,
		hostOption,
		routingOption,
		metricsCtx,

		// Setting baseProcess
		fx.Provide(baseProcess),

		// Storage configuration
		fx.Provide(repoDS),
		fx.Provide(node.BaseBlockstoreCtor(blockstore.DefaultCacheOpts(),
			false, cfg.Datastore.HashOnRead)),
		fx.Provide(node.GcBlockstoreCtor),

		// Identity dependencies
		node.Identity(cfg),

		//IPNS dependencies
		node.IPNS,

		// Network dependencies
		// Set exchange option.
		fx.Provide(exch),
		// Provide graphsync
		fx.Provide(Graphsync),
		fx.Provide(node.Namesys(ipnsCacheSize)),
		fx.Provide(node.Peering),
		node.PeerWith(cfg.Peering.Peers...),

		fx.Invoke(node.IpnsRepublisher(repubPeriod, recordLifetime)),

		fx.Provide(p2p.New),

		// Libp2p dependencies
		node.BaseLibP2P,
		fx.Provide(libp2p.AddrFilters(cfg.Swarm.AddrFilters)),
		fx.Provide(libp2p.AddrsFactory(cfg.Addresses.Announce, cfg.Addresses.NoAnnounce)),
		fx.Provide(libp2p.SmuxTransport(cfg.Swarm.Transports)),
		fx.Provide(libp2p.Relay(enableRelay, cfg.Swarm.EnableRelayHop)),
		fx.Provide(libp2p.Transports(cfg.Swarm.Transports)),
		fx.Invoke(libp2p.StartListening(cfg.Addresses.Swarm)),
		// TODO: Reminder. MDN discovery disabled.
		fx.Invoke(libp2p.SetupDiscovery(false, cfg.Discovery.MDNS.Interval)),
		fx.Provide(libp2p.Routing),
		fx.Provide(libp2p.BaseRouting),
		// Enable IPFS bandwidth metrics.
		fx.Provide(libp2p.BandwidthCounter),

		// Here you can see some more of the libp2p dependencies you could set.
		// fx.Provide(libp2p.Security(!bcfg.DisableEncryptedConnections, cfg.Swarm.Transports)),
		// maybeProvide(libp2p.PubsubRouter, bcfg.getOpt("ipnsps")),
		// maybeProvide(libp2p.BandwidthCounter, !cfg.Swarm.DisableBandwidthMetrics),
		// maybeProvide(libp2p.NatPortMap, !cfg.Swarm.DisableNatPortMap),
		// maybeProvide(libp2p.AutoRelay, cfg.Swarm.EnableAutoRelay),
		// autonat,		// Sets autonat
		// connmgr,		// Set connection manager
		// ps,			// Sets pubsub router
		// disc,		// Sets discovery service
		node.OnlineProviders(cfg.Experimental.StrategicProviding, cfg.Reprovider.Strategy, cfg.Reprovider.Interval),

		// Core configuration
		node.Core,
	)
}

// baseProcess creates a goprocess which is closed when the lifecycle signals it to stop
func baseProcess(lc fx.Lifecycle) goprocess.Process {
	p := goprocess.WithParent(goprocess.Background())
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return p.Close()
		},
	})
	return p
}

// CreateIPFSNodeWithConfig constructs and returns an IpfsNode using the given cfg.
func CreateIPFSNodeWithConfig(ctx context.Context, nConfig *NodeConfig, DHTEnabled bool) (*IPFSNode, error) {
	// save this context as the "lifetime" ctx.
	lctx := ctx

	// derive a new context that ignores cancellations from the lifetime ctx.
	ctx, cancel := context.WithCancel(ctx)

	// add a metrics scope.
	ctx = mint.CtxScope(ctx, "ipfs")

	n := &core.IpfsNode{}

	app := fx.New(
		// Inject dependencies in the node.
		setConfig(ctx, nConfig, DHTEnabled),

		fx.NopLogger,
		fx.Extract(n),
	)

	var once sync.Once
	var stopErr error
	stopNode := func() error {
		once.Do(func() {
			stopErr = app.Stop(context.Background())
			if stopErr != nil {
				fmt.Errorf("failure on stop: %w", stopErr)
			}
			// Cancel the context _after_ the app has stopped.
			cancel()
		})
		return stopErr
	}
	// Set node to Online mode.
	n.IsOnline = true

	go func() {
		// Shut down the application if the lifetime context is canceled.
		// NOTE: we _should_ stop the application by calling `Close()`
		// on the process. But we currently manage everything with contexts.
		select {
		case <-lctx.Done():
			err := stopNode()
			if err != nil {
				fmt.Errorf("failure on stop: %v", err)
			}
		case <-ctx.Done():
		}
	}()

	if app.Err() != nil {
		return nil, app.Err()
	}

	if err := app.Start(ctx); err != nil {
		return nil, err
	}

	if err := n.Bootstrap(bootstrap.DefaultBootstrapConfig); err != nil {
		return nil, fmt.Errorf("Failed starting the node: %s", err)
	}
	api, err := coreapi.NewCoreAPI(n)
	if err != nil {
		return nil, fmt.Errorf("Failed starting API: %s", err)

	}
	fmt.Println("Listening at: ", n.PeerHost.Addrs())
	for _, i := range n.PeerHost.Addrs() {
		a := strings.Split(i.String(), "/")
		if a[1] == "ip4" && a[2] == "127.0.0.1" && a[3] == "tcp" {
			fmt.Println("[*] Connect from other peers using: ")
			fmt.Printf("connect_/ip4/127.0.0.1/tcp/%v/p2p/%s\n", a[4], n.PeerHost.ID().Pretty())
		}
	}
	fmt.Println("[*] Try graphsync from other peers using: ")
	fmt.Printf("graphsync_<multiaddr>_<CID>")

	// Attach the Core API to the constructed node
	return &IPFSNode{n, api, stopNode}, nil
}
