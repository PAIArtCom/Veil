// White-box tests (package proxy): the suite constructs Proxy values directly so
// it can substitute a custom engineAPI whose SSE stream restorer deterministically
// fails on a chosen event (exit criterion #5) and one whose restorer fails to
// build (streaming fail-closed) — the real gjson/sjson restore is too lenient to
// error on demand. All upstream traffic targets an httptest loopback server (no
// real network); the engine uses a fixed key so masked tokens are deterministic.
package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/tidwall/gjson"

	veil "github.com/PAIArtCom/Veil"
)

// tokenRe matches a PAIArtVeil_ token, mirroring the engine's token grammar
// used elsewhere in the repo's tests.
var tokenRe = regexp.MustCompile(`PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}`)
var mixedPlaceholderRe = regexp.MustCompile(`PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}|user-[0-9a-f]{12}@veil\.paiart\.com|(?:https?|postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis)://[^\s"\\<>]*veil\.paiart\.com[^\s"\\<>]*|(?:127\.0\.0|10\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}|192\.168\.\d{1,3}|169\.254\.\d{1,3}|203\.0\.113)\.\d{1,3}|(?:2001:db8:[0-9a-f]{1,4}:[0-9a-f]{1,4}::[0-9a-f]{1,4}|fd00:[0-9a-f]{1,4}:[0-9a-f]{1,4}::[0-9a-f]{1,4}|fe80::[0-9a-f]{1,4}:[0-9a-f]{1,4}:[0-9a-f]{1,4})`)

// recorder is a mutex-guarded holder for byte slices an httptest handler records
// on its own goroutine and the test goroutine reads after the response
// completes. It avoids relying on incidental happens-before from response
// flushing, keeping the suite clean under -race regardless of scheduling.
type recorder struct {
	mu     sync.Mutex
	last   []byte
	all    [][]byte
	header http.Header
}

func (r *recorder) record(body []byte, h http.Header) {
	r.mu.Lock()
	r.last = body
	r.all = append(r.all, body)
	if h != nil {
		r.header = h.Clone()
	}
	r.mu.Unlock()
}

func (r *recorder) lastBody() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

func (r *recorder) bodies() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.all
}

func (r *recorder) headers() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.header
}

// secrets reliably flagged by the L1 detector (proven by the detector and wire
// provider test suites): an AWS access key id (TypeSecret), an email
// (TypeEmail), and a private IPv4 (TypeIPv4).
const (
	awsKey  = "AKIAIOSFODNN7EXAMPLE"
	email   = "user@example.com"
	privIP  = "192.168.1.100"
	apiKey  = "sk-ant-test-credential-value"
	authHdr = "Bearer oauth-test-token"
)

// newTestEngine builds an Engine with a fixed deterministic key.
func newTestEngine(t *testing.T) *veil.Engine {
	t.Helper()
	return newTestEngineCfg(t, veil.Config{})
}

// newTestEngineCfg builds an Engine with a fixed deterministic key, merging any
// caller-supplied Config fields (Detector/Policy/Audit). KeyPath is always set
// to a per-test temp file so tokens are deterministic and isolated.
func newTestEngineCfg(t *testing.T, cfg veil.Config) *veil.Engine {
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
	cfg.KeyPath = keyPath
	e, err := veil.New(cfg)
	if err != nil {
		t.Fatalf("veil.New: %v", err)
	}
	return e
}

// newTestProxy wires a Proxy at engine pointed at upstream with a buffered logger
// so tests can assert on log output. It returns the proxy and the log buffer.
func newTestProxy(t *testing.T, engine *veil.Engine, upstream string) (*Proxy, *bytes.Buffer) {
	t.Helper()
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	px, err := New(engine, upstream, logger)
	if err != nil {
		t.Fatalf("proxy.New: %v", err)
	}
	return px, &logBuf
}

// ---- Test 1: buffered round-trip (exit #1, #3, #6) ---------------------------

// A request whose message content and a tool_result both carry a detectable
// secret is masked on the way out (the fake upstream sees only PAIArtVeil_ tokens, no
// plaintext) and restored on the way back (the client sees plaintext, no PAIArtVeil_).
// The fake upstream closes the loop: it extracts a token from the masked request
// it received and echoes that exact token in its response, proving the proxy
// threads the live State.
func TestBufferedRoundTrip(t *testing.T) {
	engine := newTestEngine(t)

	var received atomic.Int64
	var rec recorder

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		body, _ := io.ReadAll(r.Body)
		rec.record(body, nil)
		tok := tokenRe.FindString(string(body))
		if tok == "" {
			t.Errorf("upstream: masked request had no PAIArtVeil_ token: %s", body)
		}
		// Echo the token in a text block and a tool_use.input field.
		resp := `{"id":"msg_1","type":"message","role":"assistant","content":[` +
			`{"type":"text","text":"Using ` + tok + ` now."},` +
			`{"type":"tool_use","id":"tu_9","name":"run","input":{"dsn":"` + tok + `"}}` +
			`],"stop_reason":"end_turn","usage":{"input_tokens":5,"output_tokens":3}}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resp)
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":1024,"messages":[` +
		`{"role":"user","content":"connect with key ` + awsKey + `"},` +
		`{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"prior key ` + awsKey + `"}]}` +
		`]}`

	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientBody, _ := io.ReadAll(resp.Body)

	if received.Load() != 1 {
		t.Fatalf("upstream received %d requests, want 1", received.Load())
	}
	receivedBody := rec.lastBody()
	// Outbound: upstream saw a token, never the plaintext (exit #1).
	if !tokenRe.Match(receivedBody) {
		t.Fatalf("upstream body missing PAIArtVeil_ token: %s", receivedBody)
	}
	if bytes.Contains(receivedBody, []byte(awsKey)) {
		t.Fatalf("plaintext secret leaked to upstream: %s", receivedBody)
	}
	// Inbound: client got plaintext back, no residual PAIArtVeil_ (exit #3, #6).
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("client status = %d, want 200; body=%s", resp.StatusCode, clientBody)
	}
	if bytes.Contains(clientBody, []byte("PAIArtVeil_")) {
		t.Fatalf("residual PAIArtVeil_ token in client response: %s", clientBody)
	}
	if !bytes.Contains(clientBody, []byte(awsKey)) {
		t.Fatalf("original secret not restored in client response: %s", clientBody)
	}
	// Content-Length must reflect the restored body the client actually received.
	if got := resp.ContentLength; got != int64(len(clientBody)) {
		t.Fatalf("Content-Length = %d, want %d", got, len(clientBody))
	}
}

