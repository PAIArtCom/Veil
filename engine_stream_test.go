package opencloak_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	opencloak "github.com/cloakia/opencloak"
)

var streamTokenRe = regexp.MustCompile(`OpenCloak_[A-Z0-9]+_[0-9a-f]{12,}`)

// recordingAudit is a test AuditSink that captures every recorded event. It is
// concurrency-safe so it can also be used under the race detector.
type recordingAudit struct {
	mu     sync.Mutex
	events []opencloak.AuditEvent
}

func (a *recordingAudit) Record(_ context.Context, ev opencloak.AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Copy the counts map so later mutations by the engine (there are none, but
	// be defensive) cannot affect the captured event.
	cp := make(map[opencloak.Type]int, len(ev.Counts))
	for k, v := range ev.Counts {
		cp[k] = v
	}
	a.events = append(a.events, opencloak.AuditEvent{Kind: ev.Kind, Counts: cp})
}

func (a *recordingAudit) snapshot() []opencloak.AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]opencloak.AuditEvent, len(a.events))
	copy(out, a.events)
	return out
}

// newTestEngineWithAudit builds an Engine with a fixed key and the given audit
// sink so token values are deterministic and audit events are observable.
func newTestEngineWithAudit(t *testing.T, audit opencloak.AuditSink) *opencloak.Engine {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key")
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		t.Fatalf("write test key: %v", err)
	}
	e, err := opencloak.New(opencloak.Config{KeyPath: keyPath, Audit: audit})
	if err != nil {
		t.Fatalf("opencloak.New: %v", err)
	}
	return e
}

// streamRestoreSplit relays masked through RestoreStreamChunk at the given
// interior split offsets, then FlushStream, and returns the concatenation.
func streamRestoreSplit(e *opencloak.Engine, st *opencloak.State, masked string, splits []int) string {
	var out bytes.Buffer
	prev := 0
	for _, off := range splits {
		out.Write(e.RestoreStreamChunk(st, []byte(masked[prev:off])))
		prev = off
	}
	out.Write(e.RestoreStreamChunk(st, []byte(masked[prev:])))
	out.Write(e.FlushStream(st))
	return out.String()
}

// ---- Test 8: end-to-end streaming round-trip via Mask ------------------------

// TestStreamRoundTripText masks a payload with real tokens, then relays the
// masked text through the streaming restorer split at MANY boundaries and
// asserts the reassembled output equals the original plaintext for every split.
func TestStreamRoundTripText(t *testing.T) {
	e := newTestEngineWithAudit(t, nil)
	text := "email user@example.com, key AKIAIOSFODNN7EXAMPLE, ip 10.0.0.1 end"

	masked, _, err := e.Mask(ctx, opencloak.Scope{}, text)
	if err != nil {
		t.Fatalf("Mask: %v", err)
	}
	if !streamTokenRe.MatchString(masked) {
		t.Fatalf("expected tokens in masked text: %q", masked)
	}

	// Split at every interior byte boundary; each must reassemble to the original.
	for cut := 1; cut < len(masked); cut++ {
		// A fresh State per relay so each stream has its own restorer.
		_, st, err := e.Mask(ctx, opencloak.Scope{}, text)
		if err != nil {
			t.Fatalf("Mask (inner): %v", err)
		}
		got := streamRestoreSplit(e, st, masked, []int{cut})
		if got != text {
			t.Fatalf("stream round-trip split@%d failed:\n original: %q\n restored: %q", cut, text, got)
		}
	}

	// Byte-by-byte (every boundary at once).
	allBytes := make([]int, 0, len(masked))
	for i := 1; i < len(masked); i++ {
		allBytes = append(allBytes, i)
	}
	_, st, _ := e.Mask(ctx, opencloak.Scope{}, text)
	if got := streamRestoreSplit(e, st, masked, allBytes); got != text {
		t.Fatalf("byte-by-byte stream round-trip failed:\n original: %q\n restored: %q", text, got)
	}
}

