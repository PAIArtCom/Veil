# OpenCloak

English | [简体中文](README.zh-CN.md)

> The de-identification layer for the LLM era — use AI coding agents without leaking secrets or PII to model providers.

OpenCloak sits between your dev tool (Claude Code, Codex, Copilot, Cursor, …) and the
LLM. Before a request leaves your machine it **deterministically replaces sensitive
values with reversible tokens**; when the response comes back it **restores them**. The
model never sees the real data — but your terminal, your files, and the agent's tool
calls all run with the real values.

> **Status: pre-implementation.** This repository currently holds the product and
> architecture documentation. The engine is being built. See [`docs/`](docs/README.md).

---

## The problem

AI coding agents stream your code, configuration, and shell context to third-party
LLMs. API keys, tokens, connection strings, and personal data routinely ride along.
Faced with this, organizations either ban the tools or quietly accept the leak.

OpenCloak removes the trade-off: keep the productivity, stop the leak — locally,
with no perceptible latency, and without breaking the agent.

## How it works (one glance)

```
  your dev tool  (Claude Code / Codex / …)
       │  request with real secrets & PII
       ▼
  ┌──────────────────────────────────────────────┐
  │  OpenCloak   (local proxy OR embedded library) │
  │  ① detect  → ② mask → reversible token         │
  │     e.g.  sk-live-abc…  →  CLK_SECRET_7f3a…    │
  └──────────────────────────────────────────────┘
       │  masked request — provider never sees real data
       ▼
  LLM provider  (Anthropic / OpenAI / …)
       │  response & tool-calls reference CLK_SECRET_7f3a…
       ▼
  ┌──────────────────────────────────────────────┐
  │  OpenCloak   ③ restore tokens → real values    │
  └──────────────────────────────────────────────┘
       │  real values — tools, files, terminal all work
       ▼
  your dev tool
```

Three properties make this safe and seamless:

- **Two transformation points only** — mask on the way *to* the LLM, restore on the way
  *back*. Everything local (tool execution, file writes, terminal display) is untouched.
  See [redaction model](docs/concepts/redaction-model.md).
- **Deterministic, reversible, type-aware tokens** (`CLK_<TYPE>_<id>`) — the same value
  always maps to the same token, so prompt caches stay warm and multi-turn context stays
  coherent. See [token spec](docs/concepts/token-spec.md).
- **Layered detection** — L1 pattern matching (secrets, structured PII) ships first; an
  optional L2 local NER model (names, addresses) comes later. See
  [detection layers](docs/concepts/detection-layers.md).

## Two ways to run it

OpenCloak is **one engine with different shells** (see
[architecture overview](docs/architecture/overview.md)):

1. **Standalone local proxy** — point your CLI's base URL at it
   (`ANTHROPIC_BASE_URL` for Claude Code, a custom `model_providers` entry for Codex).
   Credentials pass straight through; only the request body is rewritten.
2. **Embeddable Go library** — drop the engine into your own gateway and call it at your
   request/response seams. The SDK is **general-purpose**, validated against several real
   gateways — not built for any single one. See the [SDK contract](docs/sdk/contract.md).

## OpenCloak vs Cloakia

| | **OpenCloak** (this repo · Apache-2.0) | **Cloakia** (commercial) |
|---|---|---|
| What | The local engine + SDK + reference proxy | The organization control plane |
| For | Individual developers — free, embeddable everywhere | Security & compliance teams |

Open-core principle: **individual value is open; organizational control is paid.**
Full breakdown in [open-core boundary](docs/product/open-core-boundary.md).

## Documentation

Start at the **[documentation map](docs/README.md)**. Highlights:

- [Product strategy](docs/product/strategy.md) · [Roadmap](docs/product/roadmap.md)
- [Architecture overview](docs/architecture/overview.md) ·
  [Threat model](docs/architecture/threat-model.md) ·
  [Decision records](docs/architecture/decisions/README.md)
- [SDK contract](docs/sdk/contract.md) ·
  [Gateway integration survey](docs/research/gateway-integration-survey.md)

## License

[Apache-2.0](LICENSE).
