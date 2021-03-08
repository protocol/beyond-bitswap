package utils

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	gsimpl "github.com/ipfs/go-graphsync/impl"
	"github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	files "github.com/ipfs/go-ipfs-files"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	unixfile "github.com/ipfs/go-unixfs/file"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/pkg/errors"

	allselector "github.com/hannahhoward/all-selector"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

type GraphsyncNode struct {
	gs            graphsync.GraphExchange
	blockStore    blockstore.Blockstore
	dserv         format.DAGService
	h             host.Host
	totalSent     uint64
	totalReceived uint64
	numSeeds      int
}

func CreateGraphsyncNode(ctx context.Context, h host.Host, bstore blockstore.Blockstore, numSeeds int) (*GraphsyncNode, error) {
	net := network.NewFromLibp2pHost(h)
	bserv := blockservice.New(bstore, offline.Exchange(bstore))
	dserv := merkledag.NewDAGService(bserv)
	gs := gsimpl.New(ctx, net,
		storeutil.LoaderForBlockstore(bstore),
		storeutil.StorerForBlockstore(bstore),
	)
	n := &GraphsyncNode{gs, bstore, dserv, h, 0, 0, numSeeds}
	gs.RegisterBlockSentListener(n.onDataSent)
	gs.RegisterIncomingBlockHook(n.onDataReceived)
	gs.RegisterIncomingRequestHook(n.onIncomingRequestHook)
	return n, nil
}

var selectAll ipld.Node = allselector.AllSelector

func (n *GraphsyncNode) Add(ctx context.Context, fileNode files.Node) (cid.Cid, error) {
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

func (n *GraphsyncNode) ClearDatastore(ctx context.Context, rootCid cid.Cid) error {
	return ClearBlockstore(ctx, n.blockStore)
}

func (n *GraphsyncNode) EmitMetrics(recorder MetricsRecorder) error {
	recorder.Record("data_sent", float64(n.totalSent))
	recorder.Record("data_rcvd", float64(n.totalReceived))
	return nil
}

func (n *GraphsyncNode) Fetch(ctx context.Context, c cid.Cid, peers []PeerInfo) (files.Node, error) {
	leechIndex := 0
	for i := 0; i < len(peers); i++ {
		if peers[i].Addr.ID == n.h.ID() {
			break
		}
		if peers[i].Nodetp == Leech {
			leechIndex++
		}
	}

	targetSeed := leechIndex % n.numSeeds
	seedCount := 0
	var seedIndex = 0
	for ; seedIndex < len(peers); seedIndex++ {
		if peers[seedIndex].Nodetp == Seed && peers[seedIndex].Addr.ID != n.h.ID() {
			if seedCount == targetSeed {
				break
			}
			seedCount++
		}
	}

	if seedCount == len(peers) {
		return nil, errors.New("no suitable seed found")
	}
	p := peers[seedIndex].Addr.ID
	resps, errs := n.gs.Request(ctx, p, cidlink.Link{Cid: c}, selectAll)
	for range resps {
	}
	var lastError error
	for err := range errs {
		if err != nil {
			lastError = err
		}
	}
	if lastError != nil {
		return nil, lastError
	}
	nd, err := n.dserv.Get(ctx, c)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get file %q", c)
	}

	return unixfile.NewUnixfsFile(ctx, n.dserv, nd)
}

func (n *GraphsyncNode) DAGService() format.DAGService {
	return n.dserv
}

func (n *GraphsyncNode) Host() host.Host {
	return n.h
}

func (n *GraphsyncNode) EmitKeepAlive(recorder MessageRecorder) error {

	recorder.RecordMessage("I am still alive! Total In: %d - TotalOut: %d",
		n.totalSent,
		n.totalReceived)

	return nil
}

func (n *GraphsyncNode) onDataSent(p peer.ID, request graphsync.RequestData, block graphsync.BlockData) {
	n.totalSent += block.BlockSizeOnWire()
}

func (n *GraphsyncNode) onDataReceived(p peer.ID, request graphsync.ResponseData, block graphsync.BlockData, ha graphsync.IncomingBlockHookActions) {
	n.totalReceived += block.BlockSizeOnWire()
}

func (n *GraphsyncNode) onIncomingRequestHook(p peer.ID, request graphsync.RequestData, ha graphsync.IncomingRequestHookActions) {
	ha.ValidateRequest()
}

var _ Node = &GraphsyncNode{}
