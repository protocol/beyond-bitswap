package utils

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	mathRand "math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	bs "github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/interface-go-ipfs-core/path"

	blockstore "github.com/ipfs/go-ipfs-blockstore"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-metrics-interface"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/jbenet/goprocess"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/p2p" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/go-ipfs/repo"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	dsync "github.com/ipfs/go-datastore/sync"
	ci "github.com/libp2p/go-libp2p-core/crypto"
)

// IPFSNode represents the node
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
func setConfig(ctx context.Context, nConfig *NodeConfig, exch ExchangeOpt, DHTenabled bool) fx.Option {

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

		// TODO: Here you can see some more of the libp2p dependencies you could set.
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

// CreateIPFSNodeWithConfig constructs and returns an IpfsNode using the given cfg.
func CreateIPFSNodeWithConfig(ctx context.Context, nConfig *NodeConfig, exch ExchangeOpt, DHTEnabled bool) (*IPFSNode, error) {
	// save this context as the "lifetime" ctx.
	lctx := ctx

	// derive a new context that ignores cancellations from the lifetime ctx.
	ctx, cancel := context.WithCancel(ctx)

	// add a metrics scope.
	ctx = metrics.CtxScope(ctx, "ipfs")

	n := &core.IpfsNode{}

	app := fx.New(
		// Inject dependencies in the node.
		setConfig(ctx, nConfig, exch, DHTEnabled),

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

	// Attach the Core API to the constructed node
	return &IPFSNode{n, api, stopNode}, nil
}

// ClearDatastore removes a block from the datastore.
// TODO: This function may be inefficient with large blockstore. Used the option above.
// This function may be cleaned in the future.
func (n *IPFSNode) ClearDatastore(ctx context.Context, rootCid cid.Cid) error {
	_, pinned, err := n.API.Pin().IsPinned(ctx, path.IpfsPath(rootCid))
	if err != nil {
		return err
	}
	if pinned {
		err := n.API.Pin().Rm(ctx, path.IpfsPath(rootCid))
		if err != nil {
			return err
		}
	}
	var ng ipld.NodeGetter = merkledag.NewSession(ctx, n.Node.DAG)
	toDelete := cid.NewSet()
	err = merkledag.Walk(ctx, merkledag.GetLinksDirect(ng), rootCid, toDelete.Visit, merkledag.Concurrent())
	if err != nil {
		return err
	}
	return toDelete.ForEach(func(c cid.Cid) error {
		return n.API.Block().Rm(ctx, path.IpfsPath(c))
	})
}

// EmitMetrics emits node's metrics for the run
func (n *IPFSNode) EmitMetrics(recorder MetricsRecorder) error {
	// TODO: We ned a way of generalizing this for any exchange type
	bsnode := n.Node.Exchange.(*bs.Bitswap)
	stats, err := bsnode.Stat()

	if err != nil {
		return fmt.Errorf("Error getting stats from Bitswap: %w", err)
	}

	recorder.Record("msgs_rcvd", float64(stats.MessagesReceived))
	recorder.Record("data_sent", float64(stats.DataSent))
	recorder.Record("data_rcvd", float64(stats.DataReceived))
	recorder.Record("block_data_rcvd", float64(stats.BlockDataReceived))
	recorder.Record("dup_data_rcvd", float64(stats.DupDataReceived))
	recorder.Record("blks_sent", float64(stats.BlocksSent))
	recorder.Record("blks_rcvd", float64(stats.BlocksReceived))
	recorder.Record("dup_blks_rcvd", float64(stats.DupBlksReceived))
	recorder.Record("wants_rcvd", float64(stats.WantsRecvd))
	recorder.Record("want_blocks_rcvd", float64(stats.WantBlocksRecvd))
	recorder.Record("want_haves_rcvd", float64(stats.WantHavesRecvd))
	recorder.Record("stream_data_sent", float64(stats.StreamDataSent))

	// IPFS Node Stats
	bwTotal := n.Node.Reporter.GetBandwidthTotals()
	recorder.Record("total_in", float64(bwTotal.TotalIn))
	recorder.Record("total_out", float64(bwTotal.TotalOut))
	recorder.Record("rate_in", float64(bwTotal.RateIn))
	recorder.Record("rate_out", float64(bwTotal.RateOut))

	// Restart all counters for the next test.
	n.Node.Reporter.Reset()
	n.Node.Exchange.(*bs.Bitswap).ResetStatCounters()

	// A few other metrics that could be collected.
	// GetBandwidthForPeer(peer.ID) Stats
	// GetBandwidthForProtocol(protocol.ID) Stats
	// GetBandwidthTotals() Stats
	// GetBandwidthByPeer() map[peer.ID]Stats
	// GetBandwidthByProtocol() map[protocol.ID]Stats

	return nil
}

func (n *IPFSNode) Add(ctx context.Context, tmpFile files.Node) (cid.Cid, error) {
	path, err := n.API.Unixfs().Add(ctx, tmpFile)
	if err != nil {
		return cid.Undef, err
	}
	return path.Cid(), nil
}

func (n *IPFSNode) Fetch(ctx context.Context, c cid.Cid, _ []PeerInfo) (files.Node, error) {
	fPath := path.IpfsPath(c)
	return n.API.Unixfs().Get(ctx, fPath)
}

func (n *IPFSNode) DAGService() ipld.DAGService {
	return n.Node.DAG
}

func (n *IPFSNode) Host() host.Host {
	return n.Node.PeerHost
}

func (n *IPFSNode) EmitKeepAlive(recorder MessageRecorder) error {

	recorder.RecordMessage("I am still alive! Total In: %d - TotalOut: %d",
		n.Node.Reporter.GetBandwidthTotals().TotalIn,
		n.Node.Reporter.GetBandwidthTotals().TotalOut)

	return nil
}

var _ Node = &IPFSNode{}
