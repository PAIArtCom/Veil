package openairesponses_test

import (
	"bytes"
	"testing"

	"github.com/tidwall/gjson"

	opencloak "github.com/cloakia/opencloak"
)

func TestResponsesStreamRestoresSplitOutputTextAndFunctionArguments(t *testing.T) {
	e := newTestEngine(t)
	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "openai-responses", "responses", []byte(`{"input":"use `+dsn+`"}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	tok := string(tokenRe.Find(masked))
	if tok == "" {
		t.Fatalf("missing token: %s", masked)
	}
	sse, err := e.NewSSEStreamRestorer(st)
	if err != nil {
		t.Fatalf("NewSSEStreamRestorer: %v", err)
	}

	cut := len(tok) / 2
	events := [][]byte{
		[]byte(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"connect ` + tok[:cut] + `"}`),
		[]byte(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"` + tok[cut:] + ` now"}`),
		[]byte(`{"type":"response.output_text.done","item_id":"msg_1","output_index":0,"content_index":0,"text":"connect ` + tok + ` now"}`),
		[]byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":1,"delta":"{\"dsn\":\"` + tok[:cut] + `"}`),
		[]byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":1,"delta":"` + tok[cut:] + `\"}"}`),
		[]byte(`{"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":1,"arguments":"{\"dsn\":\"` + tok + `\"}"}`),
	}

	var out [][]byte
	for _, ev := range events {
		chunks, err := sse.Event(ctx, ev)
		if err != nil {
			t.Fatalf("Event(%s): %v", ev, err)
		}
		out = append(out, chunks...)
	}
	flushed, err := sse.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush: %v", err)
	}
	out = append(out, flushed...)
	joined := bytes.Join(out, []byte("\n"))
	if bytes.Contains(joined, []byte("OpenCloak_")) {
		t.Fatalf("residual token in stream output: %s", joined)
	}
	if !bytes.Contains(joined, []byte(dsn)) {
		t.Fatalf("missing restored dsn in stream output: %s", joined)
	}

	var foundArgs bool
	for _, payload := range out {
		if gjson.GetBytes(payload, "type").Str == "response.function_call_arguments.delta" {
			foundArgs = bytes.Contains([]byte(gjson.GetBytes(payload, "delta").Str), []byte(dsn))
		}
	}
	if !foundArgs {
		t.Fatalf("missing restored synthetic function_call_arguments.delta in %s", joined)
	}
}

func TestResponsesStreamRestoresCompletedEventOutput(t *testing.T) {
	e := newTestEngine(t)
	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "openai-responses", "responses", []byte(`{"input":"email `+email+`"}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	tok := string(tokenRe.Find(masked))
	sse, err := e.NewSSEStreamRestorer(st)
	if err != nil {
		t.Fatalf("NewSSEStreamRestorer: %v", err)
	}
	outs, err := sse.Event(ctx, []byte(`{"type":"response.completed","response":{"id":"resp_1","output":[{"type":"message","content":[{"type":"output_text","text":"`+tok+`"}]}]}}`))
	if err != nil {
		t.Fatalf("Event: %v", err)
	}
	if len(outs) != 1 {
		t.Fatalf("outs len = %d, want 1", len(outs))
	}
	if bytes.Contains(outs[0], []byte("OpenCloak_")) || !bytes.Contains(outs[0], []byte(email)) {
		t.Fatalf("completed event not restored: %s", outs[0])
	}
}
