# Security Policy

OpenCloak is security-sensitive software: its purpose is to keep secrets and PII out of
supported text and tool-I/O provider egress while preserving local tool behavior.

## Supported Versions

| Version | Status |
|---|---|
| `main` / release-candidate branch | Security fixes accepted |
| `v0.1.0` | Pending release cut |
| Earlier tags | None |

## Reporting a Vulnerability

Do not open a public issue for vulnerabilities, suspected plaintext egress, credential
logging, local key disclosure, policy fail-open behavior, or restore-state isolation bugs.

Until a dedicated security address is published, contact the repository maintainers
privately and include:

- A concise description of the issue and affected path.
- Minimal reproduction steps using throwaway values only.
- Whether provider egress, local credential handling, local key storage, policy parsing,
  or restore state isolation is involved.
- Any logs with secrets removed.

Do not send real API keys, raw provider captures, local key files, customer data, or
private source code as evidence.

## Security Invariants

- Provider-bound protected text and tool-I/O payloads must be masked before egress.
- Detection, masking, policy, provider parsing, and state errors fail closed.
- The standalone proxy binds loopback only.
- Provider credentials pass through unchanged and are not logged or stored.
- Local policy files reject unknown or unsupported behavior rather than ignoring it.
- Restore state must not cross scopes.

## Known v0.1.0 Limits

- Claude Code / Anthropic Messages is live-accepted.
- Codex / OpenAI Responses is implemented with offline verification and local Codex CLI
  Responses live acceptance. This is the v0.1.0 OpenAI Responses protocol evidence; a
  separate direct `https://api.openai.com` official-service run is not claimed.
- OpenAI Chat, Gemini, remote MCP egress classification, HTTP/gRPC service, local web
  console, L2 default-on detection, `redact`, `format_preserving`, and configurable rule
  packs are not shipped v0.1.0 behavior.
- v0.1.0 does not OCR, parse, rewrite, or regenerate opaque media/document attachments.
  Anthropic image/document blocks preserve provider-native payload semantics and are not
  part of the text/tool-I/O de-identification surface.
- Provider thinking/control traces are not user text. They preserve provider-native
  semantics and are outside the masking contract.
