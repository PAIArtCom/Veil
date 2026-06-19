# Integration Guide

**Status:** v0.1.0 pre-release. The maintained `examples/embed` reference integration
validates the SDK seams outside the standalone proxy. Anthropic Messages is
live-accepted through Claude Code; OpenAI Responses is implemented with offline fixtures
and local Codex CLI Responses live acceptance. Direct `https://api.openai.com` upstream
acceptance is not claimed for v0.1.0 evidence.

How to embed the OpenCloak engine into a gateway. The patterns below are the
lowest-common-denominator wiring confirmed against three real gateways — see the
[survey](../research/gateway-integration-survey.md) for the exact insertion points in each.

## The two seams

A gateway needs to call the engine at exactly two places:

1. **Outbound** — where the full request body is buffered and still mutable, *after*
   upstream/credential selection and *before* the upstream HTTP request is built.
2. **Inbound** — where the response (streaming or buffered) is relayed back to the client.

The in-repo [`examples/embed`](../../examples/embed/) package is the maintained reference
for these seams. It is not a production gateway and does not claim external clipal
integration; it exists to prove the SDK contract with tests.

## Pattern A — outbound mask

At your outbound choke point (e.g. clipal's `requestPayload.providerBody`, Orbit's
`buildUpstreamRequest`, a CLIProxyAPI request interceptor):

```go
scope := opencloak.Scope{
    Tenant:  tenantID,  // empty is fine for single-user local use
    Session: sessionID, // stable agent/session id when available
    Project: projectID,
}
masked, st, err := engine.MaskRequest(ctx, scope, provider, op, body)
if err != nil {
    return blockRequest(err) // fail-closed — never forward the original
}
body = masked
// stash st for the response stage (request context, or your request scope)
```

Notes:

- The engine takes native provider JSON; pass it your unmodified body bytes.
- Run mask **once** on the buffered body before same-provider/same-operation retries, so
  a single mapping serves all retries. If failover changes provider or operation, remask
  and use the new `State`; provider-aware restore dispatches by `st.Provider()`/`st.Op()`.
- Keep `st` alive until the response is fully restored and any stream buffer has been
  flushed. Do not let TTL cleanup remove active stream state.
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
ev.Data, err = engine.RestoreSSEEvent(ctx, st, ev.Data)
if err != nil { return handleRestoreError(err) }
emit(ev)
```

## Pattern C — inbound restore (non-streaming)

```go
body, err = engine.RestoreResponse(ctx, st, body)
if err != nil { return handleRestoreError(err) }
```

## Tool-call arguments

Tool-call arguments arrive complete before dispatch, while streaming providers may emit
partial JSON deltas on the wire. Provider adapters must cover the whole agentic tool I/O
surface: tool results fed back to the model, tool-use/tool-call argument payloads, and
provider-specific argument deltas. This is the correctness-critical path: the agent must
execute with real values.

## If you build the standalone proxy instead of embedding

The reference proxy applies the same patterns, plus transport rules from the ADRs:

- Bind `127.0.0.1` only ([threat model](../architecture/threat-model.md)).
- Pass the client's auth header through unchanged
  ([ADR-0004](../architecture/decisions/0004-auth-pass-through.md)).
- Codex/OpenAI Responses support is implemented with offline verification and local Codex
  CLI Responses live acceptance. Use a custom `model_providers` entry (not
  `openai_base_url`) to force the HTTP+SSE transport
  ([ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md),
  [ADR-0013](../architecture/decisions/0013-openai-responses-provider.md)).

## Checklist

- [ ] Outbound mask runs before the upstream request is built, fail-closed on error.
- [ ] A `Scope` is supplied for tenant/session/project isolation where available.
- [ ] `State` is threaded from the outbound call to the inbound calls.
- [ ] Buffered/SSE restore errors are surfaced to logs/audit and handled deliberately.
- [ ] The right streaming method is used for your relay style.
- [ ] No `CLK_` token can leak to disk (residual-token scan / the identifier-safe token form).
- [ ] Inbound listener is localhost-only (standalone) or your existing auth (embedded).
