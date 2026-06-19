# examples/embed

`examples/embed` is the maintained reference integration for embedding the
OpenCloak SDK in a gateway-style request/response path.

## Purpose

This module proves the public SDK contract outside the standalone proxy. It
models the lowest-common-denominator gateway seams: a buffered outbound provider
request, a buffered inbound provider response, a raw byte streaming relay, and a
parsed SSE event relay.

## Principles

- MUST: Use only the public `github.com/cloakia/opencloak` package.
- MUST: Mask the protected text/tool-I/O surface exactly once before provider egress and keep the returned `State` with the matching response lifecycle.
- MUST: Fail closed on outbound mask errors; callers must not forward the original request body.
- MUST: Restore buffered responses, raw stream chunks, and parsed SSE events only with the matching `State`.
- SHOULD: Keep examples deterministic and credential-free with throwaway fixture values.

## Boundaries

- Does NOT handle: A production gateway, network listener, auth, retry scheduler, or provider client (see: ../../docs/sdk/integration-guide.md).
- Does NOT handle: External clipal repository integration; this is a maintained in-repo reference path (see: ../../docs/architecture/formal-release-plan.md).
- Does NOT handle: OpenAI Chat, Gemini, or other unimplemented providers (see: ../../docs/architecture/system-design.md).
- Does NOT handle: Policy file parsing or local configuration behavior (see: ../../docs/architecture/formal-release-plan.md).

## Adversarial Surfaces

- **Plaintext protected-field egress**: The outbound seam must return masked provider bytes for the protected text/tool-I/O surface and an error must stop forwarding that original protected payload. Verified by: gateway_test.go.
- **State lifecycle mismatch**: Buffered, raw-stream, and parsed-SSE restore must use the `State` returned by the matching outbound mask call. Verified by: gateway_test.go.
- **Cross-scope restore**: A token minted in one scope must not restore through another scope's state. Verified by: gateway_test.go.
- **Stream split handling**: Raw stream restore must tolerate arbitrary byte chunk boundaries and flush held tails at stream end. Verified by: gateway_test.go.
- **Provider claim scope**: The example must not imply Codex/OpenAI Responses, Gemini, or a real external gateway integration is shipped. Verified by: README.md.

## Open Questions

- [ ] Should the first external gateway validation target remain clipal after the maintained reference path is accepted? (open since: 2026-06)
- [ ] Should a future example include retry/failover state ownership once non-Anthropic providers ship? (open since: 2026-06)
