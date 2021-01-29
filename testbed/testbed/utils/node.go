package utils

import (
	"context"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

type Node interface {
	Add(ctx context.Context, file files.Node) (cid.Cid, error)
	Fetch(ctx context.Context, cid cid.Cid, peers []peer.AddrInfo) (files.Node, error)
	ClearDatastore(ctx context.Context) error
	EmitMetrics(recorder MetricsRecorder) error
	Host() host.Host
	DAGService() ipld.DAGService
	EmitKeepAlive(recorder MessageRecorder) error
}

type MetricsRecorder interface {
	Record(key string, value float64)
}

type MessageRecorder interface {
	RecordMessage(msg string, a ...interface{})
}
