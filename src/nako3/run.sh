#!/bin/sh
cd parser
echo "--- token ---"
cnako3 extract_token.nako3
echo "--- parse ---"
goyacc parser.y
echo "--- build ---"
go build y.go
cd ..
#go run cnako3.go -d -e "(1+2)+"
go run cnako3.go -d -e "'----------------'を表示"
#go run cnako3.go -d -e "(1+2*3)を表示。"
#go run cnako3.go -d -e "'----------------'を表示"
# go run cnako3.go -e "1+2*3を表示。"


