# ADR-0013 — OpenAI Responses provider for Codex CLI

**Status:** Accepted

## Context

Veil v0.1.0 must support a second high-value AI coding egress path after the
accepted Claude Code / Anthropic Messages baseline. Codex CLI speaks the OpenAI
Responses API through configurable `model_providers` entries. A local controlled
Codex CLI 0.140.0 capture against a loopback fake upstream confirmed `POST
/v1/responses`, `stream=true`, an array `input`, top-level `instructions`, static
`tools`, and Responses SSE framing. The capture used only throwaway fixture data and is
recorded as sanitized runtime evidence, not as a raw provider payload.

OpenAI Responses is not OpenAI Chat Completions. Its agentic stream carries tool-call
arguments and text through event families such as `response.output_text.delta`,
`response.function_call_arguments.delta`, MCP/custom-tool argument deltas, and final
output-item/completed events.

## Decision

Veil adds an internal provider named `openai-responses` for the Responses
operation only.

The provider masks:

- top-level `instructions`;
- string values in `prompt.variables`;
- message `input_text` / text content;
- `function_call_output.output` tool results;
- agentic call argument/input/code fields when they appear in provider-bound input.

The provider restores:

- buffered message output text and refusal text;
- function-call, MCP-call, custom-tool, code-interpreter, and function-call-output fields;
- streaming output text/refusal deltas with cross-event token holdback;
- streaming tool argument/input/code deltas by buffering complete argument text and
  emitting a restored synthetic delta before the corresponding done event.

Static `tools` definitions, prompt ids, model fields, cache keys, `client_metadata`, and
provider control fields are not masked in v0.1.0. Unknown plaintext-bearing request item
shapes fail closed instead of forwarding the request unchanged. `input_image`,
`input_file`, and non-string prompt variables fail closed until v0.1.0 has explicit
file/image payload handling.

The standalone proxy routes `POST /v1/responses` and `POST /responses` to this provider
while preserving `POST /v1/messages` for Anthropic Messages. This ADR does not add OpenAI
Chat, Gemini, remote MCP egress classification, or a public provider plugin API.

## Alternatives considered

- **Treat Responses as transparent until live Codex acceptance.** Rejected: the release plan
  requires offline fixtures and provider-aware tests before live acceptance, and transparent
  provider egress would leak plaintext.
- **Normalize OpenAI and Anthropic into one internal message schema.** Rejected: ADR-0002 and
  the SDK contract keep provider-native walkers to avoid losing provider-specific tool and
  stream semantics.
- **Mask static `tools` definitions.** Rejected: tool schemas are provider instructions, not
  local user/tool output; masking them can break tool selection and cache behavior.
- **Emit restored tool arguments only in done events.** Rejected: Codex and other streaming
  clients may build tool arguments from deltas, so the stream restorer emits a restored
  synthetic delta before the done event.

## Consequences

- Codex CLI can be supported through the same loopback base-URL proxy architecture as
  Claude Code, with provider-specific fail-closed behavior.
- OpenAI Responses support remains bounded to the verified Responses operation and sanitized
  fixture shapes until live acceptance expands coverage.
- Provider shape drift becomes a release risk: new Responses input item types that can carry
  plaintext must either be added deliberately or continue to fail closed.
- The R3 release gate still needs live controlled Codex acceptance before documentation can
  claim full release readiness.
