// Package stream restores placeholders in streaming responses. It provides a
// stateful chunk-level restorer that tolerates placeholders split across
// arbitrary byte boundaries (the universal case), residual-token scanning at
// flush/end-of-stream, plus an SSE-event-level helper for hosts that already parse SSE. See
// docs/concepts/redaction-model.md.
//
// Status: Phase 0 implemented.
package stream
