package utils

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	bs "github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	delayed "github.com/ipfs/go-datastore/delayed"
	ds_sync "github.com/ipfs/go-datastore/sync"
	badgerds "github.com/ipfs/go-ds-badger2"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	delay "github.com/ipfs/go-ipfs-delay"
	files "github.com/ipfs/go-ipfs-files"
	nilrouting "github.com/ipfs/go-ipfs-routing/none"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	unixfile "github.com/ipfs/go-unixfs/file"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"
	"github.com/testground/sdk-go/runtime"
	"golang.org/x/sync/errgroup"

	dgbadger "github.com/dgraph-io/badger/v2"
)

type NodeType int

const (
	// Seeds data
	Seed NodeType = iota
	// Fetches data from seeds
	Leech
	// Doesn't seed or fetch data
	Passive
)

func (nt NodeType) String() string {
	return [...]string{"Seed", "Leech", "Passive"}[nt]
}

// Adapted from the netflix/p2plab repo under an Apache-2 license.
// Original source code located at https://github.com/Netflix/p2plab/blob/master/peer/peer.go
type BitswapNode struct {
	bitswap    *bs.Bitswap
	blockStore blockstore.Blockstore
	dserv      ipld.DAGService
	h          host.Host
}

func (n *BitswapNode) Close() error {
	return n.bitswap.Close()
}

func CreateBlockstore(ctx context.Context, dStore ds.Batching) (blockstore.Blockstore, error) {
	return blockstore.CachedBlockstore(ctx,
		blockstore.NewBlockstore(dStore),
		blockstore.DefaultCacheOpts())
}

// CreateDatastore creates a data store to use for the transfer.
// If diskStore=false, it returns an in-memory store that uses the given delay for each read/write.
// If diskStore=true, it returns a Badger data store and ignores the bsdelay param.
func CreateDatastore(runenv *runtime.RunEnv, diskStore bool, noSync bool, bsdelay time.Duration) (ds.Batching, error) {
	if !diskStore {
		dstore := ds_sync.MutexWrap(delayed.New(ds.NewMapDatastore(), delay.Fixed(bsdelay)))
		return dstore, nil
	}

	// create temporary directory for badger datastore
	path := fmt.Sprintf("badger-%d", rand.Uint64())
	runenv.RecordMessage("will create Badger at path %s", path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		runenv.RecordMessage("path %s does NOT exist, creating it...", path)
		if err := os.MkdirAll(path, 0777); err != nil {
			return nil, err
		}
		runenv.RecordMessage("created path %s for Badger", path)
	} else if err != nil {
		return nil, err
	}

	// create disk based badger datastore
	defopts := badgerds.DefaultOptions

	defopts.Options = dgbadger.DefaultOptions("").WithTruncate(true).
		WithValueThreshold(1 << 10)

	if noSync {
		defopts.Options = defopts.Options.WithSyncWrites(false)
	} else {
		defopts.Options = defopts.Options.WithSyncWrites(true)
	}

	runenv.RecordMessage("badger sync write is set to %v", defopts.SyncWrites)
	datastore, err := badgerds.NewDatastore(path, &defopts)
	if err != nil {
		return nil, err
	}

	return datastore, nil
}

func ClearBlockstore(ctx context.Context, bstore blockstore.Blockstore) error {
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

func CreateBitswapNode(ctx context.Context, h host.Host, bstore blockstore.Blockstore) (*BitswapNode, error) {
	routing, err := nilrouting.ConstructNilRouting(ctx, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	net := bsnet.NewFromIpfsHost(h, routing)
	bitswap := bs.New(ctx, net, bstore).(*bs.Bitswap)
	bserv := blockservice.New(bstore, bitswap)
	dserv := merkledag.NewDAGService(bserv)
	return &BitswapNode{bitswap, bstore, dserv, h}, nil
}

func (n *BitswapNode) Add(ctx context.Context, fileNode files.Node) (cid.Cid, error) {
	settings := AddSettings{
		Layout:    "balanced",
		Chunker:   "size-262144",
		RawLeaves: false,
		NoCopy:    false,
		HashFunc:  "sha2-256",
		MaxLinks:  helpers.DefaultLinksPerBlock,
	}
	adder, err := NewDAGAdder(ctx, n.dserv, settings)
	if err != nil {
		return cid.Undef, err
	}
	ipldNode, err := adder.Add(fileNode)
	if err != nil {
		return cid.Undef, err
	}
	return ipldNode.Cid(), nil
}

func (n *BitswapNode) ClearDatastore(ctx context.Context, _ cid.Cid) error {
	return ClearBlockstore(ctx, n.blockStore)
}

func (n *BitswapNode) EmitMetrics(recorder MetricsRecorder) error {
	stats, err := n.bitswap.Stat()

	if err != nil {
		return err
	}
	recorder.Record("msgs_rcvd", float64(stats.MessagesReceived))
	recorder.Record("data_sent", float64(stats.DataSent))
	recorder.Record("data_rcvd", float64(stats.DataReceived))
	recorder.Record("dup_data_rcvd", float64(stats.DupDataReceived))
	recorder.Record("blks_sent", float64(stats.BlocksSent))
	recorder.Record("blks_rcvd", float64(stats.BlocksReceived))
	recorder.Record("dup_blks_rcvd", float64(stats.DupBlksReceived))
	return err
}

func (n *BitswapNode) Fetch(ctx context.Context, c cid.Cid, _ []PeerInfo) (files.Node, error) {
	err := merkledag.FetchGraph(ctx, c, n.dserv)
	if err != nil {
		return nil, err
	}
	nd, err := n.dserv.Get(ctx, c)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get file %q", c)
	}

	return unixfile.NewUnixfsFile(ctx, n.dserv, nd)
}

func (n *BitswapNode) DAGService() ipld.DAGService {
	return n.dserv
}

func (n *BitswapNode) Host() host.Host {
	return n.h
}

func (n *BitswapNode) EmitKeepAlive(recorder MessageRecorder) error {
	stats, err := n.bitswap.Stat()

	if err != nil {
		return err
	}

	recorder.RecordMessage("I am still alive! Total In: %d - TotalOut: %d",
		stats.DataReceived,
		stats.DataSent)

	return nil
}

var _ Node = &BitswapNode{}
