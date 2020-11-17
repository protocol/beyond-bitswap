# P2P File-Sharing Testbed

*Disclaimer: This is a work in progress, expect the base code to change rapidly and potentially to frequently break. Post issues, and contributions are not only welcome but needed. Let's drive speed-ups to file-sharing in P2P networks together!*

This testbed offers a set of Testground test cases to evaluate the the performance of different exchange interface implementations over IPFS. It is composed
by the following parts:
* [Test Cases](./test): It includes the code for the test cases.
* [Scripts](./scripts): Set of scripts to easily configure, run, and process your own test.
* [Benchmarks](./scripts/benchmarks): Set of benchmarking scripts that enables the replication of certain tests without the need of additional configuration.


### Plans
* [`ipfs-transfer`](./test/ipfsTransfer.go): Tests the exchange of files over a network of IPFS nodes.
* [`bitswap-transfer`](./test/transfer.go): Tests the exchange of files over a network of Bitswap nodes.
* [`bitswap-transfer`](./test/graphsyncTransfer.go): Tests the exchange of files beetween two IPFS nodes using Graphsync.

* [`tcp-transfer`](./test/TCPtransfer.go): Tests the exchange of files using TCP between two nodes.
* [`waves`](./test/waves.go): Tests the request of files by subsequent waves of leechers.
* [`sparse`](./test/sparse.go): Tests seeders which are not connected directly to seeders. Between them there are always a set of passive nodes.

### Benchmarks
* [`Template`](./benchmarks/template.sh): Use this template if you want to configure your own benchmark.
* [`Number of Wants`](./benchmarks/wantcount.sh): Tests the number of want messages seen by a node when exchanging a large number of small files.

## Installation
Clone the repository to start the installation:
```
$ git clone https://github.com/adlrocha/beyond-bitswap.git
```

### Testground

