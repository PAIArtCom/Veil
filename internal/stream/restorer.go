package stream

import (
	"regexp"

	"github.com/PAIArtCom/Veil/internal/mask"
	"github.com/PAIArtCom/Veil/internal/token"
)

// tokenRe matches a complete PAIArtVeil_… token. It is compiled from the canonical
// token.TokenPattern so the streaming scanner and the rest of the engine agree
// on exactly what a token looks like.
var tokenRe = regexp.MustCompile(token.TokenPattern)

// Restorer restores placeholders in a streamed byte relay where placeholders
// may be split across arbitrary chunk boundaries. It buffers a trailing
// "dangerous" suffix — the longest tail that could still be (or grow into) a
// placeholder — and only emits bytes it can prove are not part of a placeholder
// that future bytes might complete. Flush drains the tail at end of stream.
//
// This is the universal byte-relay path: it works on any stream of bytes (e.g.
// an SSE response body relayed verbatim) without parsing structure.
//
// Limitation: raw byte substitution does NOT re-escape JSON. If a restored
// value contains JSON-special characters (quotes, backslashes, control bytes)
// it could break a JSON string inside an SSE `data:` line. The structured,
// escaping-correct restore path for a known provider is the engine's
// RestoreSSEEvent; use this Restorer only where a verbatim byte relay is
// required and restored values are known to be JSON-safe, or where the consumer
// does not parse the bytes as JSON.
//
// Concurrency: a Restorer is single-writer. One *Restorer serves exactly one
// stream (one response body, one goroutine) and its methods are called
// sequentially by that goroutine. It therefore holds no mutex; do not share a
// Restorer across goroutines.
type Restorer struct {
	buf      []byte
	lookup   func(token string) (value string, found bool)
	residual map[string]int // residual token TYPE -> count of unrestored matches
}

// NewRestorer returns a Restorer that resolves placeholders via lookup. lookup
// reports the original value for a placeholder and whether it was found in the
// active scope.
func NewRestorer(lookup func(string) (string, bool)) *Restorer {
	return &Restorer{
		lookup:   lookup,
		residual: make(map[string]int),
	}
}

// Write appends chunk to the buffer, emits the bytes that are safe to release
// (with complete placeholders restored), and holds back the trailing partial
// placeholder suffix. The returned slice is freshly built and owned by the
// caller.
func (r *Restorer) Write(chunk []byte) []byte {
	r.buf = append(r.buf, chunk...)

	// danger is the index where the held-back suffix begins: the longest tail of
	// the buffer that is a prefix of a possibly complete-and-extendable
	// placeholder. The suffix helpers always return a value in [0, len(buf)] —
	// len(buf) meaning "hold nothing".
	tokenDanger := token.PartialSuffixStart(r.buf)
	surrogateDanger := mask.PartialSurrogateSuffixStart(r.buf)
	danger := tokenDanger
	maxHold := token.MaxTokenLen
	if surrogateDanger < danger {
		danger = surrogateDanger
		maxHold = mask.MaxSurrogateLen
	}

	// Growth guard: a dangerous suffix longer than any real token is garbage
	// (e.g. a long "PAIArtVeil_AAAA…" run that can never complete because it has no
	// "_<hex>" id). Emit the excess so the buffer cannot grow without bound.
	// Restoring across the forced cut point is safe: such an over-long run
	// contains no complete real token there, so no match spans the cut.
	if len(r.buf)-danger > maxHold {
		danger = len(r.buf) - maxHold
	}

	out := r.restore(r.buf[:danger])

	// Retain only the held-back suffix, copied into a fresh small backing array
	// so the (possibly large) chunk backing array can be garbage collected and
	// the buffer's capacity stays bounded.
	r.buf = append([]byte(nil), r.buf[danger:]...)

	return out
}

// Flush restores complete tokens in everything still buffered, clears the
// buffer, and returns the result. Call it once at end of stream. Any token-shaped
// tail that never terminated is treated as a complete token if it matches
// TokenPattern (restored or counted as residual) and otherwise emitted verbatim.
func (r *Restorer) Flush() []byte {
	out := r.restore(r.buf)
	r.buf = nil
	return out
}

// ResidualCounts returns a copy of the accumulated residual-token counts keyed
// by token TYPE. A residual is a validly-shaped token (it matches TokenPattern)
// that the lookup did not resolve — it is emitted unchanged and counted here so
// the caller can audit how often unknown tokens reached the client.
func (r *Restorer) ResidualCounts() map[string]int {
	out := make(map[string]int, len(r.residual))
	for k, v := range r.residual {
		out[k] = v
	}
	return out
}

// restore replaces every complete placeholder in b with its looked-up value.
// Unknown opaque tokens (validly shaped but absent from lookup) are emitted
// unchanged and counted as residuals by TYPE. Strings that merely look Veil-ish
// but do not match TokenPattern are left untouched and not counted.
func (r *Restorer) restore(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := mask.RestoreKnownPlaceholders(string(b), r.lookup, func(tok string) {
		// Residual: a real-shaped token we cannot restore. Leave it as-is and
		// record its TYPE. Every TokenPattern match has a parseable TYPE.
		if typ, ok := token.ParseType(tok); ok {
			r.residual[typ]++
		}
	})
	return []byte(out)
}
