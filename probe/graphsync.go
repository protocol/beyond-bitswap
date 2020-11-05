package main

import (
	"context"
	"fmt"
	"time"

	p2p "github.com/libp2p/go-libp2p-core"
	"go.uber.org/fx"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	gsimpl "github.com/ipfs/go-graphsync/impl"
	"github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	unixfsFile "github.com/ipfs/go-unixfs/file"

	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	ipldselector "github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// Graphsync constructs a graphsync
func Graphsync(lc fx.Lifecycle, mctx helpers.MetricsCtx, host p2p.Host, bs blockstore.GCBlockstore) graphsync.GraphExchange {
	ctx := helpers.LifecycleCtx(mctx, lc)

	network := network.NewFromLibp2pHost(host)
	return gsimpl.New(ctx, network,
		storeutil.LoaderForBlockstore(bs),
		storeutil.StorerForBlockstore(bs),
	)
}

func newGraphsync(ctx context.Context, p2p host.Host, bs blockstore.Blockstore) (graphsync.GraphExchange, error) {
	network := network.NewFromLibp2pHost(p2p)
	return gsimpl.New(ctx,
		network,
		storeutil.LoaderForBlockstore(bs),
		storeutil.StorerForBlockstore(bs),
	), nil
}

var selectAll ipld.Node = func() ipld.Node {
	ssb := builder.NewSelectorSpecBuilder(basicnode.Style.Any)
	return ssb.ExploreRecursive(
		ipldselector.RecursionLimitDepth(100), // default max
		ssb.ExploreAll(ssb.ExploreRecursiveEdge()),
	).Node()
}()

func fetch(ctx context.Context, gs graphsync.GraphExchange, p peer.ID, c cid.Cid) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resps, errs := gs.Request(ctx, p, cidlink.Link{Cid: c}, selectAll)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case _, ok := <-resps:
			if !ok {
				resps = nil
			}
		case err, ok := <-errs:
			if !ok {
				// done.
				return nil
			}
			if err != nil {
				return fmt.Errorf("got an unexpected error: %s", err)
			}
		}
	}
}

// getContent gets a file from the network and computes time_to_fetch
func getGraphsync(ctx context.Context, n *IPFSNode, p string, cidString string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var (
		timeToFetch time.Duration
		// f           files.Node
		err error
	)

	// Parse CID
	target, err := cid.Decode(cidString)
	if err != nil {
		return err
	}
	// Parse target
	maddr, err := ma.NewMultiaddr(p)
	ai, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return err
	}
	fmt.Printf("Searching for cid %v in peer %v\n", target, p)

	// Store in /tmp/
	fileName := "/tmp/" + time.Now().String()

	start := time.Now()
	gs := n.Node.GraphExchange
	err = fetch(ctx, gs, ai.ID, target)
	if err != nil {
		return err
	}
	dag := n.Node.DAG
	// Get the DAG
	root, err := dag.Get(ctx, target)
	if err != nil {
		return err
	}
	// reader, err := uio.NewDirectoryFromNode(dag, root)
	// reader, err := uio.NewDagReader(ctx, root, dag)
	// TODO: I am here!
	node, err := unixfsFile.NewUnixfsFile(ctx, dag, root)
	if err != nil {
		return err
	}
	files.WriteTo(node, fileName)
	timeToFetch = time.Since(start)
	// TODO: Store in a file to check also the size retrieved.
	// s, _ := f.Size()
	fmt.Println("[*] Time to fetch: ", timeToFetch)

	fmt.Println("Cleaning datastore")
	n.ClearDatastore(ctx)
	err = n.ClearBlockstore(ctx)
	if err != nil {
		fmt.Println("Error cleaning blockstore", err)
	}
	return nil
}