// TestStreamRoundTripWireBody masks a provider request, then relays the masked
// REQUEST body's tokens back through the raw streaming restorer to confirm the
// universal byte path restores genuine tokens minted by MaskRequest. (The body
// is JSON, but each token value here is JSON-safe, so the raw path is valid.)
func TestStreamRoundTripWireWithMaskRequest(t *testing.T) {
	e := newTestEngineWithAudit(t, nil)
	scope := opencloak.Scope{Session: "stream-wire"}
	reqBody := []byte(`{"model":"m","max_tokens":10,"system":"key AKIAIOSFODNN7EXAMPLE","messages":[{"role":"user","content":"email user@example.com"}]}`)

	maskedReq, st, err := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	toks := streamTokenRe.FindAllString(string(maskedReq), -1)
	if len(toks) == 0 {
		t.Fatal("no tokens minted by MaskRequest")
	}

	// Build a fake streamed body that simply concatenates the tokens with text.
	streamed := "values: " + strings.Join(toks, " | ")
	// Reference: restore the same string in one shot via the text Restore.
	want, err := e.Restore(ctx, st, streamed)
	if err != nil {
		t.Fatalf("Restore reference: %v", err)
	}

	allBytes := make([]int, 0, len(streamed))
	for i := 1; i < len(streamed); i++ {
		allBytes = append(allBytes, i)
	}
	got := streamRestoreSplit(e, st, streamed, allBytes)
	if got != want {
		t.Fatalf("wire-token stream round-trip:\n want %q\n got  %q", want, got)
	}
	if strings.Contains(got, "OpenCloak_") {
		t.Fatalf("residual token after restore: %q", got)
	}
}

// ---- Test 9: residual auditing on FlushStream and RestoreResponse ------------

// TestStreamResidualAuditOnFlush configures a recording audit sink, streams a
// body containing a residual token (a valid token shape that was never minted in
// this scope), and asserts exactly one residual_token event with the right
// Counts is recorded on FlushStream.
func TestStreamResidualAuditOnFlush(t *testing.T) {
	audit := &recordingAudit{}
	e := newTestEngineWithAudit(t, audit)

	// A validly-shaped token that the store does not know in this scope.
	const residualTok = "OpenCloak_IPV4_001122334455"
	_, st, err := e.Mask(ctx, opencloak.Scope{}, "plain text no secrets")
	if err != nil {
		t.Fatalf("Mask: %v", err)
	}

	streamed := "the address is " + residualTok + " ok"
	var out bytes.Buffer
	out.Write(e.RestoreStreamChunk(st, []byte(streamed)))
	// No audit event should have been recorded before flush.
	if len(audit.snapshot()) != 0 {
		t.Fatalf("audit event recorded before FlushStream: %v", audit.snapshot())
	}
	out.Write(e.FlushStream(st))

	if !strings.Contains(out.String(), residualTok) {
		t.Fatalf("residual token should pass through unchanged: %q", out.String())
	}

	evs := audit.snapshot()
	if len(evs) != 1 {
		t.Fatalf("expected exactly 1 audit event, got %d: %v", len(evs), evs)
	}
	if evs[0].Kind != "residual_token" {
		t.Fatalf("event Kind = %q, want residual_token", evs[0].Kind)
	}
	if evs[0].Counts[opencloak.TypeIPv4] != 1 {
		t.Fatalf("event Counts = %v, want IPV4:1", evs[0].Counts)
	}
}

