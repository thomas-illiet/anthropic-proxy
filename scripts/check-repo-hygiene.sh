#!/usr/bin/env bash
set -euo pipefail

bad=()

while IFS= read -r -d '' path; do
	case "${path}" in
		.env|.env.*|*/.env|*/.env.*)
			if [[ "${path}" != ".env.example" ]]; then
				bad+=("${path}")
			fi
			;;
		.DS_Store|*/.DS_Store|anthropic-proxy|anthropic-proxy.exe|dist/*|coverage/*|coverage.out|*.log|*.test)
			bad+=("${path}")
			;;
	esac
done < <(git ls-files -z)

if (( ${#bad[@]} > 0 )); then
	printf 'tracked local artifact or secret-like file:\n' >&2
	printf '  %s\n' "${bad[@]}" >&2
	exit 1
fi
