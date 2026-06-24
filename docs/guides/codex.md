# Guide: Codex CLI

Use this guide when you want Codex CLI traffic to pass through Veil's local
de-identification proxy.

**Supported path:** Codex CLI using the OpenAI Responses API (`/v1/responses`) through a
custom `model_providers` entry. Credentials pass through unchanged; Veil rewrites
supported request/response body fields only.

## Prerequisites

- Go installed for source builds.
- Codex CLI installed.
- An OpenAI-compatible credential available through `OPENAI_API_KEY`.

## 1. Build Veil

From the repository root:

```sh
go build -o ./bin/veil ./cmd/veil
./bin/veil version
./bin/veil proxy --help
```

## 2. Start the local proxy

```sh
./bin/veil proxy --addr 127.0.0.1:8788 --upstream https://api.openai.com
```

Add `--policy /path/to/policy.json` if you want local per-type `token`, `ignore`, or
`block` behavior.

## 3. Configure Codex

Edit `~/.codex/config.toml`:

```toml
model_provider = "veil"

[model_providers.veil]
name     = "Veil"
base_url = "http://127.0.0.1:8788/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

Start Codex:

```sh
export OPENAI_API_KEY=...
codex
```

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
| Codex bypasses Veil | Confirm `model_provider = "veil"` is active and `base_url` is `http://127.0.0.1:8788/v1`. |
| Traffic appears to use WebSockets | Use the custom provider entry above instead of `openai_base_url`. |
| Proxy refuses to start | Confirm `--addr` uses a loopback host such as `127.0.0.1`. |
| Request is blocked | Check for unsupported Responses input item shapes or a local policy selecting `block`. |
| Policy file is rejected | Remove unknown keys and use only `token`, `ignore`, or `block` operators in v0.1.0. |

## Known Limits

- Static `tools` definitions are not masked; they are provider instructions, not local
  tool output.
- Unsupported Responses input item shapes fail closed before upstream egress.
- A separate direct `https://api.openai.com` official-service run is not claimed for
  v0.1.0; the local Codex CLI Responses path is the release evidence boundary.
- AWS Bedrock, where SigV4 signs body and host, cannot be served by a rewrite proxy.
- Avoid `CODEX_SANDBOX=seatbelt` interactions with OS-level proxies; the explicit
  `base_url` route is unaffected.

## Validation Evidence

The Codex path is implemented and live-accepted for the local Codex CLI Responses route.
Maintainers can review the release evidence in the
[Codex live acceptance report](../architecture/codex-live-acceptance.md). The behavior is
grounded in [ADR-0001](../architecture/decisions/0001-base-url-proxy-over-hooks.md) and
[ADR-0013](../architecture/decisions/0013-openai-responses-provider.md).
