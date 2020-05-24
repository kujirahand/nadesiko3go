#!/bin/sh
BASE_DIR=$(cd $(dirname $0); pwd)
CNAKO3GO=$BASE_DIR/cnako3go
FMAKE=$BASE_DIR/make.sh
TEST_DIR=$BASE_DIR/test

# build
$FMAKE

# test
echo "--- fizzbizz ---"
$CNAKO3GO -d $TEST_DIR/fizzbuzz.nako3
echo "--- basic ---"
$CNAKO3GO $TEST_DIR/basic.nako3


