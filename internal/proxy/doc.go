// Package proxy is the standalone base-URL local proxy transport. It is an
// http.Handler that terminates the AI tool's request locally, masks the
// outbound Anthropic /v1/messages JSON body through the OpenCloak engine,
// forwards the masked request to the configured upstream with the client's own
// credential header untouched, and restores tokens on the way back (buffered or
// streamed). See docs/architecture/decisions/0001-base-url-proxy-over-hooks.md
// (base-URL proxy) and 0004-auth-pass-through.md (credential pass-through).
//
// The handler is transport-agnostic: the loopback-only binding mandated by the
// threat model is enforced by the CLI that constructs the listener (cmd/opencloak),
// not here, so the handler stays unit-testable over httptest's ephemeral
// loopback server.
//
// Phase 0 masks only POST /v1/messages. Every other method/path is forwarded
// transparently. Masking count_tokens and other Anthropic endpoints is Phase 1
// scope (the Phase 0 exit criteria are all expressed against /v1/messages).
package proxy
