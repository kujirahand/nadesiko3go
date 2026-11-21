# Ubuntu 24.04 で gonako-gui をビルドする。
# バイナリは libgtk-3-0 と libwebkit2gtk-4.1-0 に動的リンクする。
#
#   docker build -f docker/gonako-gui/Dockerfile -t gonako-gui-builder .
#   docker create --name gonako-gui-out gonako-gui-builder
#   docker cp gonako-gui-out:/usr/local/bin/gonako-gui ./bin/gonako-gui
#   docker rm gonako-gui-out
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates curl git gcc g++ pkg-config \
        libgtk-3-dev libwebkit2gtk-4.1-dev \
    && rm -rf /var/lib/apt/lists/*

# go.mod が要求する版。GOTOOLCHAIN=auto なので、go.mod が新しければビルド時に追従する。
ARG GO_VERSION=1.26.0
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz
ENV PATH=/usr/local/go/bin:${PATH} \
    GOTOOLCHAIN=auto \
    CGO_ENABLED=1

WORKDIR /src
COPY . .
RUN chmod +x scripts/webkit-gtk-tag.sh scripts/go-gui.sh \
    && ./scripts/go-gui.sh build -trimpath -ldflags="-s -w" -o /usr/local/bin/gonako-gui ./cmd/gonako-gui

CMD ["gonako-gui", "-help"]
