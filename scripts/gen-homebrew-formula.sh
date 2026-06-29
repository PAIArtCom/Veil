#!/usr/bin/env bash
set -euo pipefail

# Generate a Homebrew formula with correct SHA256 values for a given release.
#
# Usage:
#   VERSION=v0.1.0 ./scripts/gen-homebrew-formula.sh > Formula/veil.rb
#   ./scripts/gen-homebrew-formula.sh v0.1.0 > Formula/veil.rb
#
# The script fetches checksums.txt from the GitHub release and substitutes
# the PLACEHOLDER_* values in homebrew/veil.rb.
#
# Requires: curl
#
# Optional:
#   CHECKSUMS_FILE=dist/release/checksums.txt

version="${VERSION:-${1:-}}"
if [[ -z "${version}" ]]; then
  echo "usage: VERSION=<tag> $0" >&2
  echo "   or: $0 <tag>" >&2
  exit 2
fi

repo="PAIArtCom/Veil"
checksums_url="https://github.com/${repo}/releases/download/${version}/checksums.txt"
template="$(dirname "$0")/../homebrew/veil.rb"

if [[ ! -f "${template}" ]]; then
  echo "error: template not found at ${template}" >&2
  exit 1
fi

if [[ -n "${CHECKSUMS_FILE:-}" ]]; then
  echo "Reading checksums for ${version} from ${CHECKSUMS_FILE}..." >&2
  checksums="$(cat "${CHECKSUMS_FILE}")"
else
  echo "Fetching checksums for ${version}..." >&2
  checksums="$(curl -fsSL "${checksums_url}")"
fi

lookup_sha() {
  local artifact="$1"
  awk -v name="${artifact}" '$2 == name { print $1; found = 1; exit } END { if (!found) exit 1 }' <<< "${checksums}"
}

sha_darwin_arm64="$(lookup_sha "veil-${version}-darwin-arm64.tar.gz")"
sha_darwin_amd64="$(lookup_sha "veil-${version}-darwin-amd64.tar.gz")"
sha_linux_arm64="$(lookup_sha "veil-${version}-linux-arm64.tar.gz")"
sha_linux_amd64="$(lookup_sha "veil-${version}-linux-amd64.tar.gz")"

for sha in "${sha_darwin_arm64}" "${sha_darwin_amd64}" "${sha_linux_arm64}" "${sha_linux_amd64}"; do
  if [[ -z "${sha}" ]]; then
    echo "error: one or more SHA256 values missing from checksums.txt" >&2
    exit 1
  fi
done

ver="${version#v}"

sed \
  -e "s/version \"[^\"]*\"/version \"${ver}\"/" \
  -e "s/PLACEHOLDER_DARWIN_ARM64/${sha_darwin_arm64}/" \
  -e "s/PLACEHOLDER_DARWIN_AMD64/${sha_darwin_amd64}/" \
  -e "s/PLACEHOLDER_LINUX_ARM64/${sha_linux_arm64}/" \
  -e "s/PLACEHOLDER_LINUX_AMD64/${sha_linux_amd64}/" \
  "${template}"