func TestOpenAIResponsesBufferedRoundTrip(t *testing.T) {
	engine := newTestEngine(t)

	var received atomic.Int64
	var rec recorder

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		if r.URL.Path != "/v1/responses" {
			t.Errorf("upstream path = %q, want /v1/responses", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		rec.record(body, r.Header)
		tok := tokenRe.FindString(string(body))
		if tok == "" {
			t.Errorf("upstream: masked request had no PAIArtVeil_ token: %s", body)
		}
		resp := `{"id":"resp_1","output":[` +
			`{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Using ` + tok + `"}]},` +
			`{"type":"function_call","call_id":"call_1","name":"run","arguments":"{\"dsn\":\"` + tok + `\"}"}` +
			`]}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resp)
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"gpt-5.4","instructions":"mask ` + awsKey + `","input":[` +
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"email ` + email + `"}]},` +
		`{"type":"function_call_output","call_id":"call_1","output":"local tool ` + awsKey + `"}` +
		`],"tools":[{"type":"function","name":"static","description":"keep ` + awsKey + `"}],"stream":false}`

	req, err := http.NewRequest(http.MethodPost, front.URL+"/v1/responses", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHdr)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientBody, _ := io.ReadAll(resp.Body)

	if received.Load() != 1 {
		t.Fatalf("upstream received %d requests, want 1", received.Load())
	}
	receivedBody := rec.lastBody()
	if !tokenRe.Match(receivedBody) {
		t.Fatalf("upstream body missing PAIArtVeil_ token: %s", receivedBody)
	}
	if bytes.Contains(receivedBody, []byte(email)) {
		t.Fatalf("plaintext email leaked to upstream: %s", receivedBody)
	}
	if !bytes.Contains(receivedBody, []byte("keep "+awsKey)) {
		t.Fatalf("static tools definition should remain unchanged: %s", receivedBody)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("client status = %d, want 200; body=%s", resp.StatusCode, clientBody)
	}
	if bytes.Contains(clientBody, []byte("PAIArtVeil_")) {
		t.Fatalf("residual PAIArtVeil_ token in client response: %s", clientBody)
	}
	if !bytes.Contains(clientBody, []byte(awsKey)) {
		t.Fatalf("original secret not restored in client response: %s", clientBody)
	}
	if rec.headers().Get("Authorization") != authHdr {
		t.Fatalf("Authorization header = %q, want pass-through", rec.headers().Get("Authorization"))
	}
}

func TestOpenAIResponsesStreamingRoundTrip(t *testing.T) {
	engine := newTestEngine(t)

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		if r.URL.Path != "/v1/responses" {
			t.Errorf("upstream path = %q, want /v1/responses", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		tok := tokenRe.FindString(string(body))
		if tok == "" {
			t.Errorf("upstream: masked request had no PAIArtVeil_ token: %s", body)
		}
		cut := len(tok) / 2
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		events := []string{
			`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"key ` + tok[:cut] + `"}`,
			`{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"` + tok[cut:] + ` done"}`,
			`{"type":"response.output_text.done","item_id":"msg_1","output_index":0,"content_index":0,"text":"key ` + tok + ` done"}`,
			`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":1,"delta":"{\"key\":\"` + tok[:cut] + `"}`,
			`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":1,"delta":"` + tok[cut:] + `\"}"}`,
			`{"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":1,"arguments":"{\"key\":\"` + tok + `\"}"}`,
		}
		for _, event := range events {
			_, _ = io.WriteString(w, "data: "+event+"\n\n")
		}
		fl.Flush()
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"gpt-5.4","stream":true,"input":"use ` + awsKey + `"}`
	resp, err := http.Post(front.URL+"/v1/responses", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientStream, _ := io.ReadAll(resp.Body)

	if received.Load() != 1 {
		t.Fatalf("upstream received %d requests, want 1", received.Load())
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, clientStream)
	}
	if bytes.Contains(clientStream, []byte("PAIArtVeil_")) {
		t.Fatalf("residual PAIArtVeil_ token in client stream: %s", clientStream)
	}
	if !bytes.Contains(clientStream, []byte(awsKey)) {
		t.Fatalf("original secret not restored in client stream: %s", clientStream)
	}
	if !bytes.Contains(clientStream, []byte("response.function_call_arguments.delta")) {
		t.Fatalf("restored function-call argument delta missing: %s", clientStream)
	}
}

