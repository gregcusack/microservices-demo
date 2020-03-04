#!/bin/bash
cd ./src
for f in *
do
    echo "Building Docker: $f..."
    # take action on each file. $f store current file name
    docker build $f -t $f
done