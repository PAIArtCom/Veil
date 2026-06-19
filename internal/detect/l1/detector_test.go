package l1

import (
	"context"
	"strings"
	"testing"

	"github.com/cloakia/opencloak/internal/types"
)

func findingsByType(findings []types.Finding, typ types.Type) []types.Finding {
	var out []types.Finding
	for _, f := range findings {
		if f.Type == typ {
			out = append(out, f)
		}
	}
	return out
}

func hasType(findings []types.Finding, typ types.Type) bool {
	return len(findingsByType(findings, typ)) > 0
}

func hasSource(findings []types.Finding, source string) bool {
	for _, f := range findings {
		if f.Source == source {
			return true
		}
	}
	return false
}

var det = New()
var ctx = context.Background()

func TestRuleKeywordsAreLowercase(t *testing.T) {
	for _, r := range buildRules() {
		for _, kw := range r.keywords {
			if kw != strings.ToLower(kw) {
				t.Fatalf("rule %s has non-lowercase keyword %q", r.source, kw)
			}
		}
	}
}

// ---- Email ----

func TestEmailPositive(t *testing.T) {
	findings, err := det.Detect(ctx, "send it to user@example.com please")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypeEmail) {
		t.Fatal("expected EMAIL finding")
	}
}

func TestEmailNegative(t *testing.T) {
	findings, err := det.Detect(ctx, "no email here just text")
	if err != nil {
		t.Fatal(err)
	}
	if hasType(findings, types.TypeEmail) {
		t.Fatal("unexpected EMAIL finding")
	}
}

// ---- IPv4 ----

func TestIPv4Positive(t *testing.T) {
	findings, err := det.Detect(ctx, "connect to 192.168.1.100")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypeIPv4) {
		t.Fatal("expected IPV4 finding")
	}
}

func TestIPv4Negative(t *testing.T) {
	findings, err := det.Detect(ctx, "version 1.2.3 released today")
	if err != nil {
		t.Fatal(err)
	}
	// 1.2.3 has only 3 octets — should not match.
	if hasType(findings, types.TypeIPv4) {
		t.Fatal("unexpected IPV4 finding for version string")
	}
}

// ---- IPv6 ----

func TestIPv6Positive(t *testing.T) {
	for _, addr := range []string{
		"connect to 2001:db8:85a3::8a2e:370:7334",
		"link-local fe80::1 here",
		"prefix 2001:db8:: end",
	} {
		findings, err := det.Detect(ctx, addr)
		if err != nil {
			t.Fatal(err)
		}
		if !hasType(findings, types.TypeIPv6) {
			t.Fatalf("expected IPV6 finding in %q", addr)
		}
	}
}

// TestIPv6Negative guards against the false positives the left boundary and the
// hextet guard exist to prevent: language scope/path syntax ("::") and bare tiny
// compressed forms that are ambiguous with code and low value. Masking any of
// these would corrupt the source code this tool is meant to protect. The
// scoped-identifier cases (where the regex would otherwise grab a hex *suffix* of
// the preceding word, e.g. "namespace::func" -> "ace::f") are the regression
// found by independent (Codex) audit.
func TestIPv6Negative(t *testing.T) {
	for _, src := range []string{
		"not an address: abc:def",
		"std::vector<int> v;",
		"use foo::bar::Baz;",
		"Class::method();",
		"namespace a::b { }",
		"result = crate::module::run();",
		"loopback ::1 only",
		// suffix-grab regression: hex tail of an identifier before "::".
		"namespace::func",
		"resolveNamespace::fetchConfig",
		"interface::method",
		"trace::span()",
		"replace::all",
		"DataFace::render",
		"grpc::Status code",
		"absl::string_view s",
		"0:0:0:0:0:0:0:1 loopback long form",
	} {
		findings, err := det.Detect(ctx, src)
		if err != nil {
			t.Fatal(err)
		}
		if hasType(findings, types.TypeIPv6) {
			for _, f := range findings {
				if f.Type == types.TypeIPv6 {
					t.Errorf("unexpected IPV6 finding %q in %q", src[f.Start:f.End], src)
				}
			}
		}
	}
}

