package wire

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ApplyMaskedSpansByRange applies changed JSON string spans in one pass when every
// span carries a valid byte range for the original JSON string literal.
//
// The boolean reports whether the fast path was applicable. A false value is not an
// error; provider adapters should fall back to their structural path setter.
func ApplyMaskedSpansByRange(body []byte, spans []MaskedSpan) ([]byte, bool, error) {
	if len(spans) == 0 {
		return body, true, nil
	}

	type replacement struct {
		start int
		end   int
		value []byte
	}

	repls := make([]replacement, 0, len(spans))
	outLen := len(body)
	for _, sp := range spans {
		if sp.Start < 0 || sp.End <= sp.Start || sp.End > len(body) {
			return nil, false, nil
		}
		raw := body[sp.Start:sp.End]
		if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
			return nil, false, nil
		}
		encoded, err := json.Marshal(sp.MaskedText)
		if err != nil {
			return nil, true, fmt.Errorf("encode masked span at %q: %w", sp.Path, err)
		}
		outLen += len(encoded) - (sp.End - sp.Start)
		repls = append(repls, replacement{
			start: sp.Start,
			end:   sp.End,
			value: encoded,
		})
	}

	sort.Slice(repls, func(i, j int) bool {
		return repls[i].start < repls[j].start
	})
	for i := 1; i < len(repls); i++ {
		if repls[i].start < repls[i-1].end {
			return nil, false, nil
		}
	}

	out := make([]byte, 0, outLen)
	cursor := 0
	for _, repl := range repls {
		out = append(out, body[cursor:repl.start]...)
		out = append(out, repl.value...)
		cursor = repl.end
	}
	out = append(out, body[cursor:]...)
	return out, true, nil
}