func TestOpenAIResponsesUnsupportedShapeFailsClosed(t *testing.T) {
	engine := newTestEngine(t)
	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		t.Fatal("upstream must not receive unsupported plaintext-bearing Responses request")
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	resp, err := http.Post(front.URL+"/v1/responses", "application/json", strings.NewReader(`{
		"input":[{"type":"future_tool_result","payload":"`+awsKey+`"}]
	}`))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if received.Load() != 0 {
		t.Fatalf("upstream received %d requests, want 0", received.Load())
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", resp.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(`"error"`)) || bytes.Contains(body, []byte(awsKey)) {
		t.Fatalf("OpenAI error envelope missing or leaked sensitive value: %s", body)
	}
}

// splitTokenStream builds an Anthropic-shaped SSE byte stream that echoes tok
// SPLIT ACROSS multiple content_block_delta events — the failure mode that
// shipped in M4 because the old fake upstream emitted whole tokens. A text block
// (index 0) splits "key <tok>" across three text_delta events at interior token
// positions; a tool_use block (index 1) splits the serialized input JSON
// {"dsn":"<tok>"} across two input_json_delta fragments at a point inside the
// token. Both blocks get start/stop; the stream is bracketed by message_start
// and message_stop. The exact split offsets are deterministic functions of the
// token shape.
func splitTokenStream(tok string) string {
	// Text fragments: cut "<tok>" at two interior positions (mid-TYPE, mid-hex).
	u := strings.Index(tok, "_") // after Veil
	typeEnd := strings.Index(tok[u+1:], "_") + u + 1
	tf := []string{tok[:u+1], tok[u+1 : typeEnd+3], tok[typeEnd+3:]}

	// Tool fragments: cut the serialized {"dsn":"<tok>"} inside the token.
	toolComplete := `{"dsn":"` + tok + `"}`
	toolCut := strings.Index(toolComplete, tok) + 5
	jf := []string{jsonEsc(toolComplete[:toolCut]), jsonEsc(toolComplete[toolCut:])}

	var sse strings.Builder
	sse.WriteString("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\"}}\n\n")
	sse.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"key " + tf[0] + "\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + tf[1] + "\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + tf[2] + " end\"}}\n\n")
	sse.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
	sse.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tu_9\",\"name\":\"run\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"" + jf[0] + "\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"" + jf[1] + "\"}}\n\n")
	sse.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n")
	sse.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	return sse.String()
}

func splitMixedPlaceholderStream(placeholders []string, ordinaryURL string) string {
	textComplete := strings.Join(placeholders, "|") + "|ordinary=" + ordinaryURL
	textCut1 := strings.Index(textComplete, placeholders[1]) + len(placeholders[1])/2
	textCut2 := strings.Index(textComplete, placeholders[len(placeholders)-1]) + len(placeholders[len(placeholders)-1])/2
	tf := splitString(textComplete, textCut1, textCut2)

	toolBytes, _ := json.Marshal(map[string]string{
		"combo":    strings.Join(placeholders, ","),
		"ordinary": ordinaryURL,
	})
	toolComplete := string(toolBytes)
	toolCut1 := strings.Index(toolComplete, placeholders[0]) + len(placeholders[0])/2
	toolCut2 := strings.Index(toolComplete, placeholders[len(placeholders)-1]) + len(placeholders[len(placeholders)-1])/2
	jf := splitString(toolComplete, toolCut1, toolCut2)

	var sse strings.Builder
	sse.WriteString("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_complex\"}}\n\n")
	sse.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"mixed " + jsonEsc(tf[0]) + "\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + jsonEsc(tf[1]) + "\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + jsonEsc(tf[2]) + " done\"}}\n\n")
	sse.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
	sse.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tu_complex\",\"name\":\"run\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"" + jsonEsc(jf[0]) + "\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"" + jsonEsc(jf[1]) + "\"}}\n\n")
	sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"" + jsonEsc(jf[2]) + "\"}}\n\n")
	sse.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n")
	sse.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	return sse.String()
}

// jsonEsc escapes s for embedding inside a JSON string literal that is itself a
// data: line value (so the partial_json string carries escaped JSON).
func jsonEsc(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1]) // drop the surrounding quotes
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

// toolInputFromStream parses the client SSE stream, reconstructs index 1's
// tool_use input by concatenating its input_json_delta partial_json fragments,
// and returns the decoded {"dsn": ...} value. It asserts the reconstruction is
// valid JSON.
func toolInputDSN(t *testing.T, clientStream []byte) string {
	t.Helper()
	input := toolInputMap(t, clientStream)
	return input["dsn"]
}

