# Architecture Decision Records (ADRs)

This directory holds the record of *why* OpenCloak is built the way it is. Each ADR
captures one decision: its context, the choice, the alternatives rejected, and the
consequences.

## Conventions

- **Numbered and immutable.** Once an ADR is `Accepted`, it is not rewritten. To change a
  decision, add a new ADR that supersedes it (and mark the old one `Superseded by ADR-NNNN`).
- **Format:** Status · Context · Decision · Alternatives considered · Consequences.
- **Evidence.** Where a decision rests on source-code research, ADRs cite it and link to
  the [gateway integration survey](../../research/gateway-integration-survey.md).

## Index

| ADR | Title | Status |
|---|---|---|
| [0001](0001-base-url-proxy-over-hooks.md) | Attach via base-URL local proxy, not in-process hooks | Accepted |
| [0002](0002-engine-transport-split.md) | Engine / transport split (one core, many shells) | Accepted |
| [0003](0003-deterministic-reversible-token-spec.md) | Deterministic, reversible, type-aware token spec | Accepted |
| [0004](0004-auth-pass-through.md) | Authentication is pass-through, engine is credential-free | Accepted |
| [0005](0005-global-in-memory-scope.md) | Global, in-memory mapping scope | Accepted |
| [0006](0006-l1-only-mvp-defer-l2.md) | L1-only MVP; defer the L2 NER model | Accepted |
