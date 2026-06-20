package token

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloakia/opencloak/internal/types"
)

// newTestKeyer writes a fixed 32-byte key to a temp file so tests are
// deterministic across runs.
func newTestKeyer(t *testing.T) *Keyer {
	t.Helper()
	key := make([]byte, keyLen)
	for i := range key {
		key[i] = byte(i + 1) // 01 02 03 ... 20
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "key")
	if err := os.WriteFile(path, key, 0600); err != nil {
		t.Fatalf("write test key: %v", err)
	}
	k, err := NewKeyer(path)
	if err != nil {
		t.Fatalf("NewKeyer: %v", err)
	}
	return k
}

func TestDeterminism(t *testing.T) {
	k := newTestKeyer(t)
	c1 := map[string]string{}
	c2 := map[string]string{}
	tok1 := k.Derive(types.TypeSecret, "sk-live-abc123", c1)
	tok2 := k.Derive(types.TypeSecret, "sk-live-abc123", c2)
	if tok1 != tok2 {
		t.Fatalf("same value → different tokens: %q vs %q", tok1, tok2)
	}
}

func TestDistinctValuesDistinctTokens(t *testing.T) {
	k := newTestKeyer(t)
	collisions := map[string]string{}
	tok1 := k.Derive(types.TypeSecret, "value-one", collisions)
	tok2 := k.Derive(types.TypeSecret, "value-two", collisions)
	if tok1 == tok2 {
		t.Fatalf("distinct values → same token: %q", tok1)
	}
}

func TestIdentifierSafe(t *testing.T) {
	k := newTestKeyer(t)
	collisions := map[string]string{}
	tok := k.Derive(types.TypeSecret, "any-secret-value", collisions)
	if len(tok) == 0 {
		t.Fatal("empty token")
	}
	first := tok[0]
	if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') || first == '_') {
		t.Fatalf("token does not start with letter/underscore: %q", tok)
	}
	for _, ch := range tok[1:] {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '_') {
			t.Fatalf("token contains non-identifier char %q: %q", ch, tok)
		}
	}
}

func TestFormat(t *testing.T) {
	k := newTestKeyer(t)
	collisions := map[string]string{}
	tok := k.Derive(types.TypeEmail, "user@Example.COM", collisions)
	if !strings.HasPrefix(tok, "OpenCloak_EMAIL_") {
		t.Fatalf("unexpected prefix: %q", tok)
	}
	id := strings.TrimPrefix(tok, "OpenCloak_EMAIL_")
	if len(id) < idBaseLen {
		t.Fatalf("id too short (%d chars): %q", len(id), tok)
	}
	for _, ch := range id {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Fatalf("id not hex: %q", id)
		}
	}
}

func TestEmailDomainNormalize(t *testing.T) {
	k := newTestKeyer(t)
	c1 := map[string]string{}
	c2 := map[string]string{}
	// user@Example.COM and user@example.com should produce the same token.
	tok1 := k.Derive(types.TypeEmail, "user@Example.COM", c1)
	tok2 := k.Derive(types.TypeEmail, "user@example.com", c2)
	if tok1 != tok2 {
		t.Fatalf("email domain case not normalized: %q vs %q", tok1, tok2)
	}
	// But local part is case-sensitive per spec (we only normalize domain).
	c3 := map[string]string{}
	c4 := map[string]string{}
	tok3 := k.Derive(types.TypeEmail, "Alice@example.com", c3)
	tok4 := k.Derive(types.TypeEmail, "alice@example.com", c4)
	if tok3 == tok4 {
		t.Fatalf("email local part should be case-sensitive: both give %q", tok3)
	}
}

func TestCollisionExtension(t *testing.T) {
	// Force a collision by pre-seeding the collision map with the base id.
	k := newTestKeyer(t)

	// Derive the token for "real-secret" so we know the base id.
	cReal := map[string]string{}
	tokReal := k.Derive(types.TypeSecret, "real-secret", cReal)
	baseID := strings.TrimPrefix(tokReal, "OpenCloak_SECRET_")

	// Now build a fresh collision map that pretends another value already owns
	// that base id. Deriving "real-secret" again must produce an extended id.
	cManip := map[string]string{
		baseID: "some-other-normalized-value",
	}
	tokExtended := k.Derive(types.TypeSecret, "real-secret", cManip)
	extID := strings.TrimPrefix(tokExtended, "OpenCloak_SECRET_")

	if len(extID) <= len(baseID) {
		t.Fatalf("expected extended id (len>%d) after collision, got %q (len=%d)", len(baseID), extID, len(extID))
	}
	// Verify the extended id is still lowercase hex.
	for _, ch := range extID {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Fatalf("extended id not hex: %q", extID)
		}
	}
}

func TestKeyerAutoCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "key")
	k, err := NewKeyer(path)
	if err != nil {
		t.Fatalf("NewKeyer auto-create: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("key file perm = %o, want 0600", info.Mode().Perm())
	}
	// A second keyer from the same file must produce the same token.
	k2, err := NewKeyer(path)
	if err != nil {
		t.Fatalf("NewKeyer reload: %v", err)
	}
	c1, c2 := map[string]string{}, map[string]string{}
	tok1 := k.Derive(types.TypeSecret, "val", c1)
	tok2 := k2.Derive(types.TypeSecret, "val", c2)
	if tok1 != tok2 {
		t.Fatalf("reloaded key produces different token: %q vs %q", tok1, tok2)
	}
}
