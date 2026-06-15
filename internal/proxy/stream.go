package proxy

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	opencloak "github.com/cloakia/opencloak"
)

// SSE wire constants. Events are separated by a blank line; within an event,
// fields are "name: value" lines. Anthropic emits one data: line per event, but
// the spec permits multiple, which a conformant relay must restore independently
// and re-concatenate.
//
// Phase 0 frames on the LF blank line "\n\n", which is what Anthropic's
// /v1/messages stream emits (confirmed by the wire fixtures). The SSE spec also
// permits CRLF ("\r\n\r\n") and bare-CR separators; supporting those is deferred
// to Phase 1 alongside non-Anthropic providers. A CRLF stream would still be
// restored correctly — restoreSSEFrame trims a trailing CR per line — but would
// be buffered to EOF and emitted as one frame rather than flushed per event.
const (
	sseEventSep = "\n\n"
	sseDataTag  = "data:"
	sseDoneData = "[DONE]"
)

// relayStream relays a Server-Sent Events response with frame-level buffering,
// threading the same State across the whole stream so multi-event token
// restoration shares one reverse mapping.
//
// Why frame-level (not byte-level) restore here: the structured RestoreSSEEvent
// is escaping-correct for the provider's event JSON, but it must see a COMPLETE
// data: payload. The raw RestoreStreamChunk/FlushStream holdback path (validated
// separately in M3) is intentionally NOT used here, so the two relay strategies
// never mix on one stream.
//
// Arbitrary byte-boundary tolerance (exit criterion #7) comes from the frame
// buffer: bytes are appended as they arrive and only COMPLETE events (terminated
// by a blank line) are split off and restored; a partial trailing event stays
// buffered until the bytes that finish it arrive on a later read. A CLK_ token
// therefore can never be restored from a half-received event — the event is not
// processed until its terminator is seen — so a token split across a read
// boundary cannot leak unrestored.
func (p *Proxy) relayStream(w http.ResponseWriter, r *http.Request, resp *http.Response, state *opencloak.State) {
	// Copy headers and commit the status before the first flush. Content-Length
	// is meaningless for a stream and copyHeaders already drops it.
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		// No flushing available (should not happen with net/http's writer, but
		// guard rather than buffer an entire unbounded stream). Fall back to a
		// straight relay of restored frames without explicit flushing.
		p.log.Warn("proxy: response writer is not a Flusher; streaming without explicit flush")
	}

	var buf bytes.Buffer
	readBuf := make([]byte, 4096)

	for {
		n, readErr := resp.Body.Read(readBuf)
		if n > 0 {
			buf.Write(readBuf[:n])
			// Split off every complete event currently in the buffer.
			for {
				idx := bytes.Index(buf.Bytes(), []byte(sseEventSep))
				if idx < 0 {
					break // no complete event yet; keep buffering
				}
				event := make([]byte, idx)
				copy(event, buf.Bytes()[:idx])
				// Drop the event plus its separator from the buffer.
				buf.Next(idx + len(sseEventSep))

				out := p.restoreSSEFrame(r, state, event)
				if _, err := w.Write(out); err != nil {
					p.log.Error("proxy: write SSE event to client", "err", err)
					return
				}
				if _, err := io.WriteString(w, sseEventSep); err != nil {
					p.log.Error("proxy: write SSE separator to client", "err", err)
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				// Mid-stream upstream read error: log and stop relaying. We do
				// not fabricate a tail event from a partial read.
				p.log.Error("proxy: upstream stream read error", "err", readErr)
				return
			}
			break
		}
	}

	// EOF: flush any final, unterminated buffered bytes as a last event. A
	// well-formed Anthropic stream ends each event with a blank line, so this is
	// usually empty, but a stream that closes without a trailing separator still
	// gets its last event restored rather than dropped.
	if buf.Len() > 0 {
		out := p.restoreSSEFrame(r, state, buf.Bytes())
		if _, err := w.Write(out); err != nil {
			p.log.Error("proxy: write final SSE event to client", "err", err)
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// restoreSSEFrame restores tokens in the data: payload(s) of one complete SSE
// event block and returns the rewritten block (without the trailing blank-line
// separator). Non-data lines (event:, id:, retry:, comments) and empty/[DONE]
// data lines are passed through verbatim. On a restore error the ORIGINAL event
// block is returned unchanged and the error is logged — the stream is never
// dropped and the error is never swallowed (exit criterion #5).
func (p *Proxy) restoreSSEFrame(r *http.Request, state *opencloak.State, event []byte) []byte {
	// Split into lines, preserving the exact set so re-emission is faithful.
	// SSE lines are LF-separated; tolerate a trailing CR (CRLF) per the spec.
	lines := strings.Split(string(event), "\n")
	changed := false

	for i, line := range lines {
		// Tolerate a trailing CR (CRLF framing) while preserving it on output.
		core := line
		cr := ""
		if strings.HasSuffix(core, "\r") {
			core = strings.TrimSuffix(core, "\r")
			cr = "\r"
		}
		if !strings.HasPrefix(core, sseDataTag) {
			continue
		}
		// Field value: everything after "data:", with a single optional leading
		// space stripped per the SSE grammar. Remember whether the space was
		// present so re-emission is byte-faithful for unchanged framing.
		raw := core[len(sseDataTag):]
		leadSpace := strings.HasPrefix(raw, " ")
		value := strings.TrimPrefix(raw, " ")
		if value == "" || value == sseDoneData {
			continue
		}

		restored, err := p.engine.RestoreSSEEvent(r.Context(), state, []byte(value))
		if err != nil {
			// Visible, not swallowed; original event relayed unchanged.
			p.log.Error("proxy: restore SSE event failed (relaying original event)", "err", err)
			return event
		}
		if !bytes.Equal(restored, []byte(value)) {
			// Reconstruct the line preserving the original "data:" prefix style
			// (leading space yes/no) and CRLF, swapping only the payload.
			prefix := sseDataTag
			if leadSpace {
				prefix += " "
			}
			lines[i] = prefix + string(restored) + cr
			changed = true
		}
	}

	if !changed {
		return event
	}
	return []byte(strings.Join(lines, "\n"))
}
