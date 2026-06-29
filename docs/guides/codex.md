# Guide: Codex CLI

Use this guide when you want Codex CLI traffic to pass through Veil's local
de-identification proxy.

**Supported path:** Codex CLI using the OpenAI Responses API (`/v1/responses`) through a
custom `model_providers` entry. Credentials pass through unchanged; Veil rewrites
supported request/response body fields only.

## Prerequisites

- Node.js/npm for the recommended install path, or another release installer.
- Codex CLI installed.
- An OpenAI-compatible credential available through `OPENAI_API_KEY`.

## 1. Install or build Veil

Use a release install for normal use:

```sh
npm i -g @paiart/veil
```

Or build from the repository root when testing a checkout:

```sh
go build -o ./bin/veil ./cmd/veil
./bin/veil version
```

## 2. Keep Veil running in the background

For day-to-day use, install the user service once instead of starting `veil proxy` in a
separate terminal every session:

```sh
veil service install --force --upstream https://api.openai.com
veil status
```

macOS uses a `launchd` user agent. Linux uses `systemd --user`. Windows uses Task
Scheduler. The service runs the same loopback-only proxy as `veil proxy`, so it still
refuses non-local bind addresses.

Useful service commands:

```sh
veil status              # check the local proxy
veil restart             # restart after config changes
veil service stop        # stop the background proxy
veil service start       # start it again
veil service uninstall   # remove the OS service
```

Use `--policy /path/to/policy.json` if you want local per-type `token`, `ignore`, or
`block` behavior:

```sh
veil service install --force --upstream https://api.openai.com --policy ~/.veil/policy.json
```

### Using OpenRouter instead of OpenAI

Use OpenRouter through its Responses API endpoint, not through Chat Completions.

Put OpenRouter directly in Codex's local `base_url`:

```toml
model_provider = "veil-openrouter"

[model_providers.veil-openrouter]
name     = "Veil OpenRouter"
base_url = "http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

Codex appends `/responses`; Veil forwards the request to
`https://openrouter.ai/api/v1/responses`, not to the default service upstream.

Do not configure a Chat Completions client or `/v1/chat/completions` base URL through
Veil. Chat Completions is not a supported wire adapter in v0.1.2, so Veil fails closed
instead of forwarding plaintext it cannot verify.

## 3. Configure Codex

Edit `~/.codex/config.toml`:

```toml
model_provider = "veil"

[model_providers.veil]
name     = "Veil"
base_url = "http://127.0.0.1:8787/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

For OpenRouter:

```toml
model_provider = "veil-openrouter"

[model_providers.veil-openrouter]
name     = "Veil OpenRouter"
base_url = "http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

Start Codex from an environment where `OPENAI_API_KEY` is available. For a quick
one-off test:

```sh
export OPENAI_API_KEY=...
codex
```

For daily use, put that key in your normal shell profile, launcher environment, or
credential manager so you do not have to export it every session.

Codex sends `POST /v1/responses` with `stream=true` through this route. Veil masks
message input, top-level instructions, function-call output, and supported agentic call
argument fields before upstream egress. It restores output text and function, MCP,
custom, and code-interpreter argument streams before local Codex consumes them.

## Why a custom provider is required

Use a custom `model_providers` entry, not the `openai_base_url` shortcut. A custom
provider defaults `supports_websockets` to false, which forces plain HTTP+SSE transport
so the proxy sees every request. The `openai_base_url` shortcut keeps the built-in
provider's WebSocket capability, which can bypass an HTTP proxy.

## Verify the path

Use a throwaway value, not a real credential. For example, ask Codex to perform a local
task using:

```text
postgresql://app:s3cr3t@localhost:5432/mydb
```

Expected result:

- provider-bound protected text/tool fields contain `PAIArtVeil_...` tokens;
- local tool calls receive the restored value;
- files written locally do not contain unresolved `PAIArtVeil_` tokens;
- proxy logs do not print credentials or raw request bodies.

## Troubleshooting

| Symptom | Check |
|---|---|
| Codex bypasses Veil | Confirm `model_provider = "veil"` is active and `base_url` points at `http://127.0.0.1:8787/...`. |
| Traffic appears to use WebSockets | Use the custom provider entry above instead of `openai_base_url`. |
| Veil is not running | Run `veil status`, then `veil service install` or `veil restart`. |
| Need to remove the service | Run `veil service uninstall`; remove the Veil provider from `~/.codex/config.toml` if you no longer want Codex to use Veil. |
| Proxy refuses to start | Confirm `--addr` uses a loopback host such as `127.0.0.1`. |
| Request is blocked | Check for unsupported Responses input item shapes or a local policy selecting `block`. |
| OpenRouter returns 404 | Confirm `base_url` is `http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1`, so Codex's `/responses` append reaches `/api/v1/responses`. |
| Policy file is rejected | Remove unknown keys and use only `token`, `ignore`, or `block` operators in v0.1.2. |

## Known Limits

- Static `tools` definitions are not masked; they are provider instructions, not local
  tool output.
- Unsupported Responses input item shapes fail closed before upstream egress.
- OpenAI-compatible does not mean Veil-compatible. Gateways must expose a supported wire
  shape, currently Responses or Anthropic Messages. Chat Completions is still out of
  scope.
- A separate direct `https://api.openai.com` official-service run is not claimed for
  v0.1.2; the local Codex CLI Responses path is the release evidence boundary.
- AWS Bedrock, where SigV4 signs body and host, cannot be served by a rewrite proxy.
- Avoid `CODEX_SANDBOX=seatbelt` interactions with OS-level proxies; the explicit
  `base_url` route is unaffected.

## Validation Evidence

The Codex path is implemented and live-accepted for the local Codex CLI Responses route.
Maintainers can review the release evidence in the
[Codex live acceptance report](../architecture/codex-live-acceptance.md). The behavior is
grounded in [ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md) and
[ADR-0013](../architecture/decisions/0013-openai-responses-provider.md).
