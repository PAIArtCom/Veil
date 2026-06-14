# Gateway Integration Survey

**Status:** Reference (evidence trail). Captured 2026-06.

This document records the source-code research behind OpenCloak's attach mechanism
([ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md)), the
engine/transport split ([ADR-0002](../architecture/decisions/0002-engine-transport-split.md)),
auth pass-through ([ADR-0004](../architecture/decisions/0004-auth-pass-through.md)), and the
general [SDK contract](../sdk/contract.md). It is the "why you can trust those decisions"
appendix.

## Methodology & sources

Direct reading of local checkouts. Two rounds: (1) how the target CLIs can be attached;
(2) how three real gateways expose insertion points (to derive the general contract).

| Subject | Source | Form |
|---|---|---|
| Claude Code | `claude-code-sourcemap/restored-src` (v2.1.88) | TS reconstructed from sourcemaps — strings/paths intact, identifiers minified |
| Codex CLI | `codex/codex-rs` | Real OpenAI Rust source |
| CLIProxyAPI | `CLIProxyAPI/` | Real Go source |
| PAIArt-Orbit | `PAIArt-Orbit/backend` | Real Go source (`github.com/paiart/orbit`) |
| clipal | `clipal/internal` | Real Go source |

---

## Part 1 — CLI attach findings

### Claude Code

- **Base URL:** the Anthropic SDK reads `ANTHROPIC_BASE_URL` (`@anthropic-ai/sdk` client),
  so `http://127.0.0.1:PORT` redirects standard `/v1/messages` traffic. Pointing off
  `api.anthropic.com` flips `isFirstPartyAnthropicBaseUrl()` false, which *disables* some
  beta request fields → a more standard payload behind a proxy.
- **No pinning / signing:** transport is `globalThis.fetch`; no cert pinning or
  response-signature validation found. Plain local HTTP works.
- **Auth:** chosen by `isClaudeAISubscriber()` — API-key users send `x-api-key`;
  OAuth/subscription users send `Authorization: Bearer`. Same header regardless of base
  URL; no host-bound signing → verbatim pass-through works.
- **Request shape:** standard Beta Messages API — `model`, `system[]` (typed blocks),
  `messages[].content`, `tools`. Text lives in `system[].text`, `messages[].content[].text`,
  and `tool_result.content`. (Leave the first attribution/fingerprint `system` block
  intact.)
- **Streaming / tool args:** raw SSE; tool-use `input_json_delta` is accumulated to a
  **complete** value, and the tool is dispatched only after the full assistant message is
  built → restoring tool args is a buffered operation, not a streaming one.
- **Hooks cannot rewrite the prompt:** the hook return schema exposes no field to edit
  `system`/`messages`. `UserPromptSubmit` only appends `additionalContext`; `PreToolUse`
  only mutates *local* tool input (`updatedInput`); `PostToolUse` only rewrites MCP tool
  *output*. **Conclusion: hooks are unusable for outbound prompt redaction.**

### Codex CLI

- **Provider config:** endpoint comes from a provider entry (`base_url`, `env_key`,
  `wire_api`). `WireApi` accepts only `responses` (Chat Completions removed → hard error).
  `http://` is allowed (no scheme validation).
- **WebSocket blind spot:** a config-defined provider's `supports_websockets` defaults
  false → forces HTTP+SSE (proxy-visible). `openai_base_url` alone keeps the built-in
  provider's WebSocket capability. **Use a custom `model_providers` entry.**
- **Auth:** all first-party auth kinds collapse to a header-only `BearerAuthProvider`
  (`Authorization: Bearer` + optional `ChatGPT-Account-ID`). No host/body signing, no cert
  pinning. (AWS Bedrock SigV4 is the exception — signs body+host.) Custom CA supported via
  `CODEX_CA_CERTIFICATE`.
- **Request shape:** Responses API — `POST {base}/responses`, `{model, instructions,
  input[], tools[], stream:true}`. Tool-call args complete in `response.output_item.done`
  before dispatch (buffered).
