package openairesponses_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	veil "github.com/PAIArtCom/Veil"
	"github.com/PAIArtCom/Veil/internal/wire/openairesponses"
)

var (
	ctx                = context.Background()
	tokenRe            = regexp.MustCompile(`PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}`)
	emailSurrogateRe   = regexp.MustCompile(`user-[0-9a-f]{12}@veil\.paiart\.com`)
	urlSurrogateRe     = regexp.MustCompile(`postgresql://user-[0-9a-f]{12}:password-[0-9a-f]{12}@db-[0-9a-f]{12}\.veil\.paiart\.com/prod`)
	placeholderRe      = regexp.MustCompile(`PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}|user-[0-9a-f]{12}@veil\.paiart\.com|postgresql://user-[0-9a-f]{12}:password-[0-9a-f]{12}@db-[0-9a-f]{12}\.veil\.paiart\.com/prod`)
	mixedPlaceholderRe = regexp.MustCompile(`PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}|user-[0-9a-f]{12}@veil\.paiart\.com|(?:https?|postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis)://[^\s"\\<>]*veil\.paiart\.com[^\s"\\<>]*|(?:127\.0\.0|10\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}|192\.168\.\d{1,3}|169\.254\.\d{1,3}|203\.0\.113)\.\d{1,3}|(?:2001:db8:[0-9a-f]{1,4}:[0-9a-f]{1,4}::[0-9a-f]{1,4}|fd00:[0-9a-f]{1,4}:[0-9a-f]{1,4}::[0-9a-f]{1,4}|fe80::[0-9a-f]{1,4}:[0-9a-f]{1,4}:[0-9a-f]{1,4})`)
)

const (
	awsKey = "AKIAIOSFODNN7EXAMPLE"
	email  = "codex-user@example.com"
	dsn    = "postgresql://app:s3cr3t@db.example.com/prod"
)

func newTestEngine(t *testing.T) *veil.Engine {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	keyPath := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	e, err := veil.New(veil.Config{KeyPath: keyPath})
	if err != nil {
		t.Fatalf("veil.New: %v", err)
	}
	return e
}

func TestExtractRequestTopLevelStringInputHasRange(t *testing.T) {
	body := []byte(`{"instructions":"system ` + awsKey + `","input":"email ` + email + `","stream":true}`)

	spans, err := openairesponses.New().ExtractRequest("responses", body)
	if err != nil {
		t.Fatalf("ExtractRequest: %v", err)
	}
	var inputSpanFound bool
	for _, span := range spans {
		if span.Path != "input" {
			continue
		}
		inputSpanFound = true
		if span.Text != "email "+email {
			t.Fatalf("input span text = %q", span.Text)
		}
		if span.Start <= 0 || span.End <= span.Start || span.End > len(body) {
			t.Fatalf("input span invalid range: start=%d end=%d body=%s", span.Start, span.End, body)
		}
		if got := string(body[span.Start:span.End]); got != `"email `+email+`"` {
			t.Fatalf("input span range points to %q", got)
		}
	}
	if !inputSpanFound {
		t.Fatalf("missing top-level input span: %+v", spans)
	}
}

