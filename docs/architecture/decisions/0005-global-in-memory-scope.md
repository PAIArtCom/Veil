# ADR-0005 — Global, in-memory mapping scope

**Status:** Superseded by [ADR-0009](0009-state-lifecycle-and-scope.md)

> Superseded decision. Do not implement the global-only scope described below. Current
> architecture uses explicit `Scope` and request/stream `State` handles per ADR-0009.

## Context

The reverse map (token→value) needs a scope: per-session, per-project, or global. Sessions
are awkward — some invocation paths expose no stable session identifier. We also want the
simplest thing that is correct, ideally without persisting secrets to disk.

A key realization makes scope nearly irrelevant to correctness:

- **Forward determinism is stateless.** `token = HMAC(value, key)` — the same value yields
  the same token with no stored state, so caching and consistency never depend on the map.
- **Restore completeness is guaranteed structurally.** A response can only contain tokens
  for values that were in the request that produced it; the engine created those
  token→value entries while masking that same request. So the map always has what a
  response needs.

## Decision

Scope is **global, held in process memory.** The standalone proxy is a long-lived daemon,
so its process memory *is* the global scope — no session identifier required. An optional
local cache may back the map for belt-and-suspenders durability, but it is not needed for
correctness.

## Alternatives considered

- **Per-session scope.** Rejected: no reliable session id across invocation paths, and it
  buys nothing over global given determinism.
- **Mandatory persisted map.** Rejected as a requirement: unnecessary for correctness;
  persisting secrets to disk should be opt-in, not default.

## Consequences

- Zero session-tracking machinery; the map is a cache, not a source of truth.
- Across a daemon restart, determinism keeps tokens stable; because the display layer
  restores to real values (see [redaction model](../../concepts/redaction-model.md)), real
  values flow back each turn and the reverse map self-heals.
- Multi-process embedding is fine: processes sharing the same `local_key` produce identical
  tokens; each process's reverse map self-heals independently.
- A `local_key` must be stable and local (see
  [ADR-0003](0003-deterministic-reversible-token-spec.md)).
