package mask

import (
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PAIArtCom/Veil/internal/mapstore"
	"github.com/PAIArtCom/Veil/internal/token"
	"github.com/PAIArtCom/Veil/internal/types"
)

// Result is returned by Apply.
type Result struct {
	// Masked is the rewritten text.
	Masked string
	// Blocked lists types that hit OperatorBlock. If non-empty, the caller
	// should wrap these in a *BlockedError and return it.
	Blocked []types.Type
}

// Apply performs offset-safe replacement of resolved findings in text.
// It consults policy to determine the operator per type, writes placeholder→value
// mappings into store under scope, and returns the masked text or collects
// blocked types.
//
// findings must be non-overlapping and in ascending start order (as produced
// by the resolver).
func Apply(
	text string,
	findings []types.Finding,
	scope types.Scope,
	policy types.Policy,
	store *mapstore.Store,
	keyer *token.Keyer,
	collisions map[string]string,
) (Result, error) {
	// Determine the default operator.
	defOp := policy.DefaultOperator
	if defOp == "" {
		defOp = types.OperatorToken
	}

	// Collect blocked types (deduplicated).
	blockedSet := map[types.Type]struct{}{}

	type action struct {
		finding types.Finding
		op      types.TransformOperator
		tok     string // pre-computed token for OperatorToken
	}
	actions := make([]action, 0, len(findings))
	for _, f := range findings {
		op := defOp
		if tp, ok := policy.Types[f.Type]; ok && tp.Operator != "" {
			op = tp.Operator
		}
		a := action{finding: f, op: op}
		switch op {
		case types.OperatorToken, types.OperatorIgnore:
			// handled after the full action list is validated
		case types.OperatorBlock:
			blockedSet[f.Type] = struct{}{}
		default:
			return Result{}, fmt.Errorf("mask: unsupported transform operator %q for type %s", op, f.Type)
		}
		actions = append(actions, a)
	}

	// If any blocked types were found, return them without modifying text.
	if len(blockedSet) > 0 {
		blocked := make([]types.Type, 0, len(blockedSet))
		for t := range blockedSet {
			blocked = append(blocked, t)
		}
		sort.Slice(blocked, func(i, j int) bool {
			return string(blocked[i]) < string(blocked[j])
		})
		return Result{Masked: text, Blocked: blocked}, nil
	}

	for i := range actions {
		if actions[i].op != types.OperatorToken {
			continue
		}
		f := actions[i].finding
		value := text[f.Start:f.End]
		if _, known := store.Get(scope, value); known {
			actions[i].tok = value
			continue
		}
		tok := keyer.Derive(f.Type, value, collisions)
		if surrogate, ok := surrogateFor(f.Type, value, tok); ok {
			if existing, known := store.Get(scope, surrogate); !known || existing == value {
				store.Put(scope, surrogate, value)
				actions[i].tok = surrogate
				continue
			}
		}
		store.Put(scope, tok, value)
		actions[i].tok = tok
	}

	outLen := len(text)
	for _, a := range actions {
		if a.op == types.OperatorToken {
			outLen += len(a.tok) - (a.finding.End - a.finding.Start)
		}
	}

	buf := make([]byte, 0, outLen)
	cursor := 0
	for _, a := range actions {
		f := a.finding
		if cursor < f.Start {
			buf = append(buf, text[cursor:f.Start]...)
		}
		switch a.op {
		case types.OperatorToken:
			buf = append(buf, a.tok...)
		case types.OperatorIgnore:
			buf = append(buf, text[f.Start:f.End]...)
		}
		cursor = f.End
	}
	if cursor < len(text) {
		buf = append(buf, text[cursor:]...)
	}

	return Result{Masked: string(buf)}, nil
}

