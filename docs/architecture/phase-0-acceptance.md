# Phase 0 Acceptance Report

**Status:** Accepted for the standalone Claude Code proxy path.

**Date:** 2026-06-17 (Asia/Shanghai)

**Code under test:** `e8f37ee` (`fix(proxy,engine): harden Phase 0 live acceptance`)

## Scope

This report closes Phase 0 for the riskiest product path: Claude Code using the
standalone loopback proxy against real Anthropic traffic.

It does not claim Phase 1 surfaces: non-Anthropic providers, HTTP/gRPC service, web
console behavior, L2 NER, configurable rule sets, remote MCP egress classification, or
the secondary real-gateway embed validation.

## Live Run

Claude Code `2.1.178` was run through:

```sh
ANTHROPIC_BASE_URL=http://127.0.0.1:8788 claude -p \
  --no-session-persistence \
  --allowedTools Bash \
  --max-budget-usd 0.80 \
  --output-format json \
  'Use Bash exactly once to run this exact command: printf "%s\n" "<THROWAWAY_DSN>" > dsn_arg.txt && cat dsn_arg.txt. Then reply with exactly the single line printed by the command and nothing else.'
```

The OpenCloak proxy listened on `127.0.0.1:8788`. For this run only, its upstream was a
local pass-through capture proxy on `127.0.0.1:8789` that forwarded to
`https://api.anthropic.com` and recorded request/response bodies without recording
credential headers. Raw captures were inspected locally and deleted; they are not
committed.

The throwaway DSN was `postgresql://app:s3cr3t@localhost:5432/mydb`.

## Acceptance Matrix

| # | Criterion | Evidence |
|---|---|---|
| 1 | Model sees only tokens | Captured upstream `/v1/messages` requests had `has_real=no`; tokenized requests carried `CLK_...` values including `CLK_URL_2d74119f6479`. |
| 2 | Overlapping findings produce one correct token | The DSN was detected as a URL and consistently represented by one `CLK_URL_...` token across the live multi-turn request; resolver overlap fixtures remain covered by unit tests. |
| 3 | Tool-call arguments and tool results are restored | The live request sequence included `tool_use` and `tool_result` blocks; the local Bash command received the real DSN, while the follow-up provider request carried the DSN only as `CLK_URL_...`. |
| 4 | Local command executes with the real value | `dsn_arg.txt` existed after the run, contained the real DSN, and contained no `CLK_` token. |
| 5 | Provider-aware restore errors are visible | The live run completed without restore-error logs; automated proxy tests cover visible buffered and streaming restore-error paths. |
| 6 | Files on disk contain no `CLK_` tokens | The only file written by the tool, `dsn_arg.txt`, had `file_has_real=yes` and `file_has_CLK=no`. |
| 7 | Streamed tokens survive provider streaming | Captured decoded SSE contained `content_block_delta` and `input_json_delta` events; final Claude output had `claude_result_has_real=yes` and `claude_result_has_CLK=no`. |
| 8 | Second turn hits prompt cache | Claude Code result reported `num_turns=2` and `cache_read_input_tokens=60809`. |

## Issue Found During Acceptance

The first live tool-use run exposed a real proxy bug: Claude Code sent `Accept-Encoding`,
Anthropic returned gzip-compressed SSE, and the proxy attempted restore over compressed
bytes, producing a residual `CLK_URL_...` final answer.

The fix is in `e8f37ee`: `internal/proxy` strips client `Accept-Encoding` before the
upstream request so Go can manage response decompression and OpenCloak restores decoded
bytes. `TestStreamGzipResponseIsDecodedBeforeRestore` covers the regression.

## Final Verification

After the fix and successful live rerun:

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

All commands passed. Specability still reports the existing advisory warning that the
structured READMEs do not declare an optional `Adversarial Surfaces` section.

## Closing Judgment

Phase 0 is accepted for the standalone Claude Code proxy path. The remaining work belongs
to Phase 1 hardening or expansion, not Phase 0 acceptance.
