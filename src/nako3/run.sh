#!/bin/sh
cd parser
cnako3 extract_token.nako3
echo "--- parse ---"
goyacc _parser_generated.y
echo "--- build ---"
go build y.go
cd ..
echo "--- run ---"

#go run cnako3.go -d -e "(1+2)+"
#go run cnako3.go -d -e "'----------------'を表示"
#go run cnako3.go -d -e "(1+2*3)を表示。"
#go run cnako3.go -d -e "'----------------'を表示"
# go run cnako3.go -e "1+2*3を表示。"
# go run cnako3.go -e -d "7>=3を表示。「OK」を表示。"
# go run cnako3.go -d test.nako3
go run cnako3.go -d -e "3回それを表示。"

