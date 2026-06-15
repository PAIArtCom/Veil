package wire

// RestoreFunc restores tokens in a text value using the State selected by the caller.
// It returns an error for missing mappings, invalid state, or other restore failures.
type RestoreFunc func(text string) (string, error)

// TextSpan is a provider-native text field extracted from request JSON. Path is a stable
// provider-local path that ApplyRequest can use to put the masked text back.
type TextSpan struct {
	Path string
	Text string
	Role string
}

// MaskedSpan is a text span after masking, ready to be applied back to provider JSON.
type MaskedSpan struct {
	Path       string
	MaskedText string
}

// Provider extracts and applies text for one provider family while preserving the
// provider's native JSON shape. It does not normalize requests into a shared IR.
type Provider interface {
	ExtractRequest(op string, body []byte) ([]TextSpan, error)
	ApplyRequest(op string, body []byte, spans []MaskedSpan) ([]byte, error)
	RestoreResponse(op string, body []byte, restore RestoreFunc) ([]byte, error)
	RestoreSSEEvent(op string, eventData []byte, restore RestoreFunc) ([]byte, error)
}
