# ADR-0011 — Streaming restore across SSE event boundaries

**Status:** Accepted

## Context

The model regenerates `CLK_<TYPE>_<id>` tokens in its *output*. Anthropic streams output as
`content_block_delta` events whose `delta.text` / `delta.partial_json` carry model-tokenizer
fragments, not semantic units. A single `CLK_…` token (≈30+ chars) therefore spans **multiple
delta events** — the split can land mid-`CLK_`, mid-TYPE, or mid-hex.

The Phase-0 streaming path restored each SSE event independently
([ADR-0010](0010-restore-dispatch-and-errors.md): `RestoreSSEEvent` is stateless; the proxy
frame-buffers only to reassemble one event split across TCP reads). That path sees an
*incomplete* token in each event, matches nothing, and re-emits the fragments verbatim. The
client reassembles a literal `CLK_…` string that is never resolved:

- a token split across `text_delta` events leaks into visible output;
- a token split across `input_json_delta` fragments reassembles into `tool_use.input`, so the
  local agent **executes the tool with a `CLK_…` literal instead of the real value** — silently.

This breaks Phase-0 exit criteria #3 (tool args restored), #4 (command runs with the real
value), and #7 (tokens survive split). The raw byte-level holdback (`internal/stream.Restorer`,
`RestoreStreamChunk`) does not help: in the raw SSE byte stream the fragments are separated by
JSON-string-close + SSE framing (`"}}\n\ndata: {…,"text":"`), so they are not contiguous and the
partial-suffix scan sees the first fragment as terminated.

Prior art (surveyed in the [gateway integration survey](../../research/gateway-integration-survey.md)):
`llm-shield` holds back partial placeholders per `text_delta` and flushes a synthetic delta — but
hardcodes `index: 0`, shares one buffer across blocks, and **never restores tool-call input** at
all. `litellm` and `llm-guard` punt by buffering/re-scanning the whole response (no incremental
delivery). No surveyed tool reassembles a reversible token into streaming tool-call JSON.

## Decision

Introduce a **stateful, per-stream SSE restorer** that holds cross-event state per content-block
`index`, driven by the proxy one complete event at a time, with a flush at end of stream. It is a
provider capability so non-Anthropic providers implement it later; per-block buffers live in the
restorer, **not** in `State` (which remains the reverse map of [ADR-0009](0009-state-lifecycle-and-scope.md)).

- **`wire.Provider` gains `NewStreamRestorer(op) (StreamRestorer, error)`.** `StreamRestorer.Event(eventData, restore) ([][]byte, error)` consumes one complete event payload and returns **zero or more** event payloads to emit (more when a held tail is flushed alongside a stop event; fewer when an `input_json_delta` is buffered). `Flush(restore) ([][]byte, error)` drains held tails at EOF. Providers without an implementation return `ErrStreamingUnsupported` (fail-closed).
- **`Engine.NewSSEStreamRestorer(st) (*SSEStream, error)`** wraps the provider walker with the scope-bound restore closure and residual-token auditing, mirroring the `RestoreStreamChunk`/`FlushStream` lifecycle. The proxy owns SSE framing (`event:`/`data:`/`\n\n`); the engine/provider exchange unframed payloads.
- **`text_delta`:** feed each event's decoded text to a per-block holdback (reuse `internal/stream.Restorer`); emit the safe restored prefix back into `delta.text` via `sjson` (escaping-correct); hold the partial-token tail; flush it at that block's `content_block_stop` (or `message_stop`) as a synthetic `text_delta`. Text stays incremental.
- **`input_json_delta`:** buffer the block's `partial_json` fragments (emit nothing); at `content_block_stop`, restore the *complete* reconstructed JSON's string leaves with the buffered-path logic (`restoreStringLeavesInto`, full-parse + `sjson`, escaping-correct), and emit **one** consolidated `input_json_delta` before the stop. Tools need complete input, so delaying to block-stop is correct and sidesteps mid-string escaping.
- All other events (`message_*`, `content_block_start`, `ping`, `error`, unknown types) pass through immediately and unchanged, preserving keepalive cadence and forward compatibility.

## Alternatives considered

- **Preserve per-event `input_json_delta` fragments with mid-string holdback.** Rejected: forces
  hand-rolled JSON-string escaping on partial input that is split mid-escape across events, for no
  benefit — tools cannot run on partial input. Consolidation reassembles for free and is
  escaping-correct by construction.
- **Make `RestoreSSEEvent` stateful via `State`.** Rejected: pollutes the reverse-map `State` with
  per-block streaming buffers and breaks the stateless contract other callers rely on. A separate
  object isolates the streaming lifecycle and keeps providers pure.
- **Buffer the whole response, restore, then replay (litellm/llm-guard model).** Rejected: kills
  incremental text delivery the agent UX depends on.
- **Restore tool input only (text-style holdback) on `input_json_delta` fragments.** Rejected:
  this is the llm-shield gap — mid-string and escaping hazards with no upside.

## Consequences

- New surface: `wire.StreamRestorer`, `wire.Provider.NewStreamRestorer`, `wire.ErrStreamingUnsupported`,
  `Engine.NewSSEStreamRestorer` returning `*SSEStream` with `Event(ctx, …) ([][]byte, error)` and
  `Flush(ctx) ([][]byte, error)`. The Anthropic provider implements it; the proxy's `relayStream`
  drives it (frame-buffer → `Event` → frame each returned payload; `Flush` at EOF).
- The stateless `RestoreSSEEvent` remains as a lower-level per-event primitive but is no longer the
  proxy's streaming path.
- Tool input is delivered as one consolidated `input_json_delta` at `content_block_stop` instead of
  incrementally — the one intentional behavioral change (a consumer rendering `partial_json`
  progressively sees one chunk, not a typing animation). The block skeleton is unchanged.
- **Deferred to Phase 1:** `thinking_delta`/`signature_delta` restore (passing thinking through
  unrestored is acceptable for Phase 0 — a residual token there is exposed only to the trusted
  local user and, if echoed back next turn, is still a masked token the API never sees in the
  clear; the residual audit counts it, so it is observable, not silent — and rewriting thinking
  would invalidate the Anthropic signature that covers it); CRLF/bare-CR SSE framing; non-Anthropic
  `StreamRestorer` implementations.
- Tests must split tokens **across logical events** (not just TCP byte boundaries): across 2–3
  `text_delta` events at every split position; across `input_json_delta` fragments; values with
  JSON-special characters; interleaved/multi-`index` blocks; `ping` mid-token; truncated-stream
  flush; residual reassembly; restore-error visibility through the 1→N path. The M4 proxy fake
  upstream must emit tokens split across events (today it never does).
