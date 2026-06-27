package mask

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PAIArtCom/Veil/internal/mapstore"
	"github.com/PAIArtCom/Veil/internal/token"
	"github.com/PAIArtCom/Veil/internal/types"
)

func TestRestoreKnownSurrogatesDelimitedCandidates(t *testing.T) {
	const (
		email     = "user-111111111111@veil.paiart.com"
		emailExt  = "user-111111111111aaaa@veil.paiart.com"
		url       = "https://api-222222222222.veil.paiart.com/v1?token=value-222222222222"
		urlDelims = "https://api-333333333333.veil.paiart.com/a;b,c?token=value-333333333333"
		ipv4      = "10.12.34.56"
		ipv6      = "2001:db8:12:34::56"
		unknown   = "user-deadbeefcafe@veil.paiart.com"
	)
	lookup := func(candidate string) (string, bool) {
		table := map[string]string{
			email:     "ops@example.com",
			emailExt:  "extended@example.com",
			url:       "https://api.example.com/v1?token=abc123",
			urlDelims: "https://api.example.com/a;b,c?token=abc123",
			ipv4:      "10.20.30.40",
			ipv6:      "2606:4700:4700::1111",
		}
		restored, ok := table[candidate]
		return restored, ok
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "pipe-delimited-known-surrogates",
			in:   email + "|" + url + "|" + ipv4 + "|" + ipv6,
			want: "ops@example.com|https://api.example.com/v1?token=abc123|10.20.30.40|2606:4700:4700::1111",
		},
		{
			name: "comma-and-semicolon-delimited-known-surrogates",
			in:   email + "," + ipv4 + ";" + ipv6,
			want: "ops@example.com,10.20.30.40;2606:4700:4700::1111",
		},
		{
			name: "known-and-unknown-mixed",
			in:   url + "|" + unknown + "|" + email,
			want: "https://api.example.com/v1?token=abc123|" + unknown + "|ops@example.com",
		},
		{
			name: "url-containing-delimiter-characters",
			in:   urlDelims + "|" + email,
			want: "https://api.example.com/a;b,c?token=abc123|ops@example.com",
		},
		{
			name: "collision-extended-email-surrogate",
			in:   emailExt,
			want: "extended@example.com",
		},
		{
			name: "unknown-only-left-unchanged",
			in:   unknown,
			want: unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RestoreKnownSurrogates(tt.in, lookup); got != tt.want {
				t.Fatalf("RestoreKnownSurrogates() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRestoreKnownPlaceholdersDoesNotRescanRestoredValues(t *testing.T) {
	const (
		tok       = "PAIArtVeil_SECRET_111111111111"
		surrogate = "user-222222222222@veil.paiart.com"
	)
	lookup := func(candidate string) (string, bool) {
		table := map[string]string{
			tok:       surrogate,
			surrogate: "ops@example.com",
		}
		restored, ok := table[candidate]
		return restored, ok
	}
	got := RestoreKnownPlaceholders("secret "+tok+" direct "+surrogate, lookup, nil)
	want := "secret " + surrogate + " direct ops@example.com"
	if got != want {
		t.Fatalf("RestoreKnownPlaceholders() = %q, want %q", got, want)
	}
}

func TestApplyFallsBackToTokenWhenSurrogateKeyCollides(t *testing.T) {
	keyer := newTestKeyer(t)
	scope := types.Scope{Session: "surrogate-collision"}
	store := mapstore.New()
	collisions := map[string]string{}
	value := "8.8.8.8"
	tok := keyer.Derive(types.TypeIPv4, value, collisions)
	surrogate, ok := surrogateFor(types.TypeIPv4, value, tok)
	if !ok {
		t.Fatal("expected IPv4 surrogate")
	}
	store.Put(scope, surrogate, "1.1.1.1")

	result, err := Apply(
		value,
		[]types.Finding{{Start: 0, End: len(value), Type: types.TypeIPv4, Score: 0.95, Source: "test"}},
		scope,
		types.Policy{DefaultOperator: types.OperatorToken},
		store,
		keyer,
		collisions,
	)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if result.Masked == surrogate {
		t.Fatalf("surrogate collision overwrote existing mapping with %q", surrogate)
	}
	if !strings.HasPrefix(result.Masked, token.Prefix+"IPV4_") {
		t.Fatalf("expected opaque IPv4 token fallback, got %q", result.Masked)
	}
	if got, _ := store.Get(scope, surrogate); got != "1.1.1.1" {
		t.Fatalf("existing surrogate mapping changed to %q", got)
	}
	if got, ok := store.Get(scope, result.Masked); !ok || got != value {
		t.Fatalf("fallback token mapping = (%q, %v), want (%q, true)", got, ok, value)
	}
}

func newTestKeyer(t *testing.T) *token.Keyer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte(strings.Repeat("k", 32)), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	keyer, err := token.NewKeyer(path)
	if err != nil {
		t.Fatalf("NewKeyer: %v", err)
	}
	return keyer
}