const knownSurrogatePattern = `(?i:\b[a-z][a-z0-9-]*-[0-9a-f]{12,}@veil\.paiart\.com\b|\b(?:https?|postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis)://[^\s"'<>|]*veil\.paiart\.com[^\s"'<>|]*|\b(?:127\.0\.0|10\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}|192\.168\.\d{1,3}|169\.254\.\d{1,3}|203\.0\.113)\.\d{1,3}\b|\b(?:2001:db8:[0-9a-f]{1,4}:[0-9a-f]{1,4}::[0-9a-f]{1,4}|fd00:[0-9a-f]{1,4}:[0-9a-f]{1,4}::[0-9a-f]{1,4}|fe80::[0-9a-f]{1,4}:[0-9a-f]{1,4}:[0-9a-f]{1,4})\b)`

var knownSurrogateRe = regexp.MustCompile(knownSurrogatePattern)
var knownPlaceholderRe = regexp.MustCompile(token.TokenPattern + `|` + knownSurrogatePattern)

// MaxSurrogateLen bounds raw-stream holdback for format-preserving values.
// Surrogates may preserve URL path/query shape, so the bound is intentionally
// larger than token.MaxTokenLen while still preventing unbounded buffering.
const MaxSurrogateLen = 4096

// RestoreKnownSurrogates replaces format-preserving surrogates that resolve
// through lookup. Unknown surrogates are left unchanged, mirroring token restore
// behavior.
func RestoreKnownSurrogates(text string, lookup func(string) (string, bool)) string {
	return knownSurrogateRe.ReplaceAllStringFunc(text, func(candidate string) string {
		return restoreKnownSurrogateCandidate(candidate, lookup)
	})
}

// RestoreKnownPlaceholders replaces opaque tokens and format-preserving
// surrogates in one pass over the original text. Replacement values are not
// scanned again, so a real value that happens to look like another placeholder
// is preserved exactly.
func RestoreKnownPlaceholders(text string, lookup func(string) (string, bool), onResidualToken func(string)) string {
	return knownPlaceholderRe.ReplaceAllStringFunc(text, func(candidate string) string {
		if strings.HasPrefix(candidate, token.Prefix) {
			if restored, ok := token.RestoreKnownPrefix(candidate, lookup); ok {
				return restored
			}
			if onResidualToken != nil {
				onResidualToken(candidate)
			}
			return candidate
		}
		return restoreKnownSurrogateCandidate(candidate, lookup)
	})
}

func restoreKnownSurrogateCandidate(candidate string, lookup func(string) (string, bool)) string {
	if restored, ok := lookup(candidate); ok {
		return restored
	}
	return restoreDelimitedSurrogateCandidate(candidate, lookup)
}

func restoreDelimitedSurrogateCandidate(candidate string, lookup func(string) (string, bool)) string {
	var out strings.Builder
	start := 0
	changed := false
	for start < len(candidate) {
		var (
			bestEnd      int
			bestRestored string
		)
		for end := len(candidate); end > start; end-- {
			if restored, ok := lookup(candidate[start:end]); ok {
				bestEnd = end
				bestRestored = restored
				break
			}
		}
		if bestEnd > 0 {
			out.WriteString(bestRestored)
			changed = true
			start = bestEnd
			continue
		}
		out.WriteByte(candidate[start])
		start++
	}
	if !changed {
		return candidate
	}
	return out.String()
}

// PartialSurrogateSuffixStart returns the start of the longest buffer suffix
// that could still become a format-preserving surrogate if more bytes arrive.
// len(b) means no holdback is needed.
func PartialSurrogateSuffixStart(b []byte) int {
	s := string(b)
	for start := 0; start < len(s); start++ {
		if isSurrogatePrefixSuffix(s[start:]) {
			return start
		}
	}
	return len(b)
}

func surrogateFor(typ types.Type, value, tok string) (string, bool) {
	id := tokenID(tok)
	if id == "" {
		return "", false
	}
	switch typ {
	case types.TypeEmail:
		return "user-" + id + "@veil.paiart.com", true
	case types.TypeIPv4:
		return ipv4Surrogate(value, id)
	case types.TypeIPv6:
		return ipv6Surrogate(value, id)
	case types.TypeURL:
		return urlSurrogate(value, id)
	default:
		return "", false
	}
}

func tokenID(tok string) string {
	i := strings.LastIndexByte(tok, '_')
	if i < 0 || i == len(tok)-1 {
		return ""
	}
	return tok[i+1:]
}

func urlSurrogate(raw, id string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}

	host := urlHostPrefix(u.Scheme) + "-" + id + ".veil.paiart.com"
	if port := u.Port(); port != "" {
		host += ":" + port
	}
	u.Host = host

	if u.User != nil {
		user := "user-" + id
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(user, "password-"+id)
		} else {
			u.User = url.User(user)
		}
	}

	if u.RawQuery != "" {
		q := u.Query()
		for key := range q {
			if sensitiveQueryKey(key) {
				q.Set(key, "value-"+id)
			}
		}
		u.RawQuery = q.Encode()
	}

	return u.String(), true
}

func ipv4Surrogate(raw, id string) (string, bool) {
	addr, err := netip.ParseAddr(raw)
	if err != nil || !addr.Is4() {
		return "", false
	}
	a := addr.As4()
	b1, b2 := surrogateByte(id, 0), surrogateByte(id, 1)
	host := 1 + int(surrogateByte(id, 2))%254

	switch {
	case a[0] == 127:
		return distinctIPv4(raw, fmt.Sprintf("127.0.0.%d", host)), true
	case a[0] == 10:
		return distinctIPv4(raw, fmt.Sprintf("10.%d.%d.%d", b1, b2, host)), true
	case a[0] == 172 && a[1] >= 16 && a[1] <= 31:
		return distinctIPv4(raw, fmt.Sprintf("172.%d.%d.%d", 16+int(b1)%16, b2, host)), true
	case a[0] == 192 && a[1] == 168:
		return distinctIPv4(raw, fmt.Sprintf("192.168.%d.%d", b1, host)), true
	case a[0] == 169 && a[1] == 254:
		return distinctIPv4(raw, fmt.Sprintf("169.254.%d.%d", b1, host)), true
	default:
		return distinctIPv4(raw, fmt.Sprintf("203.0.113.%d", host)), true
	}
}

func ipv6Surrogate(raw, id string) (string, bool) {
	addr, err := netip.ParseAddr(raw)
	if err != nil || !addr.Is6() {
		return "", false
	}
	h1, h2, h3 := idHextet(id, 0), idHextet(id, 1), idHextet(id, 2)
	switch {
	case addr.IsPrivate():
		return distinctIPv6(raw, fmt.Sprintf("fd00:%s:%s::%s", h1, h2, h3)), true
	case addr.IsLinkLocalUnicast():
		return distinctIPv6(raw, fmt.Sprintf("fe80::%s:%s:%s", h1, h2, h3)), true
	default:
		return distinctIPv6(raw, fmt.Sprintf("2001:db8:%s:%s::%s", h1, h2, h3)), true
	}
}

func distinctIPv4(raw, candidate string) string {
	rawAddr, rawErr := netip.ParseAddr(raw)
	candidateAddr, candidateErr := netip.ParseAddr(candidate)
	if rawErr != nil || candidateErr != nil || rawAddr != candidateAddr {
		return candidate
	}
	a := candidateAddr.As4()
	a[3] = a[3]%254 + 1
	if a[3] == candidateAddr.As4()[3] {
		a[3] = a[3]%254 + 1
	}
	return netip.AddrFrom4(a).String()
}

func distinctIPv6(raw, candidate string) string {
	rawAddr, rawErr := netip.ParseAddr(raw)
	candidateAddr, candidateErr := netip.ParseAddr(candidate)
	if rawErr != nil || candidateErr != nil || rawAddr != candidateAddr {
		return candidate
	}
	a := candidateAddr.As16()
	a[15]++
	if a[15] == 0 {
		a[15] = 1
	}
	return netip.AddrFrom16(a).String()
}

func urlHostPrefix(scheme string) string {
	switch strings.ToLower(scheme) {
	case "postgres", "postgresql", "mysql", "mongodb", "mongodb+srv", "redis":
		return "db"
	default:
		return "api"
	}
}

func sensitiveQueryKey(key string) bool {
	k := strings.ToLower(key)
	for _, part := range []string{"token", "key", "secret", "password", "passwd", "pwd", "auth", "credential"} {
		if strings.Contains(k, part) {
			return true
		}
	}
	return false
}

func surrogateByte(id string, index int) byte {
	start := index * 2
	if start+2 > len(id) {
		return 0
	}
	v, err := strconv.ParseUint(id[start:start+2], 16, 8)
	if err != nil {
		return 0
	}
	return byte(v)
}

func idHextet(id string, index int) string {
	start := index * 4
	if start+4 > len(id) {
		return "0"
	}
	trimmed := strings.TrimLeft(id[start:start+4], "0")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

func isSurrogatePrefixSuffix(s string) bool {
	if s == "" {
		return true
	}
	return isEmailSurrogatePrefixSuffix(s) || isURLSurrogatePrefixSuffix(s) ||
		isIPv4SurrogatePrefixSuffix(s) || isIPv6SurrogatePrefixSuffix(s)
}

func isEmailSurrogatePrefixSuffix(s string) bool {
	const prefix = "user-"
	if len(s) < len(prefix) {
		return strings.HasPrefix(prefix, s)
	}
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	rest := s[len(prefix):]
	hexEnd := 0
	for hexEnd < len(rest) && isLowerHex(rest[hexEnd]) {
		hexEnd++
	}
	if hexEnd < 12 {
		return hexEnd == len(rest)
	}
	rest = rest[hexEnd:]
	if rest == "" {
		return true
	}
	const domain = "@veil.paiart.com"
	if len(rest) <= len(domain) {
		return strings.HasPrefix(domain, rest)
	}
	return false
}

func isURLSurrogatePrefixSuffix(s string) bool {
	if hasURLDelimiter(s) {
		return false
	}
	for _, scheme := range surrogateURLSchemes {
		start := scheme + "://"
		if len(s) < len(start) {
			if strings.HasPrefix(start, strings.ToLower(s)) {
				return true
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(s), start) {
			return true
		}
	}
	return false
}

func isIPv4SurrogatePrefixSuffix(s string) bool {
	if hasIPDelimiter(s) {
		return false
	}
	for _, prefix := range surrogateIPv4Prefixes {
		if len(s) < len(prefix) {
			if strings.HasPrefix(prefix, s) {
				return true
			}
			continue
		}
		if strings.HasPrefix(s, prefix) && isIPv4Tail(s[len(prefix):]) {
			return true
		}
	}
	return false
}

func isIPv6SurrogatePrefixSuffix(s string) bool {
	if hasIPDelimiter(s) {
		return false
	}
	lower := strings.ToLower(s)
	for _, prefix := range surrogateIPv6Prefixes {
		if len(lower) < len(prefix) {
			if strings.HasPrefix(prefix, lower) {
				return true
			}
			continue
		}
		if strings.HasPrefix(lower, prefix) && isIPv6Tail(lower[len(prefix):]) {
			return true
		}
	}
	return false
}

var surrogateIPv4Prefixes = []string{
	"127.0.0.",
	"10.",
	"172.",
	"192.168.",
	"169.254.",
	"203.0.113.",
}

var surrogateIPv6Prefixes = []string{
	"2001:db8:",
	"fd00:",
	"fe80::",
}

func isIPv4Tail(s string) bool {
	if s == "" {
		return true
	}
	for i := 0; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && s[i] != '.' {
			return false
		}
	}
	return true
}

func isIPv6Tail(s string) bool {
	if s == "" {
		return true
	}
	for i := 0; i < len(s); i++ {
		if !isLowerHex(s[i]) && s[i] != ':' {
			return false
		}
	}
	return true
}

var surrogateURLSchemes = []string{
	"http",
	"https",
	"postgres",
	"postgresql",
	"mysql",
	"mongodb",
	"mongodb+srv",
	"redis",
}

func hasURLDelimiter(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r', '"', '\'', '<', '>', '|':
			return true
		}
	}
	return false
}

func hasIPDelimiter(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r', '"', '\'', '<', '>', '/', '\\', '@', '|', ',', ';':
			return true
		}
	}
	return false
}

func isLowerHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f')
}
