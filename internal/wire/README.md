# internal/wire

`internal/wire` owns provider-native JSON walkers for masking and restore.

## Purpose

This module defines the provider registry and the `Provider` contract used by the
public engine wire surface. Each provider package extracts protected text and tool-I/O
fields from its own native request shape, applies masked text back into that shape, and
restores placeholders from buffered and streaming provider responses.

## Principles

- MUST: Preserve provider-native JSON shape; do not normalize into a shared intermediate schema.
- MUST: Fail closed on malformed JSON, unsupported operations, or unsupported plaintext-bearing text/tool-I/O shapes.
- MUST: Keep static tool schemas and provider metadata out of masking unless a provider contract identifies them as user data.
- MUST: Keep opaque media/document payloads and provider thinking/control traces out of text replacement unless a future provider contract explicitly supports parsing and regeneration.
- MUST: Restore provider streams with provider-aware event semantics before local tool execution consumes arguments.
- SHOULD: Keep provider packages small, fixture-driven, and isolated behind the registry.

## Boundaries

- Does NOT handle: Detection, masking policy, token storage, or scoped restore state (see: ../detect/README.md and ../../docs/sdk/contract.md).
- Does NOT handle: HTTP routing, credentials, headers, or socket safety (see: ../proxy/README.md and ../../cmd/veil/README.md).
- Does NOT handle: Public third-party provider plugin registration for v0.1.0 (see: ../../docs/architecture/formal-release-plan.md).

## Adversarial Surfaces

- **Provider shape drift**: New provider fields can carry protected text/tool I/O if walkers silently skip unsupported shapes. Opaque media/document payloads and provider thinking/control traces are a declared non-goal, not a skipped text surface. Verified by: anthropic/provider_test.go and openairesponses/provider_test.go.
- **Tool I/O egress**: Tool-call arguments and tool-result outputs can contain restored local values, so provider walkers must cover those request and response fields. Verified by: openairesponses/provider_test.go.
- **Cross-event placeholder splits**: Streaming deltas can split an `PAIArtVeil_` token or format-preserving surrogate across provider events, so stream restorers must hold partial placeholder tails until safe. Verified by: anthropic/stream_test.go and openairesponses/stream_test.go.
- **Static schema mutation**: Tool definitions are provider instructions, not user data, and changing them can break agent behavior. Verified by: anthropic/provider_test.go and openairesponses/provider_test.go.

## Open Questions

- [ ] Should v0.1.x expose a public provider adapter registration API, or keep adapters internal until more provider families are proven? (open since: 2026-06)
- [ ] Which OpenAI Responses optional item types should move from fail-closed to supported after live Codex acceptance expands beyond shell/file tools? (open since: 2026-06)
