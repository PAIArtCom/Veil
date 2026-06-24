package token

import "testing"

func TestParseType(t *testing.T) {
	cases := []struct {
		in      string
		wantTyp string
		wantOK  bool
	}{
		{"PAIArtVeil_IPV4_0a1b2c3d4e5f", "IPV4", true},
		{"PAIArtVeil_SECRET_deadbeef0001", "SECRET", true},
		{"PAIArtVeil_EMAIL_aabbccddeeff00", "EMAIL", true},
		{"PAIArtVeil_SECRET_0a1b2c3d4e5fabcd", "SECRET", true}, // collision-extended id
		{"PAIArtVeil_FOO_zzz", "", false},                      // non-hex id
		{"PAIArtVeil_BAR_0a1b", "", false},                     // <12 hex
		{"Veilsomething", "", false},                           // no separators
		{"not a token", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		typ, ok := ParseType(c.in)
		if typ != c.wantTyp || ok != c.wantOK {
			t.Errorf("ParseType(%q) = (%q,%v), want (%q,%v)", c.in, typ, ok, c.wantTyp, c.wantOK)
		}
	}
}

func TestPartialSuffixStart(t *testing.T) {
	cases := []struct {
		in       string
		wantHeld string // the suffix that must be held back
	}{
		{"hello world", ""},                                                // plain text → hold nothing
		{"abc P", "P"},                                                     // could start PAIArtVeil_
		{"abc PAI", "PAI"},                                                 // could start PAIArtVeil_
		{"abc PAIArtVeil", "PAIArtVeil"},                                   // could start PAIArtVeil_
		{"abc PAIArtVeil_", "PAIArtVeil_"},                                 // type pending
		{"abc PAIArtVeil_SEC", "PAIArtVeil_SEC"},                           // type started
		{"abc PAIArtVeil_SECRET_", "PAIArtVeil_SECRET_"},                   // id pending
		{"abc PAIArtVeil_SECRET_0a1b", "PAIArtVeil_SECRET_0a1b"},           // partial hex (<12)
		{"x PAIArtVeil_IPV4_0a1b2c3d4e5f", "PAIArtVeil_IPV4_0a1b2c3d4e5f"}, // complete 12-hex, extendable
		{"x PAIArtVeil_IPV4_0a1b2c3d4e5f ", ""},                            // terminated by space
		{"PAIArtVeil_FOO_zz", ""},                                          // non-hex after sep → never completes
		{"text PAIArtVeil_IPV4_0a1b2c3d4e5fAB", ""},                        // uppercase terminates hex
		{"", ""}, // empty
	}
	for _, c := range cases {
		start := PartialSuffixStart([]byte(c.in))
		held := c.in[start:]
		if held != c.wantHeld {
			t.Errorf("PartialSuffixStart(%q): held %q, want %q", c.in, held, c.wantHeld)
		}
	}
}

// TestPartialSuffixAlwaysMatches confirms the anchored, fully-optional pattern
// always returns an index in [0, len], so the streaming restorer can rely on it
// never failing to match.
func TestPartialSuffixAlwaysMatches(t *testing.T) {
	for _, in := range []string{"", "a", "PAIArtVeil_", "\x00\xff", "PAIArtVeil_X_0123456789ab"} {
		start := PartialSuffixStart([]byte(in))
		if start < 0 || start > len(in) {
			t.Fatalf("PartialSuffixStart(%q) = %d out of range [0,%d]", in, start, len(in))
		}
	}
}

func TestMaxTokenLenCoversRealTokens(t *testing.T) {
	// A real token is "PAIArtVeil_" + TYPE + "_" + id. Even with a long TYPE and a fully
	// collision-extended 64-hex id, it stays well under MaxTokenLen.
	longest := Prefix + "SECRETTYPE" + "_" + // generous TYPE
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 64 hex
	if len(longest) > MaxTokenLen {
		t.Fatalf("MaxTokenLen %d too small for plausible token len %d", MaxTokenLen, len(longest))
	}
}
