# Veil

Local de-identification proxy for AI coding agents.

Veil masks secrets and PII before Claude Code or Codex traffic leaves your machine, then
restores real values locally on the way back. The model provider sees deterministic,
reversible placeholders instead of raw API keys, passwords, database URLs, emails, phone
numbers, IP addresses, and similar sensitive values.

## Install

```sh
npm install -g @paiart/veil
```

The npm package downloads the matching Veil binary for your platform from the GitHub
Release and verifies it against `checksums.txt` during `postinstall`.

Supported platforms:

- macOS arm64 / amd64
- Linux arm64 / amd64
- Windows arm64 / amd64

## Quickstart

Install the background service once:

```sh
veil service install
veil status
```

Point Claude Code at it in `~/.claude/settings.json`:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8787"
  }
}
```

For Codex CLI, make OpenAI the service default upstream:

```sh
veil service install --force --upstream https://api.openai.com
```

Then add this provider to `~/.codex/config.toml`:

```toml
model_provider = "veil"

[model_providers.veil]
name     = "Veil"
base_url = "http://127.0.0.1:8787/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

For OpenRouter, put the upstream directly in the local base URL:

```sh
export OPENAI_API_KEY="sk-or-v1-..."
```

```toml
model_provider = "veil-openrouter"

[model_providers.veil-openrouter]
name     = "Veil OpenRouter"
base_url = "http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

Codex appends `/responses`; Veil forwards to `https://openrouter.ai/api/v1/responses`.

Do not send Chat Completions clients through Veil yet; unsupported endpoints fail closed.

## What It Protects

Veil detects and masks:

- API keys, tokens, passwords, and connection strings
- Email addresses
- Phone numbers
- IPv4 and IPv6 addresses
- Payment card numbers
- Account numbers and financial identifiers
- URLs
- Optional policy-enabled dates, names, and addresses

Unsupported or unrecognized provider request formats fail closed instead of being silently
forwarded.

## More

- Website: https://veil.paiart.com
- GitHub: https://github.com/PAIArtCom/Veil
- Releases: https://github.com/PAIArtCom/Veil/releases
- Homebrew: `brew install PAIArtCom/veil/veil`
