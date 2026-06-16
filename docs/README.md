# OpenCloak Documentation

This is the map of OpenCloak's documentation. Docs are organized by audience and purpose;
nothing is stored ad hoc.

> **Repository status:** Phase 0 is accepted for the standalone Claude Code proxy path.
> Docs describe the implemented text engine, Anthropic wire path, streaming restore,
> and loopback proxy, plus planned Phase 1+ surfaces. Documents that describe interfaces
> or setup which do **not exist yet** are explicitly marked **Planned / Draft**.

## Purpose

This directory is the authoritative map for OpenCloak product, architecture, concept,
SDK, guide, and research documentation.

## Principles

- MUST: Keep normative contracts in concepts, SDK docs, threat model, and ADRs.
- MUST: Keep product status honest by distinguishing implemented, simulation-verified, live-accepted, planned, and draft.
- MUST: Preserve ADR history by superseding decisions instead of silently rewriting them.
- SHOULD: Keep cross-links relative so the docs are usable on disk and on Git hosts.

## Boundaries

- Does NOT handle: Hiding code gaps or laundering bugs as documentation status (see: ../README.md)
- Does NOT handle: Treating historical research as the active implementation contract (see: research/)
- Does NOT handle: Presenting Phase 1+ documents as implemented before code and verification exist (see: product/roadmap.md)

## Open Questions

- [ ] Which embedded gateway will be the first Phase 1 validation target? (open since: 2026-06)

## Layout

| Directory | Audience | Contents |
|---|---|---|
| [`product/`](product/) | Decision-makers, community | Vision, strategy, open-core boundary, roadmap |
| [`architecture/`](architecture/) | Engineers | System overview, threat model, decision records (ADRs), Phase 0 implementation plan |
| [`concepts/`](concepts/) | Engineers, integrators | Normative specs: redaction model, token spec, detection layers |
| [`sdk/`](sdk/) | Integrators | The general integration contract, API reference, integration guide |
| [`guides/`](guides/) | End users, operators | Setup & deployment for specific tools |
| [`research/`](research/) | Contributors | Evidence trail behind the architecture decisions |

## Reading order

**New here?** → [Product vision](product/vision.md) → [Strategy](product/strategy.md) →
[Architecture overview](architecture/overview.md).

**Integrating OpenCloak into a gateway?** → [SDK contract](sdk/contract.md) →
[Integration guide](sdk/integration-guide.md) → [Gateway survey](research/gateway-integration-survey.md).

**Want the "why" behind a decision?** → [Decision records](architecture/decisions/README.md).

## Conventions

- **Language:** English (per the project's English-only convention).
- **ADRs are immutable and numbered.** A decision is changed by adding a new ADR that
  supersedes an old one, never by silently editing history.
- **Status markers.** Forward-looking docs carry a `Status:` line (`Planned`, `Draft`,
  `Accepted`, `Implemented`).
- **Cross-links are relative**, so the tree is browsable on disk and on any Git host.
