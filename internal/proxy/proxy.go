package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	veil "github.com/PAIArtCom/Veil"
)

// engineAPI is the slice of *veil.Engine the proxy depends on. Depending on
// an interface rather than the concrete type lets tests substitute a restorer
// that deterministically fails on a chosen SSE event so the "restore errors are
// surfaced, never swallowed" guarantee (exit criterion #5) can be exercised —
// the real gjson/sjson restore path is too lenient to error on demand. New
// accepts the concrete *veil.Engine, so production wiring is unaffected.
//
// NewSSEStreamRestorer returns an sseRestorer (an interface, not the concrete
// *veil.SSEStream) so the streaming-restore error path stays injectable.
type engineAPI interface {
	MaskRequest(ctx context.Context, scope veil.Scope, provider, op string, body []byte) ([]byte, *veil.State, error)
	RestoreResponse(ctx context.Context, st *veil.State, body []byte) ([]byte, error)
	NewSSEStreamRestorer(st *veil.State) (sseRestorer, error)
}

// sseRestorer is the per-stream stateful SSE restorer the proxy drives: it
// consumes one complete event payload at a time (Event) and drains held
// cross-event state at end of stream (Flush). *veil.SSEStream satisfies it;
// tests substitute a failing implementation to exercise exit criterion #5.
type sseRestorer interface {
	Event(ctx context.Context, eventData []byte) ([][]byte, error)
	Flush(ctx context.Context) ([][]byte, error)
}

// engineAdapter adapts a concrete *veil.Engine to engineAPI: the engine's
// NewSSEStreamRestorer returns the concrete *veil.SSEStream, but engineAPI
// needs it widened to the sseRestorer interface so the seam stays testable.
type engineAdapter struct {
	*veil.Engine
}

// NewSSEStreamRestorer widens the engine's concrete *veil.SSEStream return
// to the sseRestorer interface. A nil *veil.SSEStream is returned as a nil
// interface (not a non-nil interface wrapping a nil pointer) so an error result
// carries a usable nil restorer.
func (a engineAdapter) NewSSEStreamRestorer(st *veil.State) (sseRestorer, error) {
	s, err := a.Engine.NewSSEStreamRestorer(st)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Proxy is the reference base-URL proxy http.Handler.
type Proxy struct {
	engine   engineAPI
	upstream *url.URL     // e.g. https://api.anthropic.com
	client   *http.Client // streaming-friendly: no overall Timeout
	scope    veil.Scope   // Phase 0: a fixed default scope (the zero value)
	log      *slog.Logger
}

// New constructs a Proxy that masks requests through engine and relays them to
// upstream (an absolute http/https base URL such as https://api.anthropic.com).
// If log is nil a default text logger to stderr is used. The returned client has
// no overall timeout so it does not abort long-lived SSE streams; transport-level
// dial/TLS timeouts come from http.DefaultTransport.
func New(engine *veil.Engine, upstream string, log *slog.Logger) (*Proxy, error) {
	if engine == nil {
		return nil, errors.New("proxy: nil engine")
	}
	u, err := parseUpstream(upstream)
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	return &Proxy{
		// Wrap the concrete engine so its NewSSEStreamRestorer (which returns the
		// concrete *veil.SSEStream) satisfies engineAPI's interface return.
		engine:   engineAdapter{Engine: engine},
		upstream: u,
		// A dedicated client (not http.DefaultClient) with no Timeout: an SSE
		// response body may stay open for minutes. Per-attempt dial/TLS
		// timeouts are inherited from http.DefaultTransport.
		client: &http.Client{Transport: http.DefaultTransport},
		scope:  veil.Scope{},
		log:    log,
	}, nil
}

// parseUpstream validates that raw is an absolute http(s) base URL with a host.
func parseUpstream(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("proxy: empty upstream URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("proxy: parse upstream %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("proxy: upstream %q must be http or https", raw)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("proxy: upstream %q has no host", raw)
	}
	return u, nil
}

// hopByHopHeaders are connection-scoped headers that must not be forwarded
// across a proxy hop (RFC 7230 §6.1). Content-Length is handled separately
// because the masked/restored body length differs from the original.
var hopByHopHeaders = map[string]struct{}{
	// Veil rewrites response bodies. Do not forward the client's
	// compression preferences upstream: net/http can transparently manage gzip
	// only when it owns Accept-Encoding, and restore must see decoded bytes.
	"Accept-Encoding":     {},
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Proxy-Connection":    {},
	"Te":                  {}, // canonicalized "TE"
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

// isHopByHop reports whether a canonicalized header key is hop-by-hop and so
// must be stripped when relaying. "Proxy-*" headers are all hop-by-hop.
func isHopByHop(canonKey string) bool {
	if _, ok := hopByHopHeaders[canonKey]; ok {
		return true
	}
	return strings.HasPrefix(canonKey, "Proxy-")
}

// ServeHTTP routes the request. Provider-native release paths are masked and
// restored; everything else fails closed. v0.1.0 does not provide a transparent
// provider proxy because unsupported endpoints can carry plaintext request
// bodies that Veil has not verified how to mask.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && (r.URL.Path == "/healthz" || r.URL.Path == "/veil/healthz") {
		p.serveHealth(w)
		return
	}

	routeReq, upstream, err := p.normalizeRequest(r)
	if err != nil {
		p.log.Warn("proxy: invalid request routing option", "err", err)
		errorWriterForPath(r.URL.Path)(w, http.StatusBadRequest, "invalid_request", "invalid Veil proxy route")
		return
	}

	if routeReq.Method == http.MethodPost && routeReq.URL.Path == "/v1/messages" {
		p.serveProvider(w, routeReq, upstream, providerRoute{
			provider: "anthropic",
			op:       "messages",
			writeErr: writeAnthropicError,
		})
		return
	}
	if routeReq.Method == http.MethodPost && (routeReq.URL.Path == "/v1/responses" || routeReq.URL.Path == "/responses") {
		p.serveProvider(w, routeReq, upstream, providerRoute{
			provider: "openai-responses",
			op:       "responses",
			writeErr: writeOpenAIError,
		})
		return
	}
	errorWriterForPath(routeReq.URL.Path)(w, http.StatusNotFound, "unsupported_endpoint", "unsupported Veil proxy endpoint")
}

