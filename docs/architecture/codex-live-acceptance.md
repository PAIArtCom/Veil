# Codex CLI Live Acceptance

**Status:** Passed for the local Codex CLI Responses path with a Responses-compatible
upstream, including the 2026-06-20 `PAIArtVeil_` prefix refresh. This is the v0.1.0
OpenAI Responses protocol evidence. A separate direct `https://api.openai.com`
official-service run is not part of the release gate and is not claimed.

**Initial run date:** 2026-06-17

**Latest prefix refresh:** 2026-06-20

**Codex CLI:** `codex-cli 0.140.0`

**Initial Veil route:** `http://127.0.0.1:8788/v1/responses`

**Latest prefix-refresh route:** `http://127.0.0.1:18888/v1/responses`

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
| Codex reached Veil over `POST /v1/responses` | Passed |
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

## Prefix Refresh Run

ADR-0014 changed the current v0.1.0 token namespace from `CLK_` to `PAIArtVeil_`. The
Codex CLI live run was rerun on 2026-06-20 using Codex CLI `0.140.0`, a temporary
`model_providers` entry pointing at Veil, and the same bounded Responses-compatible
`clipal` upstream class. The global `~/.codex/config.toml` was not edited.

The temporary workspace contained a throwaway database URL and throwaway email address.
Codex was instructed to read the fixture through a local shell command and return exactly
the two fixture lines. The capture proxy recorded only booleans and byte counts.

| Observation | Result |
|---|---|
| Codex reached Veil over `POST /v1/responses` | Passed |
| Upstream requests observed | 2 |
| Upstream request path | `/clipal/v1/responses` |
| Upstream response status/content type | `200`, `text/event-stream` |
| Upstream request 1 contained `PAIArtVeil_` tokens | Yes (`10` token-prefix occurrences) |
| Upstream request 1 contained old `CLK_` tokens | No |
| Upstream request 1 contained the throwaway plaintext values | No |
| Upstream request 2 contained `PAIArtVeil_` tokens | Yes (`12` token-prefix occurrences) |
| Upstream request 2 contained old `CLK_` tokens | No |
| Upstream request 2 contained the throwaway plaintext values | No |
| Codex local command execution completed | Yes |
| Local final output contained the expected restored throwaway values | Yes |
| Local final output contained residual `PAIArtVeil_` or `CLK_` tokens | No |
| Codex event stream completed | Yes |

The first attempt found a real release-blocking CLI wiring bug: when no policy file was
configured, `cmd/veil` printed `policy: built-in defaults` but passed a typed nil
`*config.Provider` into the engine as a non-nil `PolicyProvider` interface. Runtime masking
therefore failed closed with `veil config: nil policy provider` before any upstream
egress. This was repaired before the passing live run; the failed attempt forwarded zero
upstream requests.

## Provider Request Optimization Refresh Run

**Date:** 2026-06-20 (Asia/Shanghai)

**Reason:** OpenAI Responses request masking changed after the prefix refresh: the provider
adapter now records JSON string-literal ranges and applies changed spans with a one-pass
range rewrite when possible, falling back to structural `sjson` updates when not. The
local Codex CLI live run was rerun after that code change.

**Code under test:** local `veil` binary built from the current working tree after the
provider batch-apply optimization.

Codex CLI `0.140.0` was run with temporary `codex exec -c ...` provider configuration
pointing at `http://127.0.0.1:18888/v1`. Veil forwarded to a sanitized pass-through
capture proxy at `127.0.0.1:18889/clipal`, which forwarded to the local
Responses-compatible `clipal` upstream. The global `~/.codex/config.toml` was not edited.
The capture proxy recorded only booleans, byte counts, paths, status codes, and content
types; raw temporary outputs were inspected for boolean checks and deleted.

| Observation | Result |
|---|---|
| Codex reached Veil over `POST /v1/responses` | Passed |
| Upstream requests observed | 2 |
| Upstream request path | `/clipal/v1/responses` |
| Upstream response status/content type | `200`, `text/event-stream` |
| Upstream request 1 contained `PAIArtVeil_` tokens | Yes (`12` token-prefix occurrences) |
| Upstream request 1 contained old `CLK_` tokens | No |
| Upstream request 1 contained the throwaway plaintext values | No |
| Upstream request 2 contained `PAIArtVeil_` tokens | Yes (`16` token-prefix occurrences) |
| Upstream request 2 contained old `CLK_` tokens | No |
| Upstream request 2 contained the throwaway plaintext values | No |
| Codex local command execution completed | Yes |
| Local final output contained the expected restored throwaway values | Yes |
| Local final output contained residual `PAIArtVeil_` or `CLK_` tokens | No |
| Codex event stream completed | Yes |

Veil logged one non-restore `context canceled` upstream stream-read event after Codex
closed the SSE stream. The Codex command exited successfully, local shell execution
completed, and the provider-bound plaintext checks passed.

## OpenAI Protocol Boundary

Codex CLI speaks the OpenAI Responses API over the custom provider's HTTP+SSE transport.
The controlled Codex live runs therefore validate Veil's OpenAI Responses protocol
path for v0.1.0. The report does not separately claim an official `https://api.openai.com`
service end-to-end run; that separate run is outside the v0.1.0 release gate.

A historical redacted `codex doctor` probe for the built-in OpenAI provider reported
stored API-key auth, but the key was rejected by the OpenAI API as invalid. That historical
probe is not part of the v0.1.0 release gate.

## Hygiene

Raw temporary outputs were inspected locally only for boolean checks and must not be
committed. The repository contains only this sanitized summary.
