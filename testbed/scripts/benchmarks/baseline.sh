#!/bin/bash

RUNNER="local:docker"
BUILDER="docker:go"

source ./exec.sh

FILE_SIZE=15728640,31457280,47185920,57671680 #,104857600
RUN_COUNT=1
INSTANCES=3
LEECH_COUNT=1
PASSIVE_COUNT=0
LATENCY=5
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100
TESTCASE=ipfs-transfer
INPUT_DATA=random
DATA_DIR=../extra/inputData
TCP_ENABLED=true
MAX_CONNECTION_RATE=100


echo "Starting random files test"
# source ./exec.sh
# eval $CMD

echo "Starting real files test"
INPUT_DATA=files
DATA_DIR=../extra/inputData
TCP_ENABLED=false

source ./exec.sh
eval $CMD
#TODO: Add tests modifying bandwidth and latency
# python ../process.py --plots latency bandwidth messages overhead tcp
