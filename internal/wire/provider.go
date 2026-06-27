package wire

import "errors"

// RestoreFunc restores placeholders in a text value using the State selected by the caller.
// It returns an error for missing mappings, invalid state, or other restore failures.
type RestoreFunc func(text string) (string, error)

// TextSpan is a provider-native text field extracted from request JSON. Path is a stable
// provider-local path that ApplyRequest can use to put the masked text back.
type TextSpan struct {
	Path  string
	Text  string
	Role  string
	Start int // optional byte offset of the JSON string literal in the original body
	End   int // optional byte offset just after the JSON string literal
}

// MaskedSpan is a text span after masking, ready to be applied back to provider JSON.
type MaskedSpan struct {
	Path       string
	MaskedText string
	Start      int // optional byte offset copied from TextSpan.Start
	End        int // optional byte offset copied from TextSpan.End
}

// ErrStreamingUnsupported is returned by NewStreamRestorer for providers/ops
// that have no streaming restorer yet (callers fail closed or fall back).
var ErrStreamingUnsupported = errors.New("wire: streaming restore unsupported")

// StreamRestorer restores placeholders across a provider's SSE event sequence,
// holding cross-event state per content block. One StreamRestorer serves one
// response stream and is single-writer.
type StreamRestorer interface {
	// Event consumes ONE complete provider SSE event payload (the bytes of a
	// single `data:` JSON value, already frame-reassembled by the caller) and
	// returns zero or more complete event payloads to emit downstream, in order
	// (more when a held tail flushes alongside a stop; fewer when a delta is
	// buffered). restore replaces complete placeholders and counts residual opaque tokens.
	Event(eventData []byte, restore RestoreFunc) ([][]byte, error)
	// Flush returns any events still held at end of stream.
	Flush(restore RestoreFunc) ([][]byte, error)
}

// Provider extracts and applies text for one provider family while preserving the
// provider's native JSON shape. It does not normalize requests into a shared IR.
type Provider interface {
	ExtractRequest(op string, body []byte) ([]TextSpan, error)
	ApplyRequest(op string, body []byte, spans []MaskedSpan) ([]byte, error)
	RestoreResponse(op string, body []byte, restore RestoreFunc) ([]byte, error)
	RestoreSSEEvent(op string, eventData []byte, restore RestoreFunc) ([]byte, error)
	// NewStreamRestorer returns a fresh, single-stream StreamRestorer for op, or
	// ErrStreamingUnsupported when the provider has no streaming restorer for it.
	NewStreamRestorer(op string) (StreamRestorer, error)
}
