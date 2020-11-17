package utils

import (
	"context"
	"errors"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/ipfs/go-ipfs/core/node/helpers"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	"go.uber.org/fx"
)

// ExchangeOpt injects exchange interface
type ExchangeOpt func(helpers.MetricsCtx, fx.Lifecycle, host.Host,
	routing.Routing, blockstore.GCBlockstore) exchange.Interface

// SetExchange sets the exchange interface to be used
func SetExchange(ctx context.Context, name string) (ExchangeOpt, error) {
	switch name {
	case "bitswap":
		// Initializing bitswap exchange
		return func(mctx helpers.MetricsCtx, lc fx.Lifecycle,
			host host.Host, rt routing.Routing, bs blockstore.GCBlockstore) exchange.Interface {
			bitswapNetwork := network.NewFromIpfsHost(host, rt)
			exch := bitswap.New(helpers.LifecycleCtx(mctx, lc), bitswapNetwork, bs)

			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return exch.Close()
				},
			})
			return exch
		}, nil

	// TODO: Add aditional exchanges here
	default:
		return nil, errors.New("This exchange interface is not implemented")
	}

}
