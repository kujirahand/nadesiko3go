GO ?= go

VERSION ?= dev
PLATFORMS ?= darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

.PHONY: all build cmd gui install release test doctest sync-compat compat-run compat-check clean

all: build

build: cmd gui

cmd:
	$(GO) build -o bin/gonako ./cmd/gonako

gui:
	$(GO) build -o bin/gonako-gui ./cmd/gonako-gui

install:
	$(GO) install ./cmd/gonako

# 配布用に各プラットフォーム向けの単一バイナリを作る。
# Goツールチェインさえあれば、受け取る側には何も要らない。
release:
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		out=bin/gonako-$(VERSION)-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then out=$$out.exe; fi; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch $(GO) build -trimpath -ldflags="-s -w" -o $$out ./cmd/gonako || exit 1; \
	done

clean:
	rm -rf bin out

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
