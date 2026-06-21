#!/usr/bin/env bash
set -euo pipefail

# Generate SHA-256 checksums for release artifacts in a single directory.
#
# Usage:
#   ./scripts/gen-checksums.sh dist/release > dist/release/checksums.txt

dir="${1:-}"
if [[ -z "${dir}" || ! -d "${dir}" ]]; then
  echo "usage: $0 <artifact-dir>" >&2
  exit 2
fi

hash_cmd=()
if command -v sha256sum >/dev/null 2>&1; then
  hash_cmd=(sha256sum)
elif command -v shasum >/dev/null 2>&1; then
  hash_cmd=(shasum -a 256)
else
  echo "error: need sha256sum or shasum" >&2
  exit 1
fi

cd "${dir}"

while IFS= read -r -d '' file; do
  name="${file#./}"
  if [[ "${name}" == "checksums.txt" ]]; then
    continue
  fi
  "${hash_cmd[@]}" "${name}"
done < <(find . -maxdepth 1 -type f -print0 | sort -z)
