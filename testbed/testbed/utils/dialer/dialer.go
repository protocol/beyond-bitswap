package dialer

import (
	"bytes"
	"context"
	"fmt"
	"math"

	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/protocol/beyond-bitswap/testbed/testbed/utils"
	"golang.org/x/sync/errgroup"
)

// PeerInfo provides all the neccessary information to dial a peer
type PeerInfo struct {
	Addr   peer.AddrInfo
	Nodetp utils.NodeType
}

// PeerInfosFromChan collects peer information from a channel of peer information
func PeerInfosFromChan(peerCh chan *PeerInfo, count int) ([]PeerInfo, error) {
	var ais []PeerInfo
	for i := 1; i <= count; i++ {
		ai, ok := <-peerCh
		if !ok {
			return ais, fmt.Errorf("subscription closed")
		}
		ais = append(ais, *ai)
	}
	return ais, nil
}

// Dialer is a function that dials other peers, following a specified pattern
type Dialer func(ctx context.Context, self core.Host, selfType utils.NodeType, ais []PeerInfo, maxConnectionRate int) ([]peer.AddrInfo, error)

// SparseDial connects to a set of peers in the experiment, but only those with the correct node type
func SparseDial(ctx context.Context, self core.Host, selfType utils.NodeType, ais []PeerInfo, maxConnectionRate int) ([]peer.AddrInfo, error) {
	// Grab list of other peers that are available for this Run
	var toDial []peer.AddrInfo
	for _, inf := range ais {
		ai := inf.Addr
		id1, _ := ai.ID.MarshalBinary()
		id2, _ := self.ID().MarshalBinary()

		// skip over dialing ourselves, and prevent TCP simultaneous
		// connect (known to fail) by only dialing peers whose peer ID
		// is smaller than ours.
		if bytes.Compare(id1, id2) < 0 {
			// In sparse topology we don't allow leechers and seeders to be directly connected.
			switch selfType {
			case utils.Seed:
				if inf.Nodetp != utils.Leech {
					toDial = append(toDial, ai)
				}
			case utils.Leech:
				if inf.Nodetp != utils.Seed {
					toDial = append(toDial, ai)
				}
			case utils.Passive:
				toDial = append(toDial, ai)
			}
		}
	}

	// Limit max number of connections for the peer according to rate.
	rate := float64(maxConnectionRate) / 100
	toDial = toDial[:int(math.Ceil(float64(len(toDial))*rate))]

	// Dial to all the other peers
	g, ctx := errgroup.WithContext(ctx)
	for _, ai := range toDial {
		ai := ai
		g.Go(func() error {
			if err := self.Connect(ctx, ai); err != nil {
				return fmt.Errorf("Error while dialing peer %v: %w", ai.Addrs, err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return toDial, nil
}

// DialOtherPeers connects to a set of peers in the experiment, dialing all of them
func DialOtherPeers(ctx context.Context, self core.Host, selfType utils.NodeType, ais []PeerInfo, maxConnectionRate int) ([]peer.AddrInfo, error) {
	// Grab list of other peers that are available for this Run
	var toDial []peer.AddrInfo
	for _, inf := range ais {
		ai := inf.Addr
		id1, _ := ai.ID.MarshalBinary()
		id2, _ := self.ID().MarshalBinary()

		// skip over dialing ourselves, and prevent TCP simultaneous
		// connect (known to fail) by only dialing peers whose peer ID
		// is smaller than ours.
		if bytes.Compare(id1, id2) < 0 {
			toDial = append(toDial, ai)
		}
	}

	// Limit max number of connections for the peer according to rate.
	rate := float64(maxConnectionRate) / 100
	toDial = toDial[:int(math.Ceil(float64(len(toDial))*rate))]

	// Dial to all the other peers
	g, ctx := errgroup.WithContext(ctx)
	for _, ai := range toDial {
		ai := ai
		g.Go(func() error {
			if err := self.Connect(ctx, ai); err != nil {
				return fmt.Errorf("Error while dialing peer %v: %w", ai.Addrs, err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return toDial, nil
}
