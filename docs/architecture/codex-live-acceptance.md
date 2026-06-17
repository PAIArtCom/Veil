# Codex CLI Live Acceptance

**Status:** Passed for the local Codex CLI Responses path with a Responses-compatible
upstream. Direct `https://api.openai.com` upstream acceptance remains unclaimed until a
valid OpenAI API key is available.

**Run date:** 2026-06-17

**Codex CLI:** `codex-cli 0.140.0`

**OpenCloak route:** `http://127.0.0.1:8788/v1/responses`

**Upstream under test:** local Responses-compatible provider route
`/clipal/v1/responses`, reached through a sanitized pass-through capture proxy.

## Controlled Task

The run used a temporary working directory containing a throwaway fixture file with a
synthetic database URL and synthetic email address. Codex was instructed to read that file
with its local shell tool and return a one-line result. No real credentials, customer data,
raw provider captures, or local key files were used.

Temporary Codex provider configuration was supplied with `codex exec -c ...`; the global
`~/.codex/config.toml` file was not edited.

## Sanitized Evidence

The pass-through capture proxy recorded only summary booleans and byte counts, not raw
request or response bodies.

| Observation | Result |
|---|---|
| Codex reached OpenCloak over `POST /v1/responses` | Passed |
| Upstream requests observed | 2 |
| Upstream request path | `/clipal/v1/responses` |
| Upstream response content type | `text/event-stream` |
| Upstream request 1 contained `CLK_` tokens | Yes |
| Upstream request 1 contained the throwaway plaintext values | No |
| Upstream request 2 contained `CLK_` tokens | Yes |
| Upstream request 2 contained the throwaway plaintext values | No |
| Local final output contained the expected restored throwaway values | Yes |
| Local final output contained residual `CLK_` tokens | No |
| Codex event stream completed | Yes |
| Codex local command execution completed | Yes |

The first attempt found a real release-blocking CLI wiring bug: when no policy file was
configured, `cmd/opencloak` printed `policy: built-in defaults` but passed a typed nil
`*config.Provider` into the engine as a non-nil `PolicyProvider` interface. Runtime masking
therefore failed closed with `opencloak config: nil policy provider` before any upstream
egress. This was repaired before the passing live run; the failed attempt forwarded zero
upstream requests.

## Direct OpenAI Upstream Status

A redacted `codex doctor` probe for the built-in OpenAI provider reported stored API-key
auth, but the key was rejected by the OpenAI API as invalid. Therefore this report does not
claim direct `https://api.openai.com` upstream live acceptance. The verified claim is the
Codex CLI 0.140.0 Responses wire path through OpenCloak with a live
Responses-compatible upstream.

## Hygiene

Raw temporary outputs were inspected locally only for boolean checks and must not be
committed. The repository contains only this sanitized summary.