// ---- Credit Card / Luhn ----

func TestCardPositiveLuhn(t *testing.T) {
	// Visa test number that passes Luhn.
	findings, err := det.Detect(ctx, "card: 4532015112830366")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypeCard) {
		t.Fatal("expected CARD finding for Luhn-valid card number")
	}
}

func TestCardNegativeLuhn(t *testing.T) {
	// Same length but fails Luhn.
	findings, err := det.Detect(ctx, "card: 4532015112830367")
	if err != nil {
		t.Fatal(err)
	}
	if hasType(findings, types.TypeCard) {
		t.Fatal("unexpected CARD finding for Luhn-invalid number")
	}
}

// ---- Account identifiers ----

func TestIBANPositive(t *testing.T) {
	findings, err := det.Detect(ctx, "iban GB82WEST12345698765432")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypeAcct) {
		t.Fatal("expected ACCT finding for valid IBAN")
	}
}

func TestIBANNegative(t *testing.T) {
	findings, err := det.Detect(ctx, "iban GB82TEST12345698765432")
	if err != nil {
		t.Fatal(err)
	}
	if hasType(findings, types.TypeAcct) {
		t.Fatal("unexpected ACCT finding for invalid IBAN")
	}
}

// ---- Phone ----

func TestPhonePositive(t *testing.T) {
	findings, err := det.Detect(ctx, "call me at 555-867-5309")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypePhone) {
		t.Fatal("expected PHONE finding")
	}
}

func TestPhoneNegative(t *testing.T) {
	findings, err := det.Detect(ctx, "no phone numbers in this sentence whatsoever")
	if err != nil {
		t.Fatal(err)
	}
	if hasType(findings, types.TypePhone) {
		t.Fatal("unexpected PHONE finding")
	}
}

func TestPhoneNegativeCodeTokens(t *testing.T) {
	for _, src := range []string{
		`OrderID: "550e8400-e29b-41d4-a716-446655440000"`,
		`TraceID: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"`,
		`const BUILD_5558675309 = true`,
	} {
		findings, err := det.Detect(ctx, src)
		if err != nil {
			t.Fatal(err)
		}
		if hasType(findings, types.TypePhone) {
			t.Fatalf("unexpected PHONE finding in code token payload %q: %+v", src, findings)
		}
	}
}

func TestPhonePositivePlainDigitsWithContext(t *testing.T) {
	findings, err := det.Detect(ctx, "call 5558675309 after lunch")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypePhone) {
		t.Fatal("expected PHONE finding for plain digits with call context")
	}
}

// ---- URL ----

func TestURLPositive(t *testing.T) {
	findings, err := det.Detect(ctx, "see https://example.com/path?q=1 for details")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypeURL) {
		t.Fatal("expected URL finding")
	}
}

func TestConnectionStringPositive(t *testing.T) {
	findings, err := det.Detect(ctx, "dsn postgres://admin:hunter2@db.example.com/prod")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypeURL) {
		t.Fatal("expected URL finding for postgres connection string")
	}
}

func TestURLNegative(t *testing.T) {
	findings, err := det.Detect(ctx, "just plain text no links")
	if err != nil {
		t.Fatal(err)
	}
	if hasType(findings, types.TypeURL) {
		t.Fatal("unexpected URL finding")
	}
}

// ---- Date ----

func TestDatePositive(t *testing.T) {
	findings, err := det.Detect(ctx, "expires on 2026-06-16")
	if err != nil {
		t.Fatal(err)
	}
	if !hasType(findings, types.TypeDate) {
		t.Fatal("expected DATE finding")
	}
}

func TestDateNegativeInvalidCalendarDate(t *testing.T) {
	findings, err := det.Detect(ctx, "not a real date 2026-02-31")
	if err != nil {
		t.Fatal(err)
	}
	if hasType(findings, types.TypeDate) {
		t.Fatal("unexpected DATE finding for invalid calendar date")
	}
}

