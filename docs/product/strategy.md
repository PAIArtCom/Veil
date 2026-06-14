# Product Strategy

**Status:** Accepted

## Positioning

OpenCloak is the local, vendor-neutral **de-identification layer** for AI coding tools.
It masks sensitive data before it reaches an LLM and restores it on return — as a
standalone local proxy or as a library embedded in an existing gateway.

## Two products (open core)

| | **OpenCloak** (open · Apache-2.0) | **Cloakia** (commercial) |
|---|---|---|
| Essence | The **engine** + reference adapters (standalone proxy, HTTP/gRPC service, embeddable library) | The **organization control plane** |
| Contents | Detection (L1/L2), deterministic tokens, mask/restore, per-provider adapters, local config | Central policy push, fleet config, compliance audit dashboards, SSO/RBAC, enterprise entity dictionaries, SIEM export |
| Buyer | Individual developers — free, embedded everywhere | CISO / security & compliance — per-seat or subscription |
| Analogy | The Sentry SDK | The Sentry.io console |

The dividing line is one sentence: **individual value is open; organizational control is
paid.** The full feature-by-feature breakdown is in
[open-core-boundary.md](open-core-boundary.md).

## The wedge and the moat

- **Embed, don't compete.** OpenCloak ships as an embeddable engine that slots into
  existing gateways (including first-party ones) rather than fighting to *be* the gateway.
  The privacy layer is a capability every gateway wants and none want to build.
- **Vendor-neutral, cross-tool.** Enterprises use Claude Code *and* Codex *and* Copilot
  *and* Cursor. A single policy/audit plane across all of them is something no individual
  model vendor will build — each only covers its own surface.
- **Buyer mismatch + regulatory tailwind.** Model vendors sell to developers and
  engineering leaders; Cloakia sells to security and compliance. A CISO specifically does
  not want to rely on the AI vendor's own "trust us, we redact" — they want an
  independent, auditable control. GDPR / EU AI Act / HIPAA / financial rules make that a
  checkbox they must tick.

## Defensibility (the honest risk)

A model vendor or tool could add "redact before send" as a built-in toggle. The bet that
makes OpenCloak durable: **neutrality across tools + a security/compliance buyer + an
independent, auditable control plane.** A per-vendor feature cannot satisfy a CISO who
must govern a heterogeneous fleet and prove it to an auditor.

## Go-to-market assets

OpenCloak can be dogfooded inside first-party gateways for immediate validation and
distribution, while the SDK stays strictly general-purpose (not coupled to any one
gateway). Open-source ubiquity of the engine is the top of the funnel; Cloakia is the
monetization layer for organizations that need to manage and audit it at scale.

## Target users & scenarios

- **Individual developer / in-house power user.** "I want to use the latest agent on my
  company's code, but I can't risk it shipping our AWS keys or private endpoints to a
  provider." → Runs OpenCloak locally; the agent only ever sees tokens; everything keeps
  working.
- **Security / compliance lead (Cloakia).** "We can't let a heterogeneous set of AI tools
  exfiltrate source and secrets, but banning them is losing us velocity." → Mandates the
  client fleet-wide, sets policy centrally, and gets an audit trail.

## Commercialization (deliberately unconstrained)

SaaS, sold license + self-hosted, or hybrid — all viable. The engineering constraint is
only that the control-plane interface stays abstract so either model can attach. **The
more local and zero-telemetry the deployment, the cleaner the trust story** — a fully
local, no-phone-home enterprise build is the easiest thing for a CISO to approve.
