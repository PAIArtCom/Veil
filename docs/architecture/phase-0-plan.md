# Phase 0 Implementation Plan — The Engine Loop

**Status:** Draft (plan for review). Scope source: [roadmap Phase 0](../product/roadmap.md) +
[system-design Phase 0 cut](system-design.md). No code is written against this plan until it
is confirmed.

## Goal

Prove the riskiest assumption: the **mask → forward → restore** loop survives a real Claude
Code agent end-to-end — no broken tool calls, no busted prompt cache, no leaked or orphaned
tokens. Nothing else in Phase 0 matters more than this.

## Exit criteria (from the roadmap)

A real task ("use my local DB connection string to run a migration") through Claude Code where:

1. the model only ever sees deterministic `CLK_<TYPE>_<id>` tokens;
2. overlapping findings produce one correct token;
3. tool-call arguments and tool results are restored to real values;
4. the local command executes with the real value;
5. provider-aware restore errors are visible (not swallowed);
6. files on disk contain no `CLK_` tokens;
7. streamed tokens survive arbitrary byte-boundary splits;
8. a second turn hits the provider prompt cache.

## Approach

- **Riskiest-first.** Two cheap de-risking spikes before the bottom-up build — they kill the
  two biggest unknowns (the integration premise and the streaming algorithm) and produce real
  fixtures the milestones reuse.
- **Fixtures are the completion gate.** ADR-0008/0009/0010 make fixtures mandatory; a milestone
  is not done until `go test ./...` covers its listed fixtures and `go vet`/`gofmt` stay clean.
- **Scope guard.** L1-only detection, Anthropic wire only. Everything else is Phase 1 (see
  Out of scope). Resist creep.
- **Fail-closed throughout.** Any masking/detection error blocks; never forward plaintext.

## Spikes — de-risk first (throwaway code)

**Spike A — integration reality check.** Point real Claude Code at a throwaway local
pass-through proxy (`ANTHROPIC_BASE_URL=http://127.0.0.1:PORT`, no masking). Confirm (a) the
base-URL route works with the user's real auth (API-key and/or OAuth header pass-through), and
(b) capture a real `/v1/messages` request and its streaming SSE response, including a turn with
`tool_use` and a follow-up `tool_result`.
→ *Output:* recorded request/response/SSE fixtures + confirmation of where text actually lives
(`system[].text`, `messages[].content[].text`, `tool_use.input`, `tool_result.content`) and
which SSE event types appear. Kills the risk of building the wire walker against assumed shapes.

**Spike B — streaming holdback algorithm.** Prototype the chunk-level restorer that holds back a
partial token spanning arbitrary byte boundaries, plus `FlushStream`. Prove on adversarial
fixtures (split mid-`CLK_`, mid-hex, across three chunks; a `CLK_`-looking string not in the map).
→ *Output:* the validated algorithm + fixtures for M3.

## Milestones

### M1 — Text engine
**Packages:** `internal/token`, `internal/mapstore`, `internal/detect/l1`,
`internal/detect/resolver`, `internal/mask`; wires `Engine.Mask` / `Engine.Restore`.

**Build:**
- `token` — `CLK_<TYPE>_<id>` derivation (`HMAC-SHA256(normalize(value), local_key)`, first 12
  hex), per-type `normalize`, collision check-and-extend; `local_key` load/generate at
  `~/.opencloak/key` (0600).
- `mapstore` — in-memory token↔value store keyed by `Scope` namespace; backs `State`.
- `detect/l1` — starter detectors: regex rule set (merged privacy-filter + gitleaks subset,
  `go:embed`), Shannon entropy + context keywords, Luhn; each emits
  `Finding{Start,End,Type,Score,Source}`.
- `detect/resolver` — drop invalid, same-type merge, cross-type precedence (score → length →
  start) per [ADR-0008](decisions/0008-finding-model-and-conflict-resolution.md).
- `mask` — offset-safe replacement; applies the per-type `TransformOperator`
  (`token` / `ignore` / `block`→`ErrBlocked`+`BlockedError`); writes mappings into `State`.

**DoD:** `Mask(ctx,scope,text)` and `Restore(ctx,st,text)` round-trip secrets + structured PII;
deterministic; fail-closed; `Restore` rejects nil `State`.

**Fixtures:** token determinism + identifier-safety + collision-extend; resolver
overlap/containment/same-type-merge/cross-type-precedence/invalid-range; entropy
true-secret-near-keyword vs false-positive (base64 blob, hash); Luhn confirm/reject; scope
isolation (same value in two scopes restores only within its scope); `OperatorBlock` →
`BlockedError{Types}`.

