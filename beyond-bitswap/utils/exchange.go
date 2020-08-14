package utils

import (
	"context"
	"errors"

	bs "github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
)

func (n *IPFSNode) SetExchange(ctx context.Context, name string) error {
	switch name {
	case "bitswap":
		net := bsnet.NewFromIpfsHost(n.Node.PeerHost, n.Node.Routing)
		n.Node.Exchange = bs.New(ctx, net, n.Node.Blockstore).(*bs.Bitswap)
		return nil
	// TODO: Add aditional exchanges here
	default:
		return errors.New("This exchange interface is not implemented")
	}

}
