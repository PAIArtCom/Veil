# Guide: Deployment & Operations

**Status: v0.1.0 operations baseline.** The standalone proxy ships from source and
release binaries for Claude Code and local Codex CLI Responses paths. Package-manager
installers are release-channel conveniences backed by the same GitHub Release assets;
service manager units remain planned.

## Run model

Veil's standalone transport is a **long-lived local daemon**. Its process memory holds
scoped token↔value maps for tools pointed at it; each proxied request/stream gets an
explicit `State`, with optional longer-lived session/project namespaces for multi-turn
self-healing ([ADR-0009](../architecture/decisions/0009-state-lifecycle-and-scope.md)).

## Install

v0.1.0 supports source builds, release binaries, curl/PowerShell installers, and npm
distribution backed by release assets. Homebrew formula generation is automated for stable
tags and publishes to `PAIArtCom/homebrew-veil` when `HOMEBREW_TAP_REPO` is set to that
repository and `HOMEBREW_TAP_TOKEN` is configured.

| Method | Use when | Steps |
|---|---|---|
| curl / PowerShell installer | You want the shortest end-user install path | Run the platform installer; it downloads the matching release binary and verifies `checksums.txt`. |
| npm | You are in a Node.js toolchain | Install `@paiart/veil`; postinstall downloads and verifies the matching release binary. |
| Homebrew | You use Homebrew on macOS or Linux | `brew tap PAIArtCom/veil`, then `brew install veil`. |
| Source build | You have Go installed or want to verify from source | Clone, build `./cmd/veil`, run `veil version`. |
| Release binary | You want the smallest end-user install path | Download the asset for your OS/architecture, verify its checksum, put it on `PATH`. |

### Source build

```sh
git clone https://github.com/PAIArtCom/Veil.git
cd Veil
go build -o ./bin/veil ./cmd/veil
./bin/veil version
./bin/veil proxy --help
```

### Release binary

Download the asset that matches your platform from the GitHub Release:

| Platform | Asset |
|---|---|
| macOS Intel | `veil-<version>-darwin-amd64` |
| macOS Apple Silicon | `veil-<version>-darwin-arm64` |
| Linux x86_64 | `veil-<version>-linux-amd64` |
| Linux ARM64 | `veil-<version>-linux-arm64` |
| Windows x86_64 | `veil-<version>-windows-amd64.exe` |
| Windows ARM64 | `veil-<version>-windows-arm64.exe` |

Verify the checksum before running it:

```sh
shasum -a 256 veil-<version>-darwin-arm64
grep veil-<version>-darwin-arm64 checksums.txt
```

Then install it somewhere on your `PATH`, for example:

```sh
mkdir -p ~/.local/bin
mv veil-<version>-darwin-arm64 ~/.local/bin/veil
chmod 0755 ~/.local/bin/veil
veil version
```

### Build release artifacts

Maintainers can build multi-platform release artifacts locally:

```sh
VERSION=v0.1.0 ./scripts/build-release.sh
./scripts/gen-checksums.sh dist/release > dist/release/checksums.txt
```

The builder injects `version`, `commit`, and `buildDate` with Go linker flags and writes
GitHub Release-ready binaries to `dist/release/`.

Expected release asset names:

| Platform | Artifact path |
|---|---|
| macOS | `dist/release/veil-<version>-darwin-amd64`, `dist/release/veil-<version>-darwin-arm64`, and matching `.tar.gz` archives |
| Linux | `dist/release/veil-<version>-linux-amd64`, `dist/release/veil-<version>-linux-arm64`, and matching `.tar.gz` archives |
| Windows | `dist/release/veil-<version>-windows-amd64.exe`, `dist/release/veil-<version>-windows-arm64.exe`, and matching `.zip` archives |
| Checksums | `dist/release/checksums.txt` |

## Upgrade

Stop any running proxy, replace the `veil` binary, then start the proxy again. The local
HMAC key at `~/.veil/key` is not regenerated during a normal upgrade.

```sh
veil version
pkill -f "veil proxy" || true
# replace the binary using your install method
veil version
```

## Uninstall

Remove the binary from your `PATH`. If you also want to remove local Veil state, delete
`~/.veil/key` and any local policy file you created. Deleting the key prevents old tokens
from being restored.

## Run the Claude Code proxy