### M2 — Anthropic wire (buffered)
**Packages:** `internal/wire`, `internal/wire/anthropic`; wires `Engine.MaskRequest` /
`Engine.RestoreResponse`.

**Build:** the Anthropic `Provider` — `ExtractRequest` over `system[].text`,
`messages[].content[].text`, `tool_use.input`, `tool_result.content`; `ApplyRequest` puts masked
text back preserving native JSON; `RestoreResponse` walks response text + `tool_use` and restores
via `State`. `MaskRequest` records provider/op in `State`.

**DoD:** a real captured request (Spike A) is masked so the model sees only tokens (including
tool-call args); a buffered response is restored; provider/op dispatch via `State`;
`ErrInvalidState` on nil/incomplete `State`.

**Fixtures:** Spike-A payloads; a tool-use turn (args masked → restored); prompt-cache prefix
stability (identical input → byte-identical masked prefix across two calls).

### M3 — Streaming restore
**Packages:** `internal/stream`; wires `Engine.RestoreStreamChunk` / `FlushStream` /
`RestoreSSEEvent`.

**Build:** the chunk-level holdback restorer (Spike B) + the provider-aware SSE-event restorer
(dispatches via `State`, restores only text fields). Residual (un-mappable) tokens pass through
and emit `AuditEvent{Kind:"residual_token"}`.

**DoD:** a streamed response restores correctly with tokens split across arbitrary byte
boundaries; `FlushStream` drains the tail; the SSE-event path leaves non-text fields untouched.

**Fixtures:** split-boundary cases (mid-`CLK_`, mid-hex, three-way); a real SSE stream (Spike A);
residual-token → emitted as-is + audit event.

### M4 — Standalone proxy + end-to-end validation
**Packages:** `internal/proxy`, `cmd/opencloak` (`proxy` subcommand).

**Build:** base-URL local proxy — binds `127.0.0.1` only, forwards the client credential
unchanged, exposes the Claude Code `/v1/messages` endpoint, masks outbound, threads `State`
across the streaming response lifetime, restores inbound, fail-closed on engine error.

**DoD (primary — exit criteria):** run real Claude Code with `ANTHROPIC_BASE_URL` at the proxy
and pass all eight exit criteria above.

**DoD (secondary — embeddable path):** embed the engine in one real gateway (candidate:
`clipal` — Go, already has base-URL takeover + SSE relay) per the
[SDK contract](../sdk/contract.md), proving the library form behaves identically. May trail the
standalone validation.

## Open implementation decisions (resolve as each milestone is reached)

- **L1 rule sourcing** — which privacy-filter + gitleaks rules to embed; both are MIT, so
  embedding/porting is fine; format under `go:embed` (JSON/TOML); start with a high-value subset,
  not all 200+ rules.
- **Entropy threshold + context-keyword list** — start conservative; tune against fixtures to
  balance recall vs false positives.
- **Token mechanics** — per-type `normalize` rules; collision check-and-extend; `local_key` = 32
  random bytes, file perms 0600, path override via `Config.KeyPath`.
- **Proxy `State` lifecycle** — scope derivation (single-user local → a fixed default scope is
  fine for Phase 0; session/tenant left to embedders); how `State` is held across the async
  streaming response and released after `FlushStream`; concurrency for parallel requests.
- **Validation gateway** — confirm `clipal` as the embed target, or defer the embed validation to
  early Phase 1.

## Out of scope (Phase 1+)

L2 NER (semantic PII), non-Anthropic providers (Codex/OpenAI/Gemini), HTTP/gRPC service, the web
console beyond a status view, remote-MCP egress, and `OperatorRedact` / `OperatorFormatPreserving`
behavior (the enum stays; only `token` / `ignore` / `block` are implemented in Phase 0).

## Tracking

Milestones are checked off here as their DoD + fixtures pass:

- [~] Spike A — integration reality check — *not run as a standalone throwaway; the
  wire walker was built against the documented Anthropic `/v1/messages` shapes, and the
  real-Claude-Code capture is folded into the M4 manual acceptance runbook.*
- [x] Spike B — streaming holdback algorithm — *folded into M3: the chunk-level holdback
  restorer was built and validated against the adversarial split fixtures.*
- [x] M1 — Text engine — `fa2d455`
- [x] M2 — Anthropic wire (buffered) — `ae157c7`
- [x] M3 — Streaming restore — `858dd58`
- [ ] M4 — Standalone proxy + end-to-end validation
