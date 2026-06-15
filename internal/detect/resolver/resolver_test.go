package resolver

import (
	"testing"

	"github.com/cloakia/opencloak/internal/types"
)

func f(start, end int, typ types.Type, score float64, source string) types.Finding {
	return types.Finding{Start: start, End: end, Type: typ, Score: score, Source: source}
}

const textLen = 1000

// TestInvalidRangeDropped verifies that invalid findings are dropped.
func TestInvalidRangeDropped(t *testing.T) {
	cases := []types.Finding{
		f(-1, 5, types.TypeEmail, 1.0, "src"),        // negative start
		f(5, 5, types.TypeEmail, 1.0, "src"),         // zero length
		f(5, 3, types.TypeEmail, 1.0, "src"),         // end < start
		f(0, textLen+1, types.TypeEmail, 1.0, "src"), // beyond text
	}
	for _, bad := range cases {
		result := Resolve([]types.Finding{bad}, textLen)
		if len(result) != 0 {
			t.Errorf("expected invalid finding to be dropped: %+v → %v", bad, result)
		}
	}
}

// TestAscendingOrder verifies the output is sorted by start offset.
func TestAscendingOrder(t *testing.T) {
	input := []types.Finding{
		f(50, 60, types.TypeEmail, 0.9, "a"),
		f(10, 20, types.TypeIPv4, 0.9, "b"),
		f(30, 40, types.TypeURL, 0.85, "c"),
	}
	result := Resolve(input, textLen)
	for i := 1; i < len(result); i++ {
		if result[i].Start < result[i-1].Start {
			t.Errorf("result not ascending at index %d: %v", i, result)
		}
	}
}

// TestSameTypeOverlapMerge verifies that two same-type overlapping findings
// are merged into one spanning finding.
func TestSameTypeOverlapMerge(t *testing.T) {
	input := []types.Finding{
		f(0, 20, types.TypeEmail, 0.9, "a"),
		f(15, 30, types.TypeEmail, 0.95, "b"),
	}
	result := Resolve(input, textLen)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged finding, got %d: %v", len(result), result)
	}
	got := result[0]
	if got.Start != 0 || got.End != 30 {
		t.Errorf("merged span = [%d,%d), want [0,30)", got.Start, got.End)
	}
	if got.Score != 0.95 {
		t.Errorf("expected higher score 0.95, got %v", got.Score)
	}
}

// TestSameTypeContainmentMerge verifies containment (one finding fully inside
// another of the same type) is merged.
func TestSameTypeContainmentMerge(t *testing.T) {
	input := []types.Finding{
		f(0, 30, types.TypeSecret, 0.9, "outer"),
		f(5, 20, types.TypeSecret, 0.99, "inner"),
	}
	result := Resolve(input, textLen)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged finding, got %d: %v", len(result), result)
	}
	got := result[0]
	if got.Start != 0 || got.End != 30 {
		t.Errorf("merged span = [%d,%d), want [0,30)", got.Start, got.End)
	}
}

// TestCrossTypeHigherScoreWins verifies that in a cross-type conflict the
// higher-score finding is kept.
func TestCrossTypeHigherScoreWins(t *testing.T) {
	input := []types.Finding{
		f(0, 20, types.TypeURL, 0.85, "url"),
		f(5, 15, types.TypeSecret, 0.99, "secret"),
	}
	result := Resolve(input, textLen)
	if len(result) != 1 {
		t.Fatalf("expected 1 finding after cross-type conflict, got %d: %v", len(result), result)
	}
	if result[0].Type != types.TypeSecret {
		t.Errorf("expected TypeSecret to win, got %v", result[0].Type)
	}
}

// TestCrossTypeLongerWinsOnEqualScore verifies that when scores are equal the
// longer finding is kept.
func TestCrossTypeLongerWinsOnEqualScore(t *testing.T) {
	input := []types.Finding{
		f(0, 30, types.TypeURL, 0.85, "long-url"),
		f(5, 15, types.TypeIPv4, 0.85, "short-ip"),
	}
	result := Resolve(input, textLen)
	if len(result) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(result), result)
	}
	if result[0].Type != types.TypeURL {
		t.Errorf("expected TypeURL (longer) to win, got %v", result[0].Type)
	}
}

// TestCrossTypeEarlierStartWinsOnEqualScoreAndLength verifies tiebreaker.
func TestCrossTypeEarlierStartWinsOnEqualScoreAndLength(t *testing.T) {
	input := []types.Finding{
		f(0, 10, types.TypeEmail, 0.90, "a"),
		f(5, 15, types.TypeURL, 0.90, "b"),
	}
	result := Resolve(input, textLen)
	if len(result) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(result), result)
	}
	if result[0].Type != types.TypeEmail {
		t.Errorf("expected TypeEmail (earlier start) to win, got %v", result[0].Type)
	}
}

// TestNonOverlappingPreservedBoth verifies that two non-overlapping findings
// are both emitted.
func TestNonOverlappingPreservedBoth(t *testing.T) {
	input := []types.Finding{
		f(0, 10, types.TypeEmail, 0.9, "a"),
		f(20, 30, types.TypeIPv4, 0.9, "b"),
	}
	result := Resolve(input, textLen)
	if len(result) != 2 {
		t.Fatalf("expected 2 non-overlapping findings, got %d: %v", len(result), result)
	}
}
