#!/bin/sh
BASE_DIR=$(cd $(dirname $0); pwd)
EXE=$BASE_DIR/nadesiko3go
FMAKE=$BASE_DIR/make.sh
TEST_DIR=$BASE_DIR/test

# build
$FMAKE

# test
echo "--- fizzbizz ---"
FILE_FIZZBUZZ=$TEST_DIR/fizzbuzz-out.txt
$EXE $TEST_DIR/fizzbuzz.nako3 > $FILE_FIZZBUZZ
diff $TEST_DIR/fizzbuzz-out.txt $TEST_DIR/fizzbuzz-result.txt 
rm -f $FILE_FIZZBUZZ
echo "--- basic ---"
$EXE $TEST_DIR/basic.nako3
echo "--- func_test ---"
$EXE $TEST_DIR/func_test.nako3
echo "--- loop_test ---"
$EXE $TEST_DIR/loop_test.nako3
echo "ok"


