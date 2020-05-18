#!/bin/sh
cd parser
echo "--- token ---"
cnako3 extract_token.nako3
echo "--- parse ---"
goyacc parser.y
echo "--- build ---"
go build y.go
cd ..
go run cnako3.go