To run the testbed in your local environment, first you need to install [Testground](https://github.com/testground/testground). Testrgound requires Docker and Go ^1.14 installed in your machine.
```
$ git clone https://github.com/testground/testground.git
$ cd testground
$ make install       # builds testground and the Docker image, used by the local:docker runner.
```

 Be sure that the testground executable is available in your `PATH`. You can easily add Testground to your `PATH` by building and moving the binary to a directory in your `PATH`. For instance, in a Linux system you can:
 
```
$ cd tesground
$ go build
$ sudo cp testground /usr/local/bin
```
You can check that Testground is conveniently installed in your environment by running `testground --help`.


With Testground installed, you need to import the testbed plan from the testbed, to do so you need to:
* Go to the root of the project: `cd beyond-bitswap`
* Run the testground daemon in an independent terminal (or the background): `testground daemon`
* And from another terminal (or in the foreground) run: `testground plan import --from testbed/`
```
$ testground plan import --from testbed/
Oct  9 06:45:09.887055  INFO    using home directory: /home/ubuntu/testground
Oct  9 06:45:09.887113  INFO    no .env.toml found at /home/ubuntu/testground/.env.toml; running with defaults
created symlink /home/ubuntu/testground/plans/testbed -> testbed/
imported plans:
testbed ipfs-transfer
testbed sparse
testbed waves
testbed bitswap-transfer
```
* To check that the plans were imported successfully run the following:
```
$ testground plan list
Oct  9 06:53:48.065686  INFO    using home directory: /home/ubuntu/testground
Oct  9 06:53:48.065747  INFO    no .env.toml found at /home/ubuntu/testground/.env.toml; running with defaults
dockercustomize
benchmarks
example
verify
splitbrain
placebo
network
testbed
```
You see that the testbed plan should be included in the list of available plans.

### Processing tools
The testbed includes several tools to help you run and process the results of your experiments. All of them are based in python and bash, so first of all be sure that you have `python3` and `python3-pip` installed. The code of these tools resides in `./scripts`. 

You may optionally want to start a virtual environment for the installation of the python dependencies:
```
$ pip3 install virtualenv
$ virtualenv env
$ source ~/env/bin/activate
```


To install all the required dependencies run:
```
$ cd scripts
$ pip install -r requirements.txt
```
With this you have the dependencies to run the processing python scripts and start the Jupyter Notebook.

### Deploying a Jupyter notebook server.
You can enable access to your testbed remotely by starting a Jupyter notebook server that allows the run of experiments through `dashboard.pynb`. To set up the Jupyter notebook:
* Create a jupyter server with a generated config file:
```
$ jupyter notebook --generate-config
$ jupyter notebook password
```
* Add at least the following variables to your config `~/.jupyter/jupyter_notebook_config.py`:
```
c.NotebookApp.ip = "*"
c.NotebookApp.notebook_dir = "TESTBED_DIR"
c.NotebookApp.open_browser = False
```
* And run the jupyter server:
```
$ jupyter notebook --config ~/.jupyter/jupyter_notebook_config.py
```

## Running experiments
You have several ways of running an experiment in the testbed. For all of these ways remember that your testground daemon need to be running (i.e. `testground daemon` in the background).

* Running a benchmark script: This is the most straightforward way, and the perfect way to test your installation you just need to go to `./scripts/benchmarks` and run any of the benchmark scripts. This will run the template experiment, collect your results and place them in `./scripts/results`.
```
$ ./template.sh
Cleaning previous results...
Starting test...
Running test with (ipfs-transfer, 5, 15728640,31457280,47185920,57671680, 1, 10, 10, 100, 1, 150, random, ../extra/testDataset, false, 100, 2) (TESTCASE, INSTANCES, FILE_SIZE, RUN_COUNT, LATENCY, JITTER, PARALLEL, LEECH, BANDWIDTH, INPUT_DATA, DATA_DIR, TCP_ENABLED, MAX_CONNECTION_RATE, PASSIVE_COUNT)
d081d14bed0c
Finished test d081d14bed0c
Oct  9 10:58:05.774013  INFO    using home directory: /home/adlrocha/testground
Oct  9 10:58:05.774168  INFO    .env.toml loaded from: /home/adlrocha/testground/.env.toml
Oct  9 10:58:05.774180  INFO    testground client initialized   {"addr": "http://localhost:8042"}

>>> Result:

Oct  9 10:58:05.784758  INFO    created file: d081d14bed0c.tgz
d081d14bed0c/.
d081d14bed0c/single
d081d14bed0c/single/0
d081d14bed0c/single/0/diagnostics.out
d081d14bed0c/single/0/results.out
d081d14bed0c/single/0/run.err
d081d14bed0c/single/0/run.out
d081d14bed0c/single/1
d081d14bed0c/single/1/diagnostics.out
d081d14bed0c/single/1/results.out
d081d14bed0c/single/1/run.err
d081d14bed0c/single/1/run.out
d081d14bed0c/single/2
d081d14bed0c/single/2/diagnostics.out
d081d14bed0c/single/2/results.out
d081d14bed0c/single/2/run.err
d081d14bed0c/single/2/run.out
d081d14bed0c/single/3
d081d14bed0c/single/3/diagnostics.out
d081d14bed0c/single/3/results.out
d081d14bed0c/single/3/run.err
d081d14bed0c/single/3/run.out
d081d14bed0c/single/4
d081d14bed0c/single/4/diagnostics.out
d081d14bed0c/single/4/results.out
d081d14bed0c/single/4/run.err
d081d14bed0c/single/4/run.out
Collected results
testground-redis
```

* Using the Jupyter Notebook: To run an experiment using the Notebook, run the first cell, and it will be kind of straightforward to you how to run a process the results. All the results are collected in `./scripts/results`.

* Directly through Testground: In the end this testbed are just a bunch of Testground testplans so you can use Testground to run experiments manually. Check [the docs](https://docs.testground.ai/v/master/running-test-plans) to learn how to do this. We currently only support single runs, in the future we will also support composite-files. If you run through Testground you wil have to collect the results manually.

* Using a Testground composition file: The same way you run Testground in simple mode you can easily replicate experiments expressed in composition files running: 
```
testground run composition -f <composition_file>
```
You can find examples of compositions files in the `./compositions` directory.

## Processing the results.
The results can be processed using the Jupyter notebook or the `scripts/process.py` script. If you want to process the results generated from a benchmark you can run diretly:
```
$ python process.py --plots messages latency overhead wants
```
If you have saved the results in some other directory, you can point to the directory using:
```
$ python ../process.py --plots messages --dir <RESULTS_DIR>
```
To see the kind of metrics that you can output with the processing scripts run
```
$ python ../process.py --help
usage: process.py [-h] [-p PLOTS [PLOTS ...]] [-o OUTPUTS [OUTPUTS ...]] [-dir DIR]

optional arguments:
  -h, --help            show this help message and exit
  -p PLOTS [PLOTS ...], --plots PLOTS [PLOTS ...]
                        One or more plots to be shown. Available: latency, throughput, overhead, messages, wants, tcp.
  -o OUTPUTS [OUTPUTS ...], --outputs OUTPUTS [OUTPUTS ...]
                        One or more outputs to be shown. Available: data
  -dir DIR, --dir DIR   Result directory to process
```



## Coming soon
* Parameters of the testplans
* Configure your own dataset for the experiments.
* Create your own testplan.
* Bring you rexchange interface.

<!-- 
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
    - All leech nodes request the file -->
