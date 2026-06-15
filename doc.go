// Package opencloak is the de-identification engine for the LLM era: it masks
// sensitive values (secrets, PII) in outbound LLM requests and restores them in
// inbound responses — deterministically and reversibly — so a model provider never
// sees real data while the user's tools, files, and tool-calls keep working.
//
// The engine is transport-agnostic. Embed it directly via this package; scaffolded
// transports live under cmd/opencloak (proxy, serve, console). The integration contract
// is documented in docs/sdk/contract.md.
//
// Status: scaffold only. Method bodies are not implemented yet; text and wire operations
// return errors so callers fail closed and never forward plaintext.
package opencloak
