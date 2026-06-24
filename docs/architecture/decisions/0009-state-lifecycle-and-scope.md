# ADR-0009 — State lifecycle and scoped mapstore

**Status:** Accepted

Supersedes [ADR-0005](0005-global-in-memory-scope.md).

## Context

Veil's reverse map (`token -> original value`) must survive long enough to restore the
response that corresponds to a masked request. ADR-0005 chose a process-global in-memory
scope because deterministic tokens let the map self-heal. That remains true for simple
single-user use, but reference implementations expose two hazards:

- a single shared vault can mix unrelated sessions or tenants;
- TTL-based caches can evict mappings while a long stream is still being restored.

Veil needs the ergonomic default for local use without making cross-session leakage or
stream eviction part of the architecture.

## Decision

`Mask` and `MaskRequest` take an explicit `Scope` and create the `State` for one outbound
mask operation plus its matching inbound restore operations:

```go
Mask(ctx, scope, text) (masked, state, err)
MaskRequest(ctx, scope, provider, op, body) (masked, state, err)
```

Standalone transports pass a non-zero `Scope` when they have a session, project, or tenant
identifier. The zero scope is reserved for single-user local use. `State` remains opaque:
callers do not construct it directly. It records the scope plus, for wire calls, the
`provider` and `op` selected by the outbound call so restore can dispatch to the same
provider adapter.

The mapstore is in-memory by default, but every entry belongs to a namespace:

- request/stream state for the concrete response being restored;
- optional session/project namespace for multi-turn self-healing;
- explicit tenant namespace for any multi-user embedding.

A `State` must remain alive until the buffered response or streaming response has fully
completed and `FlushStream` has run. It must not be removed by TTL while a stream is active.
Local persistent cache remains opt-in and must be encrypted or otherwise documented as
holding the user's own secrets on the user's own disk.

Cross-scope restore is forbidden. If a token exists only in another tenant/session/project
namespace, restore must leave a residual token or surface an error; it must never consult a
different namespace to make restoration "work."

Deterministic token generation is still stateless: the same `(type, normalized value,
local key)` yields the same token regardless of mapstore state. The mapstore only supports
reverse restoration.

## Alternatives considered

- **One process-global vault for everything.** Rejected: convenient, but it risks
  accidental cross-session or cross-tenant restoration when Veil is embedded in a
  shared process.
- **Per-request only, no longer-lived namespace.** Rejected: it handles the immediate
  response but loses the self-healing benefit across retries, cached responses, and
  multi-turn agent workflows.
- **Mandatory persisted SQLite map.** Rejected: it improves durability but expands the
  local sensitive-data surface. Persistence should be explicit.
- **TTL eviction for all mappings.** Rejected: TTL must not be able to break an active
  streaming restore.

## Consequences

- `State` documentation must describe lifecycle, namespace semantics, and provider/op
  metadata, not just an opaque reverse map.
- The standalone proxy and service wrappers should thread explicit `*State` through
  request context and streaming restore paths.
- Embedders use the `Scope` argument on `MaskRequest` for tenant/session/project
- Embedders use the `Scope` argument on `Mask` and `MaskRequest` for
  tenant/session/project namespaces. Multi-user embedders must set a tenant namespace.
- Tests must cover concurrent requests with identical tokens in different namespaces,
  cross-scope non-restoration, long streams without TTL eviction, and state cleanup after
  stream flush.
