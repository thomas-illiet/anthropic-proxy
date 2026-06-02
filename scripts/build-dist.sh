#!/usr/bin/env bash
set -euo pipefail

version="${1:-dev}"
ldflags="${2:--s -w -X github.com/thomas-illiet/anthropic-proxy/internal/cli.version=${version}}"
cmd="${3:-.}"
binary="${4:-anthropic-proxy}"
dist_dir="${PWD}/dist/release"
package_dir="${PWD}/dist/package"

rm -rf "${dist_dir}" "${package_dir}"
mkdir -p "${dist_dir}" "${package_dir}"

create_readme() {
	local package_path="$1"
	printf '%s\n' \
		"anthropic-proxy ${version}" \
		"" \
		"Start the server:" \
		"" \
		"  ./${binary} serve" \
		"" \
		"On Windows PowerShell:" \
		"" \
		"  .\\${binary}.exe serve" \
		"" \
		"Configuration is loaded from real ANTHROPIC_PROXY_* environment variables and an optional .env file in the current directory." \
		> "${package_path}/README.txt"
}

build_package() {
	local goos="$1"
	local goarch="$2"
	local binary_name="${binary}"
	local archive_name="${binary}_${version}_${goos}_${goarch}"
	local package_path="${package_dir}/${archive_name}"

	if [[ "${goos}" == "windows" ]]; then
		binary_name="${binary}.exe"
	fi

	rm -rf "${package_path}"
	mkdir -p "${package_path}"
	CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
		go build -trimpath -ldflags="${ldflags}" -o "${package_path}/${binary_name}" "${cmd}"
	cp .env.example "${package_path}/.env.example"
	create_readme "${package_path}"

	if [[ "${goos}" == "windows" ]]; then
		(cd "${package_dir}" && zip -qr "${dist_dir}/${archive_name}.zip" "${archive_name}")
	else
		(cd "${package_dir}" && tar -czf "${dist_dir}/${archive_name}.tar.gz" "${archive_name}")
	fi
}

write_checksums() {
	(
		cd "${dist_dir}"
		if command -v sha256sum >/dev/null 2>&1; then
			sha256sum ./*.{tar.gz,zip} > SHA256SUMS
		else
			shasum -a 256 ./*.{tar.gz,zip} > SHA256SUMS
		fi
	)
}

build_package linux amd64
build_package linux arm64
build_package darwin amd64
build_package darwin arm64
build_package windows amd64
write_checksums