func toolInputMap(t *testing.T, clientStream []byte) map[string]string {
	t.Helper()
	var pj strings.Builder
	sc := bufio.NewScanner(bytes.NewReader(clientStream))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSuffix(sc.Text(), "\r")
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		val := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		if val == "" || val == "[DONE]" {
			continue
		}
		if gjson.Get(val, "delta.type").Str == "input_json_delta" && gjson.Get(val, "index").Int() == 1 {
			pj.WriteString(gjson.Get(val, "delta.partial_json").Str)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan client stream: %v", err)
	}
	var input map[string]string
	if err := json.Unmarshal([]byte(pj.String()), &input); err != nil {
		t.Fatalf("reassembled tool_use.input is not valid JSON: %q: %v", pj.String(), err)
	}
	return input
}

// reassembleTextDeltas parses the client SSE stream and concatenates every
// text_delta's delta.text — the visible text the agent renders.
func reassembleTextDeltas(t *testing.T, clientStream []byte) string {
	t.Helper()
	var b strings.Builder
	sc := bufio.NewScanner(bytes.NewReader(clientStream))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSuffix(sc.Text(), "\r")
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		val := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		if val == "" || val == "[DONE]" {
			continue
		}
		if !json.Valid([]byte(val)) {
			t.Fatalf("client data payload is not valid JSON: %q", val)
		}
		if gjson.Get(val, "delta.type").Str == "text_delta" {
			b.WriteString(gjson.Get(val, "delta.text").Str)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan client stream: %v", err)
	}
	return b.String()
}

// ---- Test 2: streaming round-trip, tokens SPLIT ACROSS events (exit #3,#4,#7)-

