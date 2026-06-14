# Guide: Deployment & Operations

**Status: Planned.** Operational guidance for the standalone proxy, to be fleshed out when
the binary ships. The security invariants below are firm regardless.

## Run model

OpenCloak's standalone transport is a **long-lived local daemon**. Its process memory holds
the token↔value map for all tools pointed at it (global scope —
[ADR-0005](../architecture/decisions/0005-global-in-memory-scope.md)).

## Security invariants (non-negotiable)

- **Bind `127.0.0.1` only.** Never expose the proxy off-host without authentication — it
  would be an open relay. ([Threat model](../architecture/threat-model.md).)
- **Fail-closed.** On any detection/engine error, the request is blocked, not forwarded in
  the clear.
- **Credentials pass through.** The proxy forwards the tool's own auth header; it stores no
  provider credentials ([ADR-0004](../architecture/decisions/0004-auth-pass-through.md)).
- **Protect the local key.** `~/.opencloak/key` (the HMAC root) and any optional local map
  cache hold sensitive material; they are git-ignored and must not be backed up to shared
  storage.

## Configuration (planned surface)

- Listen address (default `127.0.0.1:8788`).
- Per-type detection toggles; rule set selection.
- Optional local map cache (off by default; in-memory is the default).
- Key path (default `~/.opencloak/key`, generated on first run).

## Observability (planned)

- Local-only counters: requests processed, spans masked by type, blocked (fail-closed)
  requests, residual-token egress flags.
- Any aggregate reporting to a control plane (Cloakia) is opt-in and subject to
  audit-data minimization ([open-core boundary](../product/open-core-boundary.md)).

_This page will gain concrete flags, service-manager units, and packaging notes when the
proxy ships._
