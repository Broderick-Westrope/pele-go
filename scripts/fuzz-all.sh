#!/usr/bin/env bash
# Fuzzes every diagram type, or the types given, against one engine for a fixed time each.
#
#   scripts/fuzz-all.sh FuzzParity 10s            # goja, every type
#   scripts/fuzz-all.sh FuzzParityV8 60s gantt    # V8, one type
#
# The types are those with a captured shard. Inputs the fuzzer finds failing are written
# under testdata/fuzz/<target>. It keeps going after a failure and exits non-zero at the end.
set -euo pipefail

target=$1
fuzztime=$2
shift 2
cd "$(dirname "$0")/.."

types=("$@")
if [ ${#types[@]} -eq 0 ]; then
	for shard in internal/oracle/testdata/captured/*.jsonl.gz; do
		typ=$(basename "$shard" .jsonl.gz)
		[ "$typ" = none ] || types+=("$typ")
	done
fi

failed=()
for typ in "${types[@]}"; do
	echo "::group::$target $typ"
	if ! PELE_FUZZ_TYPE=$typ go test -run '^$' -fuzz "^$target\$" -fuzztime "$fuzztime" .; then
		failed+=("$typ")
	fi
	echo "::endgroup::"
done

if [ ${#failed[@]} -gt 0 ]; then
	echo "$target failed for: ${failed[*]}" >&2
	exit 1
fi