// The fake upstream emits a token split across multiple content_block_delta
// events (text and tool_use). The reassembled client stream must restore the
// text token, deliver the tool input as ONE consolidated JSON value that decodes
// to the real secret (exit #4), and carry no residual PAIArtVeil_.
func TestStreamingRoundTripSplitAcrossEvents(t *testing.T) {
	engine := newTestEngine(t)

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		body, _ := io.ReadAll(r.Body)
		tok := tokenRe.FindString(string(body))
		if tok == "" {
			t.Errorf("upstream: masked request had no PAIArtVeil_ token: %s", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		_, _ = io.WriteString(w, splitTokenStream(tok))
		fl.Flush()
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":1024,"stream":true,"messages":[{"role":"user","content":"use ` + awsKey + `"}]}`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("client Content-Type = %q, want text/event-stream", ct)
	}
	clientStream, _ := io.ReadAll(resp.Body)

	if received.Load() != 1 {
		t.Fatalf("upstream received %d requests, want 1", received.Load())
	}
	if bytes.Contains(clientStream, []byte("PAIArtVeil_")) {
		t.Fatalf("residual PAIArtVeil_ token in client stream: %s", clientStream)
	}
	// Text token restored despite the 3-way split.
	if got := reassembleTextDeltas(t, clientStream); got != "key "+awsKey+" end" {
		t.Fatalf("reassembled text = %q, want %q", got, "key "+awsKey+" end")
	}
	// Exit #4: the tool input decodes to the REAL secret, not a PAIArtVeil_ literal.
	if dsn := toolInputDSN(t, clientStream); dsn != awsKey {
		t.Fatalf("reassembled tool_use.input dsn = %q, want %q", dsn, awsKey)
	}
	// Exactly one consolidated input_json_delta survived for the tool block.
	if n := strings.Count(string(clientStream), `"input_json_delta"`); n != 1 {
		t.Fatalf("want exactly 1 input_json_delta in client stream, got %d: %s", n, clientStream)
	}
	// Block skeleton preserved.
	if !bytes.Contains(clientStream, []byte("message_stop")) {
		t.Fatalf("final message_stop missing: %s", clientStream)
	}
	if n := strings.Count(string(clientStream), "event: content_block_start"); n != 2 {
		t.Fatalf("want 2 content_block_start events, got %d", n)
	}
	if n := strings.Count(string(clientStream), "event: content_block_stop"); n != 2 {
		t.Fatalf("want 2 content_block_stop events, got %d", n)
	}
}

func TestStreamingRoundTripComplexMixedPlaceholders(t *testing.T) {
	engine := newTestEngine(t)
	const sensitiveURL = "https://api.example.com/v1?token=abc123"
	const dsn = "postgresql://app:s3cr3t@db.example.com:5432/prod"
	const ipv4 = "10.20.30.40"
	const ipv6 = "2606:4700:4700::1111"
	const ordinaryURL = "https://supabase.com/docs"
	sensitiveValues := []string{awsKey, email, sensitiveURL, dsn, ipv4, ipv6}

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		body, _ := io.ReadAll(r.Body)
		for _, value := range sensitiveValues {
			if bytes.Contains(body, []byte(value)) {
				t.Errorf("upstream masked request leaked %q: %s", value, body)
			}
		}
		if !bytes.Contains(body, []byte(ordinaryURL)) {
			t.Errorf("ordinary URL should remain visible upstream: %s", body)
		}
		placeholders := uniqueStringMatches(mixedPlaceholderRe, string(body))
		if len(placeholders) < len(sensitiveValues) {
			t.Errorf("upstream got %d placeholders, want at least %d in %s: %v", len(placeholders), len(sensitiveValues), body, placeholders)
			http.Error(w, "missing placeholders", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		_, _ = io.WriteString(w, splitMixedPlaceholderStream(placeholders, ordinaryURL))
		fl.Flush()
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqText := strings.Join([]string{
		"key " + awsKey,
		"email " + email,
		"url " + sensitiveURL,
		"dsn " + dsn,
		"ipv4 " + ipv4,
		"ipv6 " + ipv6,
		"ordinary " + ordinaryURL,
	}, " ")
	reqBody, err := json.Marshal(map[string]any{
		"model":      "claude-opus-4-5",
		"max_tokens": 1024,
		"stream":     true,
		"messages": []any{
			map[string]string{"role": "user", "content": reqText},
		},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientStream, _ := io.ReadAll(resp.Body)

	if received.Load() != 1 {
		t.Fatalf("upstream received %d requests, want 1", received.Load())
	}
	if bytes.Contains(clientStream, []byte("PAIArtVeil_")) || bytes.Contains(clientStream, []byte("veil.paiart.com")) {
		t.Fatalf("known placeholder residue in client stream: %s", clientStream)
	}
	gotText := reassembleTextDeltas(t, clientStream)
	for _, value := range append(sensitiveValues, ordinaryURL) {
		if !strings.Contains(gotText, value) {
			t.Fatalf("reassembled text missing %q: %q", value, gotText)
		}
	}
	toolInput := toolInputMap(t, clientStream)
	combo := toolInput["combo"]
	for _, value := range sensitiveValues {
		if !strings.Contains(combo, value) {
			t.Fatalf("tool input combo missing %q: %#v", value, toolInput)
		}
	}
	if toolInput["ordinary"] != ordinaryURL {
		t.Fatalf("ordinary URL in tool input = %q, want %q", toolInput["ordinary"], ordinaryURL)
	}
	if n := strings.Count(string(clientStream), `"input_json_delta"`); n != 1 {
		t.Fatalf("want exactly 1 consolidated input_json_delta in client stream, got %d: %s", n, clientStream)
	}
}

// ---- Test 2b: F (byte × event) — split-token stream, one byte at a time ------

// The same split-across-events stream is written ONE BYTE AT A TIME with a flush
// after each byte, exercising the frame buffer (cross-TCP-read reassembly) AND
// the cross-event holdback together. This is the real exit-#7 test.
func TestStreamingSplitAcrossEventsByteAtATime(t *testing.T) {
	engine := newTestEngine(t)

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		body, _ := io.ReadAll(r.Body)
		tok := tokenRe.FindString(string(body))
		if tok == "" {
			t.Errorf("upstream: masked request had no PAIArtVeil_ token: %s", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		for _, b := range []byte(splitTokenStream(tok)) {
			_, _ = w.Write([]byte{b})
			fl.Flush()
		}
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":1024,"stream":true,"messages":[{"role":"user","content":"use ` + awsKey + `"}]}`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientStream, _ := io.ReadAll(resp.Body)

	if received.Load() != 1 {
		t.Fatalf("upstream received %d requests, want 1", received.Load())
	}
	if bytes.Contains(clientStream, []byte("PAIArtVeil_")) {
		t.Fatalf("byte-at-a-time: residual PAIArtVeil_ token in client stream: %s", clientStream)
	}
	if got := reassembleTextDeltas(t, clientStream); got != "key "+awsKey+" end" {
		t.Fatalf("byte-at-a-time: reassembled text = %q, want %q", got, "key "+awsKey+" end")
	}
	if dsn := toolInputDSN(t, clientStream); dsn != awsKey {
		t.Fatalf("byte-at-a-time: tool_use.input dsn = %q, want %q", dsn, awsKey)
	}
}

// ---- Test 3: blocked -> fail-closed (exit, fail-closed) ----------------------

// blockingPolicy blocks a fixed set of types via OperatorBlock.
type blockingPolicy struct {
	block []veil.Type
}

func (p blockingPolicy) Policy(_ context.Context, _ veil.Scope) (veil.Policy, error) {
	types := make(map[veil.Type]veil.TypePolicy, len(p.block))
	for _, t := range p.block {
		types[t] = veil.TypePolicy{Operator: veil.OperatorBlock}
	}
	return veil.Policy{
		DefaultOperator: veil.OperatorToken,
		Types:           types,
	}, nil
}

func TestBlockedFailClosed(t *testing.T) {
	engine := newTestEngineCfg(t, veil.Config{
		Policy: blockingPolicy{block: []veil.Type{veil.TypeSecret}},
	})

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	px, logBuf := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":1024,"messages":[{"role":"user","content":"key ` + awsKey + `"}]}`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", resp.StatusCode, clientBody)
	}
	// Fail-closed proof: upstream saw zero requests.
	if received.Load() != 0 {
		t.Fatalf("blocked request was forwarded upstream %d times, want 0", received.Load())
	}
	// Anthropic-shaped blocked error body naming the blocked type.
	for _, want := range []string{`"type":"error"`, `"type":"blocked_by_policy"`, "SECRET"} {
		if !strings.Contains(string(clientBody), want) {
			t.Fatalf("blocked error body missing %q: %s", want, clientBody)
		}
	}
	if !strings.Contains(logBuf.String(), "blocked by policy") {
		t.Fatalf("expected blocked-by-policy warn log, got: %s", logBuf.String())
	}
}

// ---- Test 4: mask error -> fail-closed ---------------------------------------

// erroringDetector returns an error from Detect, which the engine propagates
// out of MaskRequest (fail-closed).
type erroringDetector struct{}

func (erroringDetector) Detect(_ context.Context, _ string) ([]veil.Finding, error) {
	return nil, errors.New("synthetic detector failure")
}

func TestMaskErrorFailClosed(t *testing.T) {
	engine := newTestEngineCfg(t, veil.Config{Detector: erroringDetector{}})

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	px, logBuf := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":1024,"messages":[{"role":"user","content":"hello ` + email + `"}]}`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", resp.StatusCode, clientBody)
	}
	if received.Load() != 0 {
		t.Fatalf("mask-error request was forwarded upstream %d times, want 0", received.Load())
	}
	if !strings.Contains(string(clientBody), `"type":"error"`) {
		t.Fatalf("expected Anthropic-shaped error body, got: %s", clientBody)
	}
	if !strings.Contains(logBuf.String(), "mask request failed") {
		t.Fatalf("expected mask-error log, got: %s", logBuf.String())
	}
}

func TestMalformedJSONFailClosed(t *testing.T) {
	engine := newTestEngine(t)

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	px, logBuf := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":1024,"messages":[{"role":"user","content":"key ` + awsKey + `"}]`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", resp.StatusCode, clientBody)
	}
	if received.Load() != 0 {
		t.Fatalf("malformed JSON request was forwarded upstream %d times, want 0", received.Load())
	}
	if bytes.Contains(clientBody, []byte(awsKey)) {
		t.Fatalf("error body leaked request secret: %s", clientBody)
	}
	if !strings.Contains(logBuf.String(), "mask request failed") {
		t.Fatalf("expected mask-error log, got: %s", logBuf.String())
	}
}

// ---- Test 5: auth pass-through -----------------------------------------------

// The client's credential headers must reach upstream byte-for-byte; the proxy
// holds no credentials of its own (ADR-0004).
func TestAuthPassThrough(t *testing.T) {
	engine := newTestEngine(t)

	var rec recorder
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(nil, r.Header)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"ok"}]}`)
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodPost, front.URL+"/v1/messages",
		strings.NewReader(`{"model":"claude-opus-4-5","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Authorization", authHdr)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "tools-2024-04-04")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	resp.Body.Close()

	h := rec.headers()
	if got := h.Get("x-api-key"); got != apiKey {
		t.Fatalf("upstream x-api-key = %q, want %q", got, apiKey)
	}
	if got := h.Get("Authorization"); got != authHdr {
		t.Fatalf("upstream Authorization = %q, want %q", got, authHdr)
	}
	if got := h.Get("anthropic-version"); got != "2023-06-01" {
		t.Fatalf("upstream anthropic-version = %q, want %q", got, "2023-06-01")
	}
	if got := h.Get("anthropic-beta"); got != "tools-2024-04-04" {
		t.Fatalf("upstream anthropic-beta = %q, want %q", got, "tools-2024-04-04")
	}
}

