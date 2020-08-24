## Beyond Bitswap Probe
This is a simple CLI tool to help debug the exchange of content between different IPFS nodes.

### Usage
* To run the tool use:
```
$ go run probe.go
```
* The command will start a new IPFS node and add a set of files from a directory and
randomly generated to start testing. When this task is finished you will be prompted to
start typing commands:
```
-- Getting an IPFS node running -- 
Spawning node on a temporary repo
Listening at:  [/ip4/192.168.1.66/tcp/33387 /ip4/127.0.0.1/tcp/33387 /ip6/::1/tcp/35697 /ip4/192.168.1.66/udp/44399/quic /ip4/127.0.0.1/udp/44399/quic /ip6/::1/udp/50214/quic]
PeerInfo:  {QmRDgb3Vq1nqBBGe8VugFSXSxt7pG49xComwN8QV7Z5m3Z: [/ip4/192.168.1.66/tcp/33387 /ip4/127.0.0.1/tcp/33387 /ip6/::1/tcp/35697 /ip4/192.168.1.66/udp/44399/quic /ip4/127.0.0.1/udp/44399/quic /ip6/::1/udp/50214/quic]}
Adding a random file to the network: /ipfs/QmVaGrB1GESwjNTVvguZbYGf1mmDgU24Jtcs8wxTP2tT3x
Adding inputData directory
Adding file to the network: /ipfs/QmNdGY4t8ZPU1StBRs7fNpyr6TarwVaYNFtWFwT2tZunw5
>> Enter command: 
```

There are currently four available commands:
* `get_<path>`: Gets `path` from the IPFS network.
* `add_<size>`: Adds a random file of size `<size>`
* `add_<path>`: Adds file from path to the network.
* `connect_<p2pAddr>`: Connects to an IPFS node.