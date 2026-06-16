package l1

import (
	"context"
	"net/netip"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cloakia/opencloak/internal/types"
)

// rule is a single pattern-based detection rule.
type rule struct {
	typ    types.Type
	source string
	score  float64
	re     *regexp.Regexp
	// group selects which regexp submatch becomes the finding span. 0 (the
	// default) uses the whole match. A positive value uses that capture group,
	// which lets a rule require surrounding context (e.g. a leading boundary)
	// without including it in the masked span. RE2 has no lookbehind, so a rule
	// that must not start mid-token captures the boundary in the full match and
	// the real value in a group.
	group int
	// validate is an optional post-match validator; returning false drops the
	// finding. It receives the selected group's text.
	validate func(match string) bool
}

// Detector is the L1 pattern-based detector. The zero value is not useful;
// use New to construct one.
type Detector struct {
	rules []rule
}

// New constructs and returns an L1 Detector.
func New() *Detector {
	return &Detector{rules: buildRules()}
}

// Detect implements the Detector interface for L1. It scans text and returns
// all validated findings.
func (d *Detector) Detect(_ context.Context, text string) ([]types.Finding, error) {
	var findings []types.Finding

	// Apply each structural rule.
	for _, r := range d.rules {
		// When a rule selects a submatch group, use SubmatchIndex so the finding
		// span is the group (the real value), not the full match (which may
		// include a leading boundary char the regex needed for anchoring).
		locs := r.re.FindAllStringSubmatchIndex(text, -1)
		for _, loc := range locs {
			si, ei := 2*r.group, 2*r.group+1
			if ei >= len(loc) || loc[si] < 0 {
				continue // group did not participate in this match
			}
			start, end := loc[si], loc[ei]
			matched := text[start:end]
			if r.validate != nil && !r.validate(matched) {
				continue // drop invalid candidates
			}
			findings = append(findings, types.Finding{
				Start:  start,
				End:    end,
				Type:   r.typ,
				Score:  r.score,
				Source: r.source,
			})
		}
	}

	// Entropy-based detection: find non-whitespace tokens and check entropy.
	findings = append(findings, entropyFindings(text)...)

	return findings, nil
}

// entropyFindings scans for high-entropy tokens and promotes them to SECRET
// when a context keyword is within ~40 chars.
func entropyFindings(text string) []types.Finding {
	var findings []types.Finding
	// Split on whitespace and common delimiters to find candidate tokens.
	words := wordSplitter.FindAllStringIndex(text, -1)
	for _, loc := range words {
		start, end := loc[0], loc[1]
		word := text[start:end]
		// Require a minimum length to avoid noise.
		if len(word) < 8 {
			continue
		}
		// Skip obvious words (all alpha, all digit).
		if isAllAlpha(word) || isAllDigit(word) {
			continue
		}
		h := shannonEntropy(word)
		if h < entropyThresholdHigh {
			continue
		}
		// High entropy found. Check for context keyword within 40 chars.
		windowStart := start - 40
		if windowStart < 0 {
			windowStart = 0
		}
		windowEnd := end + 40
		if windowEnd > len(text) {
			windowEnd = len(text)
		}
		context := strings.ToLower(text[windowStart:windowEnd])
		hasKeyword := false
		for _, kw := range contextKeywords {
			if strings.Contains(context, kw) {
				hasKeyword = true
				break
			}
		}
		if hasKeyword {
			findings = append(findings, types.Finding{
				Start:  start,
				End:    end,
				Type:   types.TypeSecret,
				Score:  0.75,
				Source: "l1:entropy:contextual",
			})
		}
		// Isolated high-entropy strings are NOT flagged per the spec.
	}
	return findings
}

// wordSplitter matches non-whitespace, non-delimiter runs.
var wordSplitter = regexp.MustCompile(`[^\s,;:\[\]{}"'<>()=]+`)

func isAllAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func isAllDigit(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// buildRules returns the starter set of L1 detection rules.
func buildRules() []rule {
	return []rule{
		// ---- Structured PII ----

		// Email
		{
			typ:    types.TypeEmail,
			source: "l1:email",
			score:  0.95,
			re:     regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		},

		// IPv4
		{
			typ:    types.TypeIPv4,
			source: "l1:ipv4",
			score:  0.90,
			re:     regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`),
		},

		// IPv6 (capture group 1), validated by netip plus two guards.
		//
		// Two false-positive classes from language scope/path syntax must be
		// blocked, because masking them would corrupt outbound source code — the
		// very traffic this tool protects:
		//
		//  1. Suffix-grabbing. The regex would otherwise grab the trailing hex run
		//     of an identifier: "namespace::func" -> "ace::f", "interface::m" ->
		//     "face::", "DataFace::x" -> "Face::". The fix is a LEFT BOUNDARY: the
		//     address must start at string start or after a byte that is neither an
		//     identifier char nor a colon. Inside an identifier every preceding byte
		//     is a letter/digit/underscore, so no in-identifier start is possible.
		//     RE2 has no lookbehind, so the boundary is part of the full match and
		//     the address is capture group 1 (see rule.group).
		//  2. Tiny ambiguous forms. "a::b" / "::1" are valid compressed IPv6 yet
		//     indistinguishable from short paths; a real, privacy-relevant literal
		//     has a hextet of >=3 hex digits (2001, db8, fe80). We require one.
		//
		// Trade-off: bare tiny forms (::1, a::b, 0:0:0:0:0:0:0:1) are not masked in
		// Phase 0. Partial IPv6 coverage is documented in detection-layers.md.
		{
			typ:    types.TypeIPv6,
			source: "l1:ipv6",
			score:  0.90,
			re:     regexp.MustCompile(`(?:^|[^0-9A-Za-z_:])((?:[0-9A-Fa-f]{0,4}:){2,7}[0-9A-Fa-f]{0,4})`),
			group:  1,
			validate: func(match string) bool {
				addr, err := netip.ParseAddr(match)
				if err != nil || !addr.Is6() {
					return false
				}
				for _, hextet := range strings.Split(match, ":") {
					if len(hextet) >= 3 {
						return true
					}
				}
				return false
			},
		},

		// Credit card — 13-19 digits, optional spaces/dashes every 4 digits, Luhn.
		{
			typ:    types.TypeCard,
			source: "l1:luhn",
			score:  1.0,
			re:     regexp.MustCompile(`\b(?:\d[ \-]?){12,18}\d\b`),
			validate: func(match string) bool {
				digits := stripNonDigit(match)
				if len(digits) < 13 || len(digits) > 19 {
					return false
				}
				return luhnValid(digits)
			},
		},

		// IBAN account identifiers — validated by ISO 13616 mod-97. IBANs are
		// canonically uppercase; the rule is case-sensitive so it does not fire on
		// the ~1% of lowercase 2-alpha/2-digit/alnum identifiers (tokens, IDs) that
		// would otherwise pass mod-97 by chance.
		{
			typ:      types.TypeAcct,
			source:   "l1:iban",
			score:    0.90,
			re:       regexp.MustCompile(`\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b`),
			validate: ibanValid,
		},

		// Phone — loose E.164 and common US/intl formats.
		{
			typ:    types.TypePhone,
			source: "l1:phone",
			score:  0.70,
			re:     regexp.MustCompile(`(?:\+?1[\s.\-]?)?\(?\d{3}\)?[\s.\-]?\d{3}[\s.\-]?\d{4}\b`),
		},

		// URL
		{
			typ:    types.TypeURL,
			source: "l1:url",
			score:  0.85,
			re:     regexp.MustCompile(`(?i)\b(?:https?|postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis)://[^\s"'<>]+`),
		},

		// ISO calendar date.
		{
			typ:    types.TypeDate,
			source: "l1:date:iso",
			score:  0.60,
			re:     regexp.MustCompile(`\b(?:19|20)\d{2}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])\b`),
			validate: func(match string) bool {
				_, err := time.Parse("2006-01-02", match)
				return err == nil
			},
		},

		// ---- Secrets ----

		// AWS access key id
		{
			typ:    types.TypeSecret,
			source: "l1:aws-access-key-id",
			score:  0.99,
			re:     regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		},

		// GitHub PATs — gh[pousr]_ prefix
		{
			typ:    types.TypeSecret,
			source: "l1:github-pat",
			score:  0.99,
			re:     regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`),
		},

		// OpenAI sk- keys
		{
			typ:    types.TypeSecret,
			source: "l1:openai-key",
			score:  0.99,
			re:     regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`),
		},

		// PEM private key header
		{
			typ:    types.TypeSecret,
			source: "l1:pem-private-key",
			score:  0.99,
			re:     regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
		},
	}
}

// stripNonDigit removes all non-digit characters from s.
func stripNonDigit(s string) string {
	var b strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}
