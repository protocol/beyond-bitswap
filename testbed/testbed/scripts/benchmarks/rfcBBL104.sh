#!/bin/bash

RUNNER="local:exec"
BUILDER="exec:go"


echo "Cleaning previous results..."

rm -rf ../results
mkdir ../results

# FILE_SIZE=15728640,31457280,47185920,57671680
FILE_SIZE=31457280
RUN_COUNT=1
INSTANCES=18
LEECH_COUNT=17
PASSIVE_COUNT=0
LATENCY=100
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100
TESTCASE=waves
INPUT_DATA=random
# DATA_DIR=../extra/testDataset
TCP_ENABLED=false
MAX_CONNECTION_RATE=100
# WAVES = 6

source ./exec.sh

eval $CMD

docker rm -f testground-redis

# Plot latency and messages to see the behavior of this RFC
# python ../process.py --plots latency messages wants