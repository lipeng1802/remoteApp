#!/bin/zsh
set -euo pipefail
cd "${0:A:h}"
poc_root="${PWD:h:h}"
poc_artifacts="$poc_root/artifacts/connection-poc"
poc_go="${GO_BIN:-$poc_artifacts/toolchain/go/bin/go}"
export GOENV=off GOTOOLCHAIN=local
export GOMODCACHE="$poc_artifacts/gomod" GOCACHE="$poc_artifacts/gocache"
"$poc_go" mod verify
"$poc_go" test -timeout 30s ./...
"$poc_go" build -o "$poc_artifacts/selfhost-node" .
"$poc_go" build -o "$poc_artifacts/lab-derp" ./cmd/lab-derp
python3 smoke.py
