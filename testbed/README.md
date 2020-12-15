# Beyond Bitswap Testbed

<p align="left">
  <a href="https://github.com/protocol/ResNetLab" title="ResNetLab">
    <img src="https://research.protocol.ai/images/resnetlab/resnetlab_logo_blue.svg" width="150" />
  </a>
</p>

This repo implements a testbed to evaluate the performance of different IPFS exchange interfaces. It is currently used in the scope of the Beyond Bitswap project to test improvement proposals over the base code.

For the full project description, please consult [BEYOND_BITSWAP](https://github.com/protocol/beyond-bitswap)

The repo is conformed by the following parts:
* [Testbed](./testbed): It implements a Testground test environment and a set of python scripts to run the tests and process the results.
* [Probe](./probe): A simple CLI tool that comes pretty handy to test new implementations and for debugging purposes.
* [Bitswap Viewer](./viewer): This ObservableHQ notebook would enable you with a visual way to observe the messages exchanged between Bitswap nodes step by step in a file-sharing execution.
* [Datasets](./test-datasets): Add in this directory any of the datasets you want to use in your tests.

You will find additional documentation in these directories.
