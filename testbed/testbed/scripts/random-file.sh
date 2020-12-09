#!/bin/sh
if [ $# -eq 0 ]
    then
        echo "[!!] No argument supplied. Example of use: ./random-file 10M <output_dir>"
        exit 0
fi
OUTPUT_DIR=$2
NAME=`date +%s`
echo "[*] Generating a random file of $1B in $OUTPUT_DIR"
if [ -z "$2" ]
  then
    OUTPUT_DIR="../../test-datasets"
fi

head -c $1 </dev/urandom > $OUTPUT_DIR/$NAME