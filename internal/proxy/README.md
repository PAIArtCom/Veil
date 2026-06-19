# internal/proxy

`internal/proxy` is the reference base-URL proxy transport for local AI coding
tools.

## Purpose

This module implements the standalone HTTP proxy handler that masks supported
provider-bound text/tool-I/O fields, forwards the provider-native request to the
configured upstream, and restores tokens on the trusted local response path. For v0.1.0
it handles Anthropic Messages `POST /v1/messages` and OpenAI Responses `POST
/v1/responses` / `POST /responses`; other paths fail closed before upstream egress.

## Principles

- MUST: Never forward plaintext in a protected text/tool-I/O field when masking, parsing, policy, or state setup fails.
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

- **Plaintext protected-field egress**: Client request bodies can contain secrets and PII in text/tool-I/O fields, so read, mask, or upstream request construction failures for those fields must stop before provider egress. Verified by: proxy_test.go.
- **Credential header disclosure**: Authorization and provider API-key headers cross the handler as pass-through data and must not appear in logs or errors. Verified by: proxy_test.go.
- **Compressed stream restore bypass**: Upstream compressed streams can hide tokens from restore, so the proxy strips client `Accept-Encoding` before upstream egress. Verified by: proxy_test.go.
- **Unsupported endpoint egress**: Unsupported paths can carry plaintext body shapes OpenCloak has not verified, so they must fail closed and not contact upstream. Verified by: proxy_test.go.
- **Provider route confusion**: Anthropic and OpenAI Responses paths select different provider walkers and error envelopes; routing drift can leak plaintext or break local tools. Verified by: proxy_test.go.
- **Streaming restore error visibility**: Streaming restore errors after headers are committed must be logged without raw event payload disclosure. Verified by: proxy_test.go.
- **Capture hygiene**: Tests and live acceptance captures must use throwaway values and commit only sanitized summaries or fixtures. Verified by: ../../docs/architecture/phase-0-acceptance.md.

## Open Questions

- [ ] Which non-release provider endpoints should become wire-aware in a later release? (open since: 2026-06)
