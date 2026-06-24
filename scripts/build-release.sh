#!/usr/bin/env bash
set -euo pipefail

# Build Veil release binaries for the supported v0.1.0 platform matrix.
#
# Usage:
#   VERSION=v0.1.0 ./scripts/build-release.sh
#
# Optional environment:
#   OUT_DIR=dist/release
#   TARGETS="darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64"
#   COMMIT=<short-commit>
#   BUILD_DATE=<rfc3339-utc>

version="${VERSION:-}"
if [[ -z "${version}" ]]; then
  if version="$(git describe --tags --exact-match 2>/dev/null)"; then
    :
  else
    version="dev"
  fi
fi

commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
build_date="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
out_dir="${OUT_DIR:-dist/release}"
targets="${TARGETS:-darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64}"

case "${out_dir}" in
  "" | "/" | "." | "..")
    echo "refusing unsafe OUT_DIR=${out_dir}" >&2
    exit 2
    ;;
esac

rm -rf "${out_dir}"
mkdir -p "${out_dir}"

ldflags="-s -w -X main.version=${version} -X main.commit=${commit} -X main.buildDate=${build_date}"

echo "Building veil ${version} (${commit})"
echo "Output: ${out_dir}"
echo

for target in ${targets}; do
  os="${target%/*}"
  arch="${target#*/}"

  if [[ -z "${os}" || -z "${arch}" || "${os}" == "${arch}" ]]; then
    echo "invalid target ${target}; expected os/arch" >&2
    exit 2
  fi

  name="veil-${version}-${os}-${arch}"
  if [[ "${os}" == "windows" ]]; then
    name="${name}.exe"
  fi

  path="${out_dir}/${name}"
  echo "Building ${os}/${arch} -> ${path}"
  CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build \
    -trimpath \
    -ldflags "${ldflags}" \
    -o "${path}" \
    ./cmd/veil

  if [[ "${os}" != "windows" ]]; then
    chmod +x "${path}"
  fi
done

echo
echo "Release artifacts:"
find "${out_dir}" -maxdepth 1 -type f -name 'veil-*' | sort
