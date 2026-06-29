#!/usr/bin/env sh
# Veil installer — Linux and macOS
#
# Usage:
#   curl -fsSL https://veil.paiart.com/install.sh | sh
#
# Environment:
#   VEIL_VERSION     - specific version tag, e.g. v0.1.0 (default: latest)
#   VEIL_INSTALL_DIR - where to put the binary (default: /usr/local/bin or ~/.local/bin)
#   VEIL_DOWNLOAD_BASE - release asset base URL override for CI smoke tests

set -e

REPO="PAIArtCom/Veil"
BIN="veil"
RELEASES="https://github.com/${REPO}/releases"

# OS / arch
OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux"  ;;
  *)
    echo "error: unsupported OS: ${OS}" >&2
    exit 1
    ;;
esac

case "${ARCH}" in
  x86_64)         ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)
    echo "error: unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

# version
if [ -z "${VEIL_VERSION:-}" ]; then
  VEIL_VERSION="$(curl -fsSL \
    "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"([^"]+)".*/\1/')"
fi

if [ -z "${VEIL_VERSION:-}" ]; then
  echo "error: could not resolve latest version" >&2
  exit 1
fi

# install dir
if [ -z "${VEIL_INSTALL_DIR:-}" ]; then
  if [ -w "/usr/local/bin" ]; then
    VEIL_INSTALL_DIR="/usr/local/bin"
  else
    VEIL_INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "${VEIL_INSTALL_DIR}"
  fi
fi

ARTIFACT="veil-${VEIL_VERSION}-${OS}-${ARCH}"
DOWNLOAD_BASE="${VEIL_DOWNLOAD_BASE:-${RELEASES}/download/${VEIL_VERSION}}"
DOWNLOAD_URL="${DOWNLOAD_BASE}/${ARTIFACT}"
CHECKSUMS_URL="${DOWNLOAD_BASE}/checksums.txt"

printf 'Installing Veil %s (%s/%s) -> %s\n' \
  "${VEIL_VERSION}" "${OS}" "${ARCH}" "${VEIL_INSTALL_DIR}"

# download
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT INT TERM

curl -fsSL --progress-bar -o "${TMP}/${BIN}" "${DOWNLOAD_URL}"
curl -fsSL -o "${TMP}/checksums.txt" "${CHECKSUMS_URL}"

# checksum
EXPECTED="$(awk -v name="${ARTIFACT}" '$2 == name { print $1; found = 1; exit } END { if (!found) exit 1 }' "${TMP}/checksums.txt")"
if [ -z "${EXPECTED}" ]; then
  echo "error: no checksum entry for ${ARTIFACT}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${TMP}/${BIN}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "${TMP}/${BIN}" | awk '{print $1}')"
else
  echo "warning: sha256sum and shasum not found — skipping checksum" >&2
  ACTUAL="${EXPECTED}"
fi

if [ "${ACTUAL}" != "${EXPECTED}" ]; then
  printf 'error: checksum mismatch\n  expected: %s\n  got:      %s\n' \
    "${EXPECTED}" "${ACTUAL}" >&2
  exit 1
fi

chmod +x "${TMP}/${BIN}"

# install
DEST="${VEIL_INSTALL_DIR}/${BIN}"
if [ -w "${VEIL_INSTALL_DIR}" ]; then
  mv "${TMP}/${BIN}" "${DEST}"
else
  echo "sudo required to write to ${VEIL_INSTALL_DIR}"
  sudo mv "${TMP}/${BIN}" "${DEST}"
fi

printf '\nVeil %s installed to %s\n' "${VEIL_VERSION}" "${DEST}"

# PATH hint
case ":${PATH}:" in
  *:"${VEIL_INSTALL_DIR}":*) ;;
  *)
    printf '\nAdd %s to your PATH:\n  export PATH="%s:${PATH}"\n' \
      "${VEIL_INSTALL_DIR}" "${VEIL_INSTALL_DIR}"
    ;;
esac

echo ""
"${DEST}" version

cat <<'EOF'

Next steps:
  1. Start Veil in the background: veil service install
  2. Check it is running:          veil status
  3. Configure your AI tool base URL:
     Claude Code: http://127.0.0.1:8787
     Codex CLI:   http://127.0.0.1:8787/v1
     OpenRouter:  http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1

Guide: https://veil.paiart.com/#install
EOF
