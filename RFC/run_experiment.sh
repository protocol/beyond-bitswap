#!/bin/bash

echo "Cleaning previous results..."
RUNNER="local:docker"

rm -rf ./results
mkdir ./results

source ../testbed/testbed/scripts/exec.sh

echo "[*] Running RFC"
run_composition ./$1/$1.toml
# Plot in pdf
python3 ../testbed/testbed/scripts/pdf.py $1


echo "Cleaning previous results..."
rm -rf ./results
mkdir ./results

echo "[*] Running baseline"
run_composition ./$1/baseline.toml
# Plot in pdf
python3 ../testbed/testbed/scripts/pdf.py $1 baseline