// ---- Test 6: prompt-cache prefix stability (exit #8) -------------------------

// Two identical requests must produce byte-identical masked bodies upstream, so
// Anthropic's prompt-cache prefix matching is preserved across turns.
func TestPromptCachePrefixStability(t *testing.T) {
	engine := newTestEngine(t)

	var rec recorder
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.record(body, nil)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"ok"}]}`)
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":1024,"system":"key ` + awsKey + `","messages":[{"role":"user","content":"email ` + email + ` ip ` + privIP + `"}]}`
	for i := 0; i < 2; i++ {
		resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("client POST %d: %v", i, err)
		}
		resp.Body.Close()
	}

	bodies := rec.bodies()
	if len(bodies) != 2 {
		t.Fatalf("upstream received %d requests, want 2", len(bodies))
	}
	if !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatalf("masked bodies differ across identical requests:\n#1: %s\n#2: %s", bodies[0], bodies[1])
	}
	// Sanity: the masking actually happened (no plaintext, has a token).
	if bytes.Contains(bodies[0], []byte(awsKey)) || !tokenRe.Match(bodies[0]) {
		t.Fatalf("first masked body not masked as expected: %s", bodies[0])
	}
}

// ---- Test 7: unsupported endpoints fail closed -------------------------------

// Unsupported methods and paths are not transparent in the release proxy. They
// can carry plaintext request bodies that Veil has not verified how to mask,
// so they must fail before upstream egress.
func TestUnsupportedEndpointFailsClosed(t *testing.T) {
	engine := newTestEngine(t)

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		t.Fatal("upstream must not receive unsupported proxy endpoint")
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "get-models", method: http.MethodGet, path: "/v1/models?limit=5"},
		{name: "unsupported-post", method: http.MethodPost, path: "/v1/complete", body: `{"raw":"` + awsKey + `"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, front.URL+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("client request: %v", err)
			}
			defer resp.Body.Close()
			clientBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body=%s", resp.StatusCode, clientBody)
			}
			if !bytes.Contains(clientBody, []byte(`unsupported_endpoint`)) || bytes.Contains(clientBody, []byte(awsKey)) {
				t.Fatalf("unsupported endpoint error missing or leaked sensitive value: %s", clientBody)
			}
		})
	}

	if received.Load() != 0 {
		t.Fatalf("upstream received %d requests, want 0", received.Load())
	}
}

// ---- Test 8: restore error visibility (exit #5) ------------------------------

// errOnTokenStream is an sseRestorer that fails Event for any payload containing
// a PAIArtVeil_ token, exercising the "restore errors are surfaced, never swallowed;
// the stream is not dropped" path (exit #5) through the 1→N event pipeline
// without relying on gjson/sjson to error (it does not). Non-token events and
// Flush delegate to a real *veil.SSEStream so the rest of the stream
// behaves normally.
type errOnTokenStream struct {
	inner   *veil.SSEStream
	failErr error
}

func (s errOnTokenStream) Event(ctx context.Context, eventData []byte) ([][]byte, error) {
	if bytes.Contains(eventData, []byte("PAIArtVeil_")) {
		return nil, s.failErr
	}
	return s.inner.Event(ctx, eventData)
}

func (s errOnTokenStream) Flush(ctx context.Context) ([][]byte, error) {
	return s.inner.Flush(ctx)
}

// errOnTokenEngine wraps a real engine but returns an errOnTokenStream from
// NewSSEStreamRestorer so the streaming-restore error path is injectable.
type errOnTokenEngine struct {
	*veil.Engine
	failErr error
}

func (e errOnTokenEngine) NewSSEStreamRestorer(st *veil.State) (sseRestorer, error) {
	inner, err := e.Engine.NewSSEStreamRestorer(st)
	if err != nil {
		return nil, err
	}
	return errOnTokenStream{inner: inner, failErr: e.failErr}, nil
}

func TestRestoreErrorVisibleStreamNotDropped(t *testing.T) {
	real := newTestEngine(t)
	failErr := errors.New("synthetic restore failure")

	var received atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		body, _ := io.ReadAll(r.Body)
		tok := tokenRe.FindString(string(body))
		if tok == "" {
			t.Errorf("upstream: masked request had no token: %s", body)
		}
		var sse strings.Builder
		// A text block whose delta carries a token (mid-block) triggers the
		// restore error...
		sse.WriteString("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n")
		sse.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + tok + " x\"}}\n\n")
		// ...and benign events that must still be relayed (stream not dropped),
		// including the message_stop after the failing block.
		sse.WriteString("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		sse.WriteString("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		_, _ = io.WriteString(w, sse.String())
		fl.Flush()
	}))
	defer upstream.Close()

	// Build the proxy by hand with the error-injecting engine wrapper.
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	px, err := New(real, upstream.URL, logger)
	if err != nil {
		t.Fatalf("proxy.New: %v", err)
	}
	px.engine = errOnTokenEngine{Engine: real, failErr: failErr}

	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"use ` + awsKey + `"}]}`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientStream, _ := io.ReadAll(resp.Body)

	if received.Load() != 1 {
		t.Fatalf("upstream received %d requests, want 1", received.Load())
	}
	// The error is surfaced (logged), not swallowed (exit #5).
	if !strings.Contains(logBuf.String(), "restore SSE event failed") {
		t.Fatalf("restore error not logged: %s", logBuf.String())
	}
	// The original (un-restored) event is relayed unchanged — token still present.
	if !bytes.Contains(clientStream, []byte("PAIArtVeil_")) {
		t.Fatalf("expected original event (with token) to be relayed on restore error: %s", clientStream)
	}
	// The stream was NOT dropped: the benign trailing event still arrives.
	if !bytes.Contains(clientStream, []byte("message_stop")) {
		t.Fatalf("stream was dropped after restore error; message_stop missing: %s", clientStream)
	}
}

