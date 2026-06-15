package opencloak

import "context"

// Engine is the OpenCloak de-identification engine. Construct it with New and use it
// directly (embedded in a gateway) or via a scaffolded transport under cmd/opencloak.
//
// The API has three entry surfaces: text, provider-native wire JSON, and streaming
// restore. See docs/sdk/contract.md.
type Engine struct {
	cfg Config
}

// New constructs an Engine from cfg. The zero Config is valid.
func New(cfg Config) (*Engine, error) {
	// TODO(phase0): load the HMAC key, build the L1 detector from the Policy resolved
	// via cfg.Policy (a PolicyProvider), and initialize the scoped mapstore.
	return &Engine{cfg: cfg}, nil
}

// ---- Text surface ---------------------------------------------------------------

// Mask replaces detected sensitive values in text with deterministic, reversible
// tokens (CLK_<TYPE>_<id>). It returns the State needed to restore the same text surface.
//
// TODO(phase0): implement. Until then it returns ErrNotImplemented so callers fail
// closed and never forward plaintext.
func (e *Engine) Mask(ctx context.Context, scope Scope, text string) (masked string, st *State, err error) {
	return "", nil, ErrNotImplemented
}

// Restore replaces tokens in text with their original values using st. A nil State is
// invalid because text restore must never consult an unrelated namespace.
//
// TODO(phase0): implement. Until then it returns ErrNotImplemented after validating st.
func (e *Engine) Restore(ctx context.Context, st *State, text string) (string, error) {
	if st == nil {
		return "", ErrInvalidState
	}
	return "", ErrNotImplemented
}

// ---- Wire-format surface (native provider JSON) ---------------------------------

// MaskRequest masks sensitive values in a provider's native request body and returns
// the masked body plus the State needed to restore the response. scope selects the
// mapstore namespace. provider is e.g. "anthropic" or "openai-responses"; op is the
// operation, e.g. "messages".
//
// TODO(phase0): implement. Until then it returns ErrNotImplemented so callers fail
// closed and never forward plaintext.
func (e *Engine) MaskRequest(ctx context.Context, scope Scope, provider, op string, body []byte) (masked []byte, st *State, err error) {
	return nil, nil, ErrNotImplemented
}

// RestoreResponse restores tokens in a complete (non-streaming) provider response body.
// It uses st.Provider and st.Op to dispatch through the same provider walker that masked
// the request. Restore errors are returned explicitly so callers can audit via ctx or
// choose whether to surface residual tokens to the trusted local user.
//
// TODO(phase0): implement. Until then it returns ErrNotImplemented.
func (e *Engine) RestoreResponse(ctx context.Context, st *State, body []byte) ([]byte, error) {
	if st == nil || st.provider == "" || st.op == "" {
		return nil, ErrInvalidState
	}
	return nil, ErrNotImplemented
}

// ---- Streaming surface ----------------------------------------------------------

// RestoreStreamChunk restores tokens in a raw streamed chunk, holding back partial
// tokens across chunk boundaries. This is the universal streaming method: it works
// even when the host relays raw bytes with arbitrary chunk boundaries. It intentionally
// has no context or error return for the hot relay path; audit around FlushStream.
//
// TODO(phase0): implement. Currently a pass-through placeholder.
func (e *Engine) RestoreStreamChunk(st *State, chunk []byte) []byte { return chunk }

// FlushStream returns any bytes held back by RestoreStreamChunk at end of stream.
//
// TODO(phase0): implement.
func (e *Engine) FlushStream(st *State) []byte { return nil }

// RestoreSSEEvent restores tokens in a single parsed SSE event payload. It dispatches
// via st.Provider/st.Op, so provider-specific event JSON can be restored without
// rewriting non-text fields. Use RestoreStreamChunk for raw byte relay.
//
// TODO(phase0): implement. Until then it returns ErrNotImplemented.
func (e *Engine) RestoreSSEEvent(ctx context.Context, st *State, eventData []byte) ([]byte, error) {
	if st == nil || st.provider == "" || st.op == "" {
		return nil, ErrInvalidState
	}
	return nil, ErrNotImplemented
}
