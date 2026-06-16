package opencloak

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/cloakia/opencloak/internal/detect"
	"github.com/cloakia/opencloak/internal/mapstore"
	"github.com/cloakia/opencloak/internal/mask"
	"github.com/cloakia/opencloak/internal/stream"
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
	cfg         Config
	store       *mapstore.Store
	keyer       *token.Keyer
	detector    *detect.Orchestrator
	collisionMu sync.Mutex
	collisions  map[string]string
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
// DATE is detected by L1 but ignored by default: most dates (timestamps,
// version dates) are not sensitive and masking them all hurts model utility
// with little privacy gain. A caller/Cloakia policy can opt in per type.
var defaultPolicy = types.Policy{
	DefaultOperator: types.OperatorToken,
	Types: map[types.Type]types.TypePolicy{
		types.TypePerson: {Operator: types.OperatorIgnore},
		types.TypeAddr:   {Operator: types.OperatorIgnore},
		types.TypeDate:   {Operator: types.OperatorIgnore},
	},
}

// activePolicy resolves the Policy for this call. If e.cfg.Policy is set it
// is consulted; otherwise defaultPolicy is used.
func (e *Engine) activePolicy(ctx context.Context, scope Scope) (types.Policy, error) {
	var policy types.Policy
	if e.cfg.Policy != nil {
		p, err := e.cfg.Policy.Policy(ctx, scope)
		if err != nil {
			return types.Policy{}, err
		}
		policy = p
	} else {
		policy = defaultPolicy
	}
	if err := validatePolicy(policy); err != nil {
		return types.Policy{}, err
	}
	return policy, nil
}

// validatePolicy enforces the Phase 0 policy matrix before detection/masking
// starts. Unknown or deferred operators/features fail closed even if a particular
// request would not have produced a finding: policy uncertainty is not a
// pass-through mode.
func validatePolicy(policy types.Policy) error {
	if len(policy.RuleSets) > 0 {
		return &UnsupportedPolicyFeatureError{Feature: "RuleSets"}
	}
	if err := validateOperator("", policy.DefaultOperator); err != nil {
		return err
	}
	for typ, tp := range policy.Types {
		if err := validateOperator(typ, tp.Operator); err != nil {
			return err
		}
	}
	return nil
}

func validateOperator(typ types.Type, op types.TransformOperator) error {
	if op == "" {
		return nil
	}
	switch op {
	case types.OperatorToken, types.OperatorBlock, types.OperatorIgnore:
		return nil
	case types.OperatorFormatPreserving, types.OperatorRedact:
		return &UnsupportedOperatorError{Type: typ, Operator: op}
	default:
		return &UnsupportedOperatorError{Type: typ, Operator: op}
	}
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

	e.collisionMu.Lock()
	result, err := mask.Apply(text, findings, scope, policy, e.store, e.keyer, e.collisions)
	e.collisionMu.Unlock()
	if err != nil {
		return "", nil, fmt.Errorf("opencloak: mask text: %w", err)
	}

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

	restoreFunc, residuals := e.restoreFuncTracking(st)
	out, restoreErr := prov.RestoreResponse(st.op, body, restoreFunc)
	e.auditResidual(ctx, residuals)
	return out, restoreErr
}

// ---- Streaming surface ----------------------------------------------------------

// RestoreStreamChunk restores tokens in a raw streamed chunk, holding back partial
// tokens across chunk boundaries. This is the universal streaming method: it works
// even when the host relays raw bytes with arbitrary chunk boundaries. It returns
// the bytes that are safe to forward now; bytes belonging to a token that may be
// completed by a later chunk are held internally until proven complete (then they
// surface on a subsequent call) or until FlushStream.
//
// A nil State disables restore: the chunk is returned unchanged. The holdback
// restorer is created lazily on the first call and bound to st's scope. There is
// intentionally no context or error return on this hot relay path; residual-token
// auditing happens once at FlushStream.
func (e *Engine) RestoreStreamChunk(st *State, chunk []byte) []byte {
	if st == nil {
		return chunk
	}
	if st.stream == nil {
		st.stream = stream.NewRestorer(e.lookup(st.scope))
	}
	return st.stream.Write(chunk)
}

