# OpenCloak Documentation

This is the map of OpenCloak's documentation. Docs are organized by audience and purpose;
nothing is stored ad hoc.

> **Repository status:** scaffold. These documents describe decided product and
> architecture intent, alongside a compiling Go scaffold with no engine behavior yet.
> Documents that describe interfaces or setup which do **not exist yet** are explicitly
> marked **Planned / Draft**.

## Layout

| Directory | Audience | Contents |
|---|---|---|
| [`product/`](product/) | Decision-makers, community | Vision, strategy, open-core boundary, roadmap |
| [`architecture/`](architecture/) | Engineers | System overview, threat model, decision records (ADRs) |
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
