package opencloak

import (
	"context"
	"regexp"
	"sort"

	"github.com/cloakia/opencloak/internal/detect"
	"github.com/cloakia/opencloak/internal/mapstore"
	"github.com/cloakia/opencloak/internal/mask"
	"github.com/cloakia/opencloak/internal/token"
	"github.com/cloakia/opencloak/internal/types"
)

// Engine is the OpenCloak de-identification engine. Construct it with New and use it
// directly (embedded in a gateway) or via a scaffolded transport under cmd/opencloak.
//
// The API has three entry surfaces: text, provider-native wire JSON, and streaming
// restore. See docs/sdk/contract.md.
type Engine struct {
	cfg        Config
	store      *mapstore.Store
	keyer      *token.Keyer
	detector   *detect.Orchestrator
	collisions map[string]string
}

// detectorAdapter wraps an opencloak.Detector (which uses the public type
// aliases) so it can be passed to detect.Orchestrator as an ExternalDetector.
// Because opencloak.Finding == types.Finding (type alias), no conversion is
// needed — but we need a separate type to satisfy the interface.
type detectorAdapter struct {
	d Detector
}

func (a *detectorAdapter) Detect(ctx context.Context, text string) ([]types.Finding, error) {
	return a.d.Detect(ctx, text)
}

// New constructs an Engine from cfg. The zero Config is valid.
func New(cfg Config) (*Engine, error) {
	keyer, err := token.NewKeyer(cfg.KeyPath)
	if err != nil {
		return nil, err
	}

	var ext detect.ExternalDetector
	if cfg.Detector != nil {
		ext = &detectorAdapter{d: cfg.Detector}
	}

	return &Engine{
		cfg:        cfg,
		store:      mapstore.New(),
		keyer:      keyer,
		detector:   detect.New(ext),
		collisions: make(map[string]string),
	}, nil
}

// defaultPolicy is the built-in local policy: structured and secret types are
// enabled with OperatorToken; PERSON and ADDR are ignored (L2, off by default).
var defaultPolicy = types.Policy{
	DefaultOperator: types.OperatorToken,
	Types: map[types.Type]types.TypePolicy{
		types.TypePerson: {Operator: types.OperatorIgnore},
		types.TypeAddr:   {Operator: types.OperatorIgnore},
	},
}

// activePolicy resolves the Policy for this call. If e.cfg.Policy is set it
// is consulted; otherwise defaultPolicy is used.
func (e *Engine) activePolicy(ctx context.Context, scope Scope) (types.Policy, error) {
	if e.cfg.Policy != nil {
		return e.cfg.Policy.Policy(ctx, scope)
	}
	return defaultPolicy, nil
}

// ---- Text surface ---------------------------------------------------------------

// ignoredByPolicy returns a predicate that reports whether a finding's type
// resolves to OperatorIgnore under policy. Used as a pre-resolution filter so
// ignored-type findings cannot suppress overlapping maskable findings during
// conflict resolution.
func ignoredByPolicy(policy types.Policy) func(types.Finding) bool {
	defOp := policy.DefaultOperator
	if defOp == "" {
		defOp = types.OperatorToken
	}
	return func(f types.Finding) bool {
		op := defOp
		if tp, ok := policy.Types[f.Type]; ok && tp.Operator != "" {
			op = tp.Operator
		}
		// Return true to KEEP the finding (preFilter keeps when true).
		return op != types.OperatorIgnore
	}
}

// Mask replaces detected sensitive values in text with deterministic, reversible
// tokens (CLK_<TYPE>_<id>). It returns the State needed to restore the same text surface.
func (e *Engine) Mask(ctx context.Context, scope Scope, text string) (masked string, st *State, err error) {
	policy, err := e.activePolicy(ctx, scope)
	if err != nil {
		return "", nil, err
	}

	// Build a pre-resolution filter so ignored-type findings are stripped
	// before conflict resolution runs. This prevents an ignored finding from
	// winning a cross-type conflict and thereby suppressing a maskable one.
	preFilter := ignoredByPolicy(policy)

	findings, err := e.detector.Detect(ctx, text, preFilter)
	if err != nil {
		return "", nil, err
	}

	result := mask.Apply(text, findings, scope, policy, e.store, e.keyer, e.collisions)

	if len(result.Blocked) > 0 {
		blocked := make([]Type, len(result.Blocked))
		for i, t := range result.Blocked {
			blocked[i] = t
		}
		sort.Slice(blocked, func(i, j int) bool {
			return string(blocked[i]) < string(blocked[j])
		})
		return "", nil, &BlockedError{Types: blocked}
	}

	return result.Masked, &State{scope: scope}, nil
}

// tokenRe matches any CLK_… token in text.
var tokenRe = regexp.MustCompile(token.TokenPattern)

// Restore replaces tokens in text with their original values using st. A nil State is
// invalid because text restore must never consult an unrelated namespace.
func (e *Engine) Restore(ctx context.Context, st *State, text string) (string, error) {
	if st == nil {
		return "", ErrInvalidState
	}
	scope := st.scope
	result := tokenRe.ReplaceAllStringFunc(text, func(tok string) string {
		if v, ok := e.store.Get(scope, tok); ok {
			return v
		}
		return tok // unknown token → leave as-is
	})
	return result, nil
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