// ---- AWS access key ----

func TestAWSPositive(t *testing.T) {
	findings, err := det.Detect(ctx, "key=AKIAIOSFODNN7EXAMPLE")
	if err != nil {
		t.Fatal(err)
	}
	secrets := findingsByType(findings, types.TypeSecret)
	found := false
	for _, f := range secrets {
		if f.Source == "l1:aws-access-key-id" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected aws-access-key-id finding")
	}
}

func TestAWSNegative(t *testing.T) {
	findings, err := det.Detect(ctx, "AKEY1234 is not an AWS key")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Source == "l1:aws-access-key-id" {
			t.Fatal("unexpected aws-access-key-id finding")
		}
	}
}

// ---- GitHub PAT ----

func TestGitHubPATPositive(t *testing.T) {
	findings, err := det.Detect(ctx, "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Source == "l1:github-pat" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected github-pat finding")
	}
}

func TestGitHubPATNegative(t *testing.T) {
	findings, err := det.Detect(ctx, "ghx_tooshort")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Source == "l1:github-pat" {
			t.Fatal("unexpected github-pat finding for short token")
		}
	}
}

// ---- OpenAI key ----

func TestOpenAIKeyPositive(t *testing.T) {
	findings, err := det.Detect(ctx, "export OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz12345")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Source == "l1:openai-key" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected openai-key finding")
	}
}

