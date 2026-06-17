# Architecture Decision Records (ADRs)

This directory holds the record of *why* OpenCloak is built the way it is. Each ADR
captures one decision: its context, the choice, the alternatives rejected, and the
consequences.

## Purpose

Architecture Decision Records preserve the reasoning behind OpenCloak's core product,
security, API, and implementation choices.

## Principles

- MUST: Keep ADRs numbered and immutable once accepted.
- MUST: Change a prior decision by adding a superseding ADR and updating this index.
- SHOULD: Record context, decision, alternatives, and consequences with concrete links when possible.
- MUST: Treat ADRs as active implementation constraints until superseded.

## Boundaries

- Does NOT handle: Task plans, release notes, or implementation logs (see: ../../README.md)
- Does NOT handle: Editing accepted ADRs to hide drift instead of adding a new ADR or explicit status change (see: README.md)
- Does NOT handle: Tracking whether every downstream implementation task is done (see: ../system-design.md)

## Adversarial Surfaces

- **Accepted decision rewriting**: Changed decisions require a superseding ADR rather than edits that erase security or API tradeoffs. Verified by: ../../README.md.
- **Trust-boundary expansion**: Provider, policy, state, and release-claim changes must name boundary effects before implementation relies on them. Verified by: ../formal-release-plan.md.
- **Secret-bearing examples**: ADR examples must not include real credentials, raw provider captures, local keys, or customer data. Verified by: git diff --check.
- **ADR index drift**: Superseded status must remain visible when a decision is no longer active. Verified by: README.md.

## Open Questions

- [ ] Does future Phase 1 provider expansion require a new ADR for public adapter boundaries? (open since: 2026-06)
- [ ] What Phase 1 ADR should define format-preserving and redact operator semantics before use? (open since: 2026-06)

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
| [0005](0005-global-in-memory-scope.md) | Global, in-memory mapping scope | Superseded by ADR-0009 |
| [0006](0006-l1-only-mvp-defer-l2.md) | L1-only MVP; defer the L2 NER model | Accepted |
| [0007](0007-code-and-module-layout.md) | Code & module layout | Partially superseded by ADR-0012 |
| [0008](0008-finding-model-and-conflict-resolution.md) | Finding model and conflict resolution | Accepted |
| [0009](0009-state-lifecycle-and-scope.md) | State lifecycle and scoped mapstore | Accepted |
| [0010](0010-restore-dispatch-and-errors.md) | Restore dispatch and error surface | Accepted |
| [0011](0011-streaming-restore-cross-event-holdback.md) | Streaming restore across SSE event boundaries | Accepted |
| [0012](0012-phase-0-l1-rule-sourcing.md) | Phase 0 L1 rule sourcing and RuleSets behavior | Accepted |
| [0013](0013-openai-responses-provider.md) | OpenAI Responses provider for Codex CLI | Accepted |
