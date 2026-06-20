package wire

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestApplyMaskedSpansByRangeRewritesStringLiteralsInOnePass(t *testing.T) {
	body := []byte(`{"a":"old","unchanged":{"nested":"keep"},"b":"line"}`)
	aStart := mustIndex(t, body, `"old"`)
	bStart := mustIndex(t, body, `"line"`)

	out, ok, err := ApplyMaskedSpansByRange(body, []MaskedSpan{
		{Path: "b", MaskedText: "x\"y\nz", Start: bStart, End: bStart + len(`"line"`)},
		{Path: "a", MaskedText: "OpenCloak_SECRET_001122334455", Start: aStart, End: aStart + len(`"old"`)},
	})
	if err != nil {
		t.Fatalf("ApplyMaskedSpansByRange error: %v", err)
	}
	if !ok {
		t.Fatal("ApplyMaskedSpansByRange declined valid ranges")
	}
	if !gjson.ValidBytes(out) {
		t.Fatalf("rewritten body is not valid JSON: %s", out)
	}
	if got := gjson.GetBytes(out, "a").Str; got != "OpenCloak_SECRET_001122334455" {
		t.Fatalf("a = %q", got)
	}
	if got := gjson.GetBytes(out, "b").Str; got != "x\"y\nz" {
		t.Fatalf("b = %q", got)
	}
	if got := gjson.GetBytes(out, "unchanged.nested").Str; got != "keep" {
		t.Fatalf("unchanged nested value = %q", got)
	}
}

func TestApplyMaskedSpansByRangeDeclinesInvalidRange(t *testing.T) {
	body := []byte(`{"a":"old"}`)
	out, ok, err := ApplyMaskedSpansByRange(body, []MaskedSpan{
		{Path: "a", MaskedText: "new", Start: 0, End: len(body)},
	})
	if err != nil {
		t.Fatalf("ApplyMaskedSpansByRange error: %v", err)
	}
	if ok {
		t.Fatalf("ApplyMaskedSpansByRange accepted non-string-literal range: %s", out)
	}
}

func mustIndex(t *testing.T, body []byte, needle string) int {
	t.Helper()
	for i := 0; i+len(needle) <= len(body); i++ {
		if string(body[i:i+len(needle)]) == needle {
			return i
		}
	}
	t.Fatalf("missing %q in %s", needle, body)
	return -1
}
