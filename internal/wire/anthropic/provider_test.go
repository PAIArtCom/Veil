package anthropic_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	opencloak "github.com/cloakia/opencloak"
)

// ---- helpers ----------------------------------------------------------------

// newTestEngine builds an Engine backed by a fixed, deterministic key.
func newTestEngine(t *testing.T) *opencloak.Engine {
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
	e, err := opencloak.New(opencloak.Config{KeyPath: keyPath})
	if err != nil {
		t.Fatalf("opencloak.New: %v", err)
	}
	return e
}

var ctx = context.Background()

var tokenRe = regexp.MustCompile(`CLK_[A-Z0-9]+_[0-9a-f]{12,}`)

// ---- ExtractRequest / ApplyRequest unit tests --------------------------------

// These tests drive the Provider indirectly through Engine.MaskRequest so we
// exercise the full stack (extract → mask → apply) without importing internal/.

// TestExtractSystemString verifies system-as-string masking.
func TestExtractSystemString(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{"model":"claude-opus-4-5","max_tokens":1024,"system":"Use key AKIAIOSFODNN7EXAMPLE for auth","messages":[]}`)

	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	if st == nil {
		t.Fatal("State is nil")
	}
	if bytes.Contains(masked, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Fatalf("AWS key not masked in system string: %s", masked)
	}
	if !tokenRe.Match(masked) {
		t.Fatalf("expected CLK_ token in masked body: %s", masked)
	}
	// Non-text fields must be byte-identical.
	if !bytes.Contains(masked, []byte(`"model":"claude-opus-4-5"`)) {
		t.Fatalf("model field altered: %s", masked)
	}
}

// TestExtractSystemArray verifies system-as-array masking; cache_control must
// not be touched and non-text blocks must be skipped.
func TestExtractSystemArray(t *testing.T) {
	e := newTestEngine(t)
	// Two system blocks: a text block with a secret and a non-text block.
	// The cache_control field must survive unchanged.
	body := []byte(`{
		"model": "claude-opus-4-5",
		"max_tokens": 1024,
		"system": [
			{"type":"text","text":"Connect with AKIAIOSFODNN7EXAMPLE","cache_control":{"type":"ephemeral"}},
			{"type":"text","text":"plain instruction"},
			{"type":"image","source":{"type":"url","url":"https://example.com/img.png"}}
		],
		"messages": []
	}`)

	masked, _, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	// The AWS key in block 0 must be masked.
	if bytes.Contains(masked, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Fatalf("AWS key not masked in system array: %s", masked)
	}
	// cache_control must be preserved.
	if !bytes.Contains(masked, []byte(`"cache_control":{"type":"ephemeral"}`)) {
		t.Fatalf("cache_control altered or missing: %s", masked)
	}
	// The image block (type="image") is not touched and must still be there.
	if !bytes.Contains(masked, []byte(`"type":"image"`)) {
		t.Fatalf("image block removed from system array: %s", masked)
	}
	// "plain instruction" has no secrets and must be unchanged.
	if !bytes.Contains(masked, []byte("plain instruction")) {
		t.Fatalf("plain instruction text altered: %s", masked)
	}
}

// TestExtractMessageStringContent verifies message string-content masking.
func TestExtractMessageStringContent(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{
		"model": "claude-opus-4-5",
		"max_tokens": 1024,
		"messages": [
			{"role":"user","content":"my email is user@example.com and key AKIAIOSFODNN7EXAMPLE"}
		]
	}`)

	masked, _, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	if bytes.Contains(masked, []byte("user@example.com")) {
		t.Fatalf("email not masked: %s", masked)
	}
	if bytes.Contains(masked, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Fatalf("AWS key not masked: %s", masked)
	}
}

