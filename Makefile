GO ?= go

VERSION ?= dev
PLATFORMS ?= darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

.PHONY: all build cmd cui gui install release release-cli release-gui test doctest sync-compat compat-run compat-check gen-command-list clean benchmark

all: build

build: cmd cui gui

cmd:
	$(GO) build -o bin/gonako ./cmd/gonako

cui:
	$(GO) build -o bin/gonako-cui ./cmd/gonako-cui

gui:
	$(GO) build -o bin/gonako-gui ./cmd/gonako-gui

install:
	$(GO) install ./cmd/gonako
	$(GO) install ./cmd/gonako-cui

# 配布用に各プラットフォーム向けのバイナリ・ツール（CLI・GUI）を作る。
release:
	$(GO) run ./scripts/build-release.go -version $(VERSION) -platforms "$(PLATFORMS)"

release-cli:
	$(GO) run ./scripts/build-release.go -version $(VERSION) -platforms "$(PLATFORMS)" -skip-gui

release-gui:
	$(GO) run ./scripts/build-release.go -version $(VERSION) -platforms "$(PLATFORMS)" -skip-cli

# 実行
run-gui:
	$(GO) run ./cmd/gonako-gui

clean:
	rm -rf bin out benchmark/build

benchmark: cmd
	$(GO) run ./benchmark/runner.go

test:
	$(GO) test ./...

# マニュアルと固定サンプルを実行して、書かれている表示結果と一致するか確かめる。
# 対象を絞るときは make doctest DOCTEST_ARGS="testdata/doctest/core/plugin_system.txt"
DOCTEST_ARGS ?=
doctest:
	$(GO) run ./cmd/gonako doctest $(DOCTEST_ARGS)

sync-compat:
	./scripts/sync-compat-fixtures.sh

compat-run:
	$(GO) run ./cmd/gonako compat run --cases ./testdata/compat/cases --out ./out

compat-check:
	cd nadesiko3 && npm run compat:check -- ../out

gen-command-list:
	$(GO) run ./scripts/gen-command-list.go
