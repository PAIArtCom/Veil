# Guide: Claude Code

**Status: Ships in Phase 0.** The standalone proxy exists (`opencloak proxy`). The engine
loop (mask → forward → restore, buffered and streaming) is implemented and covered by tests;
this guide is also the **Phase 0 acceptance runbook** — the manual end-to-end check against
real Claude Code that confirms the eight exit criteria on live traffic.

Grounded in verified Claude Code behavior
([survey](../research/gateway-integration-survey.md),
[ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md)).

## How it works

Claude Code reads `ANTHROPIC_BASE_URL` through the Anthropic SDK and sends standard
`/v1/messages` requests there. OpenCloak runs a local proxy on `127.0.0.1`; you point Claude
Code at it. The proxy masks the outbound body, forwards to `api.anthropic.com` with your
credentials **unchanged**, and restores tokens in the response — including tokens the model
regenerates split across streaming events ([ADR-0011](../architecture/decisions/0011-streaming-restore-cross-event-holdback.md)).

## Setup

```sh
# 1. build the proxy
go build -o opencloak ./cmd/opencloak
./opencloak version
./opencloak proxy --help

# 2. start it (binds 127.0.0.1 only; refuses non-loopback addresses)
./opencloak proxy --addr 127.0.0.1:8788
#   --upstream defaults to https://api.anthropic.com
#   --policy may point to a strict local policy JSON file
#   on first run it generates a local HMAC key at ~/.opencloak/key (0600)

# 3. point Claude Code at it, in the same or another shell
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

Your authentication is untouched (the engine holds no credentials —
[ADR-0004](../architecture/decisions/0004-auth-pass-through.md)):

- **API-key users** — `x-api-key` is forwarded as-is.
- **Pro/Max/subscription (OAuth) users** — `Authorization: Bearer …` is forwarded as-is.

## Phase 0 acceptance checklist

Run a real task that puts a secret in front of the agent — the canonical one is **"use my
local Postgres connection string `postgresql://app:s3cr3t@localhost:5432/mydb` to run a
migration"** (use a throwaway value). Then confirm each criterion:

| # | Criterion | How to check |
|---|---|---|
| 1 | Model sees only tokens | Watch the proxy stderr / a capture (below): the outbound `/v1/messages` body carries `CLK_…`, never the secret. |
| 2 | Overlapping findings → one token | A value that matches two rules still yields a single consistent `CLK_…`. (Unit-covered by the resolver; visible if you craft an overlapping secret.) |
| 3 | Tool-call args + results restored | The tool the agent invokes receives the **real** connection string in its arguments, not a `CLK_…`. |
| 4 | Local command runs with the real value | The migration actually connects/runs (it would fail with a `CLK_…` host). |
| 5 | Restore errors are visible | Proxy logs any restore error at `ERROR`; the stream/response is not silently dropped. |
| 6 | No tokens on disk | Files the agent writes contain real values, never `CLK_…`. |
| 7 | Streamed tokens survive splits | Streamed assistant text and tool input render as real values even though the model emits the token across deltas. |
| 8 | Second turn hits prompt cache | A repeated identical prefix produces a byte-identical masked prefix (deterministic tokens); the provider reports a cache hit. |

Latest accepted run: 2026-06-17, recorded in the
[Phase 0 acceptance report](../architecture/phase-0-acceptance.md).

### Capturing a real fixture (recommended, the Spike-A capture)

To harden the regression suite with a *real* Anthropic request/response/SSE (including a
`tool_use` turn and its `tool_result` follow-up), capture traffic while running the task —
e.g. point `--upstream` at a small logging pass-through, or tee the proxy's upstream
request/response during a session — and add the recording as a fixture under
`internal/wire/anthropic`. This replaces the synthetic fixtures (built from the documented
wire shapes) with ground truth and pins the exact SSE event types Claude Code emits.

## Known Phase 0 limits

- **Anthropic `/v1/messages` only for Claude Code.** Other Anthropic endpoints (e.g.
  `count_tokens`) are forwarded transparently **without** masking in Phase 0; OpenAI
  Responses is documented separately for Codex, while OpenAI Chat/Gemini are Phase 1+.
- **Thinking is not restored** in Phase 0 — a regenerated token can surface in the
  (local-only) thinking trace; it is still a masked token if echoed back to the API, and the
  residual-token audit counts it ([ADR-0011](../architecture/decisions/0011-streaming-restore-cross-event-holdback.md)).
- **SSE framing assumes LF** (`\n\n`), which is what Anthropic emits; CRLF is Phase 1.
- No TLS pinning or response-signature checks block a local HTTP proxy (verified).
- Bedrock/Vertex egress paths are separate and out of scope for the MVP.