// TestExtractMessageBlockContent verifies block-content masking for text and
// tool_result blocks (both string and array forms), plus tool_use.input.
//
// We use detectable values: an email and AWS key for the text block, a
// postgres URL (detected as TypeURL) and an AWS key for tool_use.input, and
// an AWS key in the tool_result.
func TestExtractMessageBlockContent(t *testing.T) {
	e := newTestEngine(t)
	// Use a postgres URL as the "password" field value — it contains an https
	// URL-shaped string that L1 detects, and the AWS key is always detected.
	// The host field uses a value that won't be detected (a plain hostname is
	// not a URL without the scheme).
	const toolURL = "https://db.example.com/prod"
	body := []byte(`{
		"model": "claude-opus-4-5",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": [
					{"type":"text","text":"my email is user@example.com"},
					{"type":"tool_use","id":"tu_1","name":"connect","input":{"dsn":"` + toolURL + `","key":"AKIAIOSFODNN7EXAMPLE"}},
					{"type":"tool_result","tool_use_id":"tu_1","content":"result with key AKIAIOSFODNN7EXAMPLE"}
				]
			}
		]
	}`)

	masked, _, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	// text block
	if bytes.Contains(masked, []byte("user@example.com")) {
		t.Fatalf("email not masked in text block: %s", masked)
	}
	// tool_use.input string leaves — URL and AWS key must be masked.
	if bytes.Contains(masked, []byte(toolURL)) {
		t.Fatalf("URL not masked in tool_use.input dsn: %s", masked)
	}
	if bytes.Contains(masked, []byte(`"key":"AKIAIOSFODNN7EXAMPLE"`)) {
		t.Fatalf("AWS key not masked in tool_use.input: %s", masked)
	}
	// tool_result string content
	if bytes.Contains(masked, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Fatalf("AWS key not masked in tool_result: %s", masked)
	}
	// tool_use id and name must be preserved (not user content)
	if !bytes.Contains(masked, []byte(`"id":"tu_1"`)) {
		t.Fatalf("tool_use id altered: %s", masked)
	}
}

// TestToolResultArrayContent verifies tool_result with array content form.
func TestToolResultArrayContent(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{
		"model": "claude-opus-4-5",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "tool_result",
						"tool_use_id": "tu_2",
						"content": [
							{"type":"text","text":"secret key AKIAIOSFODNN7EXAMPLE"},
							{"type":"image","source":{"type":"url","url":"https://example.com/img.png"}}
						]
					}
				]
			}
		]
	}`)

	masked, _, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	if bytes.Contains(masked, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Fatalf("AWS key not masked in tool_result array content: %s", masked)
	}
	// image block inside tool_result content must be preserved
	if !bytes.Contains(masked, []byte(`"type":"image"`)) {
		t.Fatalf("image block in tool_result content removed: %s", masked)
	}
}

// TestToolsArrayUnchanged verifies that tools[] (schema definitions) are NOT
// masked. We embed a fake secret-looking string in a description field and
// confirm it passes through untouched.
func TestToolsArrayUnchanged(t *testing.T) {
	e := newTestEngine(t)
	// Note: AKIAIOSFODNN7EXAMPLE in a tool description must NOT be masked.
	body := []byte(`{
		"model": "claude-opus-4-5",
		"max_tokens": 1024,
		"tools": [
			{
				"name": "get_secret",
				"description": "Retrieve AKIAIOSFODNN7EXAMPLE from vault",
				"input_schema": {
					"type": "object",
					"properties": {
						"key": {"type": "string"}
					}
				}
			}
		],
		"messages": [
			{"role":"user","content":"hello"}
		]
	}`)

	masked, _, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	// tools[] must not be modified.
	if !bytes.Contains(masked, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Fatalf("tools[] description was masked (must be left unchanged): %s", masked)
	}
}

// TestImageThinkingBlocksSkipped verifies image and thinking blocks in messages
// are not touched.
func TestImageThinkingBlocksSkipped(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{
		"model": "claude-opus-4-5",
		"max_tokens": 1024,
		"messages": [
			{
				"role": "user",
				"content": [
					{"type":"image","source":{"type":"base64","media_type":"image/png","data":"abc123=="}},
					{"type":"thinking","thinking":"some model internal thought"},
					{"type":"text","text":"hello world"}
				]
			}
		]
	}`)

	masked, _, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	// image data must be unchanged.
	if !bytes.Contains(masked, []byte(`"data":"abc123=="`)) {
		t.Fatalf("image data was altered: %s", masked)
	}
	// thinking block must be unchanged.
	if !bytes.Contains(masked, []byte("some model internal thought")) {
		t.Fatalf("thinking block was altered: %s", masked)
	}
	// text block with no secrets must pass through.
	if !bytes.Contains(masked, []byte("hello world")) {
		t.Fatalf("plain text altered: %s", masked)
	}
}

// ---- MaskRequest end-to-end --------------------------------------------------

