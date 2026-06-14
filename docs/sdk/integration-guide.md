# Integration Guide

**Status:** Draft (patterns are accepted; code examples are illustrative pending the engine).

How to embed the OpenCloak engine into a gateway. The patterns below are the
lowest-common-denominator wiring confirmed against three real gateways — see the
[survey](../research/gateway-integration-survey.md) for the exact insertion points in each.

## The two seams

A gateway needs to call the engine at exactly two places:

1. **Outbound** — where the full request body is buffered and still mutable, *after*
   upstream/credential selection and *before* the upstream HTTP request is built.
2. **Inbound** — where the response (streaming or buffered) is relayed back to the client.

## Pattern A — outbound mask

At your outbound choke point (e.g. clipal's `requestPayload.providerBody`, Orbit's
`buildUpstreamRequest`, a CLIProxyAPI request interceptor):

```go
masked, st, err := engine.MaskRequest(provider, op, body)
if err != nil {
    return blockRequest(err) // fail-closed — never forward the original
}
body = masked
// stash st for the response stage (request context, or your request scope)
```

Notes:

- The engine takes native provider JSON; pass it your unmodified body bytes.
- Run mask **once** on the buffered body before any failover loop, so a single mapping
  serves all retries.
- The engine never needs credentials — keep doing auth your way.

## Pattern B — inbound restore (streaming)

Pick the method that matches how your gateway relays the stream.

**B1 — raw byte relay (the common case: clipal default, Orbit same-protocol).** You hand
the engine raw chunks; it holds back partial tokens across chunk boundaries:

```go
for {
    n, _ := upstream.Read(buf)
    if n > 0 {
        w.Write(engine.RestoreStreamChunk(st, buf[:n]))
    }
    if done { w.Write(engine.FlushStream(st)); break }
}
```

**B2 — parsed SSE relay (CLIProxyAPI, Orbit transform path).** You already have whole
events; restore per event:

```go
ev.Data = engine.RestoreSSEEvent(st, ev.Data)
emit(ev)
```

## Pattern C — inbound restore (non-streaming)

```go
body = engine.RestoreResponse(st, body)
```

## Tool-call arguments

Tool-call arguments arrive complete (both Claude Code and Codex buffer them before
dispatch), so they are restored by the same `RestoreResponse` / `RestoreSSEEvent` /
`RestoreStreamChunk` pass that handles the rest of the payload — no special handling, and
this is the correctness-critical path (the agent executes with real values).

## If you build the standalone proxy instead of embedding

The reference proxy applies the same patterns, plus transport rules from the ADRs:

- Bind `127.0.0.1` only ([threat model](../architecture/threat-model.md)).
- Pass the client's auth header through unchanged
  ([ADR-0004](../architecture/decisions/0004-auth-pass-through.md)).
- For Codex, use a custom `model_providers` entry (not `openai_base_url`) to force the
  HTTP+SSE transport ([ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md)).

## Checklist

- [ ] Outbound mask runs before the upstream request is built, fail-closed on error.
- [ ] `State` is threaded from the outbound call to the inbound calls.
- [ ] The right streaming method is used for your relay style.
- [ ] No `CLK_` token can leak to disk (egress scan / the identifier-safe token form).
- [ ] Inbound listener is localhost-only (standalone) or your existing auth (embedded).
