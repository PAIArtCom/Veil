<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="brand/Veil%20Design%20System/assets/logo-dark.svg">
    <img src="brand/Veil%20Design%20System/assets/logo.svg" alt="Veil" width="170">
  </picture>
</p>

<h1 align="center">Veil</h1>

<p align="center">
  <strong>Your AI coding agent reads your secrets. Veil masks them before they leave your machine.</strong>
</p>

English | [简体中文](README.zh-CN.md)

When you use Claude Code or Codex, your prompts carry everything in context: environment
variables, database URLs, API keys, connection strings. That traffic goes to a third-party
model provider's servers. Veil intercepts it on localhost, replaces every sensitive value
with a deterministic reversible token, and restores real values locally on the way back.
The provider sees only tokens. Your workflow stays unchanged.

| Status | License | Platform |
|---|---|---|
| v0.1.0 | Apache-2.0 | macOS · Linux · Windows (amd64 / arm64) |

## Let your AI agent set it up

Paste this into your AI assistant and it will handle the full installation:

```
Install and configure Veil for me (https://github.com/PAIArtCom/Veil). Veil is a
local proxy that replaces API keys, database passwords, and other sensitive values
in prompts with placeholders before they reach AI providers, then restores real
values locally on the way back.

Please complete these steps:
① check whether Go is installed and install it if not;
② clone the repo and build the binary to ~/bin/veil, making sure it is on PATH;
③ append `export ANTHROPIC_BASE_URL=http://127.0.0.1:8788` to ~/.zshrc or ~/.bashrc;
④ write a one-command shortcut to start the proxy in the background;
⑤ verify with test value `postgresql://app:s3cr3t@localhost:5432/mydb` that masking works;
⑥ summarize how to use Veil day-to-day.

Fix any errors and continue without stopping to ask me.
```

After it finishes, open a new terminal (or `source ~/.zshrc`) and restart Claude Code.
For Codex, the config is slightly different — see [Quickstart](#quickstart) below.

## What changes

```
# Without Veil — what your model provider receives today:
"...connect to postgresql://app:s3cr3t@db.internal:5432/mydb..."
"export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"

# With Veil — what your model provider receives:
"...connect to PAIArtVeil_SECRET_a1b2c3d4..."
"export AWS_ACCESS_KEY_ID=PAIArtVeil_SECRET_e5f6g7h8..."
```

Real values are restored locally before your terminal, files, or tool calls see the response.

## Quickstart

Download a pre-built binary from the [releases page](https://github.com/PAIArtCom/Veil/releases/latest),
or build from source:

```sh
git clone https://github.com/PAIArtCom/Veil.git
cd Veil
go build -o ./bin/veil ./cmd/veil
./bin/veil version
```

**Claude Code — two commands to get started:**

```sh
# Terminal 1: start the local proxy
./bin/veil proxy --addr 127.0.0.1:8788

# Terminal 2: point Claude Code at it
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

**Codex CLI:**

```sh
# Terminal 1
./bin/veil proxy --addr 127.0.0.1:8788 --upstream https://api.openai.com
```

Add to `~/.codex/config.toml`:

```toml
model_provider = "veil"

[model_providers.veil]
name     = "Veil"
base_url = "http://127.0.0.1:8788/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

That's it. Your credentials still flow to the provider unchanged — only sensitive values
in request and response bodies are masked.

## What Veil protects

Veil detects and masks these types before provider egress, then restores them locally:

| Type | Examples |
|---|---|
| **Secrets** | API keys, tokens, passwords, connection strings |
| **Email** | `user@example.com` |
| **Phone** | `+1 555 123 4567` |
| **IPv4 / IPv6** | `192.168.1.1`, `2001:db8::1` |
| **Payment cards** | `4111 1111 1111 1111` |
| **Account numbers** | Bank and financial identifiers |
| **URLs** | `https://internal.company.com/api` |
| **Dates** | Off by default — opt in via policy if needed |
| **Names / addresses** | Off by default — opt-in L2 semantic detection |

Supported in v0.1.0:
- **Claude Code** via Anthropic Messages (`/v1/messages`)
- **Codex CLI** via OpenAI Responses (`/v1/responses`)
- **Go SDK** integrations via `github.com/PAIArtCom/Veil`

Not yet supported: OpenAI Chat Completions, Gemini, remote MCP egress classification,
OCR, document parsing, attachment rewriting, or provider thinking/control traces.

## How it works

