#!/bin/sh
BASE_DIR=$(cd $(dirname $0); pwd)
FILE_OUT=$BASE_DIR/bin/cnako3g
SRC_DIR=$BASE_DIR/src/nako3

cd $SRC_DIR
go build -o $FILE_OUT
cd $BASE_DIR




