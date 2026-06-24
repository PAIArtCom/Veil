package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/PAIArtCom/Veil/internal/wire"
)

// mapRestore returns a RestoreFunc that replaces each known token (substring)
// with its mapped value. It is intentionally simple — the engine owns the real
// token grammar; these in-package tests only exercise the restorer's event
// dispatch, holdback, and synthetic-payload shapes.
func mapRestore(m map[string]string) wire.RestoreFunc {
	return func(text string) (string, error) {
		for tok, val := range m {
			text = strings.ReplaceAll(text, tok, val)
		}
		return text, nil
	}
}

// collect feeds events to a fresh restorer and returns all emitted payloads
// (Event outputs then Flush outputs).
func collect(t *testing.T, restore wire.RestoreFunc, events ...[]byte) [][]byte {
	t.Helper()
	p := &provider{}
	sr, err := p.NewStreamRestorer("messages")
	if err != nil {
		t.Fatalf("NewStreamRestorer: %v", err)
	}
	var out [][]byte
	for i, ev := range events {
		outs, err := sr.Event(ev, restore)
		if err != nil {
			t.Fatalf("Event[%d]: %v", i, err)
		}
		out = append(out, outs...)
	}
	outs, err := sr.Flush(restore)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	return append(out, outs...)
}

// NewStreamRestorer accepts the Phase 0 Anthropic Messages op and rejects every
// other op so unsupported endpoint shapes fail closed.
func TestNewStreamRestorerSupported(t *testing.T) {
	p := &provider{}
	sr, err := p.NewStreamRestorer("messages")
	if err != nil {
		t.Fatalf("NewStreamRestorer(messages) error = %v, want nil", err)
	}
	if sr == nil {
		t.Fatal("NewStreamRestorer(messages) = nil restorer")
	}

	for _, op := range []string{"", "responses", "anything"} {
		if sr, err := p.NewStreamRestorer(op); err == nil || sr != nil {
			t.Fatalf("NewStreamRestorer(%q) = (%v, %v), want nil restorer and error", op, sr, err)
		}
	}
}

// A token wholly inside a single text_delta with trailing non-hex restores in
// place and re-encodes delta.text as valid JSON.
func TestStreamTextDeltaInPlace(t *testing.T) {
	restore := mapRestore(map[string]string{"PAIArtVeil_SECRET_0a1b2c3d4e5f": "SECRET"})
	out := collect(t, restore,
		[]byte(`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`),
		[]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"x PAIArtVeil_SECRET_0a1b2c3d4e5f y"}}`),
		[]byte(`{"type":"content_block_stop","index":0}`),
	)
	got := textOf(out)
	if got != "x SECRET y" {
		t.Fatalf("text = %q, want %q", got, "x SECRET y")
	}
	for _, p := range out {
		if !json.Valid(p) {
			t.Fatalf("payload not valid JSON: %s", p)
		}
	}
}

// A token split across two text_delta events: the first event is wholly held
// (returns zero payloads) and the token surfaces restored on the second.
func TestStreamTextDeltaHeldThenFlushed(t *testing.T) {
	restore := mapRestore(map[string]string{"PAIArtVeil_SECRET_0a1b2c3d4e5f": "SECRET"})
	p := &provider{}
	sr, _ := p.NewStreamRestorer("messages")

	// "PAIArtVeil_SE" is wholly a partial-token prefix → entire delta held.
	outs, err := sr.Event([]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"PAIArtVeil_SE"}}`), restore)
	if err != nil {
		t.Fatalf("Event 1: %v", err)
	}
	if len(outs) != 0 {
		t.Fatalf("first (wholly partial) delta should be suppressed, got %d payloads: %s", len(outs), outs)
	}
	// The rest completes the token plus a trailing space proving termination.
	outs2, err := sr.Event([]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"CRET_0a1b2c3d4e5f "}}`), restore)
	if err != nil {
		t.Fatalf("Event 2: %v", err)
	}
	if len(outs2) != 1 {
		t.Fatalf("want 1 payload after completion, got %d", len(outs2))
	}
	if got := gjson.GetBytes(outs2[0], "delta.text").Str; got != "SECRET " {
		t.Fatalf("restored delta.text = %q, want %q", got, "SECRET ")
	}
}

// input_json_delta fragments are swallowed (zero payloads) and emitted as ONE
// consolidated input_json_delta before the stop.
func TestStreamToolConsolidation(t *testing.T) {
	restore := mapRestore(map[string]string{"PAIArtVeil_SECRET_0a1b2c3d4e5f": "real-dsn"})
	p := &provider{}
	sr, _ := p.NewStreamRestorer("messages")

	o0, _ := sr.Event([]byte(`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use"}}`), restore)
	if len(o0) != 1 || gjson.GetBytes(o0[0], "type").Str != "content_block_start" {
		t.Fatalf("content_block_start should pass through, got %s", o0)
	}
	o1, _ := sr.Event([]byte(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"dsn\":\"PAIArtVeil_SEC"}}`), restore)
	o2, _ := sr.Event([]byte(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"RET_0a1b2c3d4e5f\"}"}}`), restore)
	if len(o1) != 0 || len(o2) != 0 {
		t.Fatalf("input_json_delta fragments must be swallowed, got %d and %d", len(o1), len(o2))
	}
	oStop, err := sr.Event([]byte(`{"type":"content_block_stop","index":1}`), restore)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(oStop) != 2 {
		t.Fatalf("stop should emit [consolidated, stop], got %d: %s", len(oStop), oStop)
	}
	consolidated, stop := oStop[0], oStop[1]
	if gjson.GetBytes(consolidated, "delta.type").Str != "input_json_delta" || gjson.GetBytes(consolidated, "index").Int() != 1 {
		t.Fatalf("first emitted payload is not the consolidated input_json_delta for index 1: %s", consolidated)
	}
	if gjson.GetBytes(stop, "type").Str != "content_block_stop" {
		t.Fatalf("second emitted payload is not the stop: %s", stop)
	}
	// The consolidated partial_json decodes to the restored value.
	var input struct {
		DSN string `json:"dsn"`
	}
	pj := gjson.GetBytes(consolidated, "delta.partial_json").Str
	if err := json.Unmarshal([]byte(pj), &input); err != nil {
		t.Fatalf("consolidated partial_json invalid: %q: %v", pj, err)
	}
	if input.DSN != "real-dsn" {
		t.Fatalf("consolidated dsn = %q, want real-dsn", input.DSN)
	}
}

// Unparseable payloads (e.g. a bare [DONE]) and unknown event types pass through
// unchanged.
func TestStreamPassThroughUnparseableAndUnknown(t *testing.T) {
	restore := mapRestore(nil)
	p := &provider{}
	sr, _ := p.NewStreamRestorer("messages")
	for _, raw := range [][]byte{
		[]byte(`[DONE]`),
		[]byte(`{"type":"some_future_event","x":1}`),
		[]byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`),
	} {
		outs, err := sr.Event(raw, restore)
		if err != nil {
			t.Fatalf("Event(%s): %v", raw, err)
		}
		if len(outs) != 1 || string(outs[0]) != string(raw) {
			t.Fatalf("pass-through altered %s -> %s", raw, outs)
		}
	}
}

// textOf concatenates the delta.text of every text_delta payload.
func textOf(payloads [][]byte) string {
	var b strings.Builder
	for _, p := range payloads {
		if gjson.GetBytes(p, "delta.type").Str == "text_delta" {
			b.WriteString(gjson.GetBytes(p, "delta.text").Str)
		}
	}
	return b.String()
}