func TestProviderSecretRulesPositive(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		source string
	}{
		{
			name:   "anthropic",
			text:   "ANTHROPIC_API_KEY=sk-ant-abcdefghijklmnopqrstuvwxyz123456",
			source: "l1:anthropic-key",
		},
		{
			name:   "anthropic v3",
			text:   "ANTHROPIC_API_KEY=sk-ant-api03-" + strings.Repeat("A", 80) + "AA",
			source: "l1:anthropic-api-key-v3",
		},
		{
			name:   "1password secret key",
			text:   "OP_SECRET_KEY=A3-ABCDEF-ABCDEFGHIJK-ABCDE-ABCDE-ABCDE",
			source: "l1:1password-secret-key",
		},
		{
			name:   "1password service account token",
			text:   "OP_SERVICE_ACCOUNT_TOKEN=ops_eyJ" + strings.Repeat("A", 120),
			source: "l1:1password-service-account-token",
		},
		{
			name:   "age secret key",
			text:   "AGE_KEY=AGE-SECRET-KEY-1" + strings.Repeat("Q", 58),
			source: "l1:age-secret-key",
		},
		{
			name:   "alibaba access key id",
			text:   "ALIBABA_ACCESS_KEY_ID=LTAIABCDEFGHIJKLMNOPQRST",
			source: "l1:alibaba-access-key-id",
		},
		{
			name:   "aws temporary access key id",
			text:   "AWS_ACCESS_KEY_ID=ASIAIOSFODNN7EXAMPLE",
			source: "l1:aws-access-key-id",
		},
		{
			name:   "aws bedrock api key",
			text:   "BEDROCK_API_KEY=ABSK" + strings.Repeat("A", 109),
			source: "l1:aws-bedrock-api-key",
		},
		{
			name:   "azure ad client secret",
			text:   "AZURE_CLIENT_SECRET=abc1Q~" + strings.Repeat("A", 31),
			source: "l1:azure-ad-client-secret",
		},
		{
			name:   "cloudflare origin ca key",
			text:   "CF_ORIGIN_CA_KEY=v1.0-" + strings.Repeat("a", 24) + "-" + strings.Repeat("b", 146),
			source: "l1:cloudflare-origin-ca-key",
		},
		{
			name:   "defined networking token",
			text:   "DN_TOKEN=dnkey-" + strings.Repeat("A", 26) + "-" + strings.Repeat("B", 52),
			source: "l1:defined-networking-token",
		},
		{
			name:   "digitalocean pat",
			text:   "DIGITALOCEAN_TOKEN=dop_v1_" + strings.Repeat("a", 64),
			source: "l1:digitalocean-token",
		},
		{
			name:   "github fine grained pat",
			text:   "GITHUB_TOKEN=github_pat_" + strings.Repeat("A", 82),
			source: "l1:github-fine-grained-pat",
		},
		{
			name:   "gitlab",
			text:   "GITLAB_TOKEN=glpat-abcdefghijklmnopqrstuvwx123456",
			source: "l1:gitlab-pat",
		},
		{
			name:   "gitlab pat in trace id field",
			text:   "trace_id=glpat-1234567890abcdef1234",
			source: "l1:gitlab-pat",
		},
		{
			name:   "github pat in request id field",
			text:   "request_id=ghp_" + strings.Repeat("A", 36),
			source: "l1:github-pat",
		},
		{
			name:   "gitlab runner token",
			text:   "GITLAB_RUNNER_TOKEN=glrt-" + strings.Repeat("A", 20),
			source: "l1:gitlab-token",
		},
		{
			name:   "huggingface token",
			text:   "HF_TOKEN=hf_" + strings.Repeat("a", 34),
			source: "l1:huggingface-token",
		},
		{
			name:   "infracost token",
			text:   "INFRACOST_API_KEY=ico-" + strings.Repeat("A", 32),
			source: "l1:infracost-token",
		},
		{
			name:   "linear api key",
			text:   "LINEAR_API_KEY=lin_api_" + strings.Repeat("A", 40),
			source: "l1:linear-api-key",
		},
		{
			name:   "npm",
			text:   "NPM_TOKEN=npm_0123456789abcdefghijklmnopqrstuvwxyz",
			source: "l1:npm-token",
		},
		{
			name:   "pypi",
			text:   "TWINE_PASSWORD=pypi-AgEIcHlwaS5vcmcCJDExMTExMTExMTEx",
			source: "l1:pypi-token",
		},
		{
			name:   "google",
			text:   "GOOGLE_API_KEY=AIzaSyD-1234567890abcdefghijklmnoQRSTUV",
			source: "l1:google-api-key",
		},
		{
			name:   "slack",
			text:   "SLACK_BOT_TOKEN=xoxb-123456789012-ABCDEFGHIJKL",
			source: "l1:slack-token",
		},
		{
			name:   "stripe",
			text:   "STRIPE_SECRET_KEY=sk_live_1234567890abcdefghijklmnop",
			source: "l1:stripe-key",
		},
		{
			name:   "typeform token",
			text:   "TYPEFORM_TOKEN=tfp_" + strings.Repeat("A", 59),
			source: "l1:typeform-token",
		},
		{
			name:   "vault token",
			text:   "VAULT_TOKEN=hvs." + strings.Repeat("A", 90),
			source: "l1:vault-token",
		},
		{
			name:   "jwt",
			text:   "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			source: "l1:jwt",
		},
		{
			name:   "bearer token",
			text:   "Authorization: Bearer abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMN1234567890",
			source: "l1:bearer-token",
		},
		{
			name:   "aws secret access key",
			text:   "AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
			source: "l1:aws-secret-access-key",
		},
		{
			name:   "dash aws secret access key",
			text:   "Secret-Access-Key: wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
			source: "l1:aws-secret-access-key",
		},
		{
			name:   "aws session token",
			text:   "AWS_SESSION_TOKEN=FwoGZXIvYXdzEJr//////////wEaDEaK1ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyzABCD",
			source: "l1:aws-session-token",
		},
		{
			name:   "azure storage account key",
			text:   "DefaultEndpointsProtocol=https;AccountName=devstore;AccountKey=abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/==;EndpointSuffix=core.windows.net",
			source: "l1:azure-storage-account-key",
		},
		{
			name:   "connection string password",
			text:   "DATABASE_URL=postgres://admin:hunter2@db.example.com/prod",
			source: "l1:connection-string-password",
		},
		{
			name:   "generic password assignment",
			text:   "password = V3ryR4nd0mSecret",
			source: "l1:secret-assignment",
		},
		{
			name:   "generic uppercase underscore secret assignment",
			text:   "password=PROD_DB_BACKUP_TOKEN_ABC123XYZ789",
			source: "l1:secret-assignment",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := det.Detect(ctx, tc.text)
			if err != nil {
				t.Fatal(err)
			}
			if !hasSource(findings, tc.source) {
				t.Fatalf("expected %s finding in %q; got %+v", tc.source, tc.text, findings)
			}
		})
	}
}

