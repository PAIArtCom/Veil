package openairesponses_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/tidwall/gjson"

	opencloak "github.com/cloakia/opencloak"
	_ "github.com/cloakia/opencloak/internal/wire/openairesponses"
)

var (
	ctx     = context.Background()
	tokenRe = regexp.MustCompile(`CLK_[A-Z0-9]+_[0-9a-f]{12,}`)
)

const (
	awsKey = "AKIAIOSFODNN7EXAMPLE"
	email  = "codex-user@example.com"
	dsn    = "https://db.example.com/prod"
)

func newTestEngine(t *testing.T) *opencloak.Engine {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	keyPath := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	e, err := opencloak.New(opencloak.Config{KeyPath: keyPath})
	if err != nil {
		t.Fatalf("opencloak.New: %v", err)
	}
	return e
}

func TestMaskRequestCoversCodexResponsesInputAndToolOutput(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{
		"model":"gpt-5.4",
		"instructions":"Use key ` + awsKey + ` only locally.",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"email ` + email + `"}]},
			{"type":"function_call_output","call_id":"call_1","output":"database ` + dsn + `"},
			{"type":"function_call","call_id":"call_2","name":"connect","arguments":"{\"dsn\":\"` + dsn + `\"}"}
		],
		"tools":[{"type":"function","name":"leaky_example","description":"static ` + awsKey + ` must stay"}],
		"stream":true
	}`)

	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "openai-responses", "responses", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	if st.Provider() != "openai-responses" || st.Op() != "responses" {
		t.Fatalf("state provider/op = %q/%q", st.Provider(), st.Op())
	}
	for _, plain := range [][]byte{[]byte(email), []byte(dsn)} {
		if bytes.Contains(masked, plain) {
			t.Fatalf("plaintext %q leaked in masked request: %s", plain, masked)
		}
	}
	if !tokenRe.Match(masked) {
		t.Fatalf("expected CLK_ token in masked body: %s", masked)
	}
	if !bytes.Contains(masked, []byte(`static `+awsKey+` must stay`)) {
		t.Fatalf("static tools definition was altered: %s", masked)
	}
}

func TestRestoreResponseCoversOutputTextAndToolCalls(t *testing.T) {
	e := newTestEngine(t)
	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "openai-responses", "responses", []byte(`{
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"connect `+dsn+` and email `+email+`"}]}]
	}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	toks := tokenRe.FindAll(masked, -1)
	if len(toks) < 2 {
		t.Fatalf("expected at least two tokens: %s", masked)
	}
	resp := []byte(`{"id":"resp_1","output":[
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"using ` + string(toks[0]) + `"}]},
		{"type":"function_call","call_id":"call_1","name":"run","arguments":"{\"dsn\":\"` + string(toks[0]) + `\"}"},
		{"type":"mcp_call","arguments":"{\"email\":\"` + string(toks[1]) + `\"}"},
		{"type":"custom_tool_call","input":"custom ` + string(toks[0]) + `"},
		{"type":"code_interpreter_call","code":"print(\"` + string(toks[1]) + `\")"}
	]}`)

	restored, err := e.RestoreResponse(ctx, st, resp)
	if err != nil {
		t.Fatalf("RestoreResponse: %v", err)
	}
	if bytes.Contains(restored, []byte("CLK_")) {
		t.Fatalf("residual CLK_ in restored response: %s", restored)
	}
	for _, plain := range [][]byte{[]byte(dsn), []byte(email)} {
		if !bytes.Contains(restored, plain) {
			t.Fatalf("missing restored value %q in %s", plain, restored)
		}
	}
}

func TestUnsupportedResponsesInputItemFailsClosed(t *testing.T) {
	e := newTestEngine(t)
	_, _, err := e.MaskRequest(ctx, opencloak.Scope{}, "openai-responses", "responses", []byte(`{
		"input":[{"type":"future_tool_result","payload":"`+dsn+`"}]
	}`))
	if err == nil {
		t.Fatal("MaskRequest succeeded for unsupported plaintext-bearing input item")
	}
}

func TestUnsupportedResponsesPlaintextFieldShapesFailClosed(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "object instructions",
			body: `{"instructions":{"text":"` + awsKey + `"},"input":"hello"}`,
		},
		{
			name: "object function arguments",
			body: `{"input":[{"type":"function_call","call_id":"call_1","name":"run","arguments":{"dsn":"` + dsn + `"}}]}`,
		},
		{
			name: "object tool output",
			body: `{"input":[{"type":"function_call_output","call_id":"call_1","output":{"dsn":"` + dsn + `"}}]}`,
		},
		{
			name: "object message text",
			body: `{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":{"secret":"` + awsKey + `"}}]}]}`,
		},
		{
			name: "array custom tool input",
			body: `{"input":[{"type":"custom_tool_call","call_id":"call_1","input":["` + dsn + `"]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEngine(t)
			masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "openai-responses", "responses", []byte(tt.body))
			if err == nil {
				t.Fatalf("MaskRequest succeeded for unsupported plaintext-bearing shape: masked=%s state=%v", masked, st)
			}
			if masked != nil || st != nil {
				t.Fatalf("unsupported shape should not return forwardable output: masked=%s state=%v", masked, st)
			}
		})
	}
}

func TestMalformedResponsesJSONFailsClosed(t *testing.T) {
	e := newTestEngine(t)
	_, _, err := e.MaskRequest(ctx, opencloak.Scope{}, "openai-responses", "responses", []byte(`{"input":[`))
	if err == nil {
		t.Fatal("MaskRequest succeeded for malformed JSON")
	}
}

func TestRestoreSSEEventRestoresParsedResponsesEvents(t *testing.T) {
	e := newTestEngine(t)
	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "openai-responses", "responses", []byte(`{
		"input":"run `+dsn+`"
	}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	tok := tokenRe.Find(masked)
	if tok == nil {
		t.Fatalf("missing token: %s", masked)
	}
	event := []byte(`{"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","call_id":"call_1","name":"run","arguments":"{\"dsn\":\"` + string(tok) + `\"}"}}`)
	restored, err := e.RestoreSSEEvent(ctx, st, event)
	if err != nil {
		t.Fatalf("RestoreSSEEvent: %v", err)
	}
	if got := gjson.GetBytes(restored, "item.arguments").Str; !bytes.Contains([]byte(got), []byte(dsn)) {
		t.Fatalf("arguments not restored: %s", restored)
	}
}
