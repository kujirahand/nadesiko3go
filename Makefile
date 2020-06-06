# param
EXE=cnako3go
PARSER_Y_GO=parser/y.go
PARSER_TMP=parser/_parser_generated.y

all: build

build: $(PARSER_Y_GO)
	go build -o cnako3go

parser/y.go: $(PARSER_TMP)
	goyacc -o parser/y.go parser/_parser_generated.y

$(PARSER_TMP): token/token.go parser/parser.go.y
	cnako3 parser/extract_token.nako3

.PHONY: clean
clean:
	rm -f $(EXE)
	rm -f $(PARSER_Y_GO)
	rm -f $(PARSER_TMP)

.PHONY: test
test:
	go test .


