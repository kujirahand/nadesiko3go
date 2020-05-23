#!/bin/sh
BASE_DIR=$(cd $(dirname $0); pwd)
CNAKO=$BASE_DIR/bin/cnako3go
FMAKE=$BASE_DIR/make.sh
SRC_DIR=$BASE_DIR/src/nako3
TEST_DIR=$BASE_DIR/test

# build
$FMAKE

# test
echo "--- fizzbizz ---"
$CNAKO -d $TEST_DIR/fizzbuzz.nako3
echo "--- basic ---"
$CNAKO $TEST_DIR/basic.nako3


