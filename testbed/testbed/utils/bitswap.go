package utils

import (
	"context"
	"time"

	bs "github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	delayed "github.com/ipfs/go-datastore/delayed"
	ds_sync "github.com/ipfs/go-datastore/sync"
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
	"golang.org/x/sync/errgroup"
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

func CreateBlockstore(ctx context.Context, bstoreDelay time.Duration) (blockstore.Blockstore, error) {
	bsdelay := delay.Fixed(bstoreDelay)
	dstore := ds_sync.MutexWrap(delayed.New(ds.NewMapDatastore(), bsdelay))
	return blockstore.CachedBlockstore(ctx,
		blockstore.NewBlockstore(ds_sync.MutexWrap(dstore)),
		blockstore.DefaultCacheOpts())
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

func (n *BitswapNode) ClearDatastore(ctx context.Context) error {
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
	recorder.Record("block_data_rcvd", float64(stats.BlockDataReceived))
	recorder.Record("dup_data_rcvd", float64(stats.DupDataReceived))
	recorder.Record("blks_sent", float64(stats.BlocksSent))
	recorder.Record("blks_rcvd", float64(stats.BlocksReceived))
	recorder.Record("dup_blks_rcvd", float64(stats.DupBlksReceived))
	return err
}

func (n *BitswapNode) FetchGraph(ctx context.Context, c cid.Cid) error {
	ng := merkledag.NewSession(ctx, n.dserv)
	return Walk(ctx, c, ng)
}

func (n *BitswapNode) Get(ctx context.Context, c cid.Cid) (files.Node, error) {
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
