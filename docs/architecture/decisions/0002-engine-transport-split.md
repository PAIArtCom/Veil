# ADR-0002 — Engine / transport split (one core, many shells)

**Status:** Accepted

## Context

OpenCloak needs to be usable two ways: as a **standalone gateway** (for users with no
gateway of their own) and as an **embeddable library** that other gateways integrate.
These are not two products — masking/restoring is the same logic regardless of how bytes
arrive. We also require the embeddable form to be **general-purpose**, not tailored to any
single host gateway.

## Decision

Build a **transport-agnostic engine** as the open-source core — pure functions for
detection, tokenization, and mask/restore — and expose it through interchangeable
transports:

- standalone local proxy (Claude/Codex base-URL),
- embeddable Go library (`import`),
- HTTP/gRPC service (for non-Go hosts),
- (optionally) a plugin for a host that offers an interceptor API.

This mirrors the proven layering of the `privacy-filter` project (core package + thin
service wrappers).

## Alternatives considered

- **Proxy-first, library extracted later.** Rejected: bakes transport assumptions into the
  core and makes the general SDK an afterthought — the opposite of the desired wedge
  (embed everywhere).
- **Library-only (no standalone proxy).** Rejected: excludes users who don't run a gateway
  and weakens the out-of-the-box story.

## Consequences

- The engine is the open-core center and the unit of adoption (the "Sentry SDK" of
  privacy). Distribution rides into existing gateways rather than competing with them.
- The integration contract must be defined to a **lowest-common-denominator** that real
  gateways can all satisfy — see [ADR-0003](0003-deterministic-reversible-token-spec.md)
  for the token half and the [SDK contract](../../sdk/contract.md) for the API.
- Sequencing: build the engine first; validate it by embedding in ≥1 real gateway; then
  ship the standalone proxy adapter for distribution.
- The engine must not depend on a host plugin system, a unified message schema, or
  pre-parsed streaming — because real gateways do not provide these uniformly
  ([survey](../../research/gateway-integration-survey.md)).
