## Beyond Bitswap Probe
This is a simple CLI tool to help debug and test the exchange of content between different IPFS nodes.

### Usage
* To run the tool use:
```
$ go build
$ ./probe
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
* Optionally you can use the `--debug` flag to show verbose Bitswap DEBUG traces.

These are the currently available commands:
* `get_<ipfs_path>`: Gets `path` from the IPFS network.
* `add_<size>`: Adds a random file of size `<size>`
* `addFile_<os_path>`: Adds file from path to the network.
* `connect_<peerMultiaddr>`: Connects to an IPFS node.
* `pin_<ipfs_path>`: Pins content to the node.
* `graphsync_<peerMultiaddr>_<rootCid>`: Fetches content from a peer using graphsync.
* `exit`: Exits the command line tool.

### Using other Bitswap versions
If you want to test this tool using other Bitswap/Graphsync versions (like an implementation of an RFC), just modify the `replace` direcive in the [`go.mod`](./go.mod) to the version you want to spawn within the IPFS node. For instance, if we want to test the implementation of RFCBBL102 we would add the following replace directive:
```
replace github.com/ipfs/go-bitswap => github.com/adlrocha/go-bitswap 6f5c6dc5e81bb7a49c73d20aa3d9004747164928
```