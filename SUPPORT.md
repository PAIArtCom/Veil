# Support

Use the channel that matches the kind of help you need.

| Need | Channel |
|---|---|
| Setup help, usage questions, or documentation gaps | Open a GitHub issue using the question template. |
| Reproducible bugs with throwaway data | Open a GitHub issue using the bug report template. |
| Feature ideas or provider requests | Open a GitHub issue using the feature request template. |
| Security issues, plaintext egress, credential logging, local key disclosure, policy fail-open behavior, or restore-state isolation bugs | Follow [SECURITY.md](SECURITY.md). Do not include details in a public issue. |

## Before Opening an Issue

- Search existing issues and docs.
- Include your OS, architecture, Veil version, tool name, and the guide you followed.
- Use throwaway values only. Do not include real API keys, raw provider captures, local key
  files, customer data, private source code, or real personal data.
- For proxy issues, include the command you ran and whether the tool was Claude Code,
  Codex CLI, or an SDK integration.

## Supported v0.1.0 Paths

- Claude Code through Anthropic Messages (`/v1/messages`).
- Codex CLI through OpenAI Responses (`/v1/responses`) using a custom `model_providers`
  entry.
- Go SDK integrations using the public `github.com/PAIArtCom/Veil` package.

OpenAI Chat Completions, Gemini, remote MCP egress classification, OCR, document parsing,
attachment rewriting, provider thinking/control traces, HTTP/gRPC service, and local web
console behavior are not shipped v0.1.0 support paths.
