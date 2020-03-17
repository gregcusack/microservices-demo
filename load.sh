#!/bin/bash
cd ./src
for f in *
do
    echo "Loading: $f..."
    # take action on each file. $f store current file name
    kind load docker-image $f
done