// TestMaskRequestEndToEnd tests a realistic multi-field request: secrets in
// system, user message, and tool_use arg. Asserts model sees only CLK_ tokens,
// State carries provider/op, and non-secret JSON keys/order is preserved.
func TestMaskRequestEndToEnd(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{"model":"claude-opus-4-5","max_tokens":2048,"system":"Connect using AKIAIOSFODNN7EXAMPLE for AWS","messages":[{"role":"user","content":"my email is user@example.com"},{"role":"assistant","content":[{"type":"tool_use","id":"tu_99","name":"run_sql","input":{"connection":"postgres://admin:hunter2@db.example.com/prod"}}]}],"extra_field":"must-survive"}`)

	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}
	if st == nil {
		t.Fatal("nil State")
	}

	// provider and op stored in state
	if st.Provider() != "anthropic" {
		t.Fatalf("State.Provider = %q, want %q", st.Provider(), "anthropic")
	}
	if st.Op() != "messages" {
		t.Fatalf("State.Op = %q, want %q", st.Op(), "messages")
	}

	// secrets must not appear in masked body
	for _, secret := range []string{"AKIAIOSFODNN7EXAMPLE", "user@example.com", "hunter2"} {
		if bytes.Contains(masked, []byte(secret)) {
			t.Fatalf("secret %q still present in masked body: %s", secret, masked)
		}
	}

	// non-secret structural fields must be preserved
	for _, must := range []string{`"model":"claude-opus-4-5"`, `"max_tokens":2048`, `"extra_field":"must-survive"`, `"id":"tu_99"`, `"name":"run_sql"`} {
		if !bytes.Contains(masked, []byte(must)) {
			t.Fatalf("expected field %q missing from masked body: %s", must, masked)
		}
	}
}

func TestMaskRequestMalformedJSONFailsClosed(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{"model":"claude-opus-4-5","messages":[{"role":"user","content":"AKIAIOSFODNN7EXAMPLE"}`)

	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages", body)
	if err == nil {
		t.Fatalf("MaskRequest returned nil error; masked=%s st=%v", masked, st)
	}
	if masked != nil || st != nil {
		t.Fatalf("malformed JSON must fail closed with nil output/state; masked=%s st=%v", masked, st)
	}
}

func TestMaskRequestUnsupportedOperationFailsClosed(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{"model":"claude-opus-4-5","max_tokens":16,"messages":[{"role":"user","content":"AKIAIOSFODNN7EXAMPLE"}]}`)

	masked, st, err := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "responses", body)
	if err == nil {
		t.Fatalf("MaskRequest returned nil error for unsupported op; masked=%s st=%v", masked, st)
	}
	if masked != nil || st != nil {
		t.Fatalf("unsupported op must fail closed with nil output/state; masked=%s st=%v", masked, st)
	}
}

// ---- Determinism / cache-stability -------------------------------------------

// TestMaskRequestDeterminism verifies that masking the same body twice
// produces byte-identical output — required for Anthropic prompt-cache prefix
// stability.
func TestMaskRequestDeterminism(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{"model":"claude-opus-4-5","max_tokens":1024,"system":"key AKIAIOSFODNN7EXAMPLE","messages":[{"role":"user","content":"email user@example.com"}]}`)

	scope := opencloak.Scope{Session: "cache-test"}
	masked1, _, err := e.MaskRequest(ctx, scope, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest 1: %v", err)
	}
	masked2, _, err := e.MaskRequest(ctx, scope, "anthropic", "messages", body)
	if err != nil {
		t.Fatalf("MaskRequest 2: %v", err)
	}
	if !bytes.Equal(masked1, masked2) {
		t.Fatalf("non-deterministic output:\ncall1: %s\ncall2: %s", masked1, masked2)
	}
}

// ---- RestoreResponse ---------------------------------------------------------

// TestRestoreResponseTextBlock verifies round-trip for a response with a text block.
func TestRestoreResponseTextBlock(t *testing.T) {
	e := newTestEngine(t)
	reqBody := []byte(`{"model":"claude-opus-4-5","max_tokens":1024,"system":"key AKIAIOSFODNN7EXAMPLE","messages":[{"role":"user","content":"hello"}]}`)

	scope := opencloak.Scope{Session: "resp-test"}
	_, st, err := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}

	// Extract the token that was minted for the AWS key.
	maskedReq, _, _ := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	sysVal := string(maskedReq)
	tok := extractFirstToken(sysVal)
	if tok == "" {
		t.Fatalf("no token found in masked request: %s", maskedReq)
	}

	// Build a synthetic assistant response containing that token in a text block.
	respBody := []byte(`{"role":"assistant","content":[{"type":"text","text":"The key is ` + tok + ` — done."}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`)

	restored, err := e.RestoreResponse(ctx, st, respBody)
	if err != nil {
		t.Fatalf("RestoreResponse: %v", err)
	}
	if bytes.Contains(restored, []byte(tok)) {
		t.Fatalf("token not restored in response: %s", restored)
	}
	if !bytes.Contains(restored, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Fatalf("original value not in restored response: %s", restored)
	}
	// usage field must survive restore unchanged.
	if !bytes.Contains(restored, []byte(`"usage":{"input_tokens":10,"output_tokens":5}`)) {
		t.Fatalf("usage field altered: %s", restored)
	}
}

