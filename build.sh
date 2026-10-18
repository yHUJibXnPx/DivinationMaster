#!/bin/sh
# 交叉编译静态二进制（无 cgo、语料已内嵌，单文件即可运行）到 go/dist/
# Termux（Android arm64）用 divination-linux-arm64；Apple Silicon 用 divination-darwin-arm64。
set -eu
cd "$(dirname "$0")"
mkdir -p dist
for target in linux/arm64 linux/amd64 darwin/arm64 darwin/amd64; do
  os=${target%/*}; arch=${target#*/}
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags="-s -w" -o "dist/divination-$os-$arch" ./cmd/divination
  echo "built dist/divination-$os-$arch"
done
cd dist
if command -v sha256sum >/dev/null 2>&1; then sha256sum divination-* > SHA256SUMS; else shasum -a 256 divination-* > SHA256SUMS; fi
