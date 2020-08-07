#!/bin/sh

FILE_SIZE=15728640,31457280,47185920,57671680
RUN_COUNT=5
INSTANCES=4
LEECH_COUNT=2
PASSIVE_COUNT=0
LATENCY=5
JITTER=10
BANDWIDTH=100
PARALLEL_GEN=100

TESTGROUND_BIN="testground"
#TODO: Add an option to save the results for future analysis.

echo "Cleaning previous results..."

rm -rf ./results
mkdir ./results
echo "Starting test..."

#TODO: Test cases determine the test scenario.
#TODO: In the testcase we use different type of files, we can add other
# configuration parameters.
# TODO: 
run_bitswap(){
    $TESTGROUND_BIN run single \
        --plan=bitswap-tuning \
        --testcase=transfer \
        --builder=docker:go \
        --runner=local:docker --instances=$1 \
        -tp file_size=$2 \
        -tp run_count=$3 \
        -tp latency_ms=$4 \
        -tp jitter_pct=$5 \
        -tp parallel_gen_mb=$6 \
        -tp leech_count=$7 \
        -tp bandwidth_mb=$8
        # | tail -n 1 | awk -F 'run with ID: ' '{ print $2 }'
    
}

run_test() {
    $TESTGROUND_BIN run single \
        --plan=bitswap-tuning \
        --testcase=transfer \
        --builder=docker:go \
        --runner=local:docker --instances=$1 
}

run() {
    echo "Running test with ($1, $2, $3, $4, $5, $6, $7, $8) (INSTANCES, FILE_SIZE, RUN_COUNT, LATENCY, JITTER, PARALLEL, LEECH, BANDWIDTH)"
    TESTID=`run_bitswap $1 $2 $3 $4 $5 $6 $7 $8 | tail -n 1 | awk -F 'run with ID: ' '{ print $2 }'`
    echo $TESTID
    echo "Finished test $TESTID"
    $TESTGROUND_BIN collect --runner=local:docker $TESTID
    tar xzvf $TESTID.tgz
    rm $TESTID.tgz
    mv $TESTID results/
    echo "Collected results"
}

run $INSTANCES $FILE_SIZE $RUN_COUNT $LATENCY $JITTER $PARALLEL_GEN $LEECH_COUNT $BANDWIDTH
BANDWIDTH=150
INSTANCES=7
LEECH_COUNT=3
run $INSTANCES $FILE_SIZE $RUN_COUNT $LATENCY $JITTER $PARALLEL_GEN $LEECH_COUNT $BANDWIDTH

BANDWIDTH=150
INSTANCES=7
LEECH_COUNT=1
run $INSTANCES $FILE_SIZE $RUN_COUNT $LATENCY $JITTER $PARALLEL_GEN $LEECH_COUNT $BANDWIDTH

python3 process.py --plots latency messages overhead throughput