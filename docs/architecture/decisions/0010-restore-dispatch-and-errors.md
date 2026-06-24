# ADR-0010 — Restore dispatch and error surface

**Status:** Accepted

## Context

Veil restores tokens on three inbound paths:

- complete buffered provider responses;
- parsed SSE event payloads;
- raw stream chunks with arbitrary byte boundaries.

Buffered responses and parsed SSE events are provider-shaped JSON, so a blind string
replace can touch non-text fields. Raw stream chunks, however, may arrive as arbitrary
bytes and are the lowest-common-denominator path for gateways that do not parse SSE.

The initial skeleton made `Engine.RestoreResponse` provider-independent and swallowed
errors, while `wire.Provider.RestoreResponse` was provider-specific and returned errors.
That left the public contract inconsistent.

## Decision

`State` records `provider` and `op` from `MaskRequest`. `RestoreResponse` and
`RestoreSSEEvent` accept `ctx`, dispatch through that provider adapter, and return
`([]byte, error)`. Text-level `Restore` also accepts `ctx` and returns an error. Restore
errors are surfaced to callers so transports can audit the event and decide whether to
block, retry, or return a response that may still contain residual tokens to the trusted
local user.

`RestoreStreamChunk` and `FlushStream` remain provider-agnostic and return `[]byte`
without `ctx`. They perform raw token scanning with holdback for partial tokens because
raw chunk relays do not provide provider event boundaries and this path is a hot relay
loop. This is an intentional asymmetry:

- text/buffered/SSE restore is structure-aware or state-aware and error-returning;
- raw stream restore is byte-oriented and best-effort until `FlushStream`.

## Alternatives considered

- **Blind string replacement for buffered responses.** Rejected: it can rewrite provider
  metadata, IDs, or non-text fields.
- **Return no error from restore.** Rejected: residual-token or malformed-provider errors
  should be observable and auditable, even if the immediate exposure is only to the
  trusted local user.
- **Require all gateways to parse streams into provider events.** Rejected: the SDK
  contract is pinned to raw byte passthrough as the lowest common denominator.

## Consequences

- `Restore(ctx, st, text)` returns `(string, error)`.
- `RestoreResponse(ctx, st, body)` returns `([]byte, error)`.
- `RestoreSSEEvent(ctx, st, eventData)` returns `([]byte, error)`.
- `RestoreStreamChunk(st, chunk)` remains `[]byte`.
- `State` must include enough metadata to route buffered and parsed-event restores.
- Tests must cover provider-aware response restore, malformed event errors, raw chunk
  split tokens, and explicit audit/error handling on restore failure.
