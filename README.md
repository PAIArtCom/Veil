# Veil

English | [简体中文](README.zh-CN.md)

Use AI coding agents without sending real secrets or structured PII to model providers.

Veil is a local de-identification engine and loopback proxy for tools such as Claude
Code and Codex. It replaces supported sensitive text and tool-I/O fields with
deterministic reversible tokens before provider egress, then restores the real values
locally before your terminal, files, or tool calls see the response.

| Status | License | Best for |
|---|---|---|
| v0.1.0 release | [Apache-2.0](LICENSE) | Individual developers and gateway integrators |

## Start Here

| I want to... | Go to |
|---|---|
| Run Claude Code through Veil | [Claude Code setup](docs/guides/claude-code.md) |
| Run Codex through Veil | [Codex CLI setup](docs/guides/codex.md) |
| Install, upgrade, or operate the proxy | [Deployment guide](docs/guides/deployment.md) |
| Embed Veil in a Go gateway | [SDK integration guide](docs/sdk/integration-guide.md) |
| Understand the security boundary | [Threat model](docs/architecture/threat-model.md) |
| Report a bug or vulnerability | [Support](SUPPORT.md) / [Security policy](SECURITY.md) |

## Quickstart

Build the proxy from a clean checkout:

```sh
git clone https://github.com/PAIArtCom/Veil.git
cd Veil
go build -o ./bin/veil ./cmd/veil
./bin/veil version
./bin/veil proxy --help
```

Run Claude Code through Veil:

```sh
./bin/veil proxy --addr 127.0.0.1:8788
```

In another shell:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

Run Codex through Veil:

```sh
./bin/veil proxy --addr 127.0.0.1:8788 --upstream https://api.openai.com
```

Then configure `~/.codex/config.toml`:

```toml
model_provider = "veil"

[model_providers.veil]
name     = "Veil"
base_url = "http://127.0.0.1:8788/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

```sh
export OPENAI_API_KEY=...
codex
```

## What Veil Protects

Veil v0.1.0 protects supported provider-native text fields, prompt text, tool-call
arguments, tool results, and streaming text/tool argument restoration for:

- Claude Code through Anthropic Messages (`/v1/messages`)
- Codex CLI through OpenAI Responses (`/v1/responses`)
- Go SDK integrations using the public `github.com/PAIArtCom/Veil` package

It does not protect every possible provider surface. v0.1.0 does **not** handle OpenAI
Chat Completions, Gemini, remote MCP egress classification, OCR, document parsing,
attachment rewriting, regenerated media/document payloads, provider thinking/control
traces, or protection against a compromised local machine.

## How It Works

```text
your coding tool
  -> Veil masks supported sensitive fields
  -> provider sees PAIArtVeil_<TYPE>_<id> tokens
  -> provider response returns tokens
  -> Veil restores real values locally
  -> your terminal, files, and tool calls use real values
```

The core properties are:

- **Local only**: the standalone proxy binds to loopback and stores no provider
  credentials.
- **Fail closed**: parsing, detection, masking, policy, or unsupported provider errors
  block egress instead of forwarding plaintext.
- **Deterministic tokens**: the same value maps to the same `PAIArtVeil_<TYPE>_<id>`
  token inside its scope, preserving multi-turn context and prompt-cache behavior.
- **Reversible locally**: the model sees tokens; local tools and files receive restored
  values.

## Local Policy

A local policy file can choose per-type behavior:

```json
{
  "default_operator": "token",
  "types": {
    "EMAIL": {"operator": "ignore"},
    "SECRET": {"operator": "block"}
  }
}
```

Load it with `--policy /path/to/policy.json`, `VEIL_POLICY`, or
`~/.veil/policy.json`. v0.1.0 supports `token`, `ignore`, and `block`; `redact`,
`format_preserving`, unknown keys, and non-empty `rule_sets` fail closed.

## Verify Your Setup

Use a throwaway value, never a real secret. For example, ask the agent to use
`postgresql://app:s3cr3t@localhost:5432/mydb` in a local task, then confirm:

- provider-bound text contains `PAIArtVeil_...` tokens, not the throwaway value;
- local tool calls receive the restored value;
- files written by the agent do not contain unresolved `PAIArtVeil_` tokens;
- the proxy stays on `127.0.0.1`.

See the tool-specific guides for deeper checks:
[Claude Code](docs/guides/claude-code.md) and [Codex CLI](docs/guides/codex.md).

## Documentation

| Area | Docs |
|---|---|
| User guides | [Deployment](docs/guides/deployment.md), [Claude Code](docs/guides/claude-code.md), [Codex CLI](docs/guides/codex.md) |
| Concepts | [Redaction model](docs/concepts/redaction-model.md), [Token spec](docs/concepts/token-spec.md), [Detection layers](docs/concepts/detection-layers.md) |
| SDK | [Contract](docs/sdk/contract.md), [API reference](docs/sdk/api-reference.md), [Integration guide](docs/sdk/integration-guide.md), [`examples/embed`](examples/embed/) |
| Architecture | [Overview](docs/architecture/overview.md), [Threat model](docs/architecture/threat-model.md), [ADRs](docs/architecture/decisions/README.md) |
| Project | [Roadmap](docs/product/roadmap.md), [Open-core boundary](docs/product/open-core-boundary.md), [Support](SUPPORT.md), [Security](SECURITY.md), [Changelog](CHANGELOG.md) |

## Veil vs PAIArt

| | Veil (this repo) | PAIArt |
|---|---|---|
| What | Local engine, SDK, and reference proxy | Organization control plane |
| For | Individual developers and gateway integrators | Security and compliance teams |
| License | Apache-2.0 | Commercial |

Open-core principle: individual value is open; organizational control is paid. See the
[open-core boundary](docs/product/open-core-boundary.md).

## Project Contract

The sections below are the repository-level Specability contract. They keep the public
claim, implementation boundary, and documentation aligned.

## Purpose

Veil provides a local de-identification engine and reference proxy for AI coding
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
