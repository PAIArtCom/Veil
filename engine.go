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
	"github.com/cloakia/opencloak/internal/wire"

	// Side-effect import: registers the "anthropic" provider in the wire
	// registry when the engine package is imported.
	_ "github.com/cloakia/opencloak/internal/wire/anthropic"
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

// maskText is the shared masking core used by both Mask and MaskRequest.
// It detects sensitive values in text, masks them using the resolved policy,
// accumulates token→value entries in the store under scope, and returns the
// masked text. If any type hits OperatorBlock the blocked types are returned
// (text will equal the original). Detection errors are returned as-is
// (fail-closed).
func (e *Engine) maskText(ctx context.Context, policy types.Policy, scope Scope, text string) (masked string, blocked []Type, err error) {
	preFilter := ignoredByPolicy(policy)

	findings, err := e.detector.Detect(ctx, text, preFilter)
	if err != nil {
		return "", nil, err
	}

	result := mask.Apply(text, findings, scope, policy, e.store, e.keyer, e.collisions)

	if len(result.Blocked) > 0 {
		blockedTypes := make([]Type, len(result.Blocked))
		for i, t := range result.Blocked {
			blockedTypes[i] = t
		}
		sort.Slice(blockedTypes, func(i, j int) bool {
			return string(blockedTypes[i]) < string(blockedTypes[j])
		})
		return "", blockedTypes, nil
	}

	return result.Masked, nil, nil
}

// Mask replaces detected sensitive values in text with deterministic, reversible
// tokens (CLK_<TYPE>_<id>). It returns the State needed to restore the same text surface.
func (e *Engine) Mask(ctx context.Context, scope Scope, text string) (masked string, st *State, err error) {
	policy, err := e.activePolicy(ctx, scope)
	if err != nil {
		return "", nil, err
	}

	maskedText, blocked, err := e.maskText(ctx, policy, scope, text)
	if err != nil {
		return "", nil, err
	}

	if len(blocked) > 0 {
		return "", nil, &BlockedError{Types: blocked}
	}

	return maskedText, &State{scope: scope}, nil
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
func (e *Engine) MaskRequest(ctx context.Context, scope Scope, provider, op string, body []byte) (masked []byte, st *State, err error) {
	policy, err := e.activePolicy(ctx, scope)
	if err != nil {
		return nil, nil, err
	}

	prov, err := wire.Lookup(provider)
	if err != nil {
		return nil, nil, err
	}

	spans, err := prov.ExtractRequest(op, body)
	if err != nil {
		return nil, nil, err
	}

	// Accumulate all blocked types across spans.
	var allBlocked []Type
	maskedSpans := make([]wire.MaskedSpan, 0, len(spans))

	for _, span := range spans {
		maskedText, blocked, maskErr := e.maskText(ctx, policy, scope, span.Text)
		if maskErr != nil {
			return nil, nil, maskErr
		}
		if len(blocked) > 0 {
			allBlocked = append(allBlocked, blocked...)
		}
		maskedSpans = append(maskedSpans, wire.MaskedSpan{
			Path:       span.Path,
			MaskedText: maskedText,
		})
	}

	if len(allBlocked) > 0 {
		// Deduplicate and sort.
		seen := make(map[Type]struct{}, len(allBlocked))
		unique := allBlocked[:0]
		for _, t := range allBlocked {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				unique = append(unique, t)
			}
		}
		sort.Slice(unique, func(i, j int) bool {
			return string(unique[i]) < string(unique[j])
		})
		return nil, nil, &BlockedError{Types: unique}
	}

	maskedBody, err := prov.ApplyRequest(op, body, maskedSpans)
	if err != nil {
		return nil, nil, err
	}

	return maskedBody, &State{scope: scope, provider: provider, op: op}, nil
}

// RestoreResponse restores tokens in a complete (non-streaming) provider response body.
// It uses st.Provider and st.Op to dispatch through the same provider walker that masked
// the request. Restore errors are returned explicitly so callers can audit via ctx or
// choose whether to surface residual tokens to the trusted local user.
func (e *Engine) RestoreResponse(ctx context.Context, st *State, body []byte) ([]byte, error) {
	if st == nil || st.provider == "" || st.op == "" {
		return nil, ErrInvalidState
	}

	prov, err := wire.Lookup(st.provider)
	if err != nil {
		return nil, err
	}

	restoreFunc := e.makeRestoreFunc(st)
	return prov.RestoreResponse(st.op, body, restoreFunc)
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
func (e *Engine) RestoreSSEEvent(ctx context.Context, st *State, eventData []byte) ([]byte, error) {
	if st == nil || st.provider == "" || st.op == "" {
		return nil, ErrInvalidState
	}

	prov, err := wire.Lookup(st.provider)
	if err != nil {
		return nil, err
	}

	restoreFunc := e.makeRestoreFunc(st)
	return prov.RestoreSSEEvent(st.op, eventData, restoreFunc)
}

// makeRestoreFunc returns a RestoreFunc that replaces CLK_… tokens found in
// text with their stored values under st's scope. Unknown tokens are left as-is
// (consistent with the text-surface Restore behavior).
func (e *Engine) makeRestoreFunc(st *State) wire.RestoreFunc {
	scope := st.scope
	return func(text string) (string, error) {
		result := tokenRe.ReplaceAllStringFunc(text, func(tok string) string {
			if v, ok := e.store.Get(scope, tok); ok {
				return v
			}
			return tok
		})
		return result, nil
	}
}
