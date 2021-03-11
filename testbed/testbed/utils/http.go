package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ipfs/go-cid"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/libp2p/go-libp2p-core/host"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

type HTTPNode struct {
	h   host.Host
	svc *http.Server
}

func CreateHTTPNode(ctx context.Context, h host.Host, nodeTP NodeType) (*HTTPNode, error) {
	var svr *http.Server
	switch nodeTP {
	case Seed:
		svr = &http.Server{Addr: ":8080"}
		go svr.ListenAndServe()
		time.Sleep(1 * time.Second)
	case Leech:
	default:
		return nil, errors.New("nodeType NOT supported")
	}

	return &HTTPNode{
		h:   h,
		svc: svr,
	}, nil
}

func (h *HTTPNode) Add(ctx context.Context, file files.Node) (cid.Cid, error) {
	f := files.ToFile(file)
	if f == nil {
		return cid.Undef, errors.New("node is NOT a File")
	}

	// associate a random CID with the file here as we don't really care about CIDs for the HTTP Libp2p transfer
	c, err := randCid()
	if err != nil {
		return c, err
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

// TODO GET IP
func (h *HTTPNode) Fetch(ctx context.Context, c cid.Cid, peers []PeerInfo) (files.Node, error) {
	seedCount := 0
	var seedAddrs []ma.Multiaddr

	for _, p := range peers {
		if p.Nodetp == Seed {
			seedCount++
			seedAddrs = p.Addr.Addrs
		}
	}
	if seedCount != 1 {
		return nil, errors.New("http should ONLY have one seed")
	}

	var ip net.IP
	for _, a := range seedAddrs {
		if _, err := a.ValueForProtocol(ma.P_IP4); err == nil {
			ip, err = manet.ToIP(a)
			if err != nil {
				return nil, err
			}
		}
	}

	resp, err := http.DefaultClient.Get(fmt.Sprintf("http://%s:8080/%s", ip.String(), c.String()))
	if err != nil {
		return nil, err
	}

	return files.NewReaderFile(resp.Body), nil
}

func (h *HTTPNode) Host() host.Host {
	return h.Host()
}

// NOOP FOR now,
// #TODO Fix
func (h *HTTPNode) EmitMetrics(recorder MetricsRecorder) error {
	return nil
}

// NO-OP
func (h *HTTPNode) ClearDatastore(ctx context.Context, rootCid cid.Cid) error {
	return nil
}

// NO-OP
func (h *HTTPNode) DAGService() ipld.DAGService {
	return nil
}

// NO-OP
func (h *HTTPNode) EmitKeepAlive(recorder MessageRecorder) error {
	return nil
}
