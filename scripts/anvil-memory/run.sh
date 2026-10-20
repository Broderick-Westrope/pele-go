#!/usr/bin/env bash
# Measures what Anvil's diagram engines add to the process, with Pele in goja and with
# pele-go, each with resvg and the wazero compilation cache, the way the design's memory
# target is set. It changes nothing in the Anvil checkout it is given: it works on a clone of its HEAD.
#
# Usage: scripts/anvil-memory/run.sh <anvil checkout>
set -euo pipefail

anvil="${1:?usage: run.sh <anvil checkout>}"
here="$(cd "$(dirname "$0")" && pwd)"
pele="$(cd "$here/../.." && pwd)"
work="$(mktemp -d)"
cache="$(mktemp -d)"
trap 'rm -rf "$work" "$cache"' EXIT

git clone -q "$anvil" "$work/anvil"
cd "$work/anvil"
git apply "$here/wazero-cache.patch"
cp "$here/zz_memory_test.go.txt" internal/mermaid/zz_memory_test.go
go mod edit -require=github.com/Broderick-Westrope/pele-go@v0.0.0 -replace="github.com/Broderick-Westrope/pele-go=$pele"
go mod tidy >/dev/null

export ANVIL_WAZERO_CACHE="$cache"
# The first run fills the compilation cache, as an earlier Anvil session would have.
go test -count=1 -run '^TestMemGo$' ./internal/mermaid/ >/dev/null
for test in TestMemGoja TestMemGo TestMemGoja TestMemGo; do
	go test -count=1 -v -run "^$test\$" ./internal/mermaid/ | grep 'loaded'
done
