# Guide: Claude Code

Use this guide when you want Claude Code traffic to pass through Veil's local
de-identification proxy.

**Supported path:** Claude Code using Anthropic Messages (`/v1/messages`) through
`ANTHROPIC_BASE_URL`. Credentials pass through unchanged; Veil rewrites supported
request/response body fields only.

## Prerequisites

- Node.js/npm for the recommended install path, or another release installer.
- Claude Code already installed and authenticated.
- Access to your Claude Code settings file at `~/.claude/settings.json`.

## 1. Install or build Veil

Use a release install for normal use:

```sh
npm i -g @paiart/veil
```

Or build from the repository root when testing a checkout:

```sh
go build -o ./bin/veil ./cmd/veil
./bin/veil version
```

## 2. Keep Veil running in the background

```sh
veil service install
veil status
```

Notes:

- `--upstream` defaults to `https://api.anthropic.com`.
- macOS uses a `launchd` user service; Linux uses `systemd --user`; Windows uses Task Scheduler.
- The service runs the same proxy and refuses non-loopback listen addresses.
- On first run, Veil creates a local HMAC key at `~/.veil/key` with restrictive file
  permissions.
- Add `--policy /path/to/policy.json` to `veil service install` when you want local
  per-type `token`, `ignore`, or `block` behavior.

Useful service commands:

```sh
veil status              # check the local proxy
veil restart             # restart after config changes
veil service stop        # stop the background proxy
veil service start       # start it again
veil service uninstall   # remove the OS service
```

## 3. Point Claude Code at Veil

For long-term use, configure Claude Code once in `~/.claude/settings.json`:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8787"
  }
}
```

For an Anthropic-compatible gateway, put the upstream directly in the local base URL:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8787/veil/upstream=https://your-gateway.example/api"
  }
}
```

Claude Code appends `/v1/messages`; Veil forwards that to
`https://your-gateway.example/api/v1/messages`. No URL escaping or extra upstream
command is required.

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
| Claude Code bypasses Veil | Confirm `~/.claude/settings.json` contains `env.ANTHROPIC_BASE_URL` and restart Claude Code. |
| Veil is not running | Run `veil status`, then `veil service install` or `veil restart`. |
| Need to remove the service | Run `veil service uninstall`; remove `ANTHROPIC_BASE_URL` from `~/.claude/settings.json` if you no longer want Claude Code to use Veil. |
| Proxy refuses to start | Confirm `--addr` uses a loopback host such as `127.0.0.1`. |
| Request is blocked | Check whether the request uses an unsupported endpoint or a strict local policy selected `block`. |
| Tokens remain visible locally | Treat this as a bug or unsupported surface; see [Support](../../SUPPORT.md) and [Security policy](../../SECURITY.md). |
| Policy file is rejected | Remove unknown keys and use only `token`, `ignore`, or `block` operators in v0.1.3. |

## Known Limits

- Claude Code support covers Anthropic Messages (`/v1/messages`) only.
- Other Anthropic endpoints, such as `count_tokens`, fail closed until they are
  wire-aware.
- v0.1.3 protects text and tool I/O, not OCR, document parsing, attachment rewriting, or
  regenerated media/document payloads.
- Provider thinking/control traces keep provider-native semantics and are outside the
  masking contract.
- SSE framing assumes LF (`\n\n`), which matches Anthropic's emitted stream shape.
- Bedrock and Vertex egress paths are separate and out of scope for v0.1.3.

## Validation Evidence

The Claude Code path is live-accepted for v0.1.3. Maintainers can review the release
evidence in the [Phase 0 acceptance report](../architecture/phase-0-acceptance.md). The
proxy behavior is grounded in [ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md),
[ADR-0004](../architecture/decisions/0004-auth-pass-through.md), and
[ADR-0011](../architecture/decisions/0011-streaming-restore-cross-event-holdback.md).
