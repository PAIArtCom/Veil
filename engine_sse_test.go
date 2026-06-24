package veil_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	veil "github.com/PAIArtCom/Veil"
)

// newTestEngineWithDetector builds an Engine with a fixed key and the given L2
// detector so tokens are deterministic and a test can flag arbitrary values.
func newTestEngineWithDetector(t *testing.T, det veil.Detector) *veil.Engine {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key")
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		t.Fatalf("write test key: %v", err)
	}
	e, err := veil.New(veil.Config{KeyPath: keyPath, Detector: det})
	if err != nil {
		t.Fatalf("veil.New: %v", err)
	}
	return e
}

// ADR-0011 §4 test matrix for the stateful, cross-event SSE restorer
// (Engine.NewSSEStreamRestorer / *veil.SSEStream). These tests split PAIArtVeil_
// tokens across logical SSE events (not just TCP byte boundaries) at every
// dangerous position and assert the client-reassembled output restores fully.
//
// The proxy-level byte-at-a-time (exit #7) and restore-error-visibility (exit
// #5) cases live in internal/proxy/proxy_test.go, which drives this restorer
// through the real relay.

// maskOneToken masks text known to contain exactly one detectable secret under
// scope (populating the engine's store for that scope) and returns a wire State
// for scope plus the minted token. The returned State has provider/op set so it
// can drive NewSSEStreamRestorer; because the store is keyed by scope, the wire
// State resolves tokens minted by the text Mask in the same scope. It fails the
// test unless exactly one token is produced, giving callers a single
// deterministic token to split across events.
func maskOneToken(t *testing.T, e *veil.Engine, scope veil.Scope, text string) (*veil.State, string) {
	t.Helper()
	masked, _, err := e.Mask(context.Background(), scope, text)
	if err != nil {
		t.Fatalf("Mask(%q): %v", text, err)
	}
	toks := streamTokenRe.FindAllString(masked, -1)
	if len(toks) != 1 {
		t.Fatalf("want exactly one token in %q, got %d: %v", masked, len(toks), toks)
	}
	return wireState(t, e, scope), toks[0]
}

// sseStreamCollect feeds each event payload to the restorer in order and returns
// the flat, ordered list of emitted payloads (Event outputs followed by Flush
// outputs). It fails on any Event/Flush error.
func sseStreamCollect(t *testing.T, s *veil.SSEStream, events [][]byte) [][]byte {
	t.Helper()
	var out [][]byte
	for i, ev := range events {
		outs, err := s.Event(context.Background(), ev)
		if err != nil {
			t.Fatalf("Event[%d] (%s): %v", i, ev, err)
		}
		out = append(out, outs...)
	}
	outs, err := s.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	return append(out, outs...)
}

// textDeltaEvent builds a content_block_delta/text_delta payload for index with
// the given (already JSON-unescaped) text, escaped via the standard encoder.
func textDeltaEvent(index int, text string) []byte {
	esc, _ := json.Marshal(text)
	return []byte(`{"type":"content_block_delta","index":` + itoa(index) + `,"delta":{"type":"text_delta","text":` + string(esc) + `}}`)
}

// inputJSONDeltaEvent builds a content_block_delta/input_json_delta payload for
// index whose partial_json string carries the given JSON fragment.
func inputJSONDeltaEvent(index int, partialJSON string) []byte {
	esc, _ := json.Marshal(partialJSON)
	return []byte(`{"type":"content_block_delta","index":` + itoa(index) + `,"delta":{"type":"input_json_delta","partial_json":` + string(esc) + `}}`)
}

func blockStart(index int, kind string) []byte {
	return []byte(`{"type":"content_block_start","index":` + itoa(index) + `,"content_block":{"type":"` + kind + `"}}`)
}

func blockStop(index int) []byte {
	return []byte(`{"type":"content_block_stop","index":` + itoa(index) + `}`)
}

