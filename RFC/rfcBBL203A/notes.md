# Notes about the experiment
For RFCBBL203A different compression strategies where implemented. In the test experiment available in this directory the compression strategy tested is the Bitswap stream compression strategy. If you want to test any of the other compression strategies, [point in the RFC toml file for the test](./rfcBBL203A.toml) to the following versions:

* Full message strategy: `0b489d490b692f338b16c8907d989e27b0ac7a11`
* Block compression strategy: `e4fd84f22580d418c32e47a5fd243bbb1dc14d20`

If, on the contrary, you want to test IPFS working with compression within `libp2p`, you need to go to the `feature/compression` branch in this repository. To run a test using libp2p compression, run the dashboard.py notebook and set the test you want to do. 