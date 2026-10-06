#!/bin/zsh
set -euo pipefail
cd "${0:A:h}"
poc_root="${PWD:h:h}"
poc_artifacts="$poc_root/artifacts/connection-poc"
poc_go="${GO_BIN:-$poc_artifacts/toolchain/go/bin/go}"
export GOENV=off GOTOOLCHAIN=local CGO_ENABLED=0
export GOMODCACHE="$poc_artifacts/gomod" GOCACHE="$poc_artifacts/gocache"
GOOS=windows GOARCH=amd64 "$poc_go" build -o "$poc_artifacts/selfhost-node-win-x64.exe" .
GOOS=windows GOARCH=amd64 "$poc_go" build -o "$poc_artifacts/lab-derp-win-x64.exe" ./cmd/lab-derp
# Unique output; never overwrite another build or recursively remove a root.
poc_bundle=$(mktemp -d "$poc_artifacts/windows-test-XXXXXXXX")
mkdir -p "$poc_bundle/poc/selfhost" "$poc_bundle/artifacts/connection-poc"
cp verify-windows.ps1 headscale.yaml policy.json WINDOWS_TEST.md "$poc_bundle/poc/selfhost/"
cp "$poc_artifacts/selfhost-node-win-x64.exe" "$poc_artifacts/lab-derp-win-x64.exe" "$poc_bundle/artifacts/connection-poc/"
GOOS=windows GOARCH=amd64 python3 bundle.py "$poc_bundle" "$poc_go"
