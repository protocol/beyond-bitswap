#!/bin/bash

TESTGROUND_BIN="testground"
#TODO: Add an option to save the results for future analysis.

echo "Cleaning previous results..."

rm -rf ./results
mkdir ./results
echo "Starting test..."

run_bitswap(){
    $TESTGROUND_BIN run single \
        --plan=bitswap-tuning \
        --testcase=$1 \
        --builder=docker:go \
        --runner=local:docker --instances=$2 \
        -tp file_size=$3 \
        -tp run_count=$4 \
        -tp latency_ms=$5 \
        -tp jitter_pct=$6 \
        -tp parallel_gen_mb=$7 \
        -tp leech_count=$8 \
        -tp bandwidth_mb=$9
        # | tail -n 1 | awk -F 'run with ID: ' '{ print $2 }'
    
}

run() {
    echo "Running test with ($1, $2, $3, $4, $5, $6, $7, $8, $9) (TESTCASE, INSTANCES, FILE_SIZE, RUN_COUNT, LATENCY, JITTER, PARALLEL, LEECH, BANDWIDTH)"
    TESTID=`run_bitswap $1 $2 $3 $4 $5 $6 $7 $8 $9 | tail -n 1 | awk -F 'run with ID: ' '{ print $2 }'`
    echo $TESTID
    echo "Finished test $TESTID"
    $TESTGROUND_BIN collect --runner=local:docker $TESTID
    tar xzvf $TESTID.tgz
    rm $TESTID.tgz
    mv $TESTID results/
    echo "Collected results"
}
