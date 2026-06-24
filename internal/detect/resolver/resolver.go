package resolver

import (
	"sort"

	"github.com/PAIArtCom/Veil/internal/types"
)

// Resolve applies ADR-0008 conflict resolution to findings:
//  1. Drop invalid ranges (Start >= End or negative offsets).
//  2. Merge same-type overlaps that describe the same underlying value.
//  3. Resolve cross-type conflicts by score (higher wins), then by longer
//     range, then by earlier start.
//  4. Return non-overlapping findings in ascending byte-offset order.
//
// The textLen parameter is the byte length of the source text; findings that
// extend beyond it are also dropped.
func Resolve(findings []types.Finding, textLen int) []types.Finding {
	// 1. Drop invalid ranges.
	valid := make([]types.Finding, 0, len(findings))
	for _, f := range findings {
		if f.Start < 0 || f.End <= f.Start || f.End > textLen {
			continue
		}
		valid = append(valid, f)
	}

	// 2. Sort by start, then longer end, then higher score.
	sort.Slice(valid, func(i, j int) bool {
		fi, fj := valid[i], valid[j]
		if fi.Start != fj.Start {
			return fi.Start < fj.Start
		}
		if fi.End != fj.End {
			return fi.End > fj.End // longer first
		}
		return fi.Score > fj.Score
	})

	// 3. Greedy sweep: for each finding decide whether it survives.
	//    We maintain the set of accepted findings; for each candidate we check
	//    whether it overlaps any already-accepted finding.  On overlap:
	//    - same type → merge (take the union span, keep higher score/source).
	//    - cross type → keep whichever wins by (score, length, start).
	accepted := make([]types.Finding, 0, len(valid))

	for _, candidate := range valid {
		merged := false
		for i := range accepted {
			a := accepted[i]
			if !overlaps(a, candidate) {
				continue
			}
			// They overlap.
			if a.Type == candidate.Type {
				// Same type → merge spans.
				accepted[i] = mergeFindings(a, candidate)
				merged = true
				break
			}
			// Cross-type conflict: keep the winner, drop the loser.
			if beats(a, candidate) {
				// Existing finding wins; drop candidate.
				merged = true
				break
			}
			// Candidate wins; replace existing.
			accepted[i] = candidate
			merged = true
			break
		}
		if !merged {
			accepted = append(accepted, candidate)
		}
	}

	// 4. Sort the result in ascending start order.
	sort.Slice(accepted, func(i, j int) bool {
		return accepted[i].Start < accepted[j].Start
	})

	return accepted
}

// overlaps returns true when a and b share at least one byte.
func overlaps(a, b types.Finding) bool {
	return a.Start < b.End && b.Start < a.End
}

// mergeFindings returns a finding that spans the union of a and b, keeping
// the higher score and the source of the higher-scoring finding.
func mergeFindings(a, b types.Finding) types.Finding {
	merged := types.Finding{
		Type:  a.Type,
		Start: minInt(a.Start, b.Start),
		End:   maxInt(a.End, b.End),
	}
	if b.Score > a.Score {
		merged.Score = b.Score
		merged.Source = b.Source
	} else {
		merged.Score = a.Score
		merged.Source = a.Source
	}
	return merged
}

// beats returns true when existing finding a should be kept over challenger b.
// Priority: higher score → longer range → earlier start.
func beats(a, b types.Finding) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	lenA := a.End - a.Start
	lenB := b.End - b.Start
	if lenA != lenB {
		return lenA > lenB
	}
	return a.Start <= b.Start
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