- **Hooks:** hook events and the `notify` command are out-of-process subprocesses; none can
  rewrite the LLM request/response body. `UserPromptSubmit` append-only; `PreToolUse`
  local-input only; `notify` fire-and-forget. **No in-process payload-rewrite seam.**

**Round-1 conclusion:** base-URL local proxy is the only viable attach for both; hooks are
ruled out for outbound rewrite.

---

## Part 2 — Gateway consumer integration surfaces

Three gateways, spanning richest → barest insertion surface.

### CLIProxyAPI (richest)

- **Plugin interceptor API** purpose-built for this: `InterceptRequestBeforeAuth` /
  `InterceptResponse` / `InterceptStreamChunk` — the last carries a bounded `HistoryChunks`
  buffer for cross-event state. Also a translator layer operating on raw JSON via
  gjson/sjson.
- **SSE** is parsed line-by-line and re-emitted (mutable per event).
- **In-tree precedent:** `internal/runtime/executor/helps/cloak_obfuscate.go` already walks
  `system` blocks + `messages[].content` and rewrites text — but with a *static word list*,
  *zero-width-space* obfuscation, and **outbound only** (no restore). This is exactly
  OpenCloak's outbound shape, half-built; OpenCloak's delta is two-layer detection +
  reversible deterministic tokens + inbound restore.
- **Auth lesson:** it *substitutes* its own upstream credentials (holds OAuth/keys). Its
  inbound auth is *open by default* if no key is set — a footgun OpenCloak avoids by
  passing credentials through and binding localhost.

### PAIArt-Orbit (middle)

- Gin-based gateway; protocol-neutral `InternalEvent` → `ProtocolWriter` bus.
- **Outbound seam:** `buildUpstreamRequest` / `planUpstreamAttempt` — raw replayable
  `[]byte` (+ parse-on-demand), provider-tagged, after auth resolution, before the
  `*http.Request` is built.
- **Inbound:** a real per-event hook exists — `Transform func(*SSEEvent) ([]*SSEEvent,…)`
  on the SSE relay — **but** the default same-protocol streaming path is **raw byte
  passthrough**. No plugin registry; insertion = editing hard-coded builders/relays.

### clipal (barest)

- **Outbound seam:** `requestPayload.providerBody()` (fed by `newRequestPayload(bodyBytes)`)
  — raw `[]byte` + lazy `map[string]any`, native provider JSON, no unified struct. **No
  middleware/hook/plugin abstraction** — hard-coded call sites.
- **Inbound:** bifurcated — Gemini-OAuth path parses SSE per event; the **dominant
  Anthropic/OpenAI path (`streamResponseToClient`) is opaque 32 KB byte-copy** with no event
  parsing. A restore step must be a stateful, chunk-spanning reassembler (tokens may split
  across arbitrary byte boundaries).
- **Provider abstraction:** none unified — per-format dispatch via `RequestContext.Family` +
  `Capability`; body stays native JSON.

---

## Part 3 — Synthesis: the four invariants

Across all three consumers:

1. **No unified payload schema** — all keep native provider JSON. → SDK dispatches by
   provider, operates on native JSON, never assumes an IR.
2. **Outbound is a buffered `[]byte` at one choke point, after auth/upstream selection** →
   SDK takes raw bytes; engine is credential-free.
3. **Streaming LCD = raw-chunk passthrough with arbitrary byte boundaries** (2 of 3
   default to it) → SDK must provide a stateful, byte-split-tolerant chunk restore, plus an
   optional event-level restore for parsed-SSE hosts.
4. **No host plugin registry can be assumed** (only 1 of 3 has one) → SDK is a pure-function
   library; integrators wire it at their own seams.

These invariants are the [SDK contract](../sdk/contract.md). The richest gateway
(CLIProxyAPI) can use the ergonomic event-level API; the barest (clipal) is fully served by
the universal chunk-level API — so a single contract covers the whole spectrum.
