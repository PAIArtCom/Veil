// Package veil is the de-identification engine for the LLM era: it masks
// sensitive values (secrets, PII) in outbound LLM requests and restores them in
// inbound responses — deterministically and reversibly — so a model provider never
// sees real data while the user's tools, files, and tool-calls keep working.
//
// The engine is transport-agnostic. Embed it directly via this package; scaffolded
// transports live under cmd/veil (proxy, serve, console). The integration contract
// is documented in docs/sdk/contract.md.
//
// Status: Phase 0 implements the text engine, Anthropic Messages wire masking/
// restore, streaming restore, and the loopback proxy. Non-Anthropic providers,
// the service API, and the console remain Phase 1+.
package veil
