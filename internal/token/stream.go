package token

import (
	"regexp"
	"strings"
)

// PartialSuffixPattern documents the anchored shape of a suffix after the full
// namespace prefix is present. PartialSuffixStart owns the exact token-prefix
// scan, including partial namespace prefixes such as "Open" or "OpenCloak", so
// the implementation follows Prefix without hand-written nested regex for the
// brand string.
//
// A token is OpenCloak_<TYPE>_<hex12+>. A buffer tail must be held back when it
// could still grow into — or already is, and could keep growing into — a token.
// The meaningful suffixes are prefixes of:
//
//	OpenCloak, OpenCloak_, OpenCloak_<TYPE>,
//	OpenCloak_<TYPE>_, and OpenCloak_<TYPE>_<partial-or-complete-hex>.
//
// Because the id is {12,} hex (collision-extend can append more hex), even a
// complete 12-hex token sitting at the very end of the buffer is extendable:
// the next chunk might continue the hex run and lengthen the id. Such a tail is
// therefore held until a non-hex byte (or Flush) proves the token terminated.
//
// The expression is not used as the source of truth; PartialSuffixStart performs
// the exact grammar check.
const PartialSuffixPattern = `OpenCloak_[A-Z0-9]+_[0-9a-f]*$`

// PartialSuffixStart returns the byte index at which the longest token-prefix
// suffix of b begins — i.e. the first index of the trailing bytes that a
// streaming restorer must hold back because they could still grow into a token.
// The result is in the range [0, len(b)]; len(b) means "hold nothing".
//
// This keeps the partial-token grammar owned by the token package; the stream
// package consumes it rather than re-deriving the shape of a token prefix.
func PartialSuffixStart(b []byte) int {
	s := string(b)
	for start := 0; start < len(s); start++ {
		if isTokenPrefixSuffix(s[start:]) {
			return start
		}
	}
	return len(b)
}

func isTokenPrefixSuffix(s string) bool {
	if s == "" {
		return true
	}
	if len(s) < len(Prefix) {
		return strings.HasPrefix(Prefix, s)
	}
	if !strings.HasPrefix(s, Prefix) {
		return false
	}
	rest := s[len(Prefix):]
	if rest == "" {
		return true
	}
	typeEnd := strings.IndexByte(rest, '_')
	if typeEnd < 0 {
		return isAllTokenType(rest)
	}
	if typeEnd == 0 || !isAllTokenType(rest[:typeEnd]) {
		return false
	}
	id := rest[typeEnd+1:]
	return isAllLowerHex(id)
}

func isAllTokenType(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isTokenTypeByte(s[i]) {
			return false
		}
	}
	return true
}

func isTokenTypeByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func isAllLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isLowerHexByte(s[i]) {
			return false
		}
	}
	return true
}

func isLowerHexByte(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f')
}

// tokenRe matches a complete OpenCloak_… token. It mirrors TokenPattern and is
// used by ParseType and (indirectly, via the same source) by the restorer's
// scanner.
var tokenRe = regexp.MustCompile(TokenPattern)

// MaxTokenLen is a generous upper bound on the byte length of any real token.
// The streaming restorer uses it as a buffer-growth guard: a dangerous suffix
// longer than this cannot be a real token, so the excess is emitted to keep the
// holdback buffer bounded. A real token is Prefix + TYPE + "_" + id; TYPE
// values are short uppercase tags and the id is a few dozen hex chars even after
// many collision extensions, so 128 leaves ample headroom.
const MaxTokenLen = 128

// ParseType returns the TYPE field of an OpenCloak_<TYPE>_<id> token — the
// segment between Prefix and the first underscore after it. ok is false when tok
// does not match TokenPattern. Token-grammar parsing is centralized here so
// callers (e.g. the streaming restorer's residual accounting) do not re-implement
// the token shape.
func ParseType(tok string) (typ string, ok bool) {
	if !tokenRe.MatchString(tok) {
		return "", false
	}
	rest := tok[len(Prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '_' {
			return rest[:i], true
		}
	}
	// Unreachable for a TokenPattern match (the id separator underscore is
	// always present), but return safely rather than panic.
	return "", false
}
