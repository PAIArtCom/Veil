// Package anthropic walks the protected text/tool-I/O surface of the Anthropic
// Messages API shape: system text, messages[].content text/tool_result text, and
// tool-use argument payloads.
// It implements wire.Provider for both the buffered request/response surface and the
// stateful streaming SSE restorer (NewStreamRestorer), which holds cross-event state
// per content block so a PAIArtVeil_ token split across content_block_delta events is
// reassembled before restore (ADR-0011). Opaque image/document payloads and provider
// thinking/control traces preserve provider-native semantics and are outside the
// v0.1.0 text replacement contract. Phase 0 provider.
package anthropic
