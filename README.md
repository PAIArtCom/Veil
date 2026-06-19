# OpenCloak

English | [简体中文](README.zh-CN.md)

> The de-identification layer for the LLM era — use AI coding agents without leaking secrets or PII to model providers.

OpenCloak sits between your dev tool (Claude Code, Codex, Copilot, Cursor, …) and the
LLM. Before a protected text or tool-I/O payload leaves your machine it
**deterministically replaces sensitive values with reversible tokens**; when the response
comes back it **restores them**. The model never sees the real values in the supported
text/tool surfaces — but your terminal, your files, and the agent's tool calls all run
with the real values.

> **Status: v0.1.0 release-candidate hardening.** The text engine, Anthropic Messages
> wire masking/restore, streaming restore, loopback Claude Code proxy, maintained SDK
> embed reference integration, OpenAI Responses wire adapter, and local policy file are
> implemented and test-verified. Claude Code is live-accepted; the local Codex CLI
> Responses path is live-accepted with sanitized evidence; direct `api.openai.com`
> upstream acceptance is not claimed until a valid OpenAI API key is available.

---

## Purpose

OpenCloak provides a local de-identification engine and reference proxy for AI coding
tools. It masks secrets and structured PII in protected text/tool surfaces before LLM
egress and restores reversible tokens on the trusted local side.

## Principles

- MUST: Mask protected text/tool-I/O before provider egress and restore only on local ingress.
- MUST: Keep tokens deterministic, reversible, type-aware, and scoped.
- MUST: Fail closed on detection, policy, parsing, provider, or masking uncertainty.
- SHOULD: Keep the root Go package as the public SDK and implementation details under `internal/`.

## Boundaries

- Does NOT handle: OpenAI Chat, Gemini, remote MCP, or unverified provider paths (see: docs/architecture/overview.md)
- Does NOT handle: OCR, document parsing, attachment rewriting, or regeneration of opaque provider media/document payloads (see: docs/sdk/contract.md)
- Does NOT handle: Provider thinking/control traces as user text; those traces preserve provider-native semantics and are outside the de-identification surface (see: docs/concepts/redaction-model.md)
- Does NOT handle: L2 semantic PII, the HTTP/gRPC service, or the web console in Phase 0 (see: docs/product/roadmap.md)
- Does NOT handle: Protection against a compromised local machine or malicious local process (see: docs/architecture/threat-model.md)

## Adversarial Surfaces

- **Protected text/tool egress**: Any unmasked sensitive value in a shipped text or tool-I/O field crossing to a provider is a release blocker. Opaque media/document payloads and provider thinking/control traces are explicit non-goals for v0.1.0 de-identification. Verified by: docs/architecture/phase-0-acceptance.md.
- **Credential pass-through**: Local proxy credentials must never be logged, stored, or interpreted by the engine. Verified by: internal/proxy/proxy_test.go.
- **Scoped restore state**: Cross-scope restore must fail visibly or leave residual tokens rather than restoring another namespace's value. Verified by: internal/mapstore/mapstore_test.go.
- **Release claim scope**: README claims must stay tied to verified providers and documented release evidence. Verified by: docs/architecture/formal-release-plan.md.

## Open Questions

- [ ] Which embedded gateway should be the first Phase 1 validation target? (open since: 2026-06)
- [ ] What exact behavior should `redact` and `format_preserving` operators use in Phase 1? (open since: 2026-06)

## Quickstart

Build and inspect the local proxy:

```sh
go build -o ./bin/opencloak ./cmd/opencloak
./bin/opencloak version
./bin/opencloak proxy --help
```

Run Claude Code through OpenCloak:

```sh
./bin/opencloak proxy --addr 127.0.0.1:8788
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

Optional local policy file:

```json
{
  "default_operator": "token",
  "types": {
    "EMAIL": {"operator": "ignore"},
    "SECRET": {"operator": "block"}
  }
}
```

Use `--policy /path/to/policy.json`, `OPENCLOAK_POLICY`, or
`~/.opencloak/policy.json`. v0.1.0 policy files support `token`, `ignore`, and `block`;
`redact`, `format_preserving`, and non-empty `rule_sets` fail closed.

## The problem

AI coding agents stream your code, configuration, and shell context to third-party
LLMs. API keys, tokens, connection strings, and personal data routinely ride along.
Faced with this, organizations either ban the tools or quietly accept the leak.

OpenCloak removes the trade-off: keep the productivity, stop the leak — locally,
with no perceptible latency, and without breaking the agent.

## How it works (one glance)

```
  your dev tool  (Claude Code / Codex / …)
       │  protected text/tool fields with real secrets & PII
       ▼
  ┌──────────────────────────────────────────────┐
  │  OpenCloak   (local proxy OR embedded library) │
  │  ① detect  → ② mask → reversible token         │
  │     e.g.  sk-live-abc…  →  CLK_SECRET_7f3a…    │
  └──────────────────────────────────────────────┘
       │  protected fields contain tokens — opaque payloads keep native shape
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

- **Two transformation points only** — mask protected text/tool fields on the way *to* the
  LLM, restore on the way *back*. Everything local (tool execution, file writes, terminal
  display) is untouched.
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
   (`ANTHROPIC_BASE_URL` for Claude Code; a custom `model_providers` entry for Codex
   Responses).
   Credentials pass straight through; only the request body is rewritten.
2. **Embeddable Go library** — drop the engine into your own gateway and call it at your
   request/response seams. The SDK is **general-purpose** and validated by the maintained
   in-repo reference integration; it is not built for any single gateway. See the
   [SDK contract](docs/sdk/contract.md) and [`examples/embed`](examples/embed/).

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
- [Deployment guide](docs/guides/deployment.md) ·
  [Release checklist](docs/guides/release-checklist.md) ·
  [Security policy](SECURITY.md) · [Changelog](CHANGELOG.md)
- [SDK contract](docs/sdk/contract.md) ·
  [Gateway integration survey](docs/research/gateway-integration-survey.md)
- [Claude Code guide](docs/guides/claude-code.md) · [Codex CLI guide](docs/guides/codex.md)

## License

[Apache-2.0](LICENSE).
