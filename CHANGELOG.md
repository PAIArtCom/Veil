# Changelog

All notable changes to OpenCloak are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) once it reaches a tagged
release.

## [Unreleased]

No changes beyond the pending v0.1.0 release notes.

## [0.1.0] - Pending

### Added
- Local OpenCloak engine with L1 detection, conflict resolution, deterministic reversible
  `CLK_` tokenization, scoped restore state, and fail-closed policy validation.
- Public SDK text, provider-native wire, and stream restore surfaces.
- Standalone loopback proxy for Anthropic Messages / Claude Code, live-accepted on
  2026-06-17 against real Claude Code traffic.
- OpenAI Responses provider path for Codex CLI with offline verification, sanitized
  fixtures, and live acceptance still pending before release-candidate readiness.
- Maintained `examples/embed` SDK reference integration outside the standalone proxy.
- Local JSON policy file support for `token`, `ignore`, and `block`, with strict
  fail-closed validation for unknown keys, reserved operators, and non-empty `rule_sets`.
- Release documentation: deployment guide, Claude Code guide, Codex guide, SDK API
  reference, security policy, and contribution guide.

### Reserved / planned
- OpenAI Chat, Gemini, remote MCP egress classification, L2 default-on semantic PII,
  HTTP/gRPC service, local web console, `redact`, `format_preserving`, and configurable
  rule packs remain planned or reserved.
- Codex/OpenAI Responses remains offline-verified until a live controlled Codex acceptance
  run passes.
