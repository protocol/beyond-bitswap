package utils

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	gostream "github.com/libp2p/go-libp2p-gostream"
	p2phttp "github.com/libp2p/go-libp2p-http"
	mh "github.com/multiformats/go-multihash"
)

type Libp2pHTTPNode struct {
	client *http.Client
	h      host.Host
	svr    *http.Server
}

func CreateLibp2pHTTPNode(ctx context.Context, h host.Host, nodeTP NodeType) (*Libp2pHTTPNode, error) {
	switch nodeTP {
	case Seed:
		// Server
		listener, err := gostream.Listen(h, p2phttp.DefaultP2PProtocol)
		if err != nil {
			return nil, err
		}
		// start an http server on port 8080
		svr := &http.Server{}
		go svr.Serve(listener)
		time.Sleep(1 * time.Second)
		return &Libp2pHTTPNode{
			h:   h,
			svr: svr,
		}, nil
	case Leech:
		tr := &http.Transport{}
		tr.RegisterProtocol("libp2p", p2phttp.NewTransport(h))
		client := &http.Client{Transport: tr}

		return &Libp2pHTTPNode{
			client: client,
			h:      h,
		}, nil
	default:
		return nil, errors.New("nodeType NOT supported")
	}
}

func (l *Libp2pHTTPNode) Add(ctx context.Context, file files.Node) (cid.Cid, error) {
	f := files.ToFile(file)
	if f == nil {
		return cid.Undef, errors.New("node is NOT a File")
	}

	c, err := randCid()
	if err != nil {
		return cid.Undef, err
	}

	// set up http server to send file
	http.HandleFunc(fmt.Sprintf("/%s", c.String()), func(w http.ResponseWriter, r *http.Request) {
		defer f.Close()
		_, err := io.Copy(w, f)
		if err != nil {
			panic(err)
		}
	})

	return c, nil
}

func (l *Libp2pHTTPNode) Fetch(ctx context.Context, cid cid.Cid, peers []PeerInfo) (files.Node, error) {
	seedCount := 0
	var seed peer.ID

	for _, p := range peers {
		if p.Nodetp == Seed {
			seedCount++
			seed = p.Addr.ID
		}
	}
	if seedCount != 1 {
		return nil, errors.New("libp2p http should ONLY have one seed")
	}

	resp, err := l.client.Get(fmt.Sprintf("libp2p://%s/%s", seed.String(), cid.String()))
	if err != nil {
		return nil, err
	}

	return files.NewReaderFile(resp.Body), nil
}

func (l *Libp2pHTTPNode) Host() host.Host {
	return l.h
}

func (l *Libp2pHTTPNode) ClearDatastore(ctx context.Context, rootCid cid.Cid) error {
	if l.svr != nil {
		return l.svr.Shutdown(ctx)
	}

	return nil
}

// NOOP FOR now,
// #TODO Fix
func (l *Libp2pHTTPNode) EmitMetrics(recorder MetricsRecorder) error {
	return nil
}

// NO-OP
func (l *Libp2pHTTPNode) DAGService() ipld.DAGService {
	return nil
}

// NO-OP
func (l *Libp2pHTTPNode) EmitKeepAlive(recorder MessageRecorder) error {
	return nil
}

func randCid() (cid.Cid, error) {
	buf := make([]byte, binary.MaxVarintLen64)
	u := rand.Uint64()
	binary.PutUvarint(buf, u)
	h1, err := mh.Sum(buf, mh.SHA2_256, -1)
	if err != nil {
		return cid.Undef, err
	}

	return cid.NewCidV1(cid.Raw, h1), nil
}
