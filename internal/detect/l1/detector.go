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
	typ      types.Type
	source   string
	score    float64
	re       *regexp.Regexp
	keywords []string
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
	// validateSpan is an optional post-match validator that can inspect the
	// whole text around the selected span. It is used for low-cost false-positive
	// suppressors that need local context.
	validateSpan func(text string, start, end int) bool
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
	lowText := strings.ToLower(text)

	// Apply each structural rule.
	for _, r := range d.rules {
		if !r.applies(lowText) {
			continue
		}
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
			if r.validateSpan != nil && !r.validateSpan(text, start, end) {
				continue
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

func (r rule) applies(lowText string) bool {
	if len(r.keywords) == 0 {
		return true
	}
	for _, kw := range r.keywords {
		if strings.Contains(lowText, kw) {
			return true
		}
	}
	return false
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
		if !validEntropySecretSpan(text, start, end) {
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

		// Phone — common NANP/US formats. Broader country-aware phone parsing is
		// intentionally deferred because international numbering needs locale
		// policy, not a single loose regex.
		{
			typ:          types.TypePhone,
			source:       "l1:phone",
			score:        0.70,
			re:           regexp.MustCompile(`(?:\+?1[\s.\-]?)?\(?\d{3}\)?[\s.\-]?\d{3}[\s.\-]?\d{4}\b`),
			validateSpan: validPhoneSpan,
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

		// AWS access key ids. This uses high-confidence fixed prefixes while
		// keeping a tight fixed length.
		{
			typ:      types.TypeSecret,
			source:   "l1:aws-access-key-id",
			score:    0.99,
			re:       regexp.MustCompile(`\b(?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z2-7]{16}\b`),
			keywords: []string{"a3t", "akia", "asia", "abia", "acca"},
		},

		// 1Password secret key.
		{
			typ:          types.TypeSecret,
			source:       "l1:1password-secret-key",
			score:        0.99,
			re:           regexp.MustCompile(`\bA3-[A-Z0-9]{6}-(?:[A-Z0-9]{11}|[A-Z0-9]{6}-[A-Z0-9]{5})-[A-Z0-9]{5}-[A-Z0-9]{5}-[A-Z0-9]{5}\b`),
			keywords:     []string{"a3-"},
			validateSpan: validSecretSpan,
		},

		// 1Password service account tokens are JWT-like but have a distinct ops_
		// prefix and long base64 payload.
		{
			typ:          types.TypeSecret,
			source:       "l1:1password-service-account-token",
			score:        0.99,
			re:           regexp.MustCompile(`\bops_eyJ[A-Za-z0-9+/]{120,}={0,3}\b`),
			keywords:     []string{"ops_eyj"},
			validateSpan: validSecretSpan,
		},

		// Age private key.
		{
			typ:          types.TypeSecret,
			source:       "l1:age-secret-key",
			score:        0.99,
			re:           regexp.MustCompile(`\bAGE-SECRET-KEY-1[QPZRY9X8GF2TVDW0S3JN54KHCE6MUA7L]{58}\b`),
			keywords:     []string{"age-secret-key-1"},
			validateSpan: validSecretSpan,
		},

		// Alibaba Cloud AccessKey ID.
		{
			typ:          types.TypeSecret,
			source:       "l1:alibaba-access-key-id",
			score:        0.95,
			re:           regexp.MustCompile(`\bLTAI[A-Za-z0-9]{20}\b`),
			keywords:     []string{"ltai"},
			validateSpan: validSecretSpan,
		},

		// Amazon Bedrock API keys.
		{
			typ:          types.TypeSecret,
			source:       "l1:aws-bedrock-api-key",
			score:        0.99,
			re:           regexp.MustCompile(`\b(?:ABSK[A-Za-z0-9+/]{109,269}={0,2}|bedrock-api-key-[A-Za-z0-9._~+/=-]{20,})\b`),
			keywords:     []string{"absk", "bedrock-api-key-"},
			validateSpan: validSecretSpan,
		},

		// Azure AD client secrets have a distinctive Q~ marker.
		{
			typ:          types.TypeSecret,
			source:       "l1:azure-ad-client-secret",
			score:        0.95,
			re:           regexp.MustCompile(`(?:^|[\\'"\x60\s>=:(,)])([A-Za-z0-9_~.]{3}\dQ~[A-Za-z0-9_~.-]{31,34})(?:$|[\\'"\x60\s<),])`),
			keywords:     []string{"q~"},
			group:        1,
			validateSpan: validSecretSpan,
		},

		// Cloudflare Origin CA keys have a distinctive v1.0-... shape.
		{
			typ:          types.TypeSecret,
			source:       "l1:cloudflare-origin-ca-key",
			score:        0.95,
			re:           regexp.MustCompile(`\bv1\.0-[a-f0-9]{24}-[a-f0-9]{146}\b`),
			keywords:     []string{"v1.0-"},
			validateSpan: validSecretSpan,
		},

		// Defined Networking API token.
		{
			typ:          types.TypeSecret,
			source:       "l1:defined-networking-token",
			score:        0.95,
			re:           regexp.MustCompile(`\bdnkey-[A-Za-z0-9=_-]{26}-[A-Za-z0-9=_-]{52}\b`),
			keywords:     []string{"dnkey-"},
			validateSpan: validSecretSpan,
		},

		// DigitalOcean OAuth and personal access token families.
		{
			typ:          types.TypeSecret,
			source:       "l1:digitalocean-token",
			score:        0.99,
			re:           regexp.MustCompile(`\bdo[opr]_v1_[a-f0-9]{64}\b`),
			keywords:     []string{"doo_v1_", "dop_v1_", "dor_v1_"},
			validateSpan: validSecretSpan,
		},

		// GitHub PATs — gh[pousr]_ prefix
		{
			typ:          types.TypeSecret,
			source:       "l1:github-pat",
			score:        0.99,
			re:           regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`),
			keywords:     []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_"},
			validateSpan: validSecretSpan,
		},

		// GitHub fine-grained PATs.
		{
			typ:          types.TypeSecret,
			source:       "l1:github-fine-grained-pat",
			score:        0.99,
			re:           regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{82}\b`),
			keywords:     []string{"github_pat_"},
			validateSpan: validSecretSpan,
		},

		// GitLab personal/project/group access tokens.
		{
			typ:          types.TypeSecret,
			source:       "l1:gitlab-pat",
			score:        0.99,
			re:           regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`),
			keywords:     []string{"glpat-"},
			validateSpan: validSecretSpan,
		},

		// GitLab token families with distinct prefixes. Broad GitLab context
		// rules are deliberately excluded; this rule covers only fixed prefixes.
		{
			typ:    types.TypeSecret,
			source: "l1:gitlab-token",
			score:  0.95,
			re: regexp.MustCompile(
				`\b(?:glcbt-[0-9A-Za-z]{1,5}_[0-9A-Za-z_-]{20}|` +
					`(?:gldt|glffct|glft|glrt|glsoat)-[0-9A-Za-z_-]{20}|` +
					`glimt-[0-9A-Za-z_-]{25}|glagent-[0-9A-Za-z_-]{50}|` +
					`gloas-[0-9A-Za-z_-]{64}|glptt-[0-9a-f]{40}|GR1348941[0-9A-Za-z_-]{20})\b`),
			keywords: []string{
				"glcbt-", "gldt-", "glffct-", "glft-", "glimt-", "glagent-",
				"gloas-", "glptt-", "gr1348941", "glrt-", "glsoat-",
			},
			validateSpan: validSecretSpan,
		},

		// Hugging Face access and organization tokens.
		{
			typ:          types.TypeSecret,
			source:       "l1:huggingface-token",
			score:        0.95,
			re:           regexp.MustCompile(`\b(?:hf_|api_org_)[A-Za-z]{34}\b`),
			keywords:     []string{"hf_", "api_org_"},
			validateSpan: validSecretSpan,
		},

		// Infracost API token.
		{
			typ:          types.TypeSecret,
			source:       "l1:infracost-token",
			score:        0.95,
			re:           regexp.MustCompile(`\bico-[A-Za-z0-9]{32}\b`),
			keywords:     []string{"ico-"},
			validateSpan: validSecretSpan,
		},

		// Linear API token.
		{
			typ:          types.TypeSecret,
			source:       "l1:linear-api-key",
			score:        0.95,
			re:           regexp.MustCompile(`\blin_api_[A-Za-z0-9]{40}\b`),
			keywords:     []string{"lin_api_"},
			validateSpan: validSecretSpan,
		},

		// npm automation/access tokens.
		{
			typ:          types.TypeSecret,
			source:       "l1:npm-token",
			score:        0.99,
			re:           regexp.MustCompile(`\bnpm_[A-Za-z0-9]{36,}\b`),
			keywords:     []string{"npm_"},
			validateSpan: validSecretSpan,
		},

		// PyPI API tokens.
		{
			typ:          types.TypeSecret,
			source:       "l1:pypi-token",
			score:        0.99,
			re:           regexp.MustCompile(`\bpypi-[A-Za-z0-9_-]{32,}\b`),
			keywords:     []string{"pypi-"},
			validateSpan: validSecretSpan,
		},

		// Typeform API token.
		{
			typ:          types.TypeSecret,
			source:       "l1:typeform-token",
			score:        0.95,
			re:           regexp.MustCompile(`\btfp_[A-Za-z0-9_.=-]{59}\b`),
			keywords:     []string{"tfp_"},
			validateSpan: validSecretSpan,
		},

		// HashiCorp Vault service and batch tokens. The legacy s.<24> shape is
		// excluded because it is too short and collision-prone for this batch.
		{
			typ:          types.TypeSecret,
			source:       "l1:vault-token",
			score:        0.95,
			re:           regexp.MustCompile(`\bhv[bs]\.[A-Za-z0-9_-]{90,300}\b`),
			keywords:     []string{"hvs.", "hvb."},
			validateSpan: validSecretSpan,
		},

		// OpenAI sk- keys
		{
			typ:          types.TypeSecret,
			source:       "l1:openai-key",
			score:        0.99,
			re:           regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_-]{20,}\b`),
			keywords:     []string{"sk-"},
			validate:     func(match string) bool { return !strings.HasPrefix(match, "sk-ant-") },
			validateSpan: validSecretSpan,
		},

		// Anthropic API keys.
		{
			typ:          types.TypeSecret,
			source:       "l1:anthropic-key",
			score:        0.99,
			re:           regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}\b`),
			keywords:     []string{"sk-ant-"},
			validateSpan: validSecretSpan,
		},

		// Anthropic API/Admin key variants with the current precise prefixes.
		{
			typ:          types.TypeSecret,
			source:       "l1:anthropic-api-key-v3",
			score:        0.99,
			re:           regexp.MustCompile(`\bsk-ant-(?:api03|admin01)-[A-Za-z0-9_-]{80,}AA\b`),
			keywords:     []string{"sk-ant-api03-", "sk-ant-admin01-"},
			validateSpan: validSecretSpan,
		},

		// Google API keys.
		{
			typ:          types.TypeSecret,
			source:       "l1:google-api-key",
			score:        0.99,
			re:           regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`),
			keywords:     []string{"aiza"},
			validateSpan: validSecretSpan,
		},

		// Slack tokens.
		{
			typ:          types.TypeSecret,
			source:       "l1:slack-token",
			score:        0.99,
			re:           regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`),
			keywords:     []string{"xox"},
			validateSpan: validSecretSpan,
		},

		// Stripe secret and restricted keys. Publishable pk_ keys are intentionally
		// not masked by this rule.
		{
			typ:          types.TypeSecret,
			source:       "l1:stripe-key",
			score:        0.99,
			re:           regexp.MustCompile(`\b(?:sk|rk)_(?:live|test)_[0-9A-Za-z]{16,}\b`),
			keywords:     []string{"sk_live_", "sk_test_", "rk_live_", "rk_test_"},
			validateSpan: validSecretSpan,
		},

		// JWTs are common bearer tokens in coding-agent tool output.
		{
			typ:          types.TypeSecret,
			source:       "l1:jwt",
			score:        0.95,
			re:           regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`),
			keywords:     []string{"eyj", "bearer", "jwt", "authorization"},
			validateSpan: validSecretSpan,
		},

		// Bearer tokens in Authorization-style contexts. Capture only the value,
		// not the "Bearer" scheme marker.
		{
			typ:          types.TypeSecret,
			source:       "l1:bearer-token",
			score:        0.90,
			re:           regexp.MustCompile(`(?i)\bBearer\s+([A-Za-z0-9._~+/=-]{40,})\b`),
			keywords:     []string{"bearer"},
			group:        1,
			validateSpan: validSecretSpan,
		},

		// Embedded credentials in database/cache connection strings. Capture only
		// the password component so the surrounding URL remains useful to the model.
		{
			typ:          types.TypeSecret,
			source:       "l1:connection-string-password",
			score:        0.99,
			re:           regexp.MustCompile(`(?i)\b(?:postgres(?:ql)?|mysql|mariadb|mongodb(?:\+srv)?|redis|amqps?)://[^:\s/'"]+:([^@\s'"]{4,})@`),
			keywords:     []string{"postgres://", "postgresql://", "mysql://", "mariadb://", "mongodb://", "mongodb+srv://", "redis://", "amqp://", "amqps://"},
			group:        1,
			validateSpan: validSecretSpan,
		},

		// AWS secret access keys are only masked in assignment/header context to
		// avoid treating every base64-looking 40-byte string as a credential.
		{
			typ:          types.TypeSecret,
			source:       "l1:aws-secret-access-key",
			score:        0.95,
			re:           regexp.MustCompile(`(?i)\b(?:aws_)?secret[_\s-]?access[_\s-]?key\b\s*(?:[:=]|=>)\s*['"]?([A-Za-z0-9/+=]{40})`),
			keywords:     []string{"secret_access_key", "secret access key", "secret-access-key", "aws_secret", "aws-secret"},
			group:        1,
			validateSpan: validSecretSpan,
		},

		// AWS session tokens are long base64-like values and are only masked in
		// explicit assignment context.
		{
			typ:          types.TypeSecret,
			source:       "l1:aws-session-token",
			score:        0.95,
			re:           regexp.MustCompile(`(?i)\b(?:aws_)?session[_\s-]?token\b\s*(?:[:=]|=>)\s*['"]?([A-Za-z0-9/+=]{80,})`),
			keywords:     []string{"session_token", "session token", "session-token", "aws_session", "aws-session"},
			group:        1,
			validateSpan: validSecretSpan,
		},

		// Azure Storage connection strings carry the account key as AccountKey=...
		// among semicolon-separated fields; capture only the value.
		{
			typ:          types.TypeSecret,
			source:       "l1:azure-storage-account-key",
			score:        0.95,
			re:           regexp.MustCompile(`(?i)\bAccountKey=([A-Za-z0-9+/=]{40,})`),
			keywords:     []string{"accountkey="},
			group:        1,
			validateSpan: validSecretSpan,
		},

		// Conservative generic assignment fallback. It masks only the value side
		// of obvious credential assignments and relies on post-match suppressors
		// to avoid placeholders, UUID/hash literals, URLs, and business IDs.
		{
			typ: types.TypeSecret, source: "l1:secret-assignment", score: 0.80,
			re: regexp.MustCompile(`(?i)\b(?:api[_\s-]?key|secret|token|password|passwd|pwd|credential|authorization)\b\s*(?:[:=]|=>)\s*['"]?([A-Za-z0-9][A-Za-z0-9._~+/=\-]{7,})`),
			keywords: []string{
				"api_key", "api-key", "api key", "apikey", "secret", "token", "password",
				"passwd", "pwd", "credential", "authorization",
			},
			group: 1,
			validate: func(match string) bool {
				if len(match) <= 16 && shannonEntropy(match) < 3.0 {
					return false
				}
				return true
			},
			validateSpan: validGenericSecretSpan,
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

var (
	templateVarRe = regexp.MustCompile(`^(?:\{\{[^{}]+\}\}|\$\{[^{}]+\}|%\{[^{}]+\}|<[^<>]+>)$`)
	uuidRe        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	hexOnlyRe     = regexp.MustCompile(`^[0-9a-fA-F]+$`)
	hostPrefixRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z0-9-]+:`)
	constIdentRe  = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,}$`)
	codeRefRe     = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(?:\.[A-Za-z_$][A-Za-z0-9_$]*)+$`)
)

var placeholderFragments = []string{
	"REPLACE_ME", "REPLACE_THIS", "REPLACE_WITH",
	"YOUR_KEY", "YOUR_TOKEN", "YOUR_SECRET", "YOUR_API_KEY", "YOUR_PASSWORD",
	"INSERT_HERE", "INSERT_KEY", "INSERT_TOKEN",
	"PLACEHOLDER", "EXAMPLE_KEY", "EXAMPLE_TOKEN",
	"TODO", "FIXME", "XXXX",
}

var benignIDNameSuffixes = []string{"_id", "_uuid", "_uid", "_oid", "_no", "_seq"}

func validSecretSpan(text string, start, end int) bool {
	if start < 0 || end <= start || end > len(text) {
		return false
	}
	cand := strings.TrimSpace(text[start:end])
	if cand == "" {
		return false
	}
	if isLikelyPlaceholder(cand) || isTemplateVar(cand) || isPublicToken(cand) ||
		isUUID(cand) || hasJSONNoise(cand) {
		return false
	}
	if looksLikeURLMatch(cand) {
		return false
	}
	return true
}

func validGenericSecretSpan(text string, start, end int) bool {
	if !validSecretSpan(text, start, end) {
		return false
	}
	cand := strings.TrimSpace(text[start:end])
	if isHexHash(cand) && !isStrongSecretAssignmentContext(text, start) {
		return false
	}
	if isBusinessIDContext(text, start) {
		return false
	}
	// Code references are not literal credentials; masking them corrupts the
	// source code coding agents send (the model then sees a CLK_ token in place
	// of an identifier). A dotted identifier path (process.env.API_KEY) or a value
	// immediately followed by '(' (a function call such as parseToken()) is code,
	// not a secret. This mirrors the suppression already applied on the entropy
	// path (validEntropySecretSpan). Hex/base64 secrets never match either shape,
	// so the leak fixes above are unaffected.
	if isCodeReference(cand) {
		return false
	}
	if end < len(text) && text[end] == '(' {
		return false
	}
	return true
}

func validEntropySecretSpan(text string, start, end int) bool {
	if !validGenericSecretSpan(text, start, end) {
		return false
	}
	if isURLOrPathBoundary(text, start, end) {
		return false
	}
	if isAssignmentNameContext(text, start, end) {
		return false
	}
	if isConstantIdentifier(text[start:end]) {
		return false
	}
	if isCodeReference(text[start:end]) {
		return false
	}
	return true
}

func validPhoneSpan(text string, start, end int) bool {
	if start < 0 || end <= start || end > len(text) {
		return false
	}
	cand := strings.TrimSpace(text[start:end])
	if cand == "" {
		return false
	}
	digits := stripNonDigit(cand)
	if len(digits) == 11 {
		if digits[0] != '1' {
			return false
		}
	} else if len(digits) != 10 {
		return false
	}
	if hasTokenBoundary(text, start, end) {
		return false
	}
	if isLikelyCodeTokenAround(text, start, end) {
		return false
	}
	if isPlainDigitPhone(cand) && !hasPhoneKeywordContext(text, start) {
		return false
	}
	return true
}

func isLikelyPlaceholder(s string) bool {
	upper := strings.ToUpper(s)
	for _, p := range placeholderFragments {
		if strings.Contains(upper, p) {
			return true
		}
	}
	return false
}

func isPublicToken(s string) bool {
	return strings.HasPrefix(s, "pk_live_") || strings.HasPrefix(s, "pk_test_")
}

func isConstantIdentifier(s string) bool {
	return strings.Contains(s, "_") && constIdentRe.MatchString(s)
}

func isCodeReference(s string) bool {
	return codeRefRe.MatchString(s)
}

func isTemplateVar(s string) bool {
	return templateVarRe.MatchString(s)
}

func isUUID(s string) bool {
	return uuidRe.MatchString(s)
}

func isHexHash(s string) bool {
	n := len(s)
	return (n == 32 || n == 40 || n == 64) && hexOnlyRe.MatchString(s)
}

func hasJSONNoise(s string) bool {
	return strings.IndexByte(s, ',') >= 0
}

func looksLikeURLMatch(s string) bool {
	return strings.Contains(s, "://") || hostPrefixRe.MatchString(s)
}

func isAssignmentNameContext(text string, start, end int) bool {
	if start < 0 || end <= start || end > len(text) {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(text[start:end]))
	if name == "" || !strings.ContainsAny(name, "_-.") {
		return false
	}
	hasSensitiveName := false
	for _, sensitive := range []string{"key", "secret", "token", "password", "passwd", "pwd", "credential", "authorization"} {
		if strings.Contains(name, sensitive) {
			hasSensitiveName = true
			break
		}
	}
	if !hasSensitiveName {
		return false
	}
	for i := end; i < len(text); i++ {
		switch text[i] {
		case ' ', '\t', '\r', '\n', '\'', '"', '`':
			continue
		case '=', ':':
			return true
		}
		return false
	}
	return false
}

