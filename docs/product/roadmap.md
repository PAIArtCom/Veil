# Roadmap

**Status:** Accepted (sequencing); phase contents will be refined as work proceeds.

The guiding rule: **prove the riskiest assumption first.** The riskiest assumption is not
the SaaS, the dashboards, or broad PII coverage — it is that the end-to-end
**mask → forward → restore** loop survives a real agent (no broken tool calls, no busted
caches, no orphaned tokens). Everything else is downstream.

## Phase 0 — MVP: the engine loop

**Goal:** prove the loop end-to-end.

- Engine library: **L1-only** detection (secrets + structured PII — no NER model yet).
- Deterministic, reversible, type-aware [token spec](../concepts/token-spec.md).
- Finding conflict resolution (same-type merge, cross-type precedence) before masking.
- Scoped `State` with provider/op metadata for provider-aware restore.
- Per-type transform operators in the public policy shape. Phase 0/v0.1.0 executes only
  `token`, `block`, and `ignore`; `format_preserving` and `redact` are reserved and fail
  closed until their semantics are designed and tested.
- Mask/restore over text, over the Anthropic wire format, and **stateful streaming
  restore** (tolerating tokens split across arbitrary byte boundaries).
- Validate with the standalone Claude Code proxy path and a maintained SDK reference
  integration outside the proxy.

**Exit criteria:** a real task ("use my local DB connection string to run a migration")
runs through Claude Code where the model only ever sees deterministic tokens, overlapping
findings produce one correct token, tool results and tool-call arguments are restored to
real values, provider-aware restore errors are visible, the local command executes with the
real value, files on disk contain no `CLK_` tokens, streamed tokens survive arbitrary byte
splits, and a second turn hits the prompt cache.

**Milestones** — detailed plan in [phase-0-plan.md](../architecture/phase-0-plan.md):

- **Spikes** — integration reality-check (real Claude Code → local proxy; capture wire + SSE shapes) and the streaming split-token holdback algorithm.
- **M1 — Text engine.** `token` · `mapstore` · `detect/l1` · `resolver` · `mask` → `Mask`/`Restore` over text, with fixtures.
- **M2 — Anthropic wire (buffered).** `wire/anthropic` + `MaskRequest`/`RestoreResponse` against real captured payloads.
- **M3 — Streaming restore.** chunk-level holdback + SSE-event; byte-split fixtures.
- **M4 — Proxy + end-to-end.** standalone Claude Code proxy; pass all exit criteria.

**Status (accepted for Claude Code proxy).** M1–M4 are implemented and `gofmt`/`vet`/`go test`/`-race` green: the mask → forward → restore loop runs end-to-end over text, the buffered Anthropic wire, and streaming — including reassembly of tokens the model regenerates split across SSE events ([ADR-0011](../architecture/decisions/0011-streaming-restore-cross-event-holdback.md)) — fail-closed, behind a loopback-only base-URL proxy. The eight exit criteria are covered by the test suite and were live-accepted against real Claude Code traffic on 2026-06-17 ([acceptance report](../architecture/phase-0-acceptance.md)). The SDK embed contract is now covered by the maintained `examples/embed` reference integration.

## Phase 1 — Ecosystem

**Goal:** breadth and hardening.

The release-oriented execution plan is tracked in
[formal-release-plan.md](../architecture/formal-release-plan.md). That plan defines the
first formal OSS developer release (`v0.1.0`) as a narrower release cut than all possible
Phase 1 ecosystem breadth.

- Codex support (OpenAI Responses API) is implemented with offline verification and still
  needs live acceptance. OpenAI Chat and Gemini remain later provider adapters, followed
  by a decision on whether a public third-party adapter registration API is worth freezing.
- HTTP/gRPC service wrapper for non-Go gateways.
- Optional **L2** local NER layer (semantic PII: names, addresses) — opt-in, run only on
  per-turn new content to bound latency. Person detection default-off, user-configurable.
- Local configuration surface for per-type `token`/`ignore`/`block` is implemented;
  configurable rule packs remain later. Continue hardening the
  [integration guide](../sdk/integration-guide.md) for third-party gateways.

## Phase 2 — Cloakia (commercial)

**Goal:** monetize organizational control.

- Control plane: central policy authoring and push to a client fleet.
- Compliance audit dashboards (built under the audit-data-minimization constraint).
- SSO / RBAC, central entity dictionaries, SIEM export.
- Commercial packaging (SaaS and/or sold-license self-hosted — kept open per the
  [strategy](strategy.md)).

## Explicitly later

- Remote MCP / remote-tool egress classification (a separate egress channel from the
  Messages/Responses APIs).
- Copilot / Cursor / additional tool adapters beyond the first two CLIs.
- Localized PII rule packs (per-country structured identifiers).
