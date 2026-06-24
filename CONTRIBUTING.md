# Contributing to Veil

Thanks for your interest. Veil is the open-source privacy engine for AI coding
tools. The Phase 0 Claude Code proxy path is accepted, and the repository is now in
v0.1.0 release-candidate hardening. Please read this before opening an issue or PR.

## Project status

Implemented release scope includes the core engine, Claude Code proxy path, maintained
SDK embed reference integration, OpenAI Responses adapter with offline verification and
local Codex CLI Responses live acceptance, and local policy file support. Direct
`https://api.openai.com` upstream acceptance, OpenAI Chat, Gemini, remote MCP, HTTP/gRPC
service, local console, L2 default-on behavior, `redact`, and `format_preserving` remain
out of the shipped v0.1.0 claim unless the formal release plan is updated.

## Ground rules

- **Language:** English for all code, comments, and documentation. `README.zh-CN.md` is
  the only maintained localized top-level document.
- **Decisions go through ADRs.** Significant architectural changes are proposed as a new
  record in [`docs/architecture/decisions/`](docs/architecture/decisions/README.md). ADRs
  are immutable once accepted — supersede, don't rewrite.
- **Docs and code stay in sync.** A change in behavior updates the relevant concept/spec
  doc in the same PR. Don't let docs drift.
- **Security first.** Veil's whole job is to *not* leak data. Any change touching
  detection, masking, restore, or egress must keep the invariants in the
  [threat model](docs/architecture/threat-model.md) — especially **fail-closed** and
  **localhost-only** binding. Never commit real secrets, tokens, or the local key store.

## Workflow

1. Open an issue describing the problem or proposal before large work.
2. Branch from `main`. Keep changes focused.
3. For code: format with `gofmt`, pass `go test -count=1 ./...`, `go test -race -count=1 ./...`,
   `go vet ./...`, `go build ./...`, and include focused tests.
4. Reference the relevant ADR / spec doc in your PR description.
5. Be explicit about what you verified and what you didn't.

## Reporting security issues

Do **not** open a public issue for vulnerabilities. A private disclosure channel will be
listed in [SECURITY.md](SECURITY.md). Until a dedicated security mailbox is published,
report suspected vulnerabilities privately to the repository maintainers.

## License

By contributing, you agree that your contributions are licensed under the project's
[Apache-2.0](LICENSE) license.
