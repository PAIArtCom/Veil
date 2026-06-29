// Package veil is the de-identification engine for the LLM era: it masks
// sensitive values (secrets, PII) in outbound LLM requests and restores them in
// inbound responses — deterministically and reversibly — so a model provider never
// sees real data while the user's tools, files, and tool-calls keep working.
//
// The engine is transport-agnostic. Embed it directly via this package; scaffolded
// transports live under cmd/veil (proxy, serve, console). The integration contract
// is documented in docs/sdk/contract.md.
//
// Status: v0.1.0 implements the text engine, streaming restore, the loopback
// proxy for Anthropic Messages and OpenAI Responses, local policy files, and
// the maintained SDK embed reference path. OpenAI Chat Completions, Gemini, the
// service API, and the console remain Phase 1+.
package veil
