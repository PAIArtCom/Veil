# OpenCloak Documentation

This is the map of OpenCloak's documentation. Docs are organized by audience and purpose;
nothing is stored ad hoc.

> **Repository status:** Phase 0 is accepted for the standalone Claude Code proxy path.
> R2/R3 release hardening has added the maintained SDK embed reference integration and
> OpenAI Responses support for Codex. R4 added the local policy file. The local Codex CLI
> Responses path is live-accepted with sanitized evidence and is the v0.1.0 OpenAI
> Responses protocol evidence. A separate direct `api.openai.com` official-service run is
> not part of the v0.1.0 release gate and is not claimed. Documents that describe
> interfaces or setup which do **not exist yet** are explicitly marked **Planned / Draft**.

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

## Adversarial Surfaces

- **Unsupported release claims**: Shipped/planned status must match code and verification evidence. Verified by: specability reconcile docs --json.
- **Secret-bearing evidence**: Raw provider captures, credentials, local keys, and real secrets must not be committed as documentation evidence. Verified by: architecture/formal-release-plan.md.
- **ADR history drift**: Accepted decisions are superseded by new ADRs rather than rewritten. Verified by: architecture/decisions/README.md.
- **Guide routing drift**: Setup instructions must name the exact supported base-URL path and known limits. Verified by: guides/claude-code.md.
- **Live-acceptance overclaim**: Codex/OpenAI Responses support must present the local Codex CLI Responses run as the v0.1.0 protocol evidence without claiming a separate direct `api.openai.com` official-service run. Verified by: architecture/codex-live-acceptance.md.

## Open Questions

- [ ] Which embedded gateway will be the first Phase 1 validation target? (open since: 2026-06)

## Layout

| Directory | Audience | Contents |
|---|---|---|
| [`product/`](product/) | Decision-makers, community | Vision, strategy, open-core boundary, roadmap |
| [`architecture/`](architecture/) | Engineers | System overview, threat model, decision records (ADRs), Phase 0 implementation plan, formal release plan |
| [`concepts/`](concepts/) | Engineers, integrators | Normative specs: redaction model, token spec, detection layers |
| [`sdk/`](sdk/) | Integrators | The general integration contract, API reference, integration guide |
| [`guides/`](guides/) | End users, operators | Setup & deployment for specific tools |
| [`research/`](research/) | Contributors | Evidence trail behind the architecture decisions |

Top-level release metadata:

- [README](../README.md) / [README.zh-CN](../README.zh-CN.md)
- [Changelog](../CHANGELOG.md)
- [Security policy](../SECURITY.md)
- [Contribution guide](../CONTRIBUTING.md)
- [Release checklist](guides/release-checklist.md)
- [Codex CLI live acceptance](architecture/codex-live-acceptance.md)
- [Performance evaluation](architecture/performance-evaluation.md)
- [v0.1.0 release report](architecture/v0.1.0-release-report.md)

## Reading order

**New here?** → [Product vision](product/vision.md) → [Strategy](product/strategy.md) →
[Architecture overview](architecture/overview.md).

**Integrating OpenCloak into a gateway?** → [SDK contract](sdk/contract.md) →
[Integration guide](sdk/integration-guide.md) → [Gateway survey](research/gateway-integration-survey.md).

**Want the "why" behind a decision?** → [Decision records](architecture/decisions/README.md).

**Planning the public OSS release?** → [Formal release plan](architecture/formal-release-plan.md).

**Installing or operating the proxy?** → [Deployment guide](guides/deployment.md) →
[Claude Code guide](guides/claude-code.md) or [Codex CLI guide](guides/codex.md).

**Preparing a release cut?** → [Release checklist](guides/release-checklist.md) →
[Performance evaluation](architecture/performance-evaluation.md) →
[v0.1.0 release report](architecture/v0.1.0-release-report.md).

## Conventions

- **Language:** English (per the project's English-only convention).
- **ADRs are immutable and numbered.** A decision is changed by adding a new ADR that
  supersedes an old one, never by silently editing history.
- **Status markers.** Forward-looking docs carry a `Status:` line (`Planned`, `Draft`,
  `Accepted`, `Implemented`).
- **Cross-links are relative**, so the tree is browsable on disk and on any Git host.
