# Guide: Claude Code

**Status: Planned.** The standalone proxy does not exist yet. This documents the intended
setup, grounded in verified Claude Code behavior
([survey](../research/gateway-integration-survey.md),
[ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md)).

## How it will work

Claude Code reads `ANTHROPIC_BASE_URL` through the Anthropic SDK and sends standard
`/v1/messages` requests there. OpenCloak runs a local proxy on `127.0.0.1`; you point
Claude Code at it. The proxy masks the outbound body, forwards to `api.anthropic.com` with
your credentials **unchanged**, and restores tokens in the response.

## Intended setup (subject to change)

```sh
# 1. start the OpenCloak proxy (planned binary)
opencloak proxy --listen 127.0.0.1:8788

# 2. point Claude Code at it
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

Your authentication is untouched:

- **API-key users** — `x-api-key` is forwarded as-is.
- **Pro/Max/subscription (OAuth) users** — `Authorization: Bearer …` is forwarded as-is.

OpenCloak holds no credentials of its own.

## What you'll see

- The model only ever receives `CLK_…` tokens for your secrets.
- Tool calls execute locally with the **real** values (restored before execution).
- Streamed assistant text is restored to real values for display.
- Files written to disk contain real values (your data, your machine) — never `CLK_…`
  tokens.

## Notes & limits

- No TLS pinning or response-signature checks block a local HTTP proxy (verified).
- Bedrock/Vertex egress paths are separate and out of scope for the MVP.
- This guide will be updated with real commands and flags when the proxy ships.
