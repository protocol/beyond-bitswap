package utils

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	mathRand "math/rand"
	"path/filepath"
	"sync"
	"time"

	bs "github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-datastore"

	blockstore "github.com/ipfs/go-ipfs-blockstore"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-metrics-interface"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/jbenet/goprocess"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/testground/sdk-go/runtime"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/p2p"
	"github.com/ipfs/go-ipfs/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/libp2p/go-libp2p-core/peer"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	ci "github.com/libp2p/go-libp2p-core/crypto"
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

// setConfig manually injects dependencies for the IPFS nodes.
func setConfig(ctx context.Context, exch ExchangeOpt) fx.Option {

	// Create new Datastore
	// TODO: This is in memory we should have some other external DataStore for big files.
	d := ds.NewMapDatastore()
	// Initialize config.
	cfg := &config.Config{}
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
	// Use defaultBootstrap
	cfg.Bootstrap = config.DefaultBootstrapAddresses

	//Allow the node to start in any available port. We do not use default ones.
	cfg.Addresses.Swarm = []string{
		"/ip4/0.0.0.0/tcp/0",
		"/ip6/::/tcp/0",
		"/ip4/0.0.0.0/udp/0/quic",
		"/ip6/::/udp/0/quic",
	}
	cfg.Identity.PeerID = pid.Pretty()
	cfg.Identity.PrivKey = base64.StdEncoding.EncodeToString(privkeyb)

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

	// Use libp2p.DHTOption. Could also use DHTClientOption.
	routingOption := fx.Provide(func() libp2p.RoutingOption {
		// return libp2p.DHTClientOption
		return libp2p.DHTOption
	})

	// Uncomment if you want to set Graphsync as exchange interface.
	// gsExchange := func(mctx helpers.MetricsCtx, lc fx.Lifecycle,
	// 	host host.Host, rt routing.Routing, bs blockstore.GCBlockstore) exchange.Interface {

	// 	// TODO: Graphsync currently doesn't follow exchange.Interface. Is missing Close()
	// 	ctx := helpers.LifecycleCtx(mctx, lc)
	// 	network := network.NewFromLibp2pHost(host)
	// 	ipldBridge := ipldbridge.NewIPLDBridge()
	// 	gsExch := gsimpl.New(ctx,
	// 		network, ipldBridge,
	// 		storeutil.LoaderForBlockstore(bs),
	// 		storeutil.StorerForBlockstore(bs),
	// 	)

	// 	lc.Append(fx.Hook{
	// 		OnStop: func(ctx context.Context) error {
	// 			if exch.Close != nil {
	// 				return exch.Close()
	// 			}
	// 			return nil

	// 		},
	// 	})
	// 	return exch
	// }

	// Return repo datastore
	repoDS := func(repo repo.Repo) datastore.Datastore {
		return d
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
		fx.Invoke(libp2p.SetupDiscovery(cfg.Discovery.MDNS.Enabled, cfg.Discovery.MDNS.Interval)),
		fx.Provide(libp2p.Routing),
		fx.Provide(libp2p.BaseRouting),

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

// NewNode constructs and returns an IpfsNode using the given cfg.
func NewNode(ctx context.Context, exch ExchangeOpt) (*IPFSNode, error) {
	// save this context as the "lifetime" ctx.
	lctx := ctx

	// derive a new context that ignores cancellations from the lifetime ctx.
	ctx, cancel := context.WithCancel(ctx)

	// add a metrics scope.
	ctx = metrics.CtxScope(ctx, "ipfs")

	n := &core.IpfsNode{}

	app := fx.New(
		// Inject dependencies in the node.
		setConfig(ctx, exch),

		fx.NopLogger,
		fx.Extract(n),
	)

	var once sync.Once
	var stopErr error
	stopNode := func() error {
		once.Do(func() {
			stopErr = app.Stop(context.Background())
			if stopErr != nil {
				fmt.Errorf("failure on stop: %v", stopErr)
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

	// Attach the Core API to the constructed node
	return &IPFSNode{n, api}, nil
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
func (n *IPFSNode) getRandomSubset(peerInfo []peer.AddrInfo, maxConnections int) []peer.AddrInfo {
	outputList := []peer.AddrInfo{}
	mathRand.Seed(time.Now().Unix())
	var i, con int

	for len(peerInfo) > 0 {
		x := mathRand.Intn(len(peerInfo))
		if n.Node.PeerHost.ID() != peerInfo[x].ID {
			outputList = append(outputList, peerInfo[x])
			// Delete from peerInfo so that it can't be selected again
			peerInfo = append(peerInfo[:x], peerInfo[x+1:]...)
			con++
		}
		if con >= maxConnections || len(peerInfo) == 1 {
			return outputList
		}
		i++
	}

	return outputList
}

// flatSubset removes self from list of peers to dial.
func (n *IPFSNode) flatSubset(peerInfo []peer.AddrInfo, maxConnections int) []peer.AddrInfo {
	outputList := []peer.AddrInfo{}

	i := 0
	for _, ai := range peerInfo {
		if n.Node.PeerHost.ID() != ai.ID {
			outputList = append(outputList, ai)
			i++
		}
		if i >= maxConnections {
			return outputList
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
	// Careful, there is a known issue with SECIO where a connection shouldn't
	// be started simultaneously.
	peerInfos = n.getRandomSubset(peerInfos, maxConnections)
	// In case we want a flat subset (in order from the start of the array) and not a random one.
	// peerInfos = n.flatSubset(peerInfos, maxConnections)
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

	// Bitswap stats
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
	runenv.R().RecordPoint(fmt.Sprintf("%s/name:dup_blks_rcvd", id), float64(stats.DupBlksReceived))

	// IPFS Node Stats
	// runenv.RecordMessage("Getting new metrics")
	// bwTotal := n.Node.Reporter.GetBandwidthTotals()
	// runenv.R().RecordPoint(fmt.Sprintf("%s/name:total_in", id), float64(bwTotal.TotalIn))
	// runenv.R().RecordPoint(fmt.Sprintf("%s/name:total_out", id), float64(bwTotal.TotalOut))
	// runenv.R().RecordPoint(fmt.Sprintf("%s/name:rate_in", id), float64(bwTotal.RateIn))
	// runenv.R().RecordPoint(fmt.Sprintf("%s/name:rate_out[", id), float64(bwTotal.RateOut))
	// runenv.RecordMessage("Finished with new metric and resetting.")

	// Restart bwCounter for the next test.
	// n.Node.Reporter = mtcs.NewBandwidthCounter()

	// A few other metrics that could be collected.
	// GetBandwidthForPeer(peer.ID) Stats
	// GetBandwidthForProtocol(protocol.ID) Stats
	// GetBandwidthTotals() Stats
	// GetBandwidthByPeer() map[peer.ID]Stats
	// GetBandwidthByProtocol() map[protocol.ID]Stats

	return nil
}
