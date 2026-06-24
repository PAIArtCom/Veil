# ADR-0001 — Attach via base-URL local proxy, not in-process hooks

**Status:** Accepted

## Context

Veil must rewrite the request body *before* it leaves the machine and transform the
response on the way back. We evaluated how to attach to the target tools (Claude Code,
Codex) by reading their source. Findings (full evidence in the
[gateway integration survey](../../research/gateway-integration-survey.md)):

- **Claude Code** reads `ANTHROPIC_BASE_URL` directly through the Anthropic SDK; pointing
  it at `http://127.0.0.1:PORT` sends standard `/v1/messages` JSON there. No TLS pinning,
  no response-signature validation. Its hook system (`PreToolUse`, `UserPromptSubmit`,
  etc.) **cannot rewrite the `system`/`messages` sent to the API** — `UserPromptSubmit`
  only *appends* `additionalContext`; `PreToolUse` only mutates *local* tool input.
- **Codex** uses the OpenAI **Responses API** and is pointed via a custom
  `[model_providers.*]` entry (`base_url` + `wire_api="responses"`). A custom provider's
  `supports_websockets` defaults to false, which **forces the HTTP+SSE transport** (so a
  proxy sees everything); `openai_base_url` alone does not (it may use a WebSocket
  transport, a blind spot). Codex hooks and `notify` are out-of-process and cannot rewrite
  the LLM payload.

## Decision

Attach via a **base-URL local proxy**. The standalone transport runs a local HTTP server;
the tool is configured to send to it; it rewrites the JSON body and relays upstream.

- Claude Code: `ANTHROPIC_BASE_URL=http://127.0.0.1:PORT`.
- Codex: a custom `model_providers` entry with `wire_api="responses"` (not `openai_base_url`).

## Alternatives considered

- **Cert-spoofing MITM proxy** (transparently intercept `api.anthropic.com`). Rejected:
  brittle against SSL pinning, invasive to the user's trust store, fragile across network
  setups.
- **In-process tool hooks.** Rejected: neither tool's hooks can rewrite the outbound
  prompt — the one operation we require. Hooks remain useful only for *local* tool-input
  mutation, which is not our need.
- **Forking the tool / SDK `fetchOverride`.** Rejected: requires running inside the tool's
  process; not an "attach," and not portable across tools.

## Consequences

- A local proxy is the standalone transport; in-process hooks are not used.
- We must speak each provider's native wire format (Anthropic Messages, OpenAI Responses).
- The proxy must bind `127.0.0.1` only and pass credentials through
  ([ADR-0004](0004-auth-pass-through.md)).
- For Codex we must document the custom-provider requirement to avoid the WebSocket blind
  spot.
- AWS Bedrock (SigV4, signs body+host) is the one upstream a transparent rewrite proxy
  cannot serve; it is out of scope for the MVP.
