# Guide: Codex CLI

**Status: Planned.** The OpenAI Responses/Codex provider path is not shipped yet. The
standalone proxy exists for the Claude Code/Anthropic path, but it does not yet mask or
restore Codex Responses traffic. This document records the intended setup, grounded in
verified Codex behavior
([survey](../research/gateway-integration-survey.md),
[ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md)).

## How it will work

Codex speaks the OpenAI **Responses API** and is pointed at an endpoint via a provider
entry in `~/.codex/config.toml`. OpenCloak runs a local proxy that masks the outbound
`/responses` body, forwards upstream with your credential unchanged, and restores tokens in
the SSE response.

## Intended setup (subject to change)

```sh
opencloak proxy --addr 127.0.0.1:8788
```

```toml
# ~/.codex/config.toml
model_provider = "opencloak"

[model_providers.opencloak]
name     = "OpenCloak"
base_url = "http://127.0.0.1:8788/v1"   # http allowed; no scheme validation
wire_api = "responses"                   # the only valid value
env_key  = "OPENAI_API_KEY"              # forwarded as Authorization: Bearer
```

## Why a custom provider (not `openai_base_url`)

Use a **custom `model_providers` entry**, not the `openai_base_url` shortcut. A custom
provider's `supports_websockets` defaults to false, which **forces the plain HTTP+SSE
transport** — so the proxy sees every request. `openai_base_url` keeps the built-in
provider's WebSocket capability, which can bypass an HTTP proxy. (Verified;
[ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md).)

## Notes & limits

- Auth is a plain `Authorization: Bearer …` header, forwarded verbatim — no host/body
  signing, no cert pinning.
- **AWS Bedrock** (SigV4 signs body+host) cannot be served by a rewrite proxy — out of
  scope for the MVP.
- Avoid `CODEX_SANDBOX=seatbelt` interactions with OS-level proxies; the explicit
  `base_url` route is unaffected.
- This guide will be updated with real commands when the proxy ships.
