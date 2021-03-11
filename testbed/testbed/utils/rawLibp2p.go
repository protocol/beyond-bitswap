package utils

import (
	"context"
	"errors"
	"io"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
)

type RawLibp2pNode struct {
	h host.Host
}

func CreateRawLibp2pNode(ctx context.Context, h host.Host, nodeTP NodeType) (*RawLibp2pNode, error) {
	return &RawLibp2pNode{
		h: h,
	}, nil
}

func (r *RawLibp2pNode) Add(ctx context.Context, file files.Node) (cid.Cid, error) {
	f := files.ToFile(file)
	if f == nil {
		return cid.Undef, errors.New("node is NOT a File")
	}

	// associate a random CID with the file here as we don't really care about CIDs for the Libp2p transfer
	c, err := randCid()
	if err != nil {
		return cid.Undef, err
	}

	// set up handler to send file
	r.h.SetStreamHandler(protocol.ID(c.String()), func(s network.Stream) {
		buf := make([]byte, network.MessageSizeMax)
		if _, err := io.CopyBuffer(s, f, buf); err != nil {
			s.Reset()
		}
		s.Close()
	})

	return c, nil
}

func (r *RawLibp2pNode) Fetch(ctx context.Context, cid cid.Cid, peers []PeerInfo) (files.Node, error) {
	seedCount := 0
	var seed peer.ID

	for _, p := range peers {
		if p.Nodetp == Seed {
			seedCount++
			seed = p.Addr.ID
		}
	}
	if seedCount != 1 {
		return nil, errors.New("libp2p should ONLY have one seed")
	}

	s, err := r.h.NewStream(ctx, seed, protocol.ID(cid.String()))
	if err != nil {
		return nil, err
	}

	return files.NewReaderFile(s), nil
}

func (r *RawLibp2pNode) Host() host.Host {
	return r.h
}

// NOOP FOR now,
// #TODO Fix
func (r *RawLibp2pNode) EmitMetrics(recorder MetricsRecorder) error {
	return nil
}

// NO-OP
func (r *RawLibp2pNode) ClearDatastore(ctx context.Context, rootCid cid.Cid) error {
	return nil
}

// NO-OP
func (r *RawLibp2pNode) DAGService() ipld.DAGService {
	return nil
}

// NO-OP
func (r *RawLibp2pNode) EmitKeepAlive(recorder MessageRecorder) error {
	return nil
}