// TestRestoreResponseToolUseInput verifies round-trip for tool_use.input string leaves.
func TestRestoreResponseToolUseInput(t *testing.T) {
	e := newTestEngine(t)
	reqBody := []byte(`{"model":"claude-opus-4-5","max_tokens":1024,"messages":[{"role":"user","content":"connect to postgres://admin:hunter2@db.example.com/prod"}]}`)

	scope := opencloak.Scope{Session: "tool-resp-test"}
	_, st, err := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}

	// Find the URL token from the masked request.
	maskedReq, _, _ := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	tok := extractFirstToken(string(maskedReq))
	if tok == "" {
		t.Fatalf("no token found in masked request: %s", maskedReq)
	}

	// Build a synthetic response where the assistant emitted a tool_use with
	// that token in the input.
	respBody := []byte(`{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"run_query","input":{"dsn":"` + tok + `","limit":100}}],"stop_reason":"tool_use"}`)

	restored, err := e.RestoreResponse(ctx, st, respBody)
	if err != nil {
		t.Fatalf("RestoreResponse: %v", err)
	}
	if bytes.Contains(restored, []byte(tok)) {
		t.Fatalf("token not restored in tool_use.input: %s", restored)
	}
	// The non-string numeric field limit:100 must survive.
	if !bytes.Contains(restored, []byte(`"limit":100`)) {
		t.Fatalf("numeric field altered: %s", restored)
	}
}

// TestRestoreResponseErrInvalidState checks ErrInvalidState conditions.
func TestRestoreResponseErrInvalidState(t *testing.T) {
	e := newTestEngine(t)
	body := []byte(`{"role":"assistant","content":[{"type":"text","text":"hello"}]}`)

	// nil state
	_, err := e.RestoreResponse(ctx, nil, body)
	if !errors.Is(err, opencloak.ErrInvalidState) {
		t.Fatalf("nil State: expected ErrInvalidState, got %v", err)
	}

	// state without provider/op (from text Mask, not MaskRequest)
	_, st, _ := e.Mask(ctx, opencloak.Scope{}, "some text")
	_, err = e.RestoreResponse(ctx, st, body)
	if !errors.Is(err, opencloak.ErrInvalidState) {
		t.Fatalf("text-only State: expected ErrInvalidState, got %v", err)
	}
}

// ---- RestoreSSEEvent ---------------------------------------------------------

// TestRestoreSSETextDelta verifies content_block_delta + text_delta restore.
func TestRestoreSSETextDelta(t *testing.T) {
	e := newTestEngine(t)
	reqBody := []byte(`{"model":"claude-opus-4-5","max_tokens":1024,"system":"key AKIAIOSFODNN7EXAMPLE","messages":[{"role":"user","content":"tell me"}]}`)

	scope := opencloak.Scope{Session: "sse-text-test"}
	_, st, err := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}

	maskedReq, _, _ := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	tok := extractFirstToken(string(maskedReq))
	if tok == "" {
		t.Fatalf("no token found: %s", maskedReq)
	}

	eventData := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"The key is ` + tok + `"}}`)

	restored, err := e.RestoreSSEEvent(ctx, st, eventData)
	if err != nil {
		t.Fatalf("RestoreSSEEvent: %v", err)
	}
	if bytes.Contains(restored, []byte(tok)) {
		t.Fatalf("token not restored in text_delta: %s", restored)
	}
	if !bytes.Contains(restored, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Fatalf("original value not in restored event: %s", restored)
	}
	// Other event fields must be preserved.
	if !bytes.Contains(restored, []byte(`"index":0`)) {
		t.Fatalf("index field altered: %s", restored)
	}
}

// TestRestoreSSEInputJsonDelta verifies content_block_delta + input_json_delta restore.
func TestRestoreSSEInputJsonDelta(t *testing.T) {
	e := newTestEngine(t)
	reqBody := []byte(`{"model":"claude-opus-4-5","max_tokens":1024,"messages":[{"role":"user","content":"email user@example.com"}]}`)

	scope := opencloak.Scope{Session: "sse-json-test"}
	_, st, err := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}

	maskedReq, _, _ := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	tok := extractFirstToken(string(maskedReq))
	if tok == "" {
		t.Fatalf("no token found: %s", maskedReq)
	}

	eventData := []byte(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"email\":\"` + tok + `\"}"}}`)

	restored, err := e.RestoreSSEEvent(ctx, st, eventData)
	if err != nil {
		t.Fatalf("RestoreSSEEvent: %v", err)
	}
	if bytes.Contains(restored, []byte(tok)) {
		t.Fatalf("token not restored in input_json_delta: %s", restored)
	}
	if !bytes.Contains(restored, []byte("user@example.com")) {
		t.Fatalf("original email not in restored event: %s", restored)
	}
}

