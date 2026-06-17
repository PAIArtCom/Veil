# internal/proxy

`internal/proxy` is the reference base-URL proxy transport for local AI coding
tools.

## Purpose

This module implements the standalone HTTP proxy handler that masks provider-bound
requests, forwards the masked request to the configured upstream, and restores
tokens on the trusted local response path. For the accepted Phase 0 baseline it
handles Anthropic Messages `POST /v1/messages` and OpenAI Responses
`POST /v1/responses` / `POST /responses`; other paths are relayed transparently.

## Principles

- MUST: Never forward a plaintext protected request when masking, parsing, policy, or state setup fails.
- MUST: Forward the client credential verbatim without storing, logging, or parsing it.
- MUST: Restore buffered and streaming responses before they reach the local AI tool whenever a masked request created state.
- MUST: Keep response errors fixed-shape and sanitized; never include raw provider payloads or secrets.
- SHOULD: Keep proxy tests on loopback `httptest` servers with deterministic throwaway values.

## Boundaries

- Does NOT handle: Listening socket ownership or loopback enforcement (see: ../../cmd/opencloak/README.md).
- Does NOT handle: Credential acquisition, refresh, validation, or storage (see: ../../docs/architecture/decisions/0004-auth-pass-through.md).
- Does NOT handle: Provider walkers directly; it calls the public engine wire and stream surfaces (see: ../../docs/sdk/contract.md).
- Does NOT handle: OpenAI Chat, Gemini, remote MCP, or provider endpoints outside the documented shipped paths until those providers are implemented and verified (see: ../../docs/architecture/formal-release-plan.md).

## Adversarial Surfaces

- **Plaintext provider egress**: Client request bodies can contain secrets and PII, so read, mask, or upstream request construction failures must stop before provider egress. Verified by: proxy_test.go.
- **Credential header disclosure**: Authorization and provider API-key headers cross the handler as pass-through data and must not appear in logs or errors. Verified by: proxy_test.go.
- **Compressed stream restore bypass**: Upstream compressed streams can hide tokens from restore, so the proxy strips client `Accept-Encoding` before upstream egress. Verified by: proxy_test.go.
- **Transparent endpoint limits**: Unsupported paths are transparent by design in Phase 0, so docs must state which endpoints are masked and which are not. Verified by: ../../docs/guides/claude-code.md.
- **Provider route confusion**: Anthropic and OpenAI Responses paths select different provider walkers and error envelopes; routing drift can leak plaintext or break local tools. Verified by: proxy_test.go.
- **Streaming restore error visibility**: Streaming restore errors after headers are committed must be logged without raw event payload disclosure. Verified by: proxy_test.go.
- **Capture hygiene**: Tests and live acceptance captures must use throwaway values and commit only sanitized summaries or fixtures. Verified by: ../../docs/architecture/phase-0-acceptance.md.

## Open Questions

- [ ] Should transparent non-release provider endpoints become fail-closed or wire-aware in a later release? (open since: 2026-06)