func itoa(i int) string {
	return string([]byte{byte('0' + i)}) // single-digit indices only in tests
}

// reassembleText concatenates every text_delta payload's delta.text across the
// emitted payloads (the client's view of the rendered text). It also asserts
// each emitted payload is valid JSON.
func reassembleText(t *testing.T, payloads [][]byte) string {
	t.Helper()
	var b strings.Builder
	for _, p := range payloads {
		if !json.Valid(p) {
			t.Fatalf("emitted payload is not valid JSON: %s", p)
		}
		if gjson.GetBytes(p, "delta.type").Str == "text_delta" {
			b.WriteString(gjson.GetBytes(p, "delta.text").Str)
		}
	}
	return b.String()
}

// newSSE builds an SSEStream from a wire State (via MaskRequest so provider/op
// are set). The returned State is bound to scope.
func newSSE(t *testing.T, e *veil.Engine, st *veil.State) *veil.SSEStream {
	t.Helper()
	s, err := e.NewSSEStreamRestorer(st)
	if err != nil {
		t.Fatalf("NewSSEStreamRestorer: %v", err)
	}
	return s
}

// wireState mints a wire State (provider/op set) for scope so the SSE restorer
// can dispatch. The request masks nothing relevant; tests add their own tokens.
func wireState(t *testing.T, e *veil.Engine, scope veil.Scope) *veil.State {
	t.Helper()
	_, st, err := e.MaskRequest(context.Background(), scope, "anthropic", "messages",
		[]byte(`{"model":"m","max_tokens":8,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	return st
}

// splitToken returns the substring boundaries of tok at the given cut positions,
// producing len(cuts)+1 fragments.
func splitToken(tok string, cuts ...int) []string {
	frags := make([]string, 0, len(cuts)+1)
	prev := 0
	for _, c := range cuts {
		frags = append(frags, tok[prev:c])
		prev = c
	}
	frags = append(frags, tok[prev:])
	return frags
}

// ---- A: text token split across 2 and 3 text_delta events --------------------

func TestSSETextTokenSplitAcrossEvents(t *testing.T) {
	scope := veil.Scope{Session: "sse-A"}
	e := newTestEngineWithAudit(t, nil)
	st, tok := maskOneToken(t, e, scope, "key AKIAIOSFODNN7EXAMPLE")

	// Dangerous split positions: mid-"PAIArtVeil_", mid-TYPE, mid-hex. Derive concrete
	// offsets from the token shape so the test is deterministic for any token.
	underscore := strings.Index(tok, "_") // after "Veil"
	typeEnd := strings.Index(tok[underscore+1:], "_") + underscore + 1
	splitPoints := []int{
		2,              // mid-"PAIArtVeil_"
		underscore + 1, // just after "PAIArtVeil_" (TYPE boundary)
		typeEnd,        // end of TYPE (before "_<hex>")
		typeEnd + 3,    // mid-hex
		len(tok) - 1,   // last hex char split off
	}

	want := "prefix key AKIAIOSFODNN7EXAMPLE suffix"

	for _, sp := range splitPoints {
		s := newSSE(t, e, st)
		frags := splitToken(tok, sp)
		// "prefix key " + frag0 ... fragN + " suffix", split across 2 events at sp.
		events := [][]byte{
			blockStart(0, "text"),
			textDeltaEvent(0, "prefix key "+frags[0]),
			textDeltaEvent(0, frags[1]+" suffix"),
			blockStop(0),
		}
		got := reassembleText(t, sseStreamCollect(t, s, events))
		if got != want {
			t.Fatalf("split@%d: got %q want %q", sp, got, want)
		}
		if strings.Contains(got, "PAIArtVeil_") {
			t.Fatalf("split@%d: residual PAIArtVeil_ in %q", sp, got)
		}
	}

	// Across THREE events at two split points simultaneously.
	s := newSSE(t, e, st)
	frags := splitToken(tok, 2, typeEnd+3)
	events := [][]byte{
		blockStart(0, "text"),
		textDeltaEvent(0, "prefix key "+frags[0]),
		textDeltaEvent(0, frags[1]),
		textDeltaEvent(0, frags[2]+" suffix"),
		blockStop(0),
	}
	if got := reassembleText(t, sseStreamCollect(t, s, events)); got != want {
		t.Fatalf("3-way split: got %q want %q", got, want)
	}
}

// Token at the very end of the stream, flushed at content_block_stop.
func TestSSETextTokenAtEndFlushedAtStop(t *testing.T) {
	scope := veil.Scope{Session: "sse-A-end"}
	e := newTestEngineWithAudit(t, nil)
	st, tok := maskOneToken(t, e, scope, "key AKIAIOSFODNN7EXAMPLE")

	// The whole token arrives in the last delta with nothing after it, so it sits
	// in the held tail (extendable hex) until content_block_stop flushes it.
	frags := splitToken(tok, 4)
	s := newSSE(t, e, st)
	events := [][]byte{
		blockStart(0, "text"),
		textDeltaEvent(0, "value "+frags[0]),
		textDeltaEvent(0, frags[1]),
		blockStop(0),
	}
	payloads := sseStreamCollect(t, s, events)
	got := reassembleText(t, payloads)
	want := "value AKIAIOSFODNN7EXAMPLE"
	if got != want {
		t.Fatalf("end-of-stream token: got %q want %q", got, want)
	}
	// The restored tail must arrive as a synthetic text_delta BEFORE the stop.
	assertStopLast(t, payloads, 0)
}

// Two tokens with a boundary between them and a split inside the second.
func TestSSETwoTokensBoundaryAndInnerSplit(t *testing.T) {
	scope := veil.Scope{Session: "sse-A-two"}
	e := newTestEngineWithAudit(t, nil)
	masked, _, err := e.Mask(context.Background(), scope, "k1 AKIAIOSFODNN7EXAMPLE k2 user@example.com")
	if err != nil {
		t.Fatalf("Mask: %v", err)
	}
	toks := streamTokenRe.FindAllString(masked, -1)
	if len(toks) != 2 {
		t.Fatalf("want 2 tokens, got %v", toks)
	}
	st := wireState(t, e, scope)
	t1, t2 := toks[0], toks[1]
	mid2 := len(t1) + len(" ") + 5 // somewhere inside t2 when concatenated

	full := t1 + " " + t2
	frags := splitToken(full, len(t1), mid2) // boundary between, then inside t2
	s := newSSE(t, e, st)
	events := [][]byte{
		blockStart(0, "text"),
		textDeltaEvent(0, frags[0]),
		textDeltaEvent(0, frags[1]),
		textDeltaEvent(0, frags[2]),
		blockStop(0),
	}
	got := reassembleText(t, sseStreamCollect(t, s, events))
	want := "AKIAIOSFODNN7EXAMPLE user@example.com"
	if got != want {
		t.Fatalf("two-token split: got %q want %q", got, want)
	}
}

// ---- B: tool_use token split across input_json_delta fragments ---------------

func TestSSEToolUseTokenSplitConsolidated(t *testing.T) {
	scope := veil.Scope{Session: "sse-B"}
	e := newTestEngineWithAudit(t, nil)
	st, tok := maskOneToken(t, e, scope, "dsn AKIAIOSFODNN7EXAMPLE")

	// The complete tool input JSON is {"dsn":"<tok>"}; split its serialized form
	// across input_json_delta fragments at an interior point inside the token.
	complete := `{"dsn":"` + tok + `"}`
	cut := strings.Index(complete, tok) + 6 // mid-token
	frags := splitToken(complete, cut)

	s := newSSE(t, e, st)
	events := [][]byte{
		blockStart(1, "tool_use"),
		inputJSONDeltaEvent(1, frags[0]),
		inputJSONDeltaEvent(1, frags[1]),
		blockStop(1),
	}
	payloads := sseStreamCollect(t, s, events)

	// Exactly ONE consolidated input_json_delta must be emitted, before the stop.
	var jsonDeltas [][]byte
	for _, p := range payloads {
		if gjson.GetBytes(p, "delta.type").Str == "input_json_delta" {
			jsonDeltas = append(jsonDeltas, p)
		}
	}
	if len(jsonDeltas) != 1 {
		t.Fatalf("want exactly 1 consolidated input_json_delta, got %d: %s", len(jsonDeltas), payloads)
	}
	assertStopLast(t, payloads, 1)
	// content_block_start must survive.
	if gjson.GetBytes(payloads[0], "type").Str != "content_block_start" {
		t.Fatalf("content_block_start did not survive as first payload: %s", payloads[0])
	}

	// The reassembled tool_use.input JSON-decodes to the real value.
	partialJSON := gjson.GetBytes(jsonDeltas[0], "delta.partial_json").Str
	var input struct {
		DSN string `json:"dsn"`
	}
	if err := json.Unmarshal([]byte(partialJSON), &input); err != nil {
		t.Fatalf("consolidated partial_json is not valid JSON: %q: %v", partialJSON, err)
	}
	if input.DSN != "AKIAIOSFODNN7EXAMPLE" {
		t.Fatalf("restored tool input dsn = %q, want the real secret", input.DSN)
	}
}

// B3: restored tool value containing JSON-special chars round-trips.
func TestSSEToolUseSpecialCharsRoundTrip(t *testing.T) {
	scope := veil.Scope{Session: "sse-B3"}
	// A restored value containing quotes/backslash/newline must re-serialize as
	// valid escaped JSON. mintSpecial builds an engine whose detector flags the
	// special value so the token's stored value IS those exact bytes.
	special := "line1\"quote\\back\nline2"
	e, st, tok := mintSpecial(t, scope, special)

	complete := `{"note":"` + tok + `"}`
	cut := strings.Index(complete, tok) + 4
	frags := splitToken(complete, cut)

	s := newSSE(t, e, st)
	events := [][]byte{
		blockStart(0, "tool_use"),
		inputJSONDeltaEvent(0, frags[0]),
		inputJSONDeltaEvent(0, frags[1]),
		blockStop(0),
	}
	payloads := sseStreamCollect(t, s, events)
	var consolidated string
	for _, p := range payloads {
		if !json.Valid(p) {
			t.Fatalf("emitted payload not valid JSON: %s", p)
		}
		if gjson.GetBytes(p, "delta.type").Str == "input_json_delta" {
			consolidated = gjson.GetBytes(p, "delta.partial_json").Str
		}
	}
	var input struct {
		Note string `json:"note"`
	}
	if err := json.Unmarshal([]byte(consolidated), &input); err != nil {
		t.Fatalf("consolidated partial_json with specials not valid JSON: %q: %v", consolidated, err)
	}
	if input.Note != special {
		t.Fatalf("round-trip of special value failed:\n got %q\nwant %q", input.Note, special)
	}
}

// ---- C: text value with JSON-special chars + split token ---------------------

func TestSSETextSpecialCharsValidJSON(t *testing.T) {
	scope := veil.Scope{Session: "sse-C"}
	special := `a"b\c` + "\n" + `d`
	e, st, tok := mintSpecial(t, scope, special)

	frags := splitToken(tok, 3)
	s := newSSE(t, e, st)
	events := [][]byte{
		blockStart(0, "text"),
		textDeltaEvent(0, "val "+frags[0]),
		textDeltaEvent(0, frags[1]+" end"),
		blockStop(0),
	}
	payloads := sseStreamCollect(t, s, events)
	// Every emitted payload must be valid JSON (escaping-correct delta.text).
	got := reassembleText(t, payloads)
	want := "val " + special + " end"
	if got != want {
		t.Fatalf("special-char text restore: got %q want %q", got, want)
	}
}

// ---- D: multi-index isolation (text idx0 split, tool idx1 split, thinking) ---

func TestSSEMultiIndexIsolation(t *testing.T) {
	scope := veil.Scope{Session: "sse-D"}
	e := newTestEngineWithAudit(t, nil)
	// Both tokens are minted in the same scope so the one State knows both. The
	// IPv4 and the AWS key are independently L1-detectable.
	st, textTok := maskOneToken(t, e, scope, "ip 192.168.1.100")
	_, toolTok := maskOneToken(t, e, scope, "key AKIAIOSFODNN7EXAMPLE")

	// Interleave three blocks: index 0 text (split), index 1 tool_use (split),
	// index 2 thinking (must pass through unrestored, isolation preserved).
	textFrags := splitToken(textTok, 4)
	toolComplete := `{"k":"` + toolTok + `"}`
	toolCut := strings.Index(toolComplete, toolTok) + 5
	toolFrags := splitToken(toolComplete, toolCut)

	s := newSSE(t, e, st)
	events := [][]byte{
		blockStart(0, "text"),
		blockStart(1, "tool_use"),
		blockStart(2, "thinking"),
		textDeltaEvent(0, "addr "+textFrags[0]),
		inputJSONDeltaEvent(1, toolFrags[0]),
		// A thinking_delta carrying a PAIArtVeil_-looking token: must be untouched.
		thinkingDeltaEvent(2, "reasoning "+textTok),
		textDeltaEvent(0, textFrags[1]+" done"),
		inputJSONDeltaEvent(1, toolFrags[1]),
		blockStop(0),
		blockStop(1),
		blockStop(2),
	}
	payloads := sseStreamCollect(t, s, events)

	// Text on index 0 restores; its held tail attaches to index 0's stop.
	gotText := reassembleTextForIndex(payloads, 0)
	if gotText != "addr 192.168.1.100 done" {
		t.Fatalf("index 0 text = %q, want %q", gotText, "addr 192.168.1.100 done")
	}
	// Tool consolidation keyed to index 1.
	var toolJSON string
	var toolDeltaIndex int64 = -1
	for _, p := range payloads {
		if gjson.GetBytes(p, "delta.type").Str == "input_json_delta" {
			toolJSON = gjson.GetBytes(p, "delta.partial_json").Str
			toolDeltaIndex = gjson.GetBytes(p, "index").Int()
		}
	}
	if toolDeltaIndex != 1 {
		t.Fatalf("consolidated tool delta keyed to index %d, want 1", toolDeltaIndex)
	}
	if !strings.Contains(toolJSON, "AKIAIOSFODNN7EXAMPLE") || strings.Contains(toolJSON, "PAIArtVeil_") {
		t.Fatalf("tool input not restored/isolated: %q", toolJSON)
	}
	// Thinking on index 2 passes through UNRESTORED (token still present).
	var thinkingText string
	for _, p := range payloads {
		if gjson.GetBytes(p, "delta.type").Str == "thinking_delta" {
			thinkingText = gjson.GetBytes(p, "delta.thinking").Str
		}
	}
	if !strings.Contains(thinkingText, textTok) {
		t.Fatalf("thinking delta should pass through with token unrestored, got %q", thinkingText)
	}
}

func thinkingDeltaEvent(index int, text string) []byte {
	esc, _ := json.Marshal(text)
	return []byte(`{"type":"content_block_delta","index":` + itoa(index) + `,"delta":{"type":"thinking_delta","thinking":` + string(esc) + `}}`)
}

func reassembleTextForIndex(payloads [][]byte, index int64) string {
	var b strings.Builder
	for _, p := range payloads {
		if gjson.GetBytes(p, "delta.type").Str == "text_delta" && gjson.GetBytes(p, "index").Int() == index {
			b.WriteString(gjson.GetBytes(p, "delta.text").Str)
		}
	}
	return b.String()
}

// ---- E: ping injected between halves of a split token ------------------------

func TestSSEPingMidTokenForwardedAndTokenRestored(t *testing.T) {
	scope := veil.Scope{Session: "sse-E"}
	e := newTestEngineWithAudit(t, nil)
	st, tok := maskOneToken(t, e, scope, "key AKIAIOSFODNN7EXAMPLE")

	frags := splitToken(tok, 5)
	ping := []byte(`{"type":"ping"}`)
	s := newSSE(t, e, st)

	// Feed first half, then a ping, then the second half. The ping must be
	// forwarded immediately and unmodified; the token must still restore.
	out1, err := s.Event(context.Background(), textDeltaEvent(0, "k "+frags[0]))
	if err != nil {
		t.Fatalf("Event half1: %v", err)
	}
	outPing, err := s.Event(context.Background(), ping)
	if err != nil {
		t.Fatalf("Event ping: %v", err)
	}
	if len(outPing) != 1 || !bytes.Equal(outPing[0], ping) {
		t.Fatalf("ping not forwarded immediately/unmodified: %v", outPing)
	}
	out2, err := s.Event(context.Background(), textDeltaEvent(0, frags[1]+" done"))
	if err != nil {
		t.Fatalf("Event half2: %v", err)
	}
	flush, err := s.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	all := append(append(append(out1, outPing...), out2...), flush...)
	if got := reassembleText(t, all); got != "k AKIAIOSFODNN7EXAMPLE done" {
		t.Fatalf("ping-mid-token restore: got %q", got)
	}
}

// message_stop defensively drains a held text tail when the stream omits the
// per-block content_block_stop (malformed but possible). The restored tail must
// be emitted as a synthetic delta BEFORE the message_stop.
func TestSSEMessageStopDrainsHeldTail(t *testing.T) {
	scope := veil.Scope{Session: "sse-msgstop"}
	e := newTestEngineWithAudit(t, nil)
	st, tok := maskOneToken(t, e, scope, "key AKIAIOSFODNN7EXAMPLE")

	frags := splitToken(tok, 4)
	s := newSSE(t, e, st)
	events := [][]byte{
		blockStart(0, "text"),
		textDeltaEvent(0, "v "+frags[0]),
		textDeltaEvent(0, frags[1]), // token now sits in the held tail (extendable hex)
		[]byte(`{"type":"message_stop"}`),
	}
	payloads := sseStreamCollect(t, s, events)
	if got := reassembleText(t, payloads); got != "v AKIAIOSFODNN7EXAMPLE" {
		t.Fatalf("message_stop drain: got %q", got)
	}
	// The last payload is message_stop, and a synthetic text_delta precedes it.
	last := payloads[len(payloads)-1]
	if gjson.GetBytes(last, "type").Str != "message_stop" {
		t.Fatalf("last payload is not message_stop: %s", last)
	}
	var sawTextBeforeStop bool
	for _, p := range payloads[:len(payloads)-1] {
		if gjson.GetBytes(p, "delta.type").Str == "text_delta" {
			sawTextBeforeStop = true
		}
	}
	if !sawTextBeforeStop {
		t.Fatalf("restored tail not emitted before message_stop: %s", payloads)
	}
}

// ---- G: flush edges (stream ends mid-token, no stop) -------------------------

func TestSSEFlushDrainsTailNoStop(t *testing.T) {
	scope := veil.Scope{Session: "sse-G"}
	e := newTestEngineWithAudit(t, nil)
	st, tok := maskOneToken(t, e, scope, "key AKIAIOSFODNN7EXAMPLE")

	// Stream ends with the token in the held tail and NO content_block_stop /
	// message_stop. Flush must drain the tail, restoring it.
	frags := splitToken(tok, 4)
	s := newSSE(t, e, st)
	out1, _ := s.Event(context.Background(), blockStart(0, "text"))
	out2, _ := s.Event(context.Background(), textDeltaEvent(0, "v "+frags[0]))
	out3, _ := s.Event(context.Background(), textDeltaEvent(0, frags[1]))
	flush, err := s.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	all := append(append(append(out1, out2...), out3...), flush...)
	if got := reassembleText(t, all); got != "v AKIAIOSFODNN7EXAMPLE" {
		t.Fatalf("flush-drains-tail: got %q", got)
	}
}

// G2: stream ends mid-token with a token that never completes -> residual +
// exactly one audit event at Flush.
func TestSSEFlushResidualAuditsOnce(t *testing.T) {
	scope := veil.Scope{Session: "sse-G2"}
	audit := &recordingAudit{}
	e := newTestEngineWithAudit(t, audit)
	st := wireState(t, e, scope)

	// A validly-shaped token never minted in this scope (residual). Deliver it
	// whole then end the stream; the held tail flushes it as residual.
	const residual = "PAIArtVeil_IPV4_00aa11bb22cc"
	s := newSSE(t, e, st)
	_, _ = s.Event(context.Background(), blockStart(0, "text"))
	_, _ = s.Event(context.Background(), textDeltaEvent(0, "x "+residual[:6]))
	_, _ = s.Event(context.Background(), textDeltaEvent(0, residual[6:]))
	if len(audit.snapshot()) != 0 {
		t.Fatalf("audit recorded before Flush: %v", audit.snapshot())
	}
	flush, err := s.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	joined := string(bytes.Join(flush, nil))
	if !strings.Contains(joined, residual) {
		t.Fatalf("residual token should survive in flushed output: %s", joined)
	}
	evs := audit.snapshot()
	if len(evs) != 1 || evs[0].Kind != "residual_token" || evs[0].Counts[veil.TypeIPv4] != 1 {
		t.Fatalf("want exactly one residual_token{IPV4:1} at Flush, got %v", evs)
	}
}

// ---- I: negatives ------------------------------------------------------------

// No-token stream passes through with zero audits.
func TestSSENoTokenPassThroughNoAudit(t *testing.T) {
	scope := veil.Scope{Session: "sse-I"}
	audit := &recordingAudit{}
	e := newTestEngineWithAudit(t, audit)
	st := wireState(t, e, scope)

	s := newSSE(t, e, st)
	events := [][]byte{
		[]byte(`{"type":"message_start","message":{"id":"m1"}}`),
		blockStart(0, "text"),
		textDeltaEvent(0, "just plain words"),
		blockStop(0),
		[]byte(`{"type":"message_stop"}`),
	}
	got := reassembleText(t, sseStreamCollect(t, s, events))
	if got != "just plain words" {
		t.Fatalf("no-token pass-through altered text: %q", got)
	}
	if len(audit.snapshot()) != 0 {
		t.Fatalf("clean stream recorded audit events: %v", audit.snapshot())
	}
}

// A PAIArtVeil_-shaped non-token (fails TokenPattern: too-short id) split across events
// is left untouched and not counted.
func TestSSEVeilShapedNonTokenUntouched(t *testing.T) {
	scope := veil.Scope{Session: "sse-I2"}
	audit := &recordingAudit{}
	e := newTestEngineWithAudit(t, audit)
	st := wireState(t, e, scope)

	// 4 hex chars (< 12) → not a token per PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}.
	const fake = "PAIArtVeil_SECRET_dead"
	frags := splitToken(fake, 6)
	s := newSSE(t, e, st)
	events := [][]byte{
		blockStart(0, "text"),
		textDeltaEvent(0, "x "+frags[0]),
		textDeltaEvent(0, frags[1]+" y"),
		blockStop(0),
	}
	got := reassembleText(t, sseStreamCollect(t, s, events))
	if got != "x "+fake+" y" {
		t.Fatalf("Veil-shaped non-token altered: got %q want %q", got, "x "+fake+" y")
	}
	if len(audit.snapshot()) != 0 {
		t.Fatalf("non-token must not be counted as residual: %v", audit.snapshot())
	}
}

// ---- Fail-closed validation of NewSSEStreamRestorer --------------------------

func TestNewSSEStreamRestorerFailClosed(t *testing.T) {
	e := newTestEngineWithAudit(t, nil)
	// nil State.
	if _, err := e.NewSSEStreamRestorer(nil); err == nil {
		t.Fatal("NewSSEStreamRestorer(nil) = nil error, want ErrInvalidState")
	}
	// State without provider/op (from text Mask).
	_, st, err := e.Mask(context.Background(), veil.Scope{}, "text")
	if err != nil {
		t.Fatalf("Mask: %v", err)
	}
	if _, err := e.NewSSEStreamRestorer(st); err == nil {
		t.Fatal("NewSSEStreamRestorer(no provider/op) = nil error, want ErrInvalidState")
	}
}

// ---- helpers shared across cases ---------------------------------------------

// assertStopLast asserts the content_block_stop for index is the LAST payload
// and that a synthetic delta for the same index precedes it (the held tail /
// consolidated input is emitted before the block closes).
func assertStopLast(t *testing.T, payloads [][]byte, index int64) {
	t.Helper()
	if len(payloads) == 0 {
		t.Fatal("no payloads emitted")
	}
	last := payloads[len(payloads)-1]
	if gjson.GetBytes(last, "type").Str != "content_block_stop" {
		t.Fatalf("last payload is not content_block_stop: %s", last)
	}
	if gjson.GetBytes(last, "index").Int() != index {
		t.Fatalf("last stop index = %d, want %d", gjson.GetBytes(last, "index").Int(), index)
	}
	// A synthetic delta for index must appear before the stop.
	var sawSyntheticBeforeStop bool
	for _, p := range payloads[:len(payloads)-1] {
		if gjson.GetBytes(p, "type").Str == "content_block_delta" && gjson.GetBytes(p, "index").Int() == index {
			sawSyntheticBeforeStop = true
		}
	}
	if !sawSyntheticBeforeStop {
		t.Fatalf("no synthetic delta for index %d before its stop: %s", index, payloads)
	}
}

// fixedDetector flags every occurrence of needle as a single Finding of typ.
// It lets a test mint a real, restorable token whose stored value is an
// arbitrary string (e.g. one with JSON-special characters) without depending on
// the L1 detector's patterns — proving the restorer's escaping correctness on a
// value the masker would otherwise never produce.
type fixedDetector struct {
	needle string
	typ    veil.Type
}

func (d fixedDetector) Detect(_ context.Context, text string) ([]veil.Finding, error) {
	var out []veil.Finding
	for i := 0; ; {
		j := strings.Index(text[i:], d.needle)
		if j < 0 {
			break
		}
		start := i + j
		out = append(out, veil.Finding{
			Start: start, End: start + len(d.needle),
			Type: d.typ, Score: 1, Source: "test:fixed",
		})
		i = start + len(d.needle)
	}
	return out, nil
}

// mintSpecial builds an engine whose detector flags value, masks a request that
// embeds value, and returns the engine, a wire State for scope, and the minted
// token whose stored value is exactly value (specials included).
func mintSpecial(t *testing.T, scope veil.Scope, value string) (*veil.Engine, *veil.State, string) {
	t.Helper()
	e := newTestEngineWithDetector(t, fixedDetector{needle: value, typ: veil.TypeSecret})
	esc, _ := json.Marshal(value)
	body := []byte(`{"model":"m","max_tokens":8,"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"n","input":{"v":` + string(esc) + `}}]}]}`)
	masked, st, err := e.MaskRequest(context.Background(), scope, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest(mint %q): %v", value, err)
	}
	tok := gjson.GetBytes(masked, "messages.0.content.0.input.v").Str
	if !streamTokenRe.MatchString(tok) {
		t.Fatalf("mintSpecial: no token minted for %q, got %q", value, tok)
	}
	return e, st, tok
}
