package utils

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"path/filepath"
	"sync"
	"time"

	bs "github.com/ipfs/go-bitswap"
	config "github.com/ipfs/go-ipfs-config"
	icore "github.com/ipfs/interface-go-ipfs-core"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/testground/sdk-go/runtime"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/libp2p/go-libp2p-core/peer"
)

// IPFSNode represents the node
type IPFSNode struct {
	Node *core.IpfsNode
	API  icore.CoreAPI
}

// setupPlugins to spawn nodes.
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

// createTempRepo creates temporal directory
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

	// Set up plugins
	if err := setupPlugins(""); err != nil {
		return nil, fmt.Errorf("Failed setting up plugins: %s", err)
	}

	// Create temporal repo.
	repoPath, err := createTempRepo(ctx)
	if err != nil {
		return nil, err
	}

	// Listen in a free port, not the default one.
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
		Routing: libp2p.DHTOption, // This option sets the node to be a full DHT node (both fetching and storing DHT Records)
		// Routing: libp2p.DHTClientOption, // This option sets the node to be a client DHT node (only fetching records)
		Repo: repo,
	}

	node, err := core.NewNode(ctx, nodeOptions)
	if err != nil {
		return nil, fmt.Errorf("Failed starting the node: %s", err)
	}
	api, err := coreapi.NewCoreAPI(node)
	// Attach the Core API to the constructed node
	return &IPFSNode{node, api}, nil
}

// Get a randomSubset of peers to connect to.
// TODO: Work in progress.
func (n *IPFSNode) getRandomSubset(peerInfo []peer.AddrInfo, maxConnections int) []peer.AddrInfo {
	outputList := []peer.AddrInfo{}
	rand.Seed(time.Now().Unix())

	// If maxConnections equals the number of peers return
	// if maxConnections >= len(peerInfo) {
	// 	return peerInfo
	// }

	for i := 0; i <= maxConnections-1; i++ {
		if n.Node.PeerHost.ID() != peerInfo[i].ID {
			x := rand.Intn(len(peerInfo) - 1)
			outputList = append(outputList, peerInfo[x])
			// Delete from peerInfo so that it can't be selected again
			peerInfo = append(peerInfo[:x], peerInfo[x+1:]...)
		}
	}
	return outputList
}

// flatSubset removes self from list of peers to dial.
// TODO: Still need to support maxConnections limitation.
func (n *IPFSNode) flatSubset(peerInfo []peer.AddrInfo, maxConnections int) []peer.AddrInfo {
	outputList := []peer.AddrInfo{}

	for _, ai := range peerInfo {
		if n.Node.PeerHost.ID() != ai.ID {
			outputList = append(outputList, ai)
		}
	}

	return outputList
}

// ConnectToPeers connects to other IPFS nodes in the network.
func (n *IPFSNode) ConnectToPeers(ctx context.Context, runenv *runtime.RunEnv,
	peerInfos []peer.AddrInfo, maxConnections int) ([]peer.AddrInfo, error) {
	ipfs := n.API
	var wg sync.WaitGroup
	// Do not include self.
	// TODO: Right now we are connecting with everyone except self.
	// peerInfos = n.getRandomSubset(peerInfos, maxConnections)
	peerInfos = n.flatSubset(peerInfos, maxConnections)
	runenv.RecordMessage("Subset of peers selected to connect: %v", peerInfos)
	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		if n.Node.PeerHost.ID() != peerInfo.ID {
			go func(peerInfo *peerstore.PeerInfo) {
				defer wg.Done()
				err := ipfs.Swarm().Connect(ctx, *peerInfo)
				if err != nil {
					log.Printf("failed to connect to %s: %s", peerInfo.ID, err)
				}
			}(&peerInfo)
		}
	}
	wg.Wait()
	return peerInfos, nil
}

// EmitMetrics emits node's metrics for the run
func (n *IPFSNode) EmitMetrics(runenv *runtime.RunEnv, runNum int, seq int64, grpseq int64,
	latency time.Duration, bandwidthMB int, fileSize int, nodetp NodeType, tpindex int, timeToFetch time.Duration) error {
	// TODO: We ned a way of generalizing this for any exchange type
	bsnode := n.Node.Exchange.(*bs.Bitswap)
	stats, err := bsnode.Stat()

	if err != nil {
		return fmt.Errorf("Error getting stats from Bitswap: %w", err)
	}

	latencyMS := latency.Milliseconds()
	instance := runenv.TestInstanceCount
	leechCount := runenv.IntParam("leech_count")
	passiveCount := runenv.IntParam("passive_count")

	id := fmt.Sprintf("topology:(%d,%d,%d)/latencyMS:%d/bandwidthMB:%d/run:%d/seq:%d/groupName:%s/groupSeq:%d/fileSize:%d/nodeType:%s/nodeTypeIndex:%d",
		instance-leechCount-passiveCount, leechCount, passiveCount,
		latencyMS, bandwidthMB, runNum, seq, runenv.TestGroupID, grpseq, fileSize, nodetp, tpindex)

	if nodetp == Leech {
		runenv.R().RecordPoint(fmt.Sprintf("%s/name:time_to_fetch", id), float64(timeToFetch))
		runenv.R().RecordPoint(fmt.Sprintf("%s/name:num_dht", id), float64(stats.NumDHT))
	}
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:msgs_rcvd", id), float64(stats.MessagesReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:data_sent", id), float64(stats.DataSent))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:data_rcvd", id), float64(stats.DataReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:block_data_rcvd", id), float64(stats.BlockDataReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:dup_data_rcvd", id), float64(stats.DupDataReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:blks_sent", id), float64(stats.BlocksSent))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:blks_rcvd", id), float64(stats.BlocksReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:dup_blks_rcvd", id), float64(stats.DupBlksReceived))
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:dup_blks_rcvd", id), float64(stats.DupBlksReceived))

	return nil
}