func TestMaskRequestCoversCodexResponsesInputAndToolOutput(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{
		"model":"gpt-5.4",
		"instructions":"Use key ` + awsKey + ` only locally.",
		"prompt":{"id":"pmpt_123","variables":{"contact":"` + email + `"}},
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"email ` + email + `"}]},
			{"type":"function_call_output","call_id":"call_1","output":"database ` + dsn + `"},
			{"type":"function_call","call_id":"call_2","name":"connect","arguments":"{\"dsn\":\"` + dsn + `\"}"}
		],
		"tools":[{"type":"function","name":"leaky_example","description":"static ` + awsKey + ` must stay"}],
		"stream":true
	}`)

	masked, st, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", body)
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
		t.Fatalf("expected PAIArtVeil_ token in masked body: %s", masked)
	}
	if !bytes.Contains(masked, []byte(`static `+awsKey+` must stay`)) {
		t.Fatalf("static tools definition was altered: %s", masked)
	}
	if !bytes.Contains(masked, []byte(`"id":"pmpt_123"`)) {
		t.Fatalf("prompt id should stay unchanged: %s", masked)
	}
}

func TestPromptVariableBackslashKeyMasksInPlace(t *testing.T) {
	e := newTestEngine(t)
	const key = `a\b`
	body, err := json.Marshal(map[string]any{
		"prompt": map[string]any{
			"id": "pmpt_123",
			"variables": map[string]string{
				key: email,
			},
		},
		"input": "hello",
	})
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}

	masked, _, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	if bytes.Contains(masked, []byte(email)) {
		t.Fatalf("prompt variable plaintext leaked: %s", masked)
	}
	var decoded struct {
		Prompt struct {
			Variables map[string]string `json:"variables"`
		} `json:"prompt"`
	}
	if err := json.Unmarshal(masked, &decoded); err != nil {
		t.Fatalf("masked JSON invalid: %v; body=%s", err, masked)
	}
	if len(decoded.Prompt.Variables) != 1 {
		t.Fatalf("expected one prompt variable, got %#v in %s", decoded.Prompt.Variables, masked)
	}
	got := decoded.Prompt.Variables[key]
	if !emailSurrogateRe.MatchString(got) {
		t.Fatalf("prompt variable %q not masked in place: %#v", key, decoded.Prompt.Variables)
	}
}

func TestRestoreResponseCoversOutputTextAndToolCalls(t *testing.T) {
	e := newTestEngine(t)
	masked, st, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", []byte(`{
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"connect `+dsn+` and email `+email+`"}]}]
	}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	placeholders := placeholderRe.FindAll(masked, -1)
	if len(placeholders) < 2 {
		t.Fatalf("expected at least two reversible placeholders: %s", masked)
	}
	resp := []byte(`{"id":"resp_1","output":[
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"using ` + string(placeholders[0]) + `"}]},
		{"type":"function_call","call_id":"call_1","name":"run","arguments":"{\"dsn\":\"` + string(placeholders[0]) + `\"}"},
		{"type":"mcp_call","arguments":"{\"email\":\"` + string(placeholders[1]) + `\"}"},
		{"type":"custom_tool_call","input":"custom ` + string(placeholders[0]) + `"},
		{"type":"code_interpreter_call","code":"print(\"` + string(placeholders[1]) + `\")"}
	]}`)

	restored, err := e.RestoreResponse(ctx, st, resp)
	if err != nil {
		t.Fatalf("RestoreResponse: %v", err)
	}
	if bytes.Contains(restored, []byte("PAIArtVeil_")) {
		t.Fatalf("residual PAIArtVeil_ in restored response: %s", restored)
	}
	for _, plain := range [][]byte{[]byte(dsn), []byte(email)} {
		if !bytes.Contains(restored, plain) {
			t.Fatalf("missing restored value %q in %s", plain, restored)
		}
	}
}