```
your coding tool
  → Veil masks sensitive fields on localhost
  → provider sees PAIArtVeil_<TYPE>_<id> tokens only
  → provider response returns tokens
  → Veil restores real values locally
  → your terminal, files, and tool calls use real values
```

**Security properties:**

- **Local only** — the proxy binds to `127.0.0.1`. No relay, no cloud, no credentials
  stored by Veil.
- **Fail closed** — parsing errors, detection errors, policy violations, or unsupported
  endpoints block the request rather than forwarding plaintext.
- **Deterministic tokens** — the same value maps to the same token within a scope,
  so multi-turn context and prompt-cache behavior survive masking.
- **Reversible locally** — the model sees tokens; your tools and files get real values.
- **No telemetry** — the engine never phones home.

## Policy

A local policy file lets you choose per-type behavior:

```json
{
  "default_operator": "token",
  "types": {
    "EMAIL": {"operator": "ignore"},
    "SECRET": {"operator": "block"}
  }
}
```

Load with `--policy /path/to/policy.json`, the `VEIL_POLICY` environment variable, or
place it at `~/.veil/policy.json` to auto-load.

| Operator | Behavior |
|---|---|
| `token` | Replace with reversible token (default) |
| `ignore` | Leave value unmodified |
| `block` | Refuse the request entirely |

## Verify your setup

Test with a throwaway value — never a real credential:

```
postgresql://app:s3cr3t@localhost:5432/mydb
```

Ask your agent to use this string in a local task, then confirm:

- Provider-bound text contains `PAIArtVeil_...` tokens, not the test value
- Local tool calls receive the restored connection string
- Files written by the agent contain no unresolved `PAIArtVeil_` tokens
- The proxy is still listening only on `127.0.0.1`

Full verification steps: [Claude Code guide](docs/guides/claude-code.md) ·
[Codex guide](docs/guides/codex.md)

## Start here

| I want to... | Go to |
|---|---|
| Run Claude Code through Veil | [Claude Code setup](docs/guides/claude-code.md) |
| Run Codex through Veil | [Codex CLI setup](docs/guides/codex.md) |
| Install, upgrade, or operate the proxy | [Deployment guide](docs/guides/deployment.md) |
| Embed Veil in a Go gateway | [SDK integration guide](docs/sdk/integration-guide.md) |
| Understand the security boundary | [Threat model](docs/architecture/threat-model.md) |
| Report a bug or vulnerability | [Support](SUPPORT.md) · [Security policy](SECURITY.md) |

## Veil vs PAIArt

Veil is the open-source de-identification engine. PAIArt is the commercial control plane
for teams that need fleet-wide policy management and compliance audit trails.

| | Veil (this repo) | PAIArt |
|---|---|---|
| What | Local engine, SDK, and reference proxy | Organization control plane |
| For | Individual developers and gateway integrators | Security and compliance teams |
| Policy | Local JSON file | Centrally pushed, fleet-wide |
| Audit | Bring your own `AuditSink` | Compliance dashboards, SIEM export |
| License | Apache-2.0 | Commercial |

Individual value is open; organizational control is paid. See the
[open-core boundary](docs/product/open-core-boundary.md).

## Documentation

| Area | Docs |
|---|---|
| User guides | [Deployment](docs/guides/deployment.md), [Claude Code](docs/guides/claude-code.md), [Codex CLI](docs/guides/codex.md) |
| Concepts | [Redaction model](docs/concepts/redaction-model.md), [Token spec](docs/concepts/token-spec.md), [Detection layers](docs/concepts/detection-layers.md) |
| SDK | [Contract](docs/sdk/contract.md), [API reference](docs/sdk/api-reference.md), [Integration guide](docs/sdk/integration-guide.md), [`examples/embed`](examples/embed/) |
| Architecture | [Overview](docs/architecture/overview.md), [Threat model](docs/architecture/threat-model.md), [ADRs](docs/architecture/decisions/README.md) |
| Project | [Roadmap](docs/product/roadmap.md), [Open-core boundary](docs/product/open-core-boundary.md), [Support](SUPPORT.md), [Security](SECURITY.md), [Changelog](CHANGELOG.md) |
| Brand | [Design system](brand/Veil%20Design%20System/readme.md), [Logo](brand/Veil%20Design%20System/assets/logo.svg), [Dark logo](brand/Veil%20Design%20System/assets/logo-dark.svg), [Favicon](brand/Veil%20Design%20System/assets/favicon.svg) |