func hasTokenBoundary(text string, start, end int) bool {
	if start > 0 && isIdentifierByte(text[start-1]) {
		return true
	}
	if end < len(text) && isIdentifierByte(text[end]) {
		return true
	}
	return false
}

func isLikelyCodeTokenAround(text string, start, end int) bool {
	left := start
	for left > 0 && isCodeTokenByte(text[left-1]) {
		left--
	}
	right := end
	for right < len(text) && isCodeTokenByte(text[right]) {
		right++
	}
	if left == start && right == end {
		return false
	}
	token := text[left:right]
	if isUUID(token) {
		return true
	}
	compact := strings.NewReplacer("-", "", "_", "").Replace(token)
	return len(compact) >= 16 && hexOnlyRe.MatchString(compact)
}

func isPlainDigitPhone(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func hasPhoneKeywordContext(text string, start int) bool {
	lo := start - 48
	if lo < 0 {
		lo = 0
	}
	prefix := strings.ToLower(text[lo:start])
	for _, kw := range []string{"phone", "call", "tel", "mobile", "sms", "fax", "contact"} {
		if strings.Contains(prefix, kw) {
			return true
		}
	}
	return false
}

func isIdentifierByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isCodeTokenByte(b byte) bool {
	return b == '-' || isIdentifierByte(b)
}

func isURLOrPathBoundary(text string, start, end int) bool {
	if start > 0 {
		switch text[start-1] {
		case '/', '\\', ':', '.', '@', '?':
			return true
		}
	}
	if end < len(text) {
		switch text[end] {
		case '/', '\\', ':', '.', '@', '?':
			return true
		}
	}
	lo := start - 16
	if lo < 0 {
		lo = 0
	}
	return strings.Contains(text[lo:start], "://")
}

func isBusinessIDContext(text string, valueStart int) bool {
	name := assignmentNameBeforeValue(text, valueStart)
	if name == "" {
		return false
	}
	for _, sensitive := range []string{"key", "secret", "token", "auth", "password", "credential"} {
		if strings.Contains(name, sensitive) {
			return false
		}
	}
	for _, suffix := range benignIDNameSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func isStrongSecretAssignmentContext(text string, valueStart int) bool {
	name := assignmentNameBeforeValue(text, valueStart)
	if name == "" {
		return false
	}
	for _, sensitive := range []string{"secret", "password", "passwd", "pwd", "credential", "authorization"} {
		if strings.Contains(name, sensitive) {
			return true
		}
	}
	return false
}

func assignmentNameBeforeValue(text string, valueStart int) string {
	lo := valueStart - 80
	if lo < 0 {
		lo = 0
	}
	prefix := text[lo:valueStart]
	eq := strings.LastIndexAny(prefix, "=:")
	if eq < 0 {
		return ""
	}
	name := strings.ToLower(strings.Trim(prefix[:eq], " \t\r\n\"'`{}[],"))
	return name
}
