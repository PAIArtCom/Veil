package token

import "regexp"

// PartialSuffixPattern is the anchored regexp source matching the longest tail
// of a buffer that is a prefix of a (possibly already-complete-and-extendable)
// token. The streaming restorer uses it to decide how many trailing bytes must
// be held back across a chunk boundary so a token can never be emitted split.
//
// A token is CLK_<TYPE>_<hex12+>. A buffer tail must be held back when it could
// still grow into — or already is, and could keep growing into — a token. The
// progressively-optional groups match every prefix of that grammar:
//
//	C, CL, CLK, CLK_, CLK_<TYPE> (type started),
//	CLK_<TYPE>_ and CLK_<TYPE>_<partial-or-complete-hex>.
//
// Because the id is {12,} hex (collision-extend can append more hex), even a
// complete 12-hex token sitting at the very end of the buffer is extendable:
// the next chunk might continue the hex run and lengthen the id. Such a tail is
// therefore held until a non-hex byte (or Flush) proves the token terminated.
//
// The whole expression is optional and anchored to the end, so it always
// matches — at minimum the empty suffix at len(buf), meaning "hold nothing".
const PartialSuffixPattern = `C(L(K(_([A-Z0-9]+(_[0-9a-f]*)?)?)?)?)?$`

// partialSuffixRe is the compiled PartialSuffixPattern. RE2 returns the leftmost
// match; combined with the `$` anchor this yields the start of the longest
// token-prefix suffix of the buffer.
var partialSuffixRe = regexp.MustCompile(PartialSuffixPattern)

// PartialSuffixStart returns the byte index at which the longest token-prefix
// suffix of b begins — i.e. the first index of the trailing bytes that a
// streaming restorer must hold back because they could still grow into a token.
// Because PartialSuffixPattern is anchored and fully optional it always matches,
// so the result is in the range [0, len(b)]; len(b) means "hold nothing".
//
// This keeps the partial-token grammar (and its single compiled regexp) owned by
// the token package; the stream package consumes it rather than re-deriving the
// shape of a token prefix.
func PartialSuffixStart(b []byte) int {
	if loc := partialSuffixRe.FindIndex(b); loc != nil {
		return loc[0]
	}
	return len(b)
}

// tokenRe matches a complete CLK_… token. It mirrors TokenPattern and is used
// by ParseType and (indirectly, via the same source) by the restorer's scanner.
var tokenRe = regexp.MustCompile(TokenPattern)

// MaxTokenLen is a generous upper bound on the byte length of any real token.
// The streaming restorer uses it as a buffer-growth guard: a dangerous suffix
// longer than this cannot be a real token, so the excess is emitted to keep the
// holdback buffer bounded. A real token is "CLK_" + TYPE + "_" + id; TYPE values
// are short uppercase tags and the id is a few dozen hex chars even after many
// collision extensions, so 128 leaves ample headroom.
const MaxTokenLen = 128

// ParseType returns the TYPE field of a CLK_<TYPE>_<id> token — the segment
// between the "CLK_" prefix and the first underscore after it. ok is false when
// token does not match TokenPattern. Token-grammar parsing is centralized here
// so callers (e.g. the streaming restorer's residual accounting) do not
// re-implement the token shape.
func ParseType(token string) (typ string, ok bool) {
	if !tokenRe.MatchString(token) {
		return "", false
	}
	// token matches CLK_[A-Z0-9]+_[0-9a-f]{12,}: strip the fixed "CLK_" prefix,
	// then the TYPE runs up to the next underscore (which separates TYPE and id).
	rest := token[len(Prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '_' {
			return rest[:i], true
		}
	}
	// Unreachable for a TokenPattern match (the id separator underscore is
	// always present), but return safely rather than panic.
	return "", false
}
