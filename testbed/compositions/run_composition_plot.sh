#!/bin/bash

echo "Cleaning previous results..."
RUNNER="local:docker"

rm -rf ./results
mkdir ./results

source ../testbed/scripts/exec.sh


echo "[*] Running Composition"
run_composition ./$1.toml
# Plot in pdf
python3 ../testbed/scripts/pdf_composition.py $1