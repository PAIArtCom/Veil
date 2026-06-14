# SDK Integration Contract

**Status:** Accepted (contract); API surface is **Draft** until the engine lands.

OpenCloak's engine is consumed as a **general-purpose library**, not a component tailored
to any one host. This document defines the contract every integration relies on, and the
evidence that shaped it.

## Design principle

> Design to the contract; validate against several real consumers; couple to none.

We studied three real gateways — CLIProxyAPI, PAIArt-Orbit, and clipal — to find the
**lowest common denominator** an embeddable redaction engine must serve. They were
validation references, not design targets. Full findings:
[gateway integration survey](../research/gateway-integration-survey.md).

## What the survey showed (the spectrum)

| Aspect | CLIProxyAPI | PAIArt-Orbit | clipal |
|---|---|---|---|
| Hook / plugin abstraction | Full interceptor (req/resp/stream-chunk) | Per-event `Transform(*SSEEvent)` + neutral event bus; no plugin registry | **None** — hard-coded seams |
| Outbound body shape | Native JSON bytes | Native JSON bytes (+ tag) | Native JSON bytes (+ lazy map) |
| Provider normalization | Per-format (translation matrix) | Routing metadata only; payload per-format | None — per-format |
| Streaming inbound | Parsed SSE, re-emitted | Mixed; **default same-protocol = raw byte passthrough** | Mixed; **dominant path = raw 32 KB passthrough** |

The three span "richest" to "barest." The contract is pinned to the barest.

## The four invariants → the contract

1. **No unified payload schema.** All three keep the body in *native provider JSON*
   (Anthropic Messages, OpenAI Responses/Chat, Gemini). → The SDK dispatches by provider
   and operates on native JSON; it **never assumes an internal representation**. It offers
   per-family wire-aware helpers plus a raw text fallback.
2. **Outbound is a buffered `[]byte` at one choke point, after auth/upstream selection.**
   → The outbound API takes raw bytes; the engine is **credential-free** (it never sees
   auth — fits [pass-through](../architecture/decisions/0004-auth-pass-through.md)).
3. **Streaming's lowest common denominator is raw-chunk passthrough with arbitrary byte
   boundaries** (2 of 3 default to this). → The SDK **must** provide a stateful streaming
   restore that tolerates a token split across chunks; it **also** offers an event-level
   restore for gateways that already parse SSE.
4. **No host plugin registry can be assumed** (only 1 of 3 has one). → The SDK is a
   **library of pure functions**. Integrators call it at their own seams; wiring is their
   (small) job.

## The layered API

```go
package opencloak

// L0 — text level (universal fallback; any integrator can use it)
func Mask(text string) string
func Restore(text string) string

// L1 — wire-format level (per provider; operates on native JSON bytes)
func MaskRequest(provider, op string, body []byte) (masked []byte, st *State, err error)
func RestoreResponse(st *State, body []byte) []byte            // buffered / non-stream

// L2 — streaming level
func RestoreStreamChunk(st *State, chunk []byte) []byte        // stateful; byte-split tolerant — UNIVERSAL
func FlushStream(st *State) []byte                             // emit any held-back tail
func RestoreSSEEvent(st *State, eventData []byte) []byte       // ergonomic; for parsed-SSE hosts
```

- **State.** A process-global store is the default (zero bookkeeping; correct by the
  [completeness guarantee](../concepts/redaction-model.md) + determinism). An explicit
  `*State` handle is available for hosts that need per-request or per-tenant isolation.
- **Provider tag.** `provider`/`op` select the wire-aware walker (which JSON paths hold
  text). Every surveyed gateway can supply this tag.
- **Choosing a streaming method.** Use `RestoreStreamChunk` if you relay raw bytes (clipal,
  Orbit default). Use `RestoreSSEEvent` if you already parse SSE events (CLIProxyAPI,
  Orbit's transform path). Both share the same `State`.

See the [integration guide](integration-guide.md) for wiring patterns and the
[API reference](api-reference.md) for the full proposed surface.
