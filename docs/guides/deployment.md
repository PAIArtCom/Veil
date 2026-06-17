# Guide: Deployment & Operations

**Status: Baseline for v0.1.0 release hardening.** The standalone Claude Code proxy path
ships from source in the Phase 0 baseline; package-manager distribution and service
manager units remain planned.

## Run model

OpenCloak's standalone transport is a **long-lived local daemon**. Its process memory holds
scoped token↔value maps for tools pointed at it; each proxied request/stream gets an
explicit `State`, with optional longer-lived session/project namespaces for multi-turn
self-healing ([ADR-0009](../architecture/decisions/0009-state-lifecycle-and-scope.md)).

## Install from a clean checkout

```sh
git clone https://github.com/cloakia/opencloak.git
cd opencloak
go build -o ./bin/opencloak ./cmd/opencloak
./bin/opencloak version
./bin/opencloak proxy --help
```

For release artifacts, the expected binary paths are:

| Platform | Artifact path |
|---|---|
| macOS/Linux | `dist/opencloak_<version>_<os>_<arch>/opencloak` |
| Windows | `dist/opencloak_<version>_windows_<arch>/opencloak.exe` |

Release builds should inject version metadata with Go linker flags:

```sh
go build \
  -ldflags "-X main.version=v0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o ./dist/opencloak ./cmd/opencloak
```

## Run the Claude Code proxy

```sh
./bin/opencloak proxy --addr 127.0.0.1:8788
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

The proxy defaults to `https://api.anthropic.com` upstream. Use `--upstream` only for a
controlled local capture proxy or a compatible provider endpoint; do not commit raw
captures.

## Security invariants (non-negotiable)

- **Bind `127.0.0.1` only.** Never expose the proxy off-host without authentication — it
  would be an open relay. ([Threat model](../architecture/threat-model.md).)
- **Fail-closed.** On any detection/engine error, the request is blocked, not forwarded in
  the clear.
- **Credentials pass through.** The proxy forwards the tool's own auth header; it stores no
  provider credentials ([ADR-0004](../architecture/decisions/0004-auth-pass-through.md)).
- **Protect the local key.** `~/.opencloak/key` (the HMAC root) and any optional local map
  cache hold sensitive material; they are git-ignored and must not be backed up to shared
  storage.

## Release verification

Before cutting a release candidate, run the full local gate from the repository root:

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

## Configuration (planned surface)

- Listen address (implemented flag: `--addr`, default `127.0.0.1:8787`).
- Per-type transform operators; rule set selection.
- Optional local map cache (off by default; in-memory is the default).
- Key path (default `~/.opencloak/key`, generated on first run).

## Observability (planned)

- Local-only counters: requests processed, findings masked by type, blocked (fail-closed)
  requests, residual-token flags.
- Any aggregate reporting to a control plane (Cloakia) is opt-in and subject to
  audit-data minimization ([open-core boundary](../product/open-core-boundary.md)).

_This page will gain service-manager units and package-manager installation notes when
distribution packaging ships._