```sh
veil proxy --addr 127.0.0.1:8788
claude
```

For normal use, put `ANTHROPIC_BASE_URL=http://127.0.0.1:8788` in
`~/.claude/settings.json` under `env`. A temporary shell export is acceptable only for a
one-off release smoke test. The proxy defaults to `https://api.anthropic.com` upstream.
Use `--upstream` only for a controlled local capture proxy or a compatible provider
endpoint; do not commit raw captures.

## Local policy

Veil can load a local policy JSON file for single-user per-type behavior. Precedence:

1. `veil proxy --policy /path/to/policy.json`
2. `VEIL_POLICY=/path/to/policy.json`
3. `~/.veil/policy.json` if the file exists
4. Built-in defaults if no configured/default policy file exists

Minimal safe config:

```json
{
  "default_operator": "token",
  "types": {
    "EMAIL": {"operator": "ignore"},
    "SECRET": {"operator": "block"}
  }
}
```

Supported v0.1.0 operators are `token`, `ignore`, and `block`. `redact`,
`format_preserving`, and non-empty `rule_sets` are reserved and fail closed. Unknown keys
also fail closed, including `comment`, `label`, `metadata`, provider labels, analytics
labels, customer labels, raw payload references, dotenv paths, or secret-looking values.
If `--policy` or `VEIL_POLICY` points to a missing or invalid file, the proxy refuses
to start. If the default file is absent, the proxy uses the built-in policy.

## Security invariants (non-negotiable)

- **Bind `127.0.0.1` only.** Never expose the proxy off-host without authentication — it
  would be an open relay. ([Threat model](../architecture/threat-model.md).)
- **Fail-closed.** On any detection/engine error, the request is blocked, not forwarded in
  the clear.
- **Credentials pass through.** The proxy forwards the tool's own auth header; it stores no
  provider credentials ([ADR-0004](../architecture/decisions/0004-auth-pass-through.md)).
- **Protect the local key.** `~/.veil/key` (the HMAC root) and any optional local map
  cache hold sensitive material; they are git-ignored and must not be backed up to shared
  storage.

## Release verification

Before cutting or validating a release, run the full local gate from the repository root:

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
gofmt -l .
git diff --check
specability validate .
specability validate docs
specability validate docs/architecture/decisions
specability scan --json
specability reconcile . --json
specability reconcile docs --json
specability reconcile docs/architecture/decisions --json
```

`gofmt -l .` must print no file names. Live Claude Code acceptance remains the manual
regression gate for the shipped proxy path; use the criteria in
[the Phase 0 acceptance report](../architecture/phase-0-acceptance.md) and record only
sanitized summaries.

## Configuration

- Listen address (implemented flag: `--addr`, default `127.0.0.1:8787`).
- Upstream provider base URL (implemented flag: `--upstream`, default `https://api.anthropic.com`).
- Local policy file (implemented flag: `--policy`; env `VEIL_POLICY`; default path `~/.veil/policy.json` if present).
- Per-type `token`, `ignore`, and `block` operators (implemented); `redact`, `format_preserving`, and rule-set selection are planned and fail closed.
- Optional local map cache (off by default; in-memory is the default).
- Key path (default `~/.veil/key`, generated on first run).

## Troubleshooting

| Symptom | Check |
|---|---|
| Proxy refuses to bind | Use a loopback address such as `127.0.0.1:8788`; non-loopback addresses are rejected. |
| Tool bypasses Veil | Confirm the tool-specific base URL points at Veil: `ANTHROPIC_BASE_URL` for Claude Code or custom `model_providers` for Codex. |
| Policy file blocks startup | Remove unknown keys and unsupported operators; v0.1.0 supports only `token`, `ignore`, and `block`. |
| Requests are blocked | The request may use an unsupported endpoint or unsupported provider JSON shape. This is fail-closed behavior. |
| Tokens appear in local files | Treat this as a bug or unsupported surface; see [Support](../../SUPPORT.md) and [Security policy](../../SECURITY.md). |

## Observability (planned)

- Local-only counters: requests processed, findings masked by type, blocked (fail-closed)
  requests, residual-token flags.
- Any aggregate reporting to a control plane (PAIArt) is opt-in and subject to
  audit-data minimization ([open-core boundary](../product/open-core-boundary.md)).

_This page will gain service-manager units when those become part of the shipped release
scope._
