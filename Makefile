GO ?= go

.PHONY: all build test sync-compat compat-run compat-check

all: build

build:
	$(GO) build -o bin/gonako ./cmd/gonako

test:
	$(GO) test ./...

sync-compat:
	./scripts/sync-compat-fixtures.sh

compat-run:
	$(GO) run ./cmd/gonako compat run --cases ./testdata/compat/cases --out ./out

compat-check:
	cd nadesiko3 && npm run compat:check -- ../out

