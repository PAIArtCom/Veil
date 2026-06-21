# Changelog

All notable changes to OpenCloak are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) once it reaches a tagged
release.

## [Unreleased]

No changes yet.

## [0.1.0] - 2026-06-21

### Added
- Local OpenCloak engine with L1 detection, conflict resolution, deterministic reversible
  `OpenCloak_` tokenization, scoped restore state, and fail-closed policy validation.
- Public SDK text, provider-native wire, and stream restore surfaces.
- Standalone loopback proxy for Anthropic Messages / Claude Code, live-accepted on
  2026-06-17 against real Claude Code traffic.
- OpenAI Responses provider path for Codex CLI with offline verification, sanitized
  fixtures, and local Codex CLI Responses live acceptance as the v0.1.0 OpenAI Responses
  protocol evidence. A separate direct `api.openai.com` official-service run is not
  claimed.
- Maintained `examples/embed` SDK reference integration outside the standalone proxy.
- Local JSON policy file support for `token`, `ignore`, and `block`, with strict
  fail-closed validation for unknown keys, reserved operators, and non-empty `rule_sets`.
- Release documentation: deployment guide, Claude Code guide, Codex guide, SDK API
  reference, security policy, and contribution guide.
- Release hardening: unsupported proxy endpoints fail closed before upstream egress, and
  Anthropic protected text/tool-I/O request-shape drift fails closed instead of silently
  forwarding unchecked plaintext-bearing blocks. Opaque media/document payloads and
  provider thinking/control traces remain outside the v0.1.0 de-identification surface.
- Codex live acceptance: Codex CLI 0.140.0 passed a controlled Responses-wire run through
  OpenCloak with a Responses-compatible upstream. This is the v0.1.0 OpenAI Responses
  protocol evidence.
- CLI policy startup: fixed the no-policy-file path so `opencloak proxy` actually uses the
  built-in default policy instead of passing a typed nil local provider into the engine.

### Security
- Hardened L1 secret suppressors so provider-prefixed credentials in `*_id` fields,
  dash-spelled AWS `Secret-Access-Key` headers, and secret-looking hex values in strong
  secret contexts are not dropped by generic false-positive suppressors.
- Made outbound masking idempotent for existing `OpenCloak_` tokens so residual or orphan tokens
  from earlier turns are not wrapped into nested tokens on a later provider-bound request.
- Suppressed code-reference false positives such as `process.env.API_KEY`,
  `config.get(...)`, and `parseToken(...)` without regressing real secret detection.
- Rejected local policy files whose effective operator coverage ignores every supported
  sensitive type.
- Tightened OpenAI Responses request handling: string `prompt.variables` values are masked,
  while non-string prompt variables, `input_image`, and `input_file` fail closed until
  explicit file/image payload handling exists.
- Escaped provider JSON path keys containing backslashes before applying masked values in
  OpenAI Responses and Anthropic provider walkers.

### Reserved / planned
- OpenAI Chat, Gemini, remote MCP egress classification, L2 default-on semantic PII,
  HTTP/gRPC service, local web console, `redact`, `format_preserving`, and configurable
  rule packs remain planned or reserved.
- A separate direct `api.openai.com` official-service end-to-end run is not part of the
  v0.1.0 release gate and is not claimed.
