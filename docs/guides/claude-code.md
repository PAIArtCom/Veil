# Guide: Claude Code

Use this guide when you want Claude Code traffic to pass through Veil's local
de-identification proxy.

**Supported path:** Claude Code using Anthropic Messages (`/v1/messages`) through
`ANTHROPIC_BASE_URL`. Credentials pass through unchanged; Veil rewrites supported
request/response body fields only.

## Prerequisites

- Go installed for source builds.
- Claude Code already installed and authenticated.
- A local shell where you can set `ANTHROPIC_BASE_URL`.

## 1. Build Veil

From the repository root:

```sh
go build -o ./bin/veil ./cmd/veil
./bin/veil version
./bin/veil proxy --help
```

## 2. Start the local proxy

```sh
./bin/veil proxy --addr 127.0.0.1:8788
```

Notes:

- `--upstream` defaults to `https://api.anthropic.com`.
- The proxy refuses non-loopback listen addresses.
- On first run, Veil creates a local HMAC key at `~/.veil/key` with restrictive file
  permissions.
- Add `--policy /path/to/policy.json` when you want local per-type `token`, `ignore`, or
  `block` behavior.

## 3. Point Claude Code at Veil

In the shell where you start Claude Code:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

Authentication stays with Claude Code:

- API-key users keep using the normal `x-api-key` flow.
- Pro, Max, or subscription users keep using the normal `Authorization: Bearer ...`
  OAuth flow.

Veil forwards those headers without storing or interpreting them.

## 4. Verify the path

Use a throwaway secret, not a real credential. For example, ask Claude Code to perform a
local task using:

```text
postgresql://app:s3cr3t@localhost:5432/mydb
```

Expected result:

- provider-bound protected text/tool fields contain `PAIArtVeil_...` tokens;
- local tool calls receive the restored connection string;
- files written locally do not contain unresolved `PAIArtVeil_` tokens;
- proxy logs do not print credentials or raw request bodies.

## Troubleshooting

| Symptom | Check |
|---|---|
| Claude Code bypasses Veil | Confirm `ANTHROPIC_BASE_URL=http://127.0.0.1:8788` is set in the same shell that launches `claude`. |
| Proxy refuses to start | Confirm `--addr` uses a loopback host such as `127.0.0.1`. |
| Request is blocked | Check whether the request uses an unsupported endpoint or a strict local policy selected `block`. |
| Tokens remain visible locally | Treat this as a bug or unsupported surface; see [Support](../../SUPPORT.md) and [Security policy](../../SECURITY.md). |
| Policy file is rejected | Remove unknown keys and use only `token`, `ignore`, or `block` operators in v0.1.0. |

## Known Limits

- Claude Code support covers Anthropic Messages (`/v1/messages`) only.
- Other Anthropic endpoints, such as `count_tokens`, fail closed until they are
  wire-aware.
- v0.1.0 protects text and tool I/O, not OCR, document parsing, attachment rewriting, or
  regenerated media/document payloads.
- Provider thinking/control traces keep provider-native semantics and are outside the
  masking contract.
- SSE framing assumes LF (`\n\n`), which matches Anthropic's emitted stream shape.
- Bedrock and Vertex egress paths are separate and out of scope for v0.1.0.

## Validation Evidence

The Claude Code path is live-accepted for v0.1.0. Maintainers can review the release
evidence in the [Phase 0 acceptance report](../architecture/phase-0-acceptance.md). The
proxy behavior is grounded in [ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md),
[ADR-0004](../architecture/decisions/0004-auth-pass-through.md), and
[ADR-0011](../architecture/decisions/0011-streaming-restore-cross-event-holdback.md).