// FlushStream restores and returns any bytes held back by RestoreStreamChunk at
// end of stream, and emits a single residual-token audit event for the whole
// stream if any validly-shaped but unknown tokens were seen. It returns nil and
// records nothing when st is nil or st never streamed.
//
// FlushStream has no caller context by design (the relay's request context may
// already be finished when the body drains), so it audits with
// context.Background().
func (e *Engine) FlushStream(st *State) []byte {
	if st == nil || st.stream == nil {
		return nil
	}
	out := st.stream.Flush()

	if e.cfg.Audit != nil {
		if counts := residualCounts(st.stream.ResidualCounts()); len(counts) > 0 {
			e.cfg.Audit.Record(context.Background(), AuditEvent{
				Kind:   "residual_token",
				Counts: counts,
			})
		}
	}
	return out
}

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

	restoreFunc, residuals := e.restoreFuncTracking(st)
	out, restoreErr := prov.RestoreSSEEvent(st.op, eventData, restoreFunc)
	e.auditResidual(ctx, residuals)
	return out, restoreErr
}

// SSEStream is a stateful, single-stream SSE restorer. It wraps a provider's
// wire.StreamRestorer with the scope-bound restore closure and accumulates
// residual-token counts across the whole stream, emitting one residual_token
// audit event at Flush — mirroring the RestoreStreamChunk/FlushStream lifecycle
// (accumulate while relaying, audit once at end of stream).
//
// Unlike the stateless RestoreSSEEvent, an SSEStream holds the provider's
// cross-event holdback (e.g. a CLK_ token split across content_block_delta
// events), so the proxy can drive it one complete event at a time and have
// split tokens reassembled before any restore is attempted.
//
// Concurrency: an SSEStream is single-writer. One instance serves exactly one
// response stream, driven sequentially by one relay goroutine; it holds no lock.
type SSEStream struct {
	sr       wire.StreamRestorer
	engine   *Engine
	restore  wire.RestoreFunc
	residual map[string]int // residual token TYPE -> count, accumulated over the stream
}

// NewSSEStreamRestorer returns a stateful SSE restorer bound to st (dispatching
// via st.provider/st.op), wrapping the provider walker with the scope-bound
// restore closure and residual-token auditing. A nil/incomplete State or an
// unsupported provider returns an error (fail-closed): the proxy must obtain a
// restorer before relaying any body, so a failure here aborts the stream rather
// than forwarding tokens unrestored.
func (e *Engine) NewSSEStreamRestorer(st *State) (*SSEStream, error) {
	if st == nil || st.provider == "" || st.op == "" {
		return nil, ErrInvalidState
	}
	prov, err := wire.Lookup(st.provider)
	if err != nil {
		return nil, err
	}
	sr, err := prov.NewStreamRestorer(st.op)
	if err != nil {
		// Includes wire.ErrStreamingUnsupported; surfaced for fail-closed handling.
		return nil, err
	}
	residual := make(map[string]int)
	return &SSEStream{
		sr:       sr,
		engine:   e,
		restore:  e.restoreScan(st.scope, residual),
		residual: residual,
	}, nil
}

// Event consumes one complete provider SSE event payload and returns zero or
// more complete event payloads to emit downstream, in order. Residual tokens are
// counted into the stream's running total and audited once at Flush.
func (s *SSEStream) Event(ctx context.Context, eventData []byte) ([][]byte, error) {
	return s.sr.Event(eventData, s.restore)
}

