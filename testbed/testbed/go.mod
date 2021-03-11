module github.com/protocol/beyond-bitswap/testbed/testbed

go 1.14

require (
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/hannahhoward/all-selector v0.2.0
	github.com/ipfs/go-bitswap v0.2.20
	github.com/ipfs/go-blockservice v0.1.3
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.5
	github.com/ipfs/go-ds-badger2 v0.1.0
	github.com/ipfs/go-filestore v1.0.0 // indirect
	github.com/ipfs/go-graphsync v0.4.3
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-blockstore v1.0.1
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-config v0.9.0
	github.com/ipfs/go-ipfs-delay v0.0.1
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-posinfo v0.0.1
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log/v2 v2.1.1
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/go-metrics-interface v0.0.1
	github.com/ipfs/go-mfs v0.1.2
	github.com/ipfs/go-unixfs v0.2.4
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/ipld/go-ipld-prime v0.5.1-0.20201021195245-109253e8a018
	github.com/jbenet/goprocess v0.1.4
	github.com/libp2p/go-libp2p v0.11.0
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/libp2p/go-libp2p-gostream v0.2.1
	github.com/libp2p/go-libp2p-http v0.1.6-0.20210310045043-5508c68db693
	// github.com/libp2p/go-libp2p-gzip v0.0.0-00010101000000-000000000000
	github.com/libp2p/go-mplex v0.1.3 // indirect
	github.com/libp2p/go-sockaddr v0.1.0 // indirect
	github.com/libp2p/go-yamux v1.3.8 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multihash v0.0.14
	github.com/pkg/errors v0.9.1
	github.com/testground/sdk-go v0.2.6-0.20201016180515-1e40e1b0ec3a
	github.com/whyrusleeping/cbor-gen v0.0.0-20200723185710-6a3894a6352b // indirect
	go.uber.org/fx v1.13.1
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de // indirect
	golang.org/x/net v0.0.0-20200822124328-c89045814202 // indirect
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20200803210538-64077c9b5642 // indirect
	golang.org/x/text v0.3.3 // indirect
)

replace github.com/ipfs/go-bitswap => github.com/adlrocha/go-bitswap v0.2.20-0.20201006081544-fad1a007cf9b
