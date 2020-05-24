#!/bin/sh
BASE_DIR=$(cd $(dirname $0); pwd)
FILE_OUT=$BASE_DIR/cnako3go
PARSER_DIR=$BASE_DIR/parser

echo "--- build parser ---"
cd $PARSER_DIR
cnako3 extract_token.nako3
goyacc _parser_generated.y
cd $BASE_DIR

echo "--- build cnako3go ---"
go build -o $FILE_OUT

