# SDK Integration Contract

**Status:** Accepted (contract); the Phase 0 API surface is implemented and the standalone
Claude Code proxy path is live-accepted, while non-Anthropic providers and Phase 1
operators remain reserved.

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

The three range from "richest" to "barest." The contract is pinned to the barest.

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
   **small library API**. Integrators call it at their own seams; wiring is their (small)
   job.

## The three API surfaces

```go
package opencloak

import "context"

// Text surface — universal fallback; any integrator can use it.
func (e *Engine) Mask(ctx context.Context, scope Scope, text string) (masked string, st *State, err error)
func (e *Engine) Restore(ctx context.Context, st *State, text string) (string, error)

// Wire surface — per provider; operates on native JSON bytes.
func (e *Engine) MaskRequest(ctx context.Context, scope Scope, provider, op string, body []byte) (masked []byte, st *State, err error)
func (e *Engine) RestoreResponse(ctx context.Context, st *State, body []byte) ([]byte, error)   // buffered / non-stream

// Stream surface.
func (e *Engine) RestoreStreamChunk(st *State, chunk []byte) []byte        // stateful; byte-split tolerant — UNIVERSAL
func (e *Engine) FlushStream(st *State) []byte                             // emit any held-back tail
func (e *Engine) RestoreSSEEvent(ctx context.Context, st *State, eventData []byte) ([]byte, error) // ergonomic; for parsed-SSE hosts
```

These names are deliberately not `L0/L1/L2`: detection already uses `L1` for pattern
rules and `L2` for optional NER. The public SDK surfaces are Text, Wire, and Stream.

- **Scope and State.** `Mask` and `MaskRequest` take a `Scope` for tenant/session/project
  namespace and return an explicit `*State` handle for the matching restore. `State`
  records `provider`/`op` for wire calls so buffered and parsed-event restore can use the
  same provider walker ([ADR-0009](../architecture/decisions/0009-state-lifecycle-and-scope.md)).
- **Provider tag.** `provider`/`op` select the wire-aware walker (which JSON paths hold
  text). Every surveyed gateway can supply this tag. Unsupported provider/op pairs and
  malformed provider JSON return errors and must be treated as fail-closed.
- **Choosing a streaming method.** Use `RestoreStreamChunk` if you relay raw bytes (clipal,
  Orbit default). Use `RestoreSSEEvent` if you already parse SSE events (CLIProxyAPI,
  Orbit's transform path). Both share the same `State`.
- **Restore error surface.** Text, buffered response, and parsed-SSE restore return
  errors and receive `ctx` so callers can audit deliberately. Raw chunk restore and
  `FlushStream` are provider-agnostic hot-path helpers without `ctx` or an error return;
  callers should audit residual-token detection around the stream lifecycle
  ([ADR-0010](../architecture/decisions/0010-restore-dispatch-and-errors.md)).

See the [integration guide](integration-guide.md) for wiring patterns and the
[API reference](api-reference.md) for the full proposed surface.
