# RFC Improvement Proposals
In this directory you will find all the content related to the [RFC Improvement Proposals](../README.md#Enhancement-RFCs). Each directory belongs to an RFC. For those of them in a `prototype` state, apart from its description in a markdown file you will find a set of `.toml` files with the configuration to run a experiment for the RFC prototype in the [testbed]("../testbed/). Be sure to install the testbed and all its dependencies before trying to run any of these experiments.

Once you have the testbed environment ready you can run the experiments for any of the `prototyped` RFCs running the following script indicating the RFC to run:
```
# Example
$ ./run_experiment.sh rfcBBL102

# General example
$ ./run_experiment.sh <rfc_code>
```

The results of the experiment are aggregated in a set of pdf files within the directory of the RFC. So in our specific example above go to [rfcBBL102 directory](./rfcBBL102) to gather the results for the experiment.
