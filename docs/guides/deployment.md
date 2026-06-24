# Guide: Deployment & Operations

**Status: Baseline for v0.1.0 release hardening.** The standalone proxy ships from
source for Claude Code and local Codex CLI Responses paths; package-manager distribution
and service manager units remain planned.

## Run model

Veil's standalone transport is a **long-lived local daemon**. Its process memory holds
scoped token↔value maps for tools pointed at it; each proxied request/stream gets an
explicit `State`, with optional longer-lived session/project namespaces for multi-turn
self-healing ([ADR-0009](../architecture/decisions/0009-state-lifecycle-and-scope.md)).

## Install from a clean checkout

```sh
git clone https://github.com/PAIArtCom/Veil.git
cd veil
go build -o ./bin/veil ./cmd/veil
./bin/veil version
./bin/veil proxy --help
```

For release artifacts, use the multi-platform release builder:

```sh
VERSION=v0.1.0 ./scripts/build-release.sh
./scripts/gen-checksums.sh dist/release > dist/release/checksums.txt
```

The builder injects `version`, `commit`, and `buildDate` with Go linker flags and writes
GitHub Release-ready binaries to `dist/release/`.

Expected release asset names:

| Platform | Artifact path |
|---|---|
| macOS | `dist/release/veil-<version>-darwin-amd64`, `dist/release/veil-<version>-darwin-arm64` |
| Linux | `dist/release/veil-<version>-linux-amd64`, `dist/release/veil-<version>-linux-arm64` |
| Windows | `dist/release/veil-<version>-windows-amd64.exe`, `dist/release/veil-<version>-windows-arm64.exe` |
| Checksums | `dist/release/checksums.txt` |

## Run the Claude Code proxy

```sh
./bin/veil proxy --addr 127.0.0.1:8788
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

The proxy defaults to `https://api.anthropic.com` upstream. Use `--upstream` only for a
controlled local capture proxy or a compatible provider endpoint; do not commit raw
captures.

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
regression gate for the shipped proxy path; use the runbook in
[Guide: Claude Code](claude-code.md) and record only sanitized summaries.

## Configuration

- Listen address (implemented flag: `--addr`, default `127.0.0.1:8787`).
- Upstream provider base URL (implemented flag: `--upstream`, default `https://api.anthropic.com`).
- Local policy file (implemented flag: `--policy`; env `VEIL_POLICY`; default path `~/.veil/policy.json` if present).
- Per-type `token`, `ignore`, and `block` operators (implemented); `redact`, `format_preserving`, and rule-set selection are planned and fail closed.
- Optional local map cache (off by default; in-memory is the default).
- Key path (default `~/.veil/key`, generated on first run).

## Observability (planned)

- Local-only counters: requests processed, findings masked by type, blocked (fail-closed)
  requests, residual-token flags.
- Any aggregate reporting to a control plane (PAIArt) is opt-in and subject to
  audit-data minimization ([open-core boundary](../product/open-core-boundary.md)).

_This page will gain service-manager units and package-manager installation notes when
distribution packaging ships._
