# Testbed
This test bed offers a set of Testground test cases to test the performance of exchange interfaces over IPFS. It is composed
by the following parts:
* [Test Cases](./test): It includes the code for the test cases.
* [Scripts](./scripts): Set of scripts to easily configure, run, and process your own test.
* [Benchmarks](./banchmarks): Set of benchmarking scripts that enables the replication of certain tests without the need of additional configuration.

### Usage
To point to a specific version of `go-bitswsap` to test a specific RFC, point to the specific commit
of the branch in go.mod and build the source, the reference will be automatically updated.

*TODO*
* Use the dashboard notebook to send a single test and process the results:
* Run a benchmark:
* Process the results from a benchmark:
* Configure your own dataset and feed it to the testbed

### Plans
* [`ipfs-transfer`](./test/ipfsTransfer.go): Tests the exchange of files over a network of IPFS nodes.
* [`bitswap-transfer`](./test/transfer.go): Tests the exchange of the files over a network of Bitswap nodes.
* [`waves`](./test/waves.go): Tests the request of files by subsequent waves of leechers.

### Benchmarks
* [`Template`](./benchmarks/template.sh): Use this template if you want to configure your own benchmark.
* [`Number of Wants`](./benchmarks/wantcount.sh): Tests the number of want messages seen by a node when exchanging a large number of small files.


# `Plan:` transfer - Combinations of Seeds and Leeches

![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square)

Create an environment in which combinations of seeds and leeches are varied. This test is not about content discovery or connectivity, it is assumed that all nodes are connected and that these are executed in an homogeneous network (same CPU, Memory, Bandwidth).

## What is being optimized (min/max, reach)

- (Minimize) The performance of fetching a file. Lower is Better
  To compute this, capture:
  - file size
  - seed count
  - leech count
  - time from the first leech request to the last leech block receipt
- (Minimize) The bandwidth consumed to fetch a file. Lower is Better
  - To compute this, capture: The byte size of duplicated blocks received vs. total blocks received
- (Minimize) The total time to transfer all data to all leeches
- (Minimize) The amount of "background" data transfer
  - To compute this, capture the total bytes transferred to all nodes (including passive nodes) vs theoretical minimum.
- (Reach) The number of nodes that were able to fetch all files as instructed. (Reach 100% success of all fetches)
- (Reach) No node is expected to crash/panic during this Test Plan. (Reach 0% crashes)

## Plan Parameters

- **Network Parameters**
  - `Region` - Region or Regions where the test should be run at (default to single region)
  - `Seeds` - Number of seeds
  - `Leeches` - Number of leeches
  - `Passive Nodes` - Number of nodes that are neither seeds nor leeches
  - `Latency Average` - The average latency of connections in the system
  - `Latency Variance` - The variance over the average latency
- **Image Parameters**
  - Single Image - The go-ipfs commit that is being tested
    - Ran with custom libp2p & IPFS suites (swap in/out Bitswap & GraphSync versions, Crypto Channels, Transports and other libp2p components)
  - `File Sizes` - An array of File Sizes to be tested (default to: `[1MB, 1GB, 10GB, 100GB, 1TB]`)
  - `Directory Depth` - An Array containing objects that describe how deep/nested a directory goes and the size of files that can be found throughout (default to `[{depth: 10, size: 1MB}, {depth: 100, size: 1MB}]`

This test is not expected to support:

- An heterogeneus network in which nodes have different configurations

## Tests

### `Test:` _NAME_

- **Test Parameters**
  - n/a
- **Narrative**
  - **Warm up**
    - Boot N nodes
    - Connect all nodes to each other
    - Create a file of random data according to the parameters `File Sizes` and `Directory Depth`
    - Distribute the file to all seed nodes
  - **Benchmark**
    - All leech nodes request the file