func TestSecretReviewRegressionDetections(t *testing.T) {
	cases := []string{
		"webhook_secret=4f3c2b1a9e8d7c6b5a4f3c2b1a9e8d7c",
	}

	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			findings, err := det.Detect(ctx, text)
			if err != nil {
				t.Fatal(err)
			}
			if !hasType(findings, types.TypeSecret) {
				t.Fatalf("expected SECRET finding in %q; got %+v", text, findings)
			}
		})
	}
}

func TestSecretSuppressorsNegative(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{
			name: "placeholder",
			text: "api_key=YOUR_API_KEY",
		},
		{
			name: "template variable",
			text: "token=${SERVICE_TOKEN}",
		},
		{
			name: "uuid business id",
			text: "order_id=550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "sha256 hash",
			text: "token=2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			name: "url value",
			text: "token=https://example.com/path",
		},
		{
			name: "stripe publishable key",
			text: "STRIPE_PUBLISHABLE_KEY=pk_live_1234567890abcdefghijklmnop",
		},
		{
			name: "aws session token template",
			text: "AWS_SESSION_TOKEN=${AWS_SESSION_TOKEN}",
		},
		{
			name: "session token url value",
			text: "session_token=https://metadata.google.internal/computeMetadata/v1",
		},
		{
			name: "azure account key placeholder",
			text: "DefaultEndpointsProtocol=https;AccountName=devstore;AccountKey=REPLACE_ME;EndpointSuffix=core.windows.net",
		},
		{
			name: "npm package variable",
			text: "npm_package_name=npm_opencloak",
		},
		{
			name: "pypi package URL",
			text: "pypi-url=https://pypi.org/project/opencloak/",
		},
		{
			name: "short bearer token",
			text: "Authorization: Bearer short-token",
		},
		{
			name: "connection string without password",
			text: "DATABASE_URL=postgres://admin@db.example.com/prod",
		},
		{
			name: "short 1password service token",
			text: "OP_SERVICE_ACCOUNT_TOKEN=ops_eyJshort",
		},
		{
			name: "age secret placeholder",
			text: "AGE_KEY=AGE-SECRET-KEY-${AGE_SECRET_KEY}",
		},
		{
			name: "cloudflare broad context hex not migrated",
			text: "cloudflare_api_key=" + strings.Repeat("a", 40),
		},
		{
			name: "process env secret reference",
			text: "const keyRef = process.env.API_KEY",
		},
		{
			name: "digitalocean invalid hex",
			text: "dop_v1_" + strings.Repeat("g", 64),
		},
		{
			name: "github fine grained short",
			text: "github_pat_short",
		},
		{
			name: "gitlab explicit prefix short",
			text: "glrt-short",
		},
		{
			name: "huggingface short token",
			text: "hf_short",
		},
		{
			name: "linear api key short",
			text: "lin_api_" + strings.Repeat("A", 39),
		},
		{
			name: "legacy vault short token not migrated",
			text: "s." + strings.Repeat("a", 24),
		},
		{
			name: "vault token short",
			text: "hvs." + strings.Repeat("A", 24),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := det.Detect(ctx, tc.text)
			if err != nil {
				t.Fatal(err)
			}
			if hasType(findings, types.TypeSecret) {
				t.Fatalf("unexpected SECRET finding in %q: %+v", tc.text, findings)
			}
		})
	}
}

// ---- PEM ----

func TestPEMPositive(t *testing.T) {
	findings, err := det.Detect(ctx, "-----BEGIN RSA PRIVATE KEY-----\nMIIE...")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Source == "l1:pem-private-key" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected pem-private-key finding")
	}
}

// ---- Entropy ----

func TestEntropyTruePositiveNearKeyword(t *testing.T) {
	// A high-entropy string near the word "secret" should be flagged.
	highEntropy := "aB3xQ9mK2pL7nR4sT6" // mixed chars, high entropy
	text := "secret=" + highEntropy
	findings, err := det.Detect(ctx, text)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range findings {
		if f.Source == "l1:entropy:contextual" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected entropy:contextual finding in %q; got %v", text, findings)
	}
}

func TestEntropyFalsePositiveBase64NoKeyword(t *testing.T) {
	// A base64 blob with no keyword context should NOT be flagged.
	text := "the checksum is dGhpcyBpcyBhIGJhc2U2NCBibG9i and nothing else"
	findings, err := det.Detect(ctx, text)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Source == "l1:entropy:contextual" {
			t.Fatalf("unexpected entropy finding for isolated base64 blob: %+v", f)
		}
	}
}

func TestEntropyFalsePositiveLowEntropy(t *testing.T) {
	// A repetitive string near a keyword should not be flagged (low entropy).
	text := "token=aaaaaaaaaaaaaaaaaaaaaa"
	findings, err := det.Detect(ctx, text)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Source == "l1:entropy:contextual" {
			t.Fatalf("unexpected entropy finding for low-entropy string: %+v", f)
		}
	}
}

func TestEntropyFalsePositiveAssignmentNameConstant(t *testing.T) {
	text := "OPENAI_API_KEY = sk-abcdefghijklmnopqrstuvwxyz12345"
	findings, err := det.Detect(ctx, text)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		if f.Source == "l1:entropy:contextual" && text[f.Start:f.End] == "OPENAI_API_KEY" {
			t.Fatalf("unexpected entropy finding for assignment name: %+v", f)
		}
	}
}

// ---- Luhn unit tests ----

func TestLuhnValid(t *testing.T) {
	cases := []struct {
		digits string
		valid  bool
	}{
		{"4532015112830366", true},  // Visa test
		{"4532015112830367", false}, // off by one
		{"79927398713", true},       // canonical example
		{"79927398710", false},
		{"1234567812345670", true},
		{"1234567812345671", false},
	}
	for _, c := range cases {
		if got := luhnValid(c.digits); got != c.valid {
			t.Errorf("luhnValid(%q) = %v, want %v", c.digits, got, c.valid)
		}
	}
}

// TestGenericSecretRuleDoesNotMaskCodeReferences locks the fix for over-masking
// code identifiers on the generic-assignment path: a dotted property path
// (process.env.API_KEY) or a value immediately followed by '(' (a function call
// like parseToken()) is source code, not a credential, and masking it would
// corrupt the code a coding agent sends to the model. The companion leak cases
// (a real secret in the same assignment shape) must still be detected.
func TestGenericSecretRuleDoesNotMaskCodeReferences(t *testing.T) {
	codeRefs := []string{
		"const token = parseToken(req.headers.authorization)",
		"api_key: process.env.API_KEY",
		"secret = config.get('db')",
		"password = os.environ.SECRET",
	}
	for _, src := range codeRefs {
		findings, err := det.Detect(ctx, src)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range findings {
			if f.Type == types.TypeSecret {
				t.Errorf("over-mask: %q masked code reference %q as SECRET", src, src[f.Start:f.End])
			}
		}
	}
	// Real secrets in the same assignment shape must still be masked (no FN regression).
	for _, src := range []string{
		"api_key = ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		"webhook_secret = 4f3c2b1a9e8d7c6b5a4f3e2d1c0b9a8f",
	} {
		findings, err := det.Detect(ctx, src)
		if err != nil {
			t.Fatal(err)
		}
		if !hasType(findings, types.TypeSecret) {
			t.Errorf("regression: real secret in %q not detected", src)
		}
	}
}
