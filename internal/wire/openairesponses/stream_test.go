package openairesponses_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	veil "github.com/PAIArtCom/Veil"
)

func TestResponsesStreamComplexMixedPlaceholdersAcrossOutputIndices(t *testing.T) {
	e := newTestEngine(t)
	const sensitiveURL = "https://api.example.com/v1?token=abc123"
	const ipv4 = "10.20.30.40"
	const ipv6 = "2606:4700:4700::1111"
	const ordinaryURL = "https://supabase.com/docs"

	body, err := json.Marshal(map[string]any{
		"input": "key " + awsKey + " email " + email + " dsn " + dsn + " url " + sensitiveURL + " ip " + ipv4 + " v6 " + ipv6 + " ordinary " + ordinaryURL,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	masked, st, err := e.MaskRequest(ctx, veil.Scope{Session: "responses-stream-complex"}, "openai-responses", "responses", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	placeholders := uniqueStringMatches(mixedPlaceholderRe, string(masked))
	if len(placeholders) < 6 {
		t.Fatalf("got %d placeholders, want at least 6 in %s: %v", len(placeholders), masked, placeholders)
	}
	sse, err := e.NewSSEStreamRestorer(st)
	if err != nil {
		t.Fatalf("NewSSEStreamRestorer: %v", err)
	}

	textComplete := strings.Join(placeholders, "|") + "|ordinary=" + ordinaryURL
	textCut1 := strings.Index(textComplete, placeholders[1]) + len(placeholders[1])/2
	textCut2 := strings.Index(textComplete, placeholders[len(placeholders)-1]) + len(placeholders[len(placeholders)-1])/2
	textFrags := splitString(textComplete, textCut1, textCut2)

	argsBytes, err := json.Marshal(map[string]string{
		"combo": strings.Join(placeholders, ","),
		"email": placeholders[1],
		"last":  placeholders[len(placeholders)-1],
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	argsComplete := string(argsBytes)
	argCut1 := strings.Index(argsComplete, placeholders[0]) + len(placeholders[0])/2
	argCut2 := strings.Index(argsComplete, placeholders[len(placeholders)-1]) + len(placeholders[len(placeholders)-1])/2
	argFrags := splitString(argsComplete, argCut1, argCut2)

	events := [][]byte{
		mustJSON(t, map[string]any{"type": "response.output_text.delta", "item_id": "msg_1", "output_index": 0, "content_index": 0, "delta": "mixed " + textFrags[0]}),
		mustJSON(t, map[string]any{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "output_index": 1, "delta": argFrags[0]}),
		mustJSON(t, map[string]any{"type": "response.output_text.delta", "item_id": "msg_1", "output_index": 0, "content_index": 0, "delta": textFrags[1]}),
		mustJSON(t, map[string]any{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "output_index": 1, "delta": argFrags[1]}),
		mustJSON(t, map[string]any{"type": "response.output_text.delta", "item_id": "msg_1", "output_index": 0, "content_index": 0, "delta": textFrags[2] + " done"}),
		mustJSON(t, map[string]any{"type": "response.function_call_arguments.delta", "item_id": "fc_1", "output_index": 1, "delta": argFrags[2]}),
		mustJSON(t, map[string]any{"type": "response.output_text.done", "item_id": "msg_1", "output_index": 0, "content_index": 0, "text": "mixed " + textComplete + " done"}),
		mustJSON(t, map[string]any{"type": "response.function_call_arguments.done", "item_id": "fc_1", "output_index": 1, "arguments": argsComplete}),
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
	for _, placeholder := range placeholders {
		if bytes.Contains(joined, []byte(placeholder)) {
			t.Fatalf("known placeholder %q survived stream restore: %s", placeholder, joined)
		}
	}
	for _, value := range [][]byte{[]byte(awsKey), []byte(email), []byte(dsn), []byte(sensitiveURL), []byte(ipv4), []byte(ipv6), []byte(ordinaryURL)} {
		if !bytes.Contains(joined, value) {
			t.Fatalf("stream output missing restored value %q: %s", value, joined)
		}
	}

	var foundArgs bool
	for _, payload := range out {
		if gjson.GetBytes(payload, "type").Str != "response.function_call_arguments.delta" {
			continue
		}
		if idx := gjson.GetBytes(payload, "output_index").Int(); idx != 1 {
			t.Fatalf("function arguments delta keyed to index %d, want 1: %s", idx, payload)
		}
		var args map[string]string
		if err := json.Unmarshal([]byte(gjson.GetBytes(payload, "delta").Str), &args); err != nil {
			t.Fatalf("restored function arguments delta is not valid JSON: %s: %v", payload, err)
		}
		if strings.Contains(args["combo"], "PAIArtVeil_") || !strings.Contains(args["combo"], dsn) || !strings.Contains(args["combo"], ipv6) {
			t.Fatalf("function arguments not restored: %#v", args)
		}
		foundArgs = true
	}
	if !foundArgs {
		t.Fatalf("missing restored function_call_arguments.delta in %s", joined)
	}
}

func TestResponsesStreamRestoresSplitOutputTextAndFunctionArguments(t *testing.T) {
	e := newTestEngine(t)
	masked, st, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", []byte(`{"input":"use `+dsn+`"}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	surrogate := string(urlSurrogateRe.Find(masked))
	if surrogate == "" {
		t.Fatalf("missing URL surrogate: %s", masked)
	}
	sse, err := e.NewSSEStreamRestorer(st)
	if err != nil {
		t.Fatalf("NewSSEStreamRestorer: %v", err)
	}

	cut := len(surrogate) / 2
	events := [][]byte{
		[]byte(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"connect ` + surrogate[:cut] + `"}`),
		[]byte(`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"` + surrogate[cut:] + ` now"}`),
		[]byte(`{"type":"response.output_text.done","item_id":"msg_1","output_index":0,"content_index":0,"text":"connect ` + surrogate + ` now"}`),
		[]byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":1,"delta":"{\"dsn\":\"` + surrogate[:cut] + `"}`),
		[]byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":1,"delta":"` + surrogate[cut:] + `\"}"}`),
		[]byte(`{"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":1,"arguments":"{\"dsn\":\"` + surrogate + `\"}"}`),
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
	if bytes.Contains(joined, []byte("PAIArtVeil_")) {
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
	masked, st, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", []byte(`{"input":"email `+email+`"}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	surrogate := string(emailSurrogateRe.Find(masked))
	if surrogate == "" {
		t.Fatalf("missing email surrogate: %s", masked)
	}
	sse, err := e.NewSSEStreamRestorer(st)
	if err != nil {
		t.Fatalf("NewSSEStreamRestorer: %v", err)
	}
	outs, err := sse.Event(ctx, []byte(`{"type":"response.completed","response":{"id":"resp_1","output":[{"type":"message","content":[{"type":"output_text","text":"`+surrogate+`"}]}]}}`))
	if err != nil {
		t.Fatalf("Event: %v", err)
	}
	if len(outs) != 1 {
		t.Fatalf("outs len = %d, want 1", len(outs))
	}
	if bytes.Contains(outs[0], []byte("PAIArtVeil_")) || !bytes.Contains(outs[0], []byte(email)) {
		t.Fatalf("completed event not restored: %s", outs[0])
	}
}

func splitString(s string, cuts ...int) []string {
	frags := make([]string, 0, len(cuts)+1)
	prev := 0
	for _, cut := range cuts {
		frags = append(frags, s[prev:cut])
		prev = cut
	}
	frags = append(frags, s[prev:])
	return frags
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal JSON event: %v", err)
	}
	return out
}
