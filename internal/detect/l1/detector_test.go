package l1

import (
	"context"
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

var det = New()
var ctx = context.Background()

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

func TestURLNegative(t *testing.T) {
	findings, err := det.Detect(ctx, "just plain text no links")
	if err != nil {
		t.Fatal(err)
	}
	if hasType(findings, types.TypeURL) {
		t.Fatal("unexpected URL finding")
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
