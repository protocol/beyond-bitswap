#!/bin/bash

RUNNER="local:exec"
BUILDER="exec:go"

echo "Cleaning previous results..."

rm -rf ../results
mkdir ../results

FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=5
INSTANCES=18
LEECH_COUNT=5
PASSIVE_COUNT=12
LATENCY=100
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100
TESTCASE=ipfs-transfer
INPUT_DATA=random
# DATA_DIR=../extra/testDataset
DATA_DIR=/home/adlrocha/Desktop/main/work/ProtocolLabs/repos/beyond-bitswap/datasets/testDataset
TCP_ENABLED=false
MAX_CONNECTION_RATE=10
# WAVES = 6

source ./exec.sh

eval $CMD

docker rm -f testground-redis

# Plot latency and data sent and received by peers.
# python ../process.py --plots latency overhead