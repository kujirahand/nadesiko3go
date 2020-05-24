#!/bin/sh
BASE_DIR=$(cd $(dirname $0); pwd)
EXE=$BASE_DIR/nadesiko3go
FMAKE=$BASE_DIR/make.sh
TEST_DIR=$BASE_DIR/test

# build
$FMAKE

# test
echo "--- fizzbizz ---"
$EXE -d $TEST_DIR/fizzbuzz.nako3
echo "--- basic ---"
$EXE $TEST_DIR/basic.nako3


