GO ?= go

VERSION ?= dev
PLATFORMS ?= darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64

.PHONY: all build install release test sync-compat compat-run compat-check clean

all: build

build:
	$(GO) build -o bin/gonako ./cmd/gonako

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

sync-compat:
	./scripts/sync-compat-fixtures.sh

compat-run:
	$(GO) run ./cmd/gonako compat run --cases ./testdata/compat/cases --out ./out

compat-check:
	cd nadesiko3 && npm run compat:check -- ../out

