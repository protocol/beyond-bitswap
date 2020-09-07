#!/bin/bash

RUNNER="local:docker"
BUILDER="docker:go"

FILE_SIZE=15728640,31457280,47185920,57671680 #,104857600
RUN_COUNT=3
INSTANCES=10
LEECH_COUNT=10
PASSIVE_COUNT=0
LATENCY=5
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100
TESTCASE=ipfs-transfer
INPUT_DATA=random
DATA_DIR=../extra/inputData
TCP_ENABLED=false
MAX_CONNECTION_RATE=60

echo "Cleaning previous results..."

rm -rf ../results
mkdir ../results

source ./exec.sh
eval $CMD

# LEECH_COUNT=10

# source ./exec.sh
# eval $CMD
#TODO: Add tests modifying bandwidth and latency
# python ../process.py --plots latency bandwidth messages overhead tcp
