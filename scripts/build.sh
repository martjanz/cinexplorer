#!/usr/bin/env bash
# Cross-compiles Cinexplorer for Windows, macOS (universal) and Linux into dist/cinexplorer/.
set -euo pipefail
cd "$(dirname "$0")/.."
out=dist/cinexplorer
mkdir -p "$out"
export CGO_ENABLED=0
build() { GOOS=$1 GOARCH=$2 go build -trimpath -ldflags "-s -w" -o "$out/$3" ./cmd/cinexplorer; }
build windows amd64 cinexplorer-windows.exe
build linux   amd64 cinexplorer-linux
build darwin  amd64 cinexplorer-macos-amd64
build darwin  arm64 cinexplorer-macos-arm64
go run github.com/randall77/makefat@latest "$out/cinexplorer-macos" "$out/cinexplorer-macos-amd64" "$out/cinexplorer-macos-arm64"
rm "$out/cinexplorer-macos-amd64" "$out/cinexplorer-macos-arm64"
ls -la "$out"
