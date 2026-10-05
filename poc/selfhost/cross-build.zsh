#!/bin/zsh
set -euo pipefail
cd "${0:A:h}"
poc_root="${PWD:h:h}"
poc_artifacts="$poc_root/artifacts/connection-poc"
poc_go="${GO_BIN:-$poc_artifacts/toolchain/go/bin/go}"
export GOENV=off GOTOOLCHAIN=local CGO_ENABLED=0
export GOMODCACHE="$poc_artifacts/gomod" GOCACHE="$poc_artifacts/gocache"
for poc_spec in windows/amd64:selfhost-node-win-x64.exe linux/amd64:selfhost-node-linux-x64 darwin/arm64:selfhost-node-mac-arm64; do
  poc_platform="${poc_spec%%:*}"
  GOOS="${poc_platform%%/*}" GOARCH="${poc_platform##*/}" "$poc_go" build -o "$poc_artifacts/${poc_spec##*:}" .
done