type providerRoute struct {
	provider string
	op       string
	writeErr func(http.ResponseWriter, int, string, string)
}

// normalizeRequest maps Veil's convenience ingress back to provider-native
// paths. The human-readable form:
//
//	/veil/upstream=https://openrouter.ai/api/v1/responses
//
// becomes provider path /v1/responses and upstream https://openrouter.ai/api.
// The older query form remains supported for compatibility. In both forms
// Veil-only routing data is removed before provider egress.
// Existing provider-native paths keep using the proxy's configured default
// upstream.
func (p *Proxy) normalizeRequest(r *http.Request) (*http.Request, *url.URL, error) {
	upstream := p.upstream
	path := r.URL.Path
	if path != "/veil" && !strings.HasPrefix(path, "/veil/") {
		return r, upstream, nil
	}

	if after, ok := strings.CutPrefix(path, "/veil/upstream="); ok {
		routePath, u, err := parsePathUpstream(after)
		if err != nil {
			return nil, nil, err
		}
		clone := r.Clone(r.Context())
		nextURL := *r.URL
		nextURL.Path = routePath
		nextURL.RawPath = ""
		clone.URL = &nextURL
		return clone, u, nil
	}

	values := r.URL.Query()
	if raw := values.Get("upstream"); raw != "" {
		u, err := parseUpstream(raw)
		if err != nil {
			return nil, nil, err
		}
		upstream = u
		values.Del("upstream")
	}

	clone := r.Clone(r.Context())
	u := *r.URL
	u.Path = strings.TrimPrefix(path, "/veil")
	if u.Path == "" {
		u.Path = "/"
	}
	u.RawPath = ""
	u.RawQuery = values.Encode()
	clone.URL = &u
	return clone, upstream, nil
}

var supportedProviderPathSuffixes = []string{
	"/v1/responses",
	"/v1/messages",
	"/responses",
}

func parsePathUpstream(after string) (routePath string, upstream *url.URL, err error) {
	for _, suffix := range supportedProviderPathSuffixes {
		if !strings.HasSuffix(after, suffix) {
			continue
		}
		raw := strings.TrimSuffix(after, suffix)
		u, err := parseUpstream(raw)
		if err != nil {
			return "", nil, err
		}
		return suffix, u, nil
	}

	u, err := parseUpstream(after)
	if err != nil {
		return "", nil, err
	}
	return "/", u, nil
}

func errorWriterForPath(path string) func(http.ResponseWriter, int, string, string) {
	if path == "/responses" || strings.HasSuffix(path, "/responses") {
		return writeOpenAIError
	}
	return writeAnthropicError
}

