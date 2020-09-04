#!/bin/bash

RUNNER="local:exec"
BUILDER="exec:go"

FILE_SIZE=1728640
RUN_COUNT=1000
INSTANCES=10
LEECH_COUNT=1
PASSIVE_COUNT=0
LATENCY=5
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100
TESTCASE=ipfs-transfer
INPUT_DATA=random
DATA_DIR=../extra/inputData
TCP_ENABLED=false
MAX_CONNECTION_RATE=100

source ./exec.sh

eval $CMD
LEECH_COUNT=2
docker rm -f testground-redis
eval $CMD
LEECH_COUNT=5
docker rm -f testground-redis
eval $CMD
LEECH_COUNT=9
docker rm -f testground-redis
eval $CMD

cd ..
python ../process.py --plots wants