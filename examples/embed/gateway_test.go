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

	veil "github.com/PAIArtCom/Veil"
)

const throwawayKey = "AKIAIOSFODNN7EXAMPLE"

var tokenRE = regexp.MustCompile(`PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}`)
var mixedPlaceholderRE = regexp.MustCompile(`PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}|user-[0-9a-f]{12}@veil\.paiart\.com|(?:https?|postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis)://[^\s"\\<>]*veil\.paiart\.com[^\s"\\<>]*|(?:127\.0\.0|10\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}|192\.168\.\d{1,3}|169\.254\.\d{1,3}|203\.0\.113)\.\d{1,3}|(?:2001:db8:[0-9a-f]{1,4}:[0-9a-f]{1,4}::[0-9a-f]{1,4}|fd00:[0-9a-f]{1,4}:[0-9a-f]{1,4}::[0-9a-f]{1,4}|fe80::[0-9a-f]{1,4}:[0-9a-f]{1,4}:[0-9a-f]{1,4})`)

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
	engine, err := veil.New(veil.Config{KeyPath: keyPath})
	if err != nil {
		t.Fatalf("veil.New: %v", err)
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

func maskOne(t *testing.T, gw *Gateway, scope veil.Scope, text string) ([]byte, *Exchange, string) {
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
	scope := veil.Scope{Tenant: "local", Session: "buffered", Project: "demo"}

	masked, ex, tok := maskOne(t, gw, scope, "run with key "+throwawayKey)
	if !bytes.Contains(masked, []byte("PAIArtVeil_SECRET_")) {
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
	if bytes.Contains(restored, []byte("PAIArtVeil_")) {
		t.Fatalf("buffered restore left residual token: %s", restored)
	}
	if !bytes.Contains(restored, []byte(throwawayKey)) {
		t.Fatalf("buffered restore did not return real value: %s", restored)
	}
}

func TestGatewayRawStreamRestoresArbitraryByteSplitsAndFlushesTail(t *testing.T) {
	gw := newTestGateway(t)
	_, ex, tok := maskOne(t, gw, veil.Scope{Session: "raw-stream"}, "key "+throwawayKey)

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
	if strings.Contains(got, "PAIArtVeil_") {
		t.Fatalf("raw stream restore left residual token: %q", got)
	}
}

func TestGatewayRawStreamRestoresComplexMixedPlaceholdersByteByByte(t *testing.T) {
	gw := newTestGateway(t)
	const (
		email        = "ops@example.com"
		sensitiveURL = "https://api.example.com/v1?token=abc123"
		dsn          = "postgresql://app:s3cr3t@db.example.com:5432/prod"
		ipv4         = "10.20.30.40"
		ipv6         = "2606:4700:4700::1111"
		ordinaryURL  = "https://supabase.com/docs"
	)
	sensitiveValues := []string{throwawayKey, email, sensitiveURL, dsn, ipv4, ipv6}
	text := strings.Join([]string{
		"key " + throwawayKey,
		"email " + email,
		"url " + sensitiveURL,
		"dsn " + dsn,
		"ipv4 " + ipv4,
		"ipv6 " + ipv6,
		"ordinary " + ordinaryURL,
	}, " ")

	masked, ex, err := gw.MaskOutbound(context.Background(), veil.Scope{Session: "raw-stream-mixed"}, anthropicRequest(text))
	if err != nil {
		t.Fatalf("MaskOutbound: %v", err)
	}
	for _, value := range sensitiveValues {
		if bytes.Contains(masked, []byte(value)) {
			t.Fatalf("masked provider body leaked %q: %s", value, masked)
		}
	}
	if !bytes.Contains(masked, []byte(ordinaryURL)) {
		t.Fatalf("ordinary URL should remain visible in masked body: %s", masked)
	}
	placeholders := uniqueStringMatches(mixedPlaceholderRE, string(masked))
	if len(placeholders) < len(sensitiveValues) {
		t.Fatalf("got %d placeholders, want at least %d in %s: %v", len(placeholders), len(sensitiveValues), masked, placeholders)
	}

	streamed := []byte("raw " + strings.Join(placeholders, "|") + " ordinary " + ordinaryURL)
	var out bytes.Buffer
	for _, b := range streamed {
		out.Write(gw.RestoreRawStreamChunk(ex, []byte{b}))
	}
	out.Write(gw.FlushRawStream(ex))
	got := out.String()

	for _, value := range append(sensitiveValues, ordinaryURL) {
		if !strings.Contains(got, value) {
			t.Fatalf("raw stream output missing %q: %q", value, got)
		}
	}
	for _, placeholder := range placeholders {
		if strings.Contains(got, placeholder) {
			t.Fatalf("known placeholder %q survived raw stream restore: %q", placeholder, got)
		}
	}
	if strings.Contains(got, "PAIArtVeil_") || strings.Contains(got, "veil.paiart.com") {
		t.Fatalf("raw stream restore left placeholder residue: %q", got)
	}
}

func TestGatewayParsedSSEEventRestoresProviderPayload(t *testing.T) {
	gw := newTestGateway(t)
	_, ex, tok := maskOne(t, gw, veil.Scope{Session: "sse-event"}, "key "+throwawayKey)

	event := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"value ` + tok + ` done"}}`)
	restored, err := gw.RestoreSSEEvent(context.Background(), ex, event)
	if err != nil {
		t.Fatalf("RestoreSSEEvent: %v", err)
	}
	if !json.Valid(restored) {
		t.Fatalf("restored SSE payload is not valid JSON: %s", restored)
	}
	if bytes.Contains(restored, []byte("PAIArtVeil_")) {
		t.Fatalf("SSE restore left residual token: %s", restored)
	}
	if !bytes.Contains(restored, []byte(throwawayKey)) {
		t.Fatalf("SSE restore did not return real value: %s", restored)
	}
}

func TestGatewayCrossScopeRestoreLeavesResidualToken(t *testing.T) {
	gw := newTestGateway(t)
	_, _, tokA := maskOne(t, gw, veil.Scope{Session: "scope-a"}, "key "+throwawayKey)

	_, exB, err := gw.MaskOutbound(context.Background(), veil.Scope{Session: "scope-b"}, anthropicRequest("plain request"))
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
	masked, ex, err := bad.MaskOutbound(context.Background(), veil.Scope{}, anthropicRequest("key "+throwawayKey))
	if err == nil {
		t.Fatal("MaskOutbound with unsupported provider returned nil error")
	}
	if masked != nil || ex != nil {
		t.Fatalf("unsupported provider should not return forwardable output: masked=%s ex=%v", masked, ex)
	}
	if errors.Is(err, veil.ErrInvalidState) {
		t.Fatalf("unexpected invalid state error for unsupported provider: %v", err)
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
