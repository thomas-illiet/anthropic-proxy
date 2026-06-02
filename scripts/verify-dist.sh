#!/usr/bin/env bash
set -euo pipefail

version="${1:-dev}"
dist_dir="${2:-dist/release}"
binary="${3:-anthropic-proxy}"

verify_tar() {
	local goos="$1"
	local goarch="$2"
	local archive="${dist_dir}/${binary}_${version}_${goos}_${goarch}.tar.gz"
	local root="${binary}_${version}_${goos}_${goarch}"
	test -s "${archive}"
	tar -tzf "${archive}" | grep -Fx "${root}/${binary}"
	tar -tzf "${archive}" | grep -Fx "${root}/.env.example"
	tar -tzf "${archive}" | grep -Fx "${root}/README.txt"
}

verify_zip() {
	local goos="$1"
	local goarch="$2"
	local archive="${dist_dir}/${binary}_${version}_${goos}_${goarch}.zip"
	local root="${binary}_${version}_${goos}_${goarch}"
	test -s "${archive}"
	unzip -Z1 "${archive}" | grep -Fx "${root}/${binary}.exe"
	unzip -Z1 "${archive}" | grep -Fx "${root}/.env.example"
	unzip -Z1 "${archive}" | grep -Fx "${root}/README.txt"
}

verify_checksums() {
	test -s "${dist_dir}/SHA256SUMS"
	if command -v sha256sum >/dev/null 2>&1; then
		(cd "${dist_dir}" && sha256sum -c SHA256SUMS)
	else
		(cd "${dist_dir}" && shasum -a 256 -c SHA256SUMS)
	fi
}

verify_tar linux amd64
verify_tar linux arm64
verify_tar darwin amd64
verify_tar darwin arm64
verify_zip windows amd64
verify_checksums
