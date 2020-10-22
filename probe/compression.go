package main

import (
	"sort"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/compression/none"

	// gzip "github.com/libp2p/go-libp2p-gzip"
	cbrotli "github.com/libp2p/go-libp2p-cbrotli"

	"go.uber.org/fx"
)

type Libp2pOpts struct {
	fx.Out

	Opts []libp2p.Option `group:"libp2p"`
}

type priorityOption struct {
	priority, defaultPriority config.Priority
	opt                       libp2p.Option
}

func prioritizeOptions(opts []priorityOption) libp2p.Option {
	type popt struct {
		priority int64
		opt      libp2p.Option
	}
	enabledOptions := make([]popt, 0, len(opts))
	for _, o := range opts {
		if prio, ok := o.priority.WithDefault(o.defaultPriority); ok {
			enabledOptions = append(enabledOptions, popt{
				priority: prio,
				opt:      o.opt,
			})
		}
	}
	sort.Slice(enabledOptions, func(i, j int) bool {
		return enabledOptions[i].priority > enabledOptions[j].priority
	})
	p2pOpts := make([]libp2p.Option, len(enabledOptions))
	for i, opt := range enabledOptions {
		p2pOpts[i] = opt.opt
	}
	return libp2p.ChainOptions(p2pOpts...)
}

func Compression(enabled bool) interface{} {
	if !enabled {
		return func() (opts Libp2pOpts) {
			opts.Opts = append(opts.Opts, prioritizeOptions([]priorityOption{{
				priority:        100,
				defaultPriority: 100,
				opt:             libp2p.Compression(none.ID, none.New),
			}}))
			return opts
		}
	}

	// Using the new config options.
	return func() (opts Libp2pOpts) {
		opts.Opts = append(opts.Opts, prioritizeOptions([]priorityOption{{
			priority:        100,
			defaultPriority: 100,
			opt:             libp2p.Compression(cbrotli.ID, cbrotli.New),
		}}))
		return opts
	}
}
