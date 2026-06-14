# ADR-0004 — Authentication is pass-through; the engine is credential-free

**Status:** Accepted

## Context

Putting a proxy in front of an AI CLI raises the question of whether it breaks the tool's
authentication. The reference gateway CLIProxyAPI **substitutes its own upstream
credentials** (it stores OAuth tokens / keys and ignores what the client sent). That works
but requires the proxy to *hold* credentials and *reimplement* each provider's OAuth flow.

Source research confirmed the tools forward a plain credential header with no
host-bound signing: Claude Code sends `x-api-key` (API-key users) or
`Authorization: Bearer` (OAuth/subscription users) verbatim regardless of base URL; Codex
sends a plain `Authorization: Bearer`. (Evidence:
[survey](../../research/gateway-integration-survey.md).)

## Decision

**Pass the tool's own credential through unchanged.** The transport reads the inbound
`Authorization` / `x-api-key` header and forwards it upstream as-is, rewriting only the
JSON body. The engine never sees, holds, or needs a provider credential.

## Alternatives considered

- **Credential substitution (CLIProxyAPI's model).** Rejected for OpenCloak: it would make
  the tool hold every user's provider secrets, require re-implementing OAuth per provider,
  and enlarge the trust surface — the opposite of a privacy tool's posture.

## Consequences

- OpenCloak stays **credential-free**: no OAuth re-implementation, no secret storage, the
  cleanest possible trust story.
- The proxy is a terminating forward proxy: accept on `127.0.0.1`, open its own connection
  upstream, forward whichever credential header is present (and `anthropic-beta`, account
  headers, etc.).
- **Inbound must be locked down.** CLIProxyAPI defaults to *open* inbound when no key is
  set — a footgun. OpenCloak's proxy binds `127.0.0.1` only. (See
  [threat model](../../architecture/threat-model.md).)
- AWS Bedrock (SigV4 signs body+host) cannot be served by a transparent rewrite proxy and
  is out of scope for the MVP.
