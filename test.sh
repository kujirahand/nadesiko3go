#!/bin/sh
BASE_DIR=$(cd $(dirname $0); pwd)
EXE=$BASE_DIR/cnako3go
TEST_DIR=$BASE_DIR/test

# build
make

# test
echo "--- basic ---"
$EXE $TEST_DIR/basic.nako3
echo "--- func_test ---"
$EXE $TEST_DIR/func_test.nako3
echo "--- loop_test ---"
$EXE $TEST_DIR/loop_test.nako3
echo "--- fizzbizz ---"
FILE_FIZZBUZZ=$TEST_DIR/fizzbuzz-out.txt
$EXE $TEST_DIR/fizzbuzz.nako3 > $FILE_FIZZBUZZ
diff $TEST_DIR/fizzbuzz-out.txt $TEST_DIR/fizzbuzz-result.txt 
rm -f $FILE_FIZZBUZZ
echo "--- fib ---"
$EXE $TEST_DIR/fib.nako3
echo "--- func_io ---"
$EXE $TEST_DIR/func_io.nako3

echo "ok"


