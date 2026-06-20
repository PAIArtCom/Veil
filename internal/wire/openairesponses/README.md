# internal/wire/openairesponses

`internal/wire/openairesponses` rewrites the OpenAI Responses API shape used by
Codex CLI.

## Purpose

This provider supports the OpenAI Responses `POST /v1/responses` operation for
local OpenCloak proxy and SDK callers. It masks user/developer input text and
local tool-result output before provider egress, then restores output text and
agentic tool-call arguments before the trusted local Codex side consumes them.

## Principles

- MUST: Support only the Responses operation; OpenAI Chat Completions is out of scope.
- MUST: Mask `instructions`, prompt string variables, message text content, function-call arguments, and function-call/tool output fields that can carry local values.
- MUST: Skip static `tools` definitions, `client_metadata`, cache keys, model names, and provider control fields.
- MUST: Restore buffered and streaming output text, function-call arguments, MCP/custom tool arguments, and code-interpreter code fields.
- MUST: Fail closed on malformed JSON and unsupported plaintext-bearing input item shapes.
- MUST: Fail closed on `input_image`, `input_file`, and non-string prompt variables until v0.1.0 has explicit file/image payload handling.
- SHOULD: Base fixtures on sanitized Codex CLI/OpenAI Responses shapes, not raw provider captures.

## Boundaries

- Does NOT handle: OpenAI Chat Completions, Gemini, Anthropic Messages, or remote MCP egress classification (see: ../../../docs/architecture/formal-release-plan.md).
- Does NOT handle: Credential acquisition or OpenAI account login; the proxy forwards caller headers verbatim (see: ../../../docs/architecture/decisions/0004-auth-pass-through.md).
- Does NOT handle: Live acceptance secrets or raw provider captures; only sanitized summaries and fixtures belong in the repository (see: ../../../docs/guides/codex.md).

## Adversarial Surfaces

- **Codex request drift**: Codex can add new Responses input item types; unknown plaintext-bearing request items must fail before provider egress. Verified by: provider_test.go.
- **Prompt variable leakage**: Stored-prompt variables can carry local user or tool data even when the prompt id itself is provider control metadata. String variables are masked and non-string variables fail closed. Verified by: provider_test.go.
- **File/image payload references**: `input_image` and `input_file` blocks can carry signed URLs, file ids, or inline payloads that v0.1.0 does not parse. These blocks fail closed before provider egress. Verified by: provider_test.go.
- **Tool-result leakage**: `function_call_output.output` can contain local file or shell data and must be masked on the next provider-bound request. Verified by: provider_test.go.
- **Streaming tool arguments**: `response.function_call_arguments.delta` and related delta streams can split `OpenCloak_` tokens across events; arguments are buffered and restored before local tool execution sees them. Verified by: stream_test.go.
- **Static tool schema integrity**: `tools` entries may contain example-looking strings but are static provider instructions and must not be masked. Verified by: provider_test.go.
- **Capture hygiene**: Tests use throwaway fixture values only and must not commit raw Codex/OpenAI captures. Verified by: ../../../docs/guides/codex.md.

## Open Questions

- [ ] Which additional Responses event families should be admitted after live Codex coverage confirms they carry local tool I/O? (open since: 2026-06)
- [ ] Should non-Codex OpenAI Responses clients opt into a stricter fail-closed mode for unknown output item types? (open since: 2026-06)
