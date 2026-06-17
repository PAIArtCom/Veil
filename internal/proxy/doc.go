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
// v0.1.0 masks only release-supported provider paths: Anthropic POST
// /v1/messages and OpenAI Responses POST /v1/responses or /responses. Every
// other method/path fails closed rather than acting as a transparent proxy,
// because unsupported provider endpoints can carry plaintext body shapes that
// OpenCloak has not verified how to mask.
package proxy
