# cmd/opencloak

`cmd/opencloak` is the user-facing command-line entry point for the OpenCloak
engine and local transports.

## Purpose

This module wires the public OpenCloak SDK and internal transports into the
`opencloak` binary. For v0.1.0 it exposes the standalone loopback proxy for
Anthropic Messages and OpenAI Responses, stable help output, and build version
metadata for release verification.

## Principles

- MUST: Enforce loopback-only listening before constructing a network server.
- MUST: Keep provider credentials outside the engine and proxy configuration.
- MUST: Keep unimplemented subcommands explicit and fail closed with non-zero exits.
- SHOULD: Keep command output stable enough for clean-checkout release verification.
- SHOULD: Put transport behavior in internal packages and keep this module as wiring.

## Boundaries

- Does NOT handle: Provider-native JSON walking or restore logic (see: `internal/wire` and `internal/stream`).
- Does NOT handle: Local policy file parsing in v0.1.0 R1 (see: `internal/config`).
- Does NOT handle: HTTP/gRPC service, web console, or one-shot masking behavior until those subcommands are implemented (see: ../../docs/architecture/system-design.md).
- Does NOT handle: Provider credential acquisition or storage (see: ../../docs/architecture/decisions/0004-auth-pass-through.md).

## Adversarial Surfaces

- **Loopback binding**: A non-loopback `--addr` would expose a credential pass-through proxy as an open relay. Verified by: main_test.go.
- **CLI output disclosure**: Startup and error output must not print request bodies, provider response bodies, authorization headers, API keys, local keys, or raw secrets. Verified by: main.go.
- **Placeholder command scope**: Placeholder commands must not silently run partial behavior or imply shipped support. Verified by: main_test.go.
- **Version claim scope**: Release version metadata must be informational only and must not expand the documented API or compatibility claim. Verified by: ../../docs/guides/deployment.md.

## Open Questions

- [ ] Which release build pipeline should inject final `version`, `commit`, and `buildDate` values? (open since: 2026-06)
- [ ] Which subcommand after `proxy` should become implemented first for Phase 1? (open since: 2026-06)