func TestRestoreResponseComplexMixedPlaceholdersAcrossOutputItems(t *testing.T) {
	e := newTestEngine(t)
	const sensitiveURL = "https://api.example.com/v1?token=abc123"
	const ipv4 = "10.20.30.40"
	const ipv6 = "2606:4700:4700::1111"
	const ordinaryURL = "https://supabase.com/docs"

	callArgs, err := json.Marshal(map[string]string{
		"dsn": dsn,
		"v6":  ipv6,
	})
	if err != nil {
		t.Fatalf("marshal call args: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"model":        "gpt-5.4",
		"instructions": "Use key " + awsKey + " and endpoint " + sensitiveURL + " only locally.",
		"prompt": map[string]any{
			"id": "pmpt_123",
			"variables": map[string]string{
				"contact": email,
			},
		},
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": []any{
				map[string]string{"type": "input_text", "text": "ordinary " + ordinaryURL + " private " + ipv4},
			}},
			map[string]string{"type": "function_call_output", "call_id": "call_1", "output": "database " + dsn},
			map[string]string{"type": "function_call", "call_id": "call_2", "name": "connect", "arguments": string(callArgs)},
		},
		"tools": []any{
			map[string]string{"type": "function", "name": "static_schema", "description": "static " + awsKey + " must stay"},
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	masked, st, err := e.MaskRequest(ctx, veil.Scope{Session: "responses-complex"}, "openai-responses", "responses", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	for _, leaked := range [][]byte{[]byte(email), []byte(dsn), []byte(sensitiveURL), []byte(ipv4), []byte(ipv6)} {
		if bytes.Contains(masked, leaked) {
			t.Fatalf("masked request leaked %q in %s", leaked, masked)
		}
	}
	if !bytes.Contains(masked, []byte(ordinaryURL)) {
		t.Fatalf("ordinary URL should remain visible in masked request: %s", masked)
	}
	if !bytes.Contains(masked, []byte(`static `+awsKey+` must stay`)) {
		t.Fatalf("static tool schema example was altered: %s", masked)
	}
	placeholders := uniqueStringMatches(mixedPlaceholderRe, string(masked))
	if len(placeholders) < 6 {
		t.Fatalf("got %d placeholders, want at least 6 in %s: %v", len(placeholders), masked, placeholders)
	}

	argJSON, err := json.Marshal(map[string]string{"combo": strings.Join(placeholders, ",")})
	if err != nil {
		t.Fatalf("marshal response args: %v", err)
	}
	mcpJSON, err := json.Marshal(map[string]string{"contact": placeholders[1], "all": strings.Join(placeholders, "|")})
	if err != nil {
		t.Fatalf("marshal mcp args: %v", err)
	}
	resp, err := json.Marshal(map[string]any{
		"id": "resp_1",
		"output": []any{
			map[string]any{"type": "message", "role": "assistant", "content": []any{
				map[string]string{"type": "output_text", "text": "using " + strings.Join(placeholders, "|") + " ordinary " + ordinaryURL},
			}},
			map[string]string{"type": "function_call", "call_id": "call_1", "name": "run", "arguments": string(argJSON)},
			map[string]string{"type": "mcp_call", "arguments": string(mcpJSON)},
			map[string]string{"type": "custom_tool_call", "input": "custom " + placeholders[0] + " " + placeholders[len(placeholders)-1]},
			map[string]string{"type": "code_interpreter_call", "code": "print(" + placeholders[0] + ")"},
		},
	})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	restored, err := e.RestoreResponse(ctx, st, resp)
	if err != nil {
		t.Fatalf("RestoreResponse: %v", err)
	}
	if !json.Valid(restored) {
		t.Fatalf("restored response is not valid JSON: %s", restored)
	}
	for _, placeholder := range placeholders {
		if bytes.Contains(restored, []byte(placeholder)) {
			t.Fatalf("known placeholder %q survived restore: %s", placeholder, restored)
		}
	}
	for _, value := range [][]byte{[]byte(awsKey), []byte(email), []byte(dsn), []byte(sensitiveURL), []byte(ipv4), []byte(ipv6), []byte(ordinaryURL)} {
		if !bytes.Contains(restored, value) {
			t.Fatalf("restored response missing %q: %s", value, restored)
		}
	}
}

func TestUnsupportedResponsesInputItemFailsClosed(t *testing.T) {
	e := newTestEngine(t)
	_, _, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", []byte(`{
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
		{
			name: "object prompt variable",
			body: `{"prompt":{"id":"pmpt_123","variables":{"image":{"type":"input_image","image_url":"https://example.com/?token=` + awsKey + `"}}},"input":"hello"}`,
		},
		{
			name: "array prompt variables",
			body: `{"prompt":{"id":"pmpt_123","variables":["` + email + `"]},"input":"hello"}`,
		},
		{
			name: "input image block",
			body: `{"input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"https://example.com/private.png?token=` + awsKey + `"}]}]}`,
		},
		{
			name: "input file block",
			body: `{"input":[{"type":"message","role":"user","content":[{"type":"input_file","file_id":"file_secret_reference"}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEngine(t)
			masked, st, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", []byte(tt.body))
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
	_, _, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", []byte(`{"input":[`))
	if err == nil {
		t.Fatal("MaskRequest succeeded for malformed JSON")
	}
}

func TestRestoreSSEEventRestoresParsedResponsesEvents(t *testing.T) {
	e := newTestEngine(t)
	masked, st, err := e.MaskRequest(ctx, veil.Scope{}, "openai-responses", "responses", []byte(`{
		"input":"run `+dsn+`"
	}`))
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	surrogate := urlSurrogateRe.Find(masked)
	if surrogate == nil {
		t.Fatalf("missing URL surrogate: %s", masked)
	}
	event := []byte(`{"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","call_id":"call_1","name":"run","arguments":"{\"dsn\":\"` + string(surrogate) + `\"}"}}`)
	restored, err := e.RestoreSSEEvent(ctx, st, event)
	if err != nil {
		t.Fatalf("RestoreSSEEvent: %v", err)
	}
	if got := gjson.GetBytes(restored, "item.arguments").Str; !bytes.Contains([]byte(got), []byte(dsn)) {
		t.Fatalf("arguments not restored: %s", restored)
	}
}

func uniqueStringMatches(re *regexp.Regexp, s string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, match := range re.FindAllString(s, -1) {
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		out = append(out, match)
	}
	return out
}
