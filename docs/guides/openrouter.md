# Guide: OpenRouter

Use this guide when you want Codex CLI traffic to reach OpenRouter through Veil.

**Supported path:** Codex CLI using the OpenAI Responses API (`/v1/responses`) through
OpenRouter's Responses-compatible endpoint. Veil forwards your OpenRouter bearer token
unchanged and rewrites only supported request/response body fields.

## What works

OpenRouter exposes several API shapes. Veil v0.1.3 supports the Responses shape:

```text
client -> Veil:       POST http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1/responses
Veil -> OpenRouter:  POST https://openrouter.ai/api/v1/responses
```

Do not point Chat Completions clients at Veil yet:

```text
POST /v1/chat/completions
```

That route is not a supported Veil wire adapter. Veil fails closed on unsupported
endpoints instead of forwarding plaintext it does not know how to mask.

## 1. Install Veil

Use any release install path:

```sh
npm i -g @paiart/veil
```

or:

```sh
curl -fsSL https://veil.paiart.com/install.sh | sh
```

## 2. Keep Veil running in the background

Install the background service once:

```sh
veil service install
veil status
```

You do not need to keep a terminal open with `veil proxy`. The service runs on
`127.0.0.1:8787` after login.

Useful service commands:

```sh
veil status              # check the local proxy
veil restart             # restart after config changes
veil service stop        # stop the background proxy
veil service start       # start it again
veil service uninstall   # remove the OS service
```

## 3. Configure Codex

Make your OpenRouter key available to Codex as `OPENAI_API_KEY`. For daily use, put it in
your normal shell profile, launcher environment, or credential manager. For a quick
one-off test:

```sh
export OPENAI_API_KEY="sk-or-v1-..."
```

Edit `~/.codex/config.toml`:

```toml
model_provider = "veil-openrouter"

[model_providers.veil-openrouter]
name     = "Veil OpenRouter"
base_url = "http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

Choose any OpenRouter model slug that supports the Responses API in your Codex model
configuration. Model selection is a Codex/OpenRouter setting, not a Veil setting.

Start Codex with that provider active. Codex sends `POST /v1/responses` through the local
base URL. Veil reads the path-local `upstream=` value and forwards to
`https://openrouter.ai/api/v1/responses`.

## Why this works

Codex speaks the OpenAI Responses wire when `wire_api = "responses"` is set. Veil already
knows how to mask and restore that request and response shape, so the provider can be
OpenAI directly or a Responses-compatible gateway such as OpenRouter.

The provider credential still passes through as an HTTP header. Veil does not store it,
log it, or substitute its own credential.

The base URL intentionally ends with OpenRouter's `/api/v1`. Codex appends `/responses`;
Veil splits the local path into upstream `https://openrouter.ai/api/v1` and provider path
`/responses`, producing OpenRouter's `/api/v1/responses` endpoint.

## Verify

Use a throwaway value, not a real secret:

```text
postgresql://app:s3cr3t@localhost:5432/mydb
```

Expected result:

- OpenRouter-bound request body contains `PAIArtVeil_...` tokens, not the throwaway
  connection string.
- Local tool calls and files receive the restored connection string.
- Unsupported endpoint attempts return an error from Veil and do not reach OpenRouter.

## Troubleshooting

| Symptom | Check |
|---|---|
| OpenRouter returns 404 | Confirm `base_url = "http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1"`. |
| Veil is not running | Run `veil status`, then `veil service install` or `veil restart`. |
| Codex bypasses Veil | Confirm `model_provider = "veil-openrouter"` and `base_url` starts with `http://127.0.0.1:8787/`. |
| Need to remove the service | Run `veil service uninstall`; remove the Veil provider from `~/.codex/config.toml` if you no longer want Codex to use Veil. |
| Request is blocked by Veil | Confirm the client is using Responses, not Chat Completions. Unsupported paths fail closed. |
| Authentication fails | Confirm `OPENAI_API_KEY` is an OpenRouter key and is available to the environment that starts Codex. |

## Current limits

- Chat Completions clients are not supported by Veil v0.1.3.
- OpenRouter's provider-specific routing, transforms, and model support can vary by model;
  if a model does not support Responses, choose a model that does.
- Anthropic Messages through OpenRouter may be possible only when the client sends
  OpenRouter-compatible auth headers. It is not the recommended path for Claude Code.