func (p *Proxy) serveHealth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"status":"ok","default_upstream":%s,"supported":["/v1/messages","/v1/responses","/responses","/veil/upstream=https://example.com/v1/messages","/veil/upstream=https://example.com/v1/responses"]}`+"\n", jsonString(p.upstream.String()))
}

// serveProvider handles one masked provider-native path.
func (p *Proxy) serveProvider(w http.ResponseWriter, r *http.Request, upstream *url.URL, route providerRoute) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// Could not read the client body; nothing was forwarded.
		p.log.Error("proxy: read request body", "err", err)
		route.writeErr(w, http.StatusBadGateway, "upstream_error", "failed to read request body")
		return
	}

	masked, state, err := p.engine.MaskRequest(r.Context(), p.scope, route.provider, route.op, body)
	if err != nil {
		// Fail-closed: on ANY masking error the plaintext request is never
		// forwarded upstream. A policy block maps to 403; any other error maps
		// to 502. Both return before a single upstream byte is sent.
		var blocked *veil.BlockedError
		if errors.As(err, &blocked) {
			p.log.Warn("proxy: request blocked by policy", "types", typeNames(blocked.Types))
			route.writeErr(w, http.StatusForbidden, "blocked_by_policy",
				"request blocked by local policy: "+strings.Join(typeNames(blocked.Types), ", "))
			return
		}
		p.log.Error("proxy: mask request failed (fail-closed, not forwarded)", "err", err)
		route.writeErr(w, http.StatusBadGateway, "upstream_error", "request could not be processed")
		return
	}

	// Build the upstream request: same method, upstream host + the incoming
	// path and raw query, masked body.
	upReq, err := p.newUpstreamRequest(r, upstream, masked)
	if err != nil {
		p.log.Error("proxy: build upstream request", "err", err)
		route.writeErr(w, http.StatusBadGateway, "upstream_error", "failed to build upstream request")
		return
	}

	resp, err := p.client.Do(upReq)
	if err != nil {
		// Transport error reaching upstream: fail-closed (nothing to relay).
		p.log.Error("proxy: upstream transport error", "err", err)
		route.writeErr(w, http.StatusBadGateway, "upstream_error", "upstream request failed")
		return
	}
	defer resp.Body.Close()

	if isEventStream(resp.Header.Get("Content-Type")) {
		p.relayStream(w, r, resp, state, route.writeErr)
		return
	}
	p.relayBuffered(w, r, resp, state)
}

// newUpstreamRequest builds the masked upstream request, copying non-hop-by-hop
// client headers verbatim — including the credential (Authorization / x-api-key)
// and Anthropic version/beta headers. The proxy holds no credentials of its own;
// it forwards whatever the client sent (ADR-0004).
func (p *Proxy) newUpstreamRequest(r *http.Request, upstream *url.URL, body []byte) (*http.Request, error) {
	if upstream == nil {
		upstream = p.upstream
	}
	target := *upstream
	target.Path = singleJoiningSlash(upstream.Path, r.URL.Path)
	target.RawQuery = r.URL.RawQuery

	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	copyHeaders(upReq.Header, r.Header)
	// Content-Length must reflect the masked body, not the original.
	upReq.ContentLength = int64(len(body))
	upReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	return upReq, nil
}

// relayBuffered relays a complete (non-streaming) upstream response: read the
// whole body, restore placeholders, and write the result. On a restore error the raw
// upstream body is written so the trusted local user still gets a response
// (residual tokens are audited by the engine); the error is logged, never
// swallowed (exit criterion #5).
func (p *Proxy) relayBuffered(w http.ResponseWriter, r *http.Request, resp *http.Response, state *veil.State) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.log.Error("proxy: read upstream response body", "err", err)
		copyHeaders(w.Header(), resp.Header)
		w.Header().Del("Content-Length")
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	out := body
	if restored, rerr := p.engine.RestoreResponse(r.Context(), state, body); rerr != nil {
		// Surface the restore error; still relay the raw upstream body so the
		// local user is not left without a response.
		p.log.Error("proxy: restore response failed (relaying raw upstream body)", "err", rerr)
	} else {
		out = restored
	}

	copyHeaders(w.Header(), resp.Header)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(out)))
	w.WriteHeader(resp.StatusCode)
	if _, err := w.Write(out); err != nil {
		p.log.Error("proxy: write buffered response to client", "err", err)
	}
}

// copyHeaders copies every non-hop-by-hop header from src to dst. Content-Length
// is treated as hop-by-hop here because the masked/restored body length differs
// from the original; callers set it explicitly for buffered bodies and drop it
// for streamed bodies.
func copyHeaders(dst, src http.Header) {
	for k, vs := range src {
		if isHopByHop(k) || k == "Content-Length" {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

// isEventStream reports whether a Content-Type names a Server-Sent Events stream.
func isEventStream(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

// typeNames renders blocked types as their string names for error messages.
func typeNames(types []veil.Type) []string {
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = string(t)
	}
	return out
}

// singleJoiningSlash joins two URL path segments with exactly one slash,
// mirroring net/http/httputil.NewSingleHostReverseProxy's joiner.
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		if a == "" {
			return b
		}
		return a + "/" + b
	}
	return a + b
}

// writeAnthropicError writes an Anthropic-shaped JSON error. The body is a fixed
// template so it never carries provider payload or sensitive values.
func writeAnthropicError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Del("Content-Length")
	w.WriteHeader(status)
	// Hand-built to keep the exact Anthropic error envelope and avoid pulling
	// json marshaling onto this small fixed shape; message is JSON-escaped.
	body := `{"type":"error","error":{"type":"` + errType + `","message":` + jsonString(message) + `}}`
	_, _ = io.WriteString(w, body)
}

// writeOpenAIError writes an OpenAI-shaped fixed JSON error. The body is
// sanitized and never includes provider payloads or sensitive values.
func writeOpenAIError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Del("Content-Length")
	w.WriteHeader(status)
	body := `{"error":{"message":` + jsonString(message) + `,"type":"` + errType + `"}}`
	_, _ = io.WriteString(w, body)
}

// jsonString returns s as a JSON string literal (with surrounding quotes),
// escaping the characters that must not appear raw inside a JSON string.
func jsonString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