// TestRestoreResponseResidualAuditUsesCtx asserts that RestoreResponse records a
// residual_token event (using the passed ctx) when the response carries a valid
// token shape unknown to the scope.
func TestRestoreResponseResidualAuditUsesCtx(t *testing.T) {
	// Distinct context value to confirm the SAME ctx flows through to Record.
	type ctxKey struct{}
	seen := make(chan context.Context, 1)
	inner := &recordingAudit{}
	capturing := &ctxCapturingAudit{inner: inner, seen: seen}
	e := newTestEngineWithAudit(t, capturing)

	// Establish a wire State via MaskRequest (so provider/op are set).
	scope := opencloak.Scope{Session: "resp-residual"}
	_, st, err := e.MaskRequest(ctx, scope, "anthropic", "messages",
		[]byte(`{"model":"m","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}

	// Response references a token never minted in this scope → residual.
	const residualTok = "OpenCloak_SECRET_0a1b2c3d4e5f"
	respBody := []byte(`{"role":"assistant","content":[{"type":"text","text":"leftover ` + residualTok + ` here"}],"stop_reason":"end_turn"}`)

	callCtx := context.WithValue(context.Background(), ctxKey{}, "resp-call")
	restored, err := e.RestoreResponse(callCtx, st, respBody)
	if err != nil {
		t.Fatalf("RestoreResponse: %v", err)
	}
	if !bytes.Contains(restored, []byte(residualTok)) {
		t.Fatalf("residual token should be left as-is in response: %s", restored)
	}

	evs := inner.snapshot()
	if len(evs) != 1 || evs[0].Kind != "residual_token" {
		t.Fatalf("expected 1 residual_token event, got %v", evs)
	}
	if evs[0].Counts[opencloak.TypeSecret] != 1 {
		t.Fatalf("event Counts = %v, want SECRET:1", evs[0].Counts)
	}
	select {
	case got := <-seen:
		if got.Value(ctxKey{}) != "resp-call" {
			t.Fatalf("Record did not receive the caller ctx")
		}
	default:
		t.Fatal("Record was not called")
	}
}

// ctxCapturingAudit forwards events to inner and records the ctx of the first
// call so a test can assert the caller's context is propagated.
type ctxCapturingAudit struct {
	inner *recordingAudit
	seen  chan context.Context
}

func (a *ctxCapturingAudit) Record(ctx context.Context, ev opencloak.AuditEvent) {
	select {
	case a.seen <- ctx:
	default:
	}
	a.inner.Record(ctx, ev)
}

// TestNoResidualNoAudit confirms a clean restore (all tokens known) records no
// audit event on any surface.
func TestNoResidualNoAudit(t *testing.T) {
	audit := &recordingAudit{}
	e := newTestEngineWithAudit(t, audit)

	text := "key AKIAIOSFODNN7EXAMPLE"
	masked, st, err := e.Mask(ctx, opencloak.Scope{}, text)
	if err != nil {
		t.Fatalf("Mask: %v", err)
	}
	// Stream the masked text (token IS known) → no residual.
	var out bytes.Buffer
	out.Write(e.RestoreStreamChunk(st, []byte(masked)))
	out.Write(e.FlushStream(st))
	if out.String() != text {
		t.Fatalf("round-trip failed: %q", out.String())
	}
	if len(audit.snapshot()) != 0 {
		t.Fatalf("clean restore should record no audit events, got %v", audit.snapshot())
	}
}

// ---- Test 10: nil / never-streamed State behaviors ---------------------------

func TestRestoreStreamChunkNilState(t *testing.T) {
	e := newTestEngineWithAudit(t, nil)
	chunk := []byte("OpenCloak_SECRET_0a1b2c3d4e5f and text")
	got := e.RestoreStreamChunk(nil, chunk)
	if !bytes.Equal(got, chunk) {
		t.Fatalf("RestoreStreamChunk(nil) must return chunk unchanged: got %q", got)
	}
}

func TestFlushStreamNilState(t *testing.T) {
	e := newTestEngineWithAudit(t, nil)
	if got := e.FlushStream(nil); got != nil {
		t.Fatalf("FlushStream(nil) must return nil, got %q", got)
	}
}

func TestFlushStreamNeverStreamedRecordsNothing(t *testing.T) {
	audit := &recordingAudit{}
	e := newTestEngineWithAudit(t, audit)
	// A State that never streamed: obtained from Mask but no RestoreStreamChunk.
	_, st, err := e.Mask(ctx, opencloak.Scope{}, "text")
	if err != nil {
		t.Fatalf("Mask: %v", err)
	}
	if got := e.FlushStream(st); got != nil {
		t.Fatalf("FlushStream on never-streamed State must return nil, got %q", got)
	}
	if len(audit.snapshot()) != 0 {
		t.Fatalf("FlushStream on never-streamed State must record nothing, got %v", audit.snapshot())
	}
}
