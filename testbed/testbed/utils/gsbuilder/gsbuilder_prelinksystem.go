// +build !linksystem

package gsbuilder

import (
	"context"

	"github.com/ipfs/go-graphsync"
	gsimpl "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipfs/go-graphsync/storeutil"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

func BuildGraphsync(ctx context.Context, net gsnet.GraphSyncNetwork, bstore blockstore.Blockstore) graphsync.GraphExchange {
	return gsimpl.New(ctx, net,
		storeutil.LoaderForBlockstore(bstore),
		storeutil.StorerForBlockstore(bstore),
	)
}
