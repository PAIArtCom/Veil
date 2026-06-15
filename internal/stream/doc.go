// Package stream restores tokens in streaming responses. It provides a stateful
// chunk-level restorer that tolerates tokens split across arbitrary byte boundaries
// (the universal case), residual-token scanning at flush/end-of-stream, plus an
// SSE-event-level helper for hosts that already parse SSE. See
// docs/concepts/redaction-model.md.
//
// Status: scaffold only; no behavior yet.
package stream
