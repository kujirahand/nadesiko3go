GO ?= go

.PHONY: all build install

all: build

build:
	$(GO) build -o bin/gonako ./cmd/gonako
	$(GO) build -o bin/gonako-cui ./cmd/gonako-cui
	$(GO) build -o bin/gonako-gui ./cmd/gonako-gui

install:
	$(GO) install ./cmd/gonako
	$(GO) install ./cmd/gonako-cui
