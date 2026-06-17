# Guide: Codex CLI

**Status: Implemented with offline verification; live acceptance pending.** The
OpenAI Responses/Codex provider path is implemented in the standalone proxy and covered by
sanitized fixtures, proxy tests, and a loopback Codex CLI 0.140.0 capture. The final
v0.1.0 release gate still requires a live controlled Codex acceptance run with real
provider credentials before OpenCloak can claim Codex release readiness.

Grounded in verified Codex behavior
([survey](../research/gateway-integration-survey.md),
[ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md),
[ADR-0013](../architecture/decisions/0013-openai-responses-provider.md)).

## How it works

Codex speaks the OpenAI **Responses API** and is pointed at an endpoint via a provider
entry in `~/.codex/config.toml`. OpenCloak runs a local proxy that masks the outbound
`/v1/responses` body, forwards upstream with your credential unchanged, and restores
tokens in buffered and SSE responses, including streamed tool-call arguments.

## Setup

```sh
go build -o opencloak ./cmd/opencloak
./opencloak proxy --addr 127.0.0.1:8788 --upstream https://api.openai.com
# Optional: add --policy /path/to/policy.json for local per-type token/ignore/block policy.
```

```toml
# ~/.codex/config.toml
model_provider = "opencloak"

[model_providers.opencloak]
name     = "OpenCloak"
base_url = "http://127.0.0.1:8788/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

```sh
export OPENAI_API_KEY=... # use your normal Codex/OpenAI credential source
codex
```

Codex sends `POST /v1/responses` with `stream=true` through this route. The proxy masks
message input, top-level instructions, function-call output, and agentic call argument
fields before upstream egress; it restores output text and function/MCP/custom/code
interpreter argument streams before local Codex consumes them.

## Why a custom provider (not `openai_base_url`)

Use a **custom `model_providers` entry**, not the `openai_base_url` shortcut. A custom
provider's `supports_websockets` defaults to false, which **forces the plain HTTP+SSE
transport** — so the proxy sees every request. `openai_base_url` keeps the built-in
provider's WebSocket capability, which can bypass an HTTP proxy. (Verified;
[ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md).)

## Notes & limits

- Auth is a plain `Authorization: Bearer …` header, forwarded verbatim — no host/body
  signing, no cert pinning.
- Static `tools` definitions are not masked; they are provider instructions, not local
  tool output.
- Unsupported Responses input item shapes fail closed before upstream egress.
- The live Codex acceptance task is still required for the v0.1.0 release candidate. Until
  that run passes, treat Codex support as offline-verified, not release-accepted.
- **AWS Bedrock** (SigV4 signs body+host) cannot be served by a rewrite proxy — out of
  scope for the MVP.
- Avoid `CODEX_SANDBOX=seatbelt` interactions with OS-level proxies; the explicit
  `base_url` route is unaffected.
