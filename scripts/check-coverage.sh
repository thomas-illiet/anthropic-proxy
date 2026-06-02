#!/usr/bin/env bash
set -euo pipefail

threshold="${1:-75.0}"
profile="${2:-coverage.out}"

go test -covermode=atomic -coverprofile="${profile}" ./...

total="$(
	go tool cover -func="${profile}" |
		awk '/^total:/ { gsub(/%/, "", $3); print $3 }'
)"

if [[ -z "${total}" ]]; then
	echo "coverage: unable to read total coverage from ${profile}" >&2
	exit 1
fi

awk -v total="${total}" -v threshold="${threshold}" '
	BEGIN {
		if (total + 0 < threshold + 0) {
			printf "coverage %.1f%% is below required %.1f%%\n", total, threshold > "/dev/stderr"
			exit 1
		}
		printf "coverage %.1f%% >= %.1f%%\n", total, threshold
	}
'