// ---- Buffered restore-error visibility (exit #5, buffered path) --------------

// errBufferedEngine wraps a real engine but fails RestoreResponse, so the
// buffered path's "log + relay raw upstream body" behavior is exercised. It
// embeds engineAdapter so the streaming seam (NewSSEStreamRestorer) is satisfied
// even though the buffered path never uses it.
type errBufferedEngine struct {
	engineAdapter
	failErr error
}

func (e errBufferedEngine) RestoreResponse(ctx context.Context, st *veil.State, body []byte) ([]byte, error) {
	return nil, e.failErr
}

func TestBufferedRestoreErrorVisibleRawRelayed(t *testing.T) {
	real := newTestEngine(t)
	failErr := errors.New("synthetic buffered restore failure")

	const rawResp = `{"type":"message","content":[{"type":"text","text":"raw upstream body"}]}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, rawResp)
	}))
	defer upstream.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	px, err := New(real, upstream.URL, logger)
	if err != nil {
		t.Fatalf("proxy.New: %v", err)
	}
	px.engine = errBufferedEngine{engineAdapter: engineAdapter{Engine: real}, failErr: failErr}

	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":16,"messages":[{"role":"user","content":"key ` + awsKey + `"}]}`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientBody, _ := io.ReadAll(resp.Body)

	if !strings.Contains(logBuf.String(), "restore response failed") {
		t.Fatalf("buffered restore error not logged: %s", logBuf.String())
	}
	// The raw upstream body is still delivered so the local user gets a response.
	if string(clientBody) != rawResp {
		t.Fatalf("raw upstream body not relayed on restore error: got %s", clientBody)
	}
}

// ---- Fail-closed: NewSSEStreamRestorer error aborts the stream (no leak) -----

// failRestorerEngine returns an error from NewSSEStreamRestorer, modeling an
// unsupported provider / invalid state on the streaming path. The proxy must
// fail closed with a 502 BEFORE committing the streamed response, never relaying
// the upstream SSE body (which would leak unrestored tokens).
type failRestorerEngine struct {
	*veil.Engine
	failErr error
}

