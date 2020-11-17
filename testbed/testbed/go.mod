module github.com/adlrocha/beyond-bitswap/testbed

go 1.14

require (
	github.com/alangpierce/go-forceexport v0.0.0-20160317203124-8f1d6941cd75 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/go-playground/locales v0.13.0 // indirect
	github.com/google/gopacket v1.1.18 // indirect
	github.com/ipfs/go-bitswap v0.2.20
	github.com/ipfs/go-blockservice v0.1.3
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.5
	github.com/ipfs/go-filestore v1.0.0
	github.com/ipfs/go-graphsync v0.4.3
	github.com/ipfs/go-ipfs v0.7.0
	github.com/ipfs/go-ipfs-blockstore v1.0.1
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-config v0.9.0
	github.com/ipfs/go-ipfs-delay v0.0.1
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.8
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-cbor v0.0.4 // indirect
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log v1.0.4
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/go-metrics-interface v0.0.1
	github.com/ipfs/go-unixfs v0.2.4
	github.com/ipfs/interface-go-ipfs-core v0.4.0
	github.com/ipld/go-ipld-prime v0.5.1-0.20201021195245-109253e8a018
	github.com/jbenet/goprocess v0.1.4
	github.com/libp2p/go-libp2p v0.11.0
	github.com/libp2p/go-libp2p-autonat v0.3.2 // indirect
	github.com/libp2p/go-libp2p-core v0.6.1
	// github.com/libp2p/go-libp2p-gzip v0.0.0-00010101000000-000000000000
	github.com/libp2p/go-libp2p-kad-dht v0.9.0
	github.com/libp2p/go-libp2p-peerstore v0.2.6
	github.com/libp2p/go-mplex v0.1.3 // indirect
	github.com/libp2p/go-reuseport-transport v0.0.4 // indirect
	github.com/libp2p/go-sockaddr v0.1.0 // indirect
	github.com/libp2p/go-yamux v1.3.8 // indirect
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multihash v0.0.14
	github.com/pkg/errors v0.9.1
	github.com/testground/sdk-go v0.2.6-0.20201016180515-1e40e1b0ec3a
	github.com/wangjia184/sortedset v0.0.0-20200422044937-080872f546ba // indirect
	github.com/whyrusleeping/cbor-gen v0.0.0-20200723185710-6a3894a6352b // indirect
	github.com/whyrusleeping/go-notifier v0.0.0-20170827234753-097c5d47330f // indirect
	go.uber.org/fx v1.13.1
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de // indirect
	golang.org/x/net v0.0.0-20200822124328-c89045814202 // indirect
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20200803210538-64077c9b5642 // indirect
	golang.org/x/text v0.3.3 // indirect
)

// Baseline
replace github.com/ipfs/go-bitswap => github.com/adlrocha/go-bitswap v0.2.20-0.20201006081544-fad1a007cf9b

// RFCBBL203A
// replace github.com/ipfs/go-bitswap => github.com/adlrocha/go-bitswap v0.2.20-0.20200928082451-4186ba0c569c

// RFCBBL102
// replace github.com/ipfs/go-bitswap => github.com/adlrocha/go-bitswap v0.2.20-0.20201104081555-6f5c6dc5e81b

// RFCBBL104
// replace github.com/ipfs/go-bitswap => github.com/adlrocha/go-bitswap 39aeae7a7b188abfc659d49c12e27059e1e7b8ee

// RFCBBL102+RFCBBL104
// replace github.com/ipfs/go-bitswap => github.com/adlrocha/go-bitswap 5f82e1f53671cdd88bb809a8a86087069d90fc08

// Replaces to support compression.
// Compression with go-libp2p-core 0.7
// replace github.com/libp2p/go-libp2p-core => github.com/adlrocha/go-libp2p-core bf5d45ca7e53c6c20d8f319f0f23c2fef93bf7f6
// Compression with go-libp2p-core 0.6
// replace github.com/libp2p/go-libp2p-core => github.com/adlrocha/go-libp2p-core v0.6.2-0.20201008055055-b309947fc237

// replace github.com/libp2p/go-libp2p => github.com/adlrocha/go-libp2p v0.11.1-0.20201008063238-d1909d9adebc

// replace github.com/libp2p/go-libp2p-gzip => github.com/adlrocha/go-libp2p-gzip v0.0.0-20201008055912-3c159e9de091

// replace github.com/libp2p/go-conn-compression-multistream => github.com/adlrocha/go-conn-compression-multistream v0.0.0-20201008054958-988bc118b907

// replace github.com/libp2p/go-libp2p-transport-upgrader => github.com/adlrocha/go-libp2p-transport-upgrader v0.3.1-0.20201007105011-1e846954f580
