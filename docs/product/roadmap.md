# Roadmap

**Status:** Accepted (sequencing); phase contents will be refined as work proceeds.

The guiding rule: **prove the riskiest assumption first.** The riskiest assumption is not
the SaaS, the dashboards, or broad PII coverage — it is that the end-to-end
**mask → forward → restore** loop survives a real agent (no broken tool calls, no busted
caches, no orphaned tokens). Everything else is downstream.

## Phase 0 — MVP: the engine loop

**Goal:** prove the loop end-to-end.

- Engine library: **L1-only** detection (secrets + structured PII — no NER model yet).
- Deterministic, reversible, type-aware [token spec](../concepts/token-spec.md).
- Mask/restore over text, over the Anthropic wire format, and **stateful streaming
  restore** (tolerating tokens split across arbitrary byte boundaries).
- Validate by embedding the engine in **one real gateway**, plus a minimal standalone
  Claude Code proxy adapter.

**Exit criteria:** a real task ("use my local DB connection string to run a migration")
runs through Claude Code where the model only ever sees tokens, the local command executes
with the real value, files on disk contain no `CLK_` tokens, and a second turn hits the
prompt cache.

## Phase 1 — Ecosystem

**Goal:** breadth and hardening.

- Codex support (OpenAI Responses API) and a generalized provider adapter set; Gemini.
- HTTP/gRPC service wrapper for non-Go gateways.
- Optional **L2** local NER layer (semantic PII: names, addresses) — opt-in, run only on
  per-turn new content to bound latency. Person detection default-off, user-configurable.
- Configuration surface (per-type rules), robustness, and the
  [integration guide](../sdk/integration-guide.md) for third-party gateways.

## Phase 2 — Cloakia (commercial)

**Goal:** monetize organizational control.

- Control plane: central policy authoring and push to a client fleet.
- Compliance audit dashboards (built under the audit-data-minimization constraint).
- SSO / RBAC, central entity dictionaries, SIEM export.
- Commercial packaging (SaaS and/or sold-license self-hosted — kept open per the
  [strategy](strategy.md)).

## Explicitly later

- Remote MCP / remote-tool egress classification (a separate egress channel from the
  Messages/Responses APIs).
- Copilot / Cursor / additional tool adapters beyond the first two CLIs.
- Localized PII rule packs (per-country structured identifiers).
