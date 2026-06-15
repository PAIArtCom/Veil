// Package anthropic walks the Anthropic Messages API shape: system strings/blocks,
// messages[].content text/thinking/tool_result fields, and tool-use argument payloads.
// It implements wire.Provider for both the buffered request/response surface and the
// stateful streaming SSE restorer (NewStreamRestorer), which holds cross-event state
// per content block so a CLK_ token split across content_block_delta events is
// reassembled before restore (ADR-0011). Phase 0 provider.
package anthropic
