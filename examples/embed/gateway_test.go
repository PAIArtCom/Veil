package embedexample

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	opencloak "github.com/cloakia/opencloak"
)

const throwawayKey = "AKIAIOSFODNN7EXAMPLE"

var tokenRE = regexp.MustCompile(`CLK_[A-Z0-9]+_[0-9a-f]{12,}`)

func newTestGateway(t *testing.T) *Gateway {
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
	engine, err := opencloak.New(opencloak.Config{KeyPath: keyPath})
	if err != nil {
		t.Fatalf("opencloak.New: %v", err)
	}
	gw, err := NewGateway(engine)
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	return gw
}

func anthropicRequest(text string) []byte {
	return []byte(`{"model":"claude-opus-4-5","max_tokens":64,"messages":[{"role":"user","content":` + jsonString(text) + `}]}`)
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func maskOne(t *testing.T, gw *Gateway, scope opencloak.Scope, text string) ([]byte, *Exchange, string) {
	t.Helper()
	masked, ex, err := gw.MaskOutbound(context.Background(), scope, anthropicRequest(text))
	if err != nil {
		t.Fatalf("MaskOutbound: %v", err)
	}
	if bytes.Contains(masked, []byte(throwawayKey)) {
		t.Fatalf("masked provider body leaked plaintext: %s", masked)
	}
	toks := tokenRE.FindAllString(string(masked), -1)
	if len(toks) != 1 {
		t.Fatalf("want exactly one token in masked body, got %d: %s", len(toks), masked)
	}
	return masked, ex, toks[0]
}

func TestGatewayMasksOutboundAndRestoresBufferedToolIO(t *testing.T) {
	gw := newTestGateway(t)
	scope := opencloak.Scope{Tenant: "local", Session: "buffered", Project: "demo"}

	masked, ex, tok := maskOne(t, gw, scope, "run with key "+throwawayKey)
	if !bytes.Contains(masked, []byte("CLK_SECRET_")) {
		t.Fatalf("masked body missing secret token: %s", masked)
	}

	resp := []byte(`{"id":"msg_1","type":"message","role":"assistant","content":[` +
		`{"type":"text","text":"using ` + tok + `"},` +
		`{"type":"tool_use","id":"tu_1","name":"run","input":{"key":"` + tok + `"}}` +
		`],"stop_reason":"end_turn"}`)

	restored, err := gw.RestoreBuffered(context.Background(), ex, resp)
	if err != nil {
		t.Fatalf("RestoreBuffered: %v", err)
	}
	if bytes.Contains(restored, []byte("CLK_")) {
		t.Fatalf("buffered restore left residual token: %s", restored)
	}
	if !bytes.Contains(restored, []byte(throwawayKey)) {
		t.Fatalf("buffered restore did not return real value: %s", restored)
	}
}

func TestGatewayRawStreamRestoresArbitraryByteSplitsAndFlushesTail(t *testing.T) {
	gw := newTestGateway(t)
	_, ex, tok := maskOne(t, gw, opencloak.Scope{Session: "raw-stream"}, "key "+throwawayKey)

	streamed := []byte("prefix " + tok + " suffix")
	var out bytes.Buffer
	for _, b := range streamed {
		out.Write(gw.RestoreRawStreamChunk(ex, []byte{b}))
	}
	out.Write(gw.FlushRawStream(ex))

	got := out.String()
	want := "prefix " + throwawayKey + " suffix"
	if got != want {
		t.Fatalf("raw stream restore mismatch:\n got %q\nwant %q", got, want)
	}
	if strings.Contains(got, "CLK_") {
		t.Fatalf("raw stream restore left residual token: %q", got)
	}
}

func TestGatewayParsedSSEEventRestoresProviderPayload(t *testing.T) {
	gw := newTestGateway(t)
	_, ex, tok := maskOne(t, gw, opencloak.Scope{Session: "sse-event"}, "key "+throwawayKey)

	event := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"value ` + tok + ` done"}}`)
	restored, err := gw.RestoreSSEEvent(context.Background(), ex, event)
	if err != nil {
		t.Fatalf("RestoreSSEEvent: %v", err)
	}
	if !json.Valid(restored) {
		t.Fatalf("restored SSE payload is not valid JSON: %s", restored)
	}
	if bytes.Contains(restored, []byte("CLK_")) {
		t.Fatalf("SSE restore left residual token: %s", restored)
	}
	if !bytes.Contains(restored, []byte(throwawayKey)) {
		t.Fatalf("SSE restore did not return real value: %s", restored)
	}
}

func TestGatewayCrossScopeRestoreLeavesResidualToken(t *testing.T) {
	gw := newTestGateway(t)
	_, _, tokA := maskOne(t, gw, opencloak.Scope{Session: "scope-a"}, "key "+throwawayKey)

	_, exB, err := gw.MaskOutbound(context.Background(), opencloak.Scope{Session: "scope-b"}, anthropicRequest("plain request"))
	if err != nil {
		t.Fatalf("MaskOutbound scope-b: %v", err)
	}
	resp := []byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"using ` + tokA + `"}],"stop_reason":"end_turn"}`)

	restored, err := gw.RestoreBuffered(context.Background(), exB, resp)
	if err != nil {
		t.Fatalf("RestoreBuffered cross-scope: %v", err)
	}
	if bytes.Contains(restored, []byte(throwawayKey)) {
		t.Fatalf("cross-scope restore leaked real value: %s", restored)
	}
	if !bytes.Contains(restored, []byte(tokA)) {
		t.Fatalf("cross-scope restore should leave residual token: %s", restored)
	}
}

func TestGatewayOutboundUnsupportedProviderFailsClosed(t *testing.T) {
	gw := newTestGateway(t)
	bad, err := NewGatewayForProvider(gw.engine, "gemini", "generateContent")
	if err != nil {
		t.Fatalf("NewGatewayForProvider: %v", err)
	}
	masked, ex, err := bad.MaskOutbound(context.Background(), opencloak.Scope{}, anthropicRequest("key "+throwawayKey))
	if err == nil {
		t.Fatal("MaskOutbound with unsupported provider returned nil error")
	}
	if masked != nil || ex != nil {
		t.Fatalf("unsupported provider should not return forwardable output: masked=%s ex=%v", masked, ex)
	}
	if errors.Is(err, opencloak.ErrInvalidState) {
		t.Fatalf("unexpected invalid state error for unsupported provider: %v", err)
	}
}