func (e failRestorerEngine) NewSSEStreamRestorer(st *veil.State) (sseRestorer, error) {
	return nil, e.failErr
}

func TestStreamRestorerBuildErrorFailsClosed(t *testing.T) {
	real := newTestEngine(t)

	var sentBody atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		tok := tokenRe.FindString(string(body))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		// An SSE body carrying a token. If the proxy relayed it on a restorer
		// build error, the token would leak — so this body must never reach the
		// client.
		_, _ = io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\""+tok+"\"}}\n\n")
		fl.Flush()
		sentBody.Store(true)
	}))
	defer upstream.Close()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	px, err := New(real, upstream.URL, logger)
	if err != nil {
		t.Fatalf("proxy.New: %v", err)
	}
	px.engine = failRestorerEngine{Engine: real, failErr: errors.New("synthetic stream-restorer build failure")}

	front := httptest.NewServer(px)
	defer front.Close()

	reqBody := `{"model":"claude-opus-4-5","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"use ` + awsKey + `"}]}`
	resp, err := http.Post(front.URL+"/v1/messages", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", resp.StatusCode, clientBody)
	}
	// No upstream token (or any SSE body) leaked to the client.
	if bytes.Contains(clientBody, []byte("PAIArtVeil_")) || bytes.Contains(clientBody, []byte("content_block_delta")) {
		t.Fatalf("upstream SSE body leaked on restorer build failure: %s", clientBody)
	}
	// The error is surfaced (logged), not swallowed.
	if !strings.Contains(logBuf.String(), "build SSE stream restorer") {
		t.Fatalf("restorer build error not logged: %s", logBuf.String())
	}
}

// ---- New() validation --------------------------------------------------------

func TestNewValidatesUpstream(t *testing.T) {
	engine := newTestEngine(t)
	cases := []string{"", "::::", "ftp://host", "https://"}
	for _, c := range cases {
		if _, err := New(engine, c, nil); err == nil {
			t.Errorf("New(upstream=%q) = nil error, want error", c)
		}
	}
	if _, err := New(nil, "https://api.anthropic.com", nil); err == nil {
		t.Error("New(nil engine) = nil error, want error")
	}
	// A valid upstream with a nil logger must default the logger and succeed.
	if _, err := New(engine, "https://api.anthropic.com", nil); err != nil {
		t.Errorf("New with nil logger: %v", err)
	}
}

// ---- SSE framing reassembly unit test ----------------------------------------

// Drives the client response through bufio scanning to confirm restored events
// remain individually well-formed (defensive: complements the byte-at-a-time
// integration test).
func TestStreamedEventsAreWellFormed(t *testing.T) {
	engine := newTestEngine(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		tok := tokenRe.FindString(string(body))
		sse := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + tok + "\"}}\n\n" +
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		_, _ = io.WriteString(w, sse)
		fl.Flush()
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	resp, err := http.Post(front.URL+"/v1/messages", "application/json",
		strings.NewReader(`{"model":"claude-opus-4-5","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"`+awsKey+`"}]}`))
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()

	// Parse out each data: line and assert no PAIArtVeil_ remains and the value restored.
	sc := bufio.NewScanner(resp.Body)
	var sawRestored bool
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "data:") {
			if strings.Contains(line, "PAIArtVeil_") {
				t.Fatalf("PAIArtVeil_ token survived in data line: %q", line)
			}
			if strings.Contains(line, awsKey) {
				sawRestored = true
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan client stream: %v", err)
	}
	if !sawRestored {
		t.Fatalf("restored secret not seen in any data line")
	}
}

func TestStreamGzipResponseIsDecodedBeforeRestore(t *testing.T) {
	engine := newTestEngine(t)
	var upstreamAcceptEncoding atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAcceptEncoding.Store(r.Header.Get("Accept-Encoding"))
		body, _ := io.ReadAll(r.Body)
		tok := tokenRe.FindString(string(body))
		if tok == "" {
			t.Fatalf("upstream: masked request had no PAIArtVeil_ token: %s", body)
		}
		sse := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + tok + "\"}}\n\n" +
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		gz := gzip.NewWriter(w)
		_, _ = io.WriteString(gz, sse)
		_ = gz.Close()
	}))
	defer upstream.Close()

	px, _ := newTestProxy(t, engine, upstream.URL)
	front := httptest.NewServer(px)
	defer front.Close()

	req, err := http.NewRequest(http.MethodPost, front.URL+"/v1/messages", strings.NewReader(`{"model":"claude-opus-4-5","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"`+awsKey+`"}]}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "br, gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client POST: %v", err)
	}
	defer resp.Body.Close()
	clientBody, _ := io.ReadAll(resp.Body)

	if got := upstreamAcceptEncoding.Load().(string); strings.Contains(got, "br") {
		t.Fatalf("client Accept-Encoding leaked upstream: %q", got)
	}
	if got := resp.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("decoded/restored response must not keep Content-Encoding, got %q", got)
	}
	if bytes.Contains(clientBody, []byte("PAIArtVeil_")) {
		t.Fatalf("token survived gzip SSE restore: %s", clientBody)
	}
	if !bytes.Contains(clientBody, []byte(awsKey)) {
		t.Fatalf("restored secret not seen in gzip SSE response: %s", clientBody)
	}
}
