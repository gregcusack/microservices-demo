#!/usr/bin/env bash

INPUT=$1
OUTPUT=$2
CWD=$(pwd)

python3 \
-m gspan_mining \
-s 100 \
-d True \
-l 2 \
-u 2 \
-w True \
"$CWD"/"$INPUT" >"$CWD"/"$OUTPUT"
