#!/bin/sh
BASE_DIR=$(cd $(dirname $0); pwd)
PARSER_DIR=$BASE_DIR/parser
GOYACC=$GOPATH/bin/goyacc

echo "--- build parser ---"
cd $PARSER_DIR
cnako3 extract_token.nako3
$GOYACC _parser_generated.y
cd $BASE_DIR

echo "--- build cnako3go ---"
go build