// Flush returns any events still held at end of stream and, if an audit sink is
// configured and any residual tokens were seen across the stream, records a
// single residual_token audit event with the accumulated counts. It uses the
// caller's ctx so a relay can attribute residuals to the in-flight request.
func (s *SSEStream) Flush(ctx context.Context) ([][]byte, error) {
	out, err := s.sr.Flush(s.restore)
	if s.engine.cfg.Audit != nil {
		if counts := residualCounts(s.residual); len(counts) > 0 {
			s.engine.cfg.Audit.Record(ctx, AuditEvent{Kind: "residual_token", Counts: counts})
		}
	}
	return out, err
}

// lookup returns a token→value resolver bound to scope. It is the single place
// that consults the mapstore for restore, shared by the wire restore closures
// and the streaming holdback Restorer.
func (e *Engine) lookup(scope Scope) func(string) (string, bool) {
	return func(tok string) (string, bool) {
		return e.store.Get(scope, tok)
	}
}

// restoreScan returns a RestoreFunc that replaces CLK_… tokens found in text
// with their stored values under scope (unknown tokens left as-is, consistent
// with the text-surface Restore), counting every validly-shaped but unresolved
// token into residual by TYPE string. It is the single token-scan shared by the
// buffered/SSE-event surface (a per-call residual map) and the stateful SSE
// stream (a per-stream residual map accumulated across many Event calls). The
// caller owns residual and converts it for audit; all invocations happen on the
// caller's single goroutine, so no locking is needed.
func (e *Engine) restoreScan(scope Scope, residual map[string]int) wire.RestoreFunc {
	lookup := e.lookup(scope)
	return func(text string) (string, error) {
		result := tokenRe.ReplaceAllStringFunc(text, func(tok string) string {
			if v, ok := lookup(tok); ok {
				return v
			}
			// Unknown but validly-shaped token: leave as-is and count by TYPE.
			if typ, ok := token.ParseType(tok); ok {
				residual[typ]++
			}
			return tok
		})
		return result, nil
	}
}

// restoreFuncTracking returns a RestoreFunc that replaces CLK_… tokens found in
// text with their stored values under st's scope (unknown tokens left as-is,
// consistent with the text-surface Restore) plus a snapshot accessor that
// reports residual tokens — validly-shaped CLK_… tokens that were not found in
// st's scope — grouped by TYPE. Callers use it to emit a residual_token audit
// event after a buffered or SSE-event restore, mirroring the streaming surface.
//
// The closure and the snapshot share a residual map; both are invoked by the
// same goroutine within one RestoreResponse/RestoreSSEEvent call, so no locking
// is needed.
func (e *Engine) restoreFuncTracking(st *State) (wire.RestoreFunc, func() map[Type]int) {
	residual := make(map[string]int)
	restore := e.restoreScan(st.scope, residual)
	snapshot := func() map[Type]int { return residualCounts(residual) }
	return restore, snapshot
}

// residualCounts converts a TYPE-string→count map (as produced by the token
// grammar parser) into the Type-keyed map carried by AuditEvent.Counts. It
// returns a non-nil map only when there is at least one count, so callers can
// use len(...) == 0 to skip emitting an empty audit event.
func residualCounts(byType map[string]int) map[Type]int {
	if len(byType) == 0 {
		return nil
	}
	out := make(map[Type]int, len(byType))
	for k, v := range byType {
		out[Type(k)] = v
	}
	return out
}

// auditResidual records a single residual_token audit event for the supplied
// snapshot when an audit sink is configured and at least one residual token was
// seen. It uses the caller's ctx so the buffered and SSE-event restore surfaces
// attribute residuals to the in-flight request (the streaming surface audits
// separately in FlushStream with a background context).
func (e *Engine) auditResidual(ctx context.Context, residuals func() map[Type]int) {
	if e.cfg.Audit == nil {
		return
	}
	if counts := residuals(); len(counts) > 0 {
		e.cfg.Audit.Record(ctx, AuditEvent{Kind: "residual_token", Counts: counts})
	}
}