// TestRestoreSSEMessageStartUnchanged verifies that a message_start event is
// returned byte-identical (not touched).
func TestRestoreSSEMessageStartUnchanged(t *testing.T) {
	e := newTestEngine(t)
	_, st, _ := e.MaskRequest(ctx, opencloak.Scope{}, "anthropic", "messages",
		[]byte(`{"model":"claude-opus-4-5","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`))

	original := []byte(`{"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","content":[],"model":"claude-opus-4-5","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":5,"output_tokens":1}}}`)

	restored, err := e.RestoreSSEEvent(ctx, st, original)
	if err != nil {
		t.Fatalf("RestoreSSEEvent: %v", err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatalf("message_start event was modified:\noriginal: %s\nrestored: %s", original, restored)
	}
}

// TestRestoreSSEErrInvalidState checks ErrInvalidState for RestoreSSEEvent.
func TestRestoreSSEErrInvalidState(t *testing.T) {
	e := newTestEngine(t)
	eventData := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)

	// nil State
	_, err := e.RestoreSSEEvent(ctx, nil, eventData)
	if !errors.Is(err, opencloak.ErrInvalidState) {
		t.Fatalf("nil State: expected ErrInvalidState, got %v", err)
	}

	// State from text Mask (no provider/op)
	_, st, _ := e.Mask(ctx, opencloak.Scope{}, "text")
	_, err = e.RestoreSSEEvent(ctx, st, eventData)
	if !errors.Is(err, opencloak.ErrInvalidState) {
		t.Fatalf("text-only State: expected ErrInvalidState, got %v", err)
	}
}

// ---- Full round-trip ---------------------------------------------------------

// TestFullRoundTrip performs a complete mask → embed-in-response → restore
// cycle and confirms the final text equals the original secrets.
func TestFullRoundTrip(t *testing.T) {
	e := newTestEngine(t)

	const awsKey = "AKIAIOSFODNN7EXAMPLE"
	const email = "user@example.com"

	reqBody := []byte(`{"model":"claude-opus-4-5","max_tokens":1024,"system":"key ` + awsKey + `","messages":[{"role":"user","content":"email is ` + email + `"}]}`)

	scope := opencloak.Scope{Session: "roundtrip"}
	maskedReq, st, err := e.MaskRequest(ctx, scope, "anthropic", "messages", reqBody)
	if err != nil {
		t.Fatalf("MaskRequest: %v", err)
	}

	// Collect all tokens that appear in the masked request.
	toks := tokenRe.FindAllString(string(maskedReq), -1)
	if len(toks) == 0 {
		t.Fatal("no tokens in masked request")
	}

	// Build a synthetic assistant response echoing the tokens.
	respText := "Here are your values: " + strings.Join(toks, " and ")
	respBody := []byte(`{"role":"assistant","content":[{"type":"text","text":"` + respText + `"}],"stop_reason":"end_turn"}`)

	restored, err := e.RestoreResponse(ctx, st, respBody)
	if err != nil {
		t.Fatalf("RestoreResponse: %v", err)
	}
	restoredStr := string(restored)
	if strings.Contains(restoredStr, "CLK_") {
		t.Fatalf("residual CLK_ token in restored response: %s", restored)
	}
	if !strings.Contains(restoredStr, awsKey) {
		t.Fatalf("AWS key not in restored response: %s", restored)
	}
	if !strings.Contains(restoredStr, email) {
		t.Fatalf("email not in restored response: %s", restored)
	}
}

// ---- helpers -----------------------------------------------------------------

// extractFirstToken finds the first CLK_… token in s.
func extractFirstToken(s string) string {
	m := tokenRe.FindString(s)
	return m
}
