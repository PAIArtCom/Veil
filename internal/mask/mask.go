package mask

import (
	"sort"

	"github.com/cloakia/opencloak/internal/mapstore"
	"github.com/cloakia/opencloak/internal/token"
	"github.com/cloakia/opencloak/internal/types"
)

// Result is returned by Apply.
type Result struct {
	// Masked is the rewritten text.
	Masked string
	// Blocked lists types that hit OperatorBlock. If non-empty, the caller
	// should wrap these in a *BlockedError and return it.
	Blocked []types.Type
}

// Apply performs offset-safe replacement of resolved findings in text.
// It consults policy to determine the operator per type, writes token→value
// mappings into store under scope, and returns the masked text or collects
// blocked types.
//
// findings must be non-overlapping and in ascending start order (as produced
// by the resolver).
func Apply(
	text string,
	findings []types.Finding,
	scope types.Scope,
	policy types.Policy,
	store *mapstore.Store,
	keyer *token.Keyer,
	collisions map[string]string,
) Result {
	// Determine the default operator.
	defOp := policy.DefaultOperator
	if defOp == "" {
		defOp = types.OperatorToken
	}

	// Collect blocked types (deduplicated).
	blockedSet := map[types.Type]struct{}{}

	type action struct {
		finding types.Finding
		op      types.TransformOperator
		tok     string // pre-computed token for OperatorToken
	}
	actions := make([]action, 0, len(findings))
	for _, f := range findings {
		op := defOp
		if tp, ok := policy.Types[f.Type]; ok && tp.Operator != "" {
			op = tp.Operator
		}
		a := action{finding: f, op: op}
		if op == types.OperatorToken {
			value := text[f.Start:f.End]
			tok := keyer.Derive(f.Type, value, collisions)
			store.Put(scope, tok, value)
			a.tok = tok
		} else if op == types.OperatorBlock {
			blockedSet[f.Type] = struct{}{}
		}
		actions = append(actions, a)
	}

	// If any blocked types were found, return them without modifying text.
	if len(blockedSet) > 0 {
		blocked := make([]types.Type, 0, len(blockedSet))
		for t := range blockedSet {
			blocked = append(blocked, t)
		}
		sort.Slice(blocked, func(i, j int) bool {
			return string(blocked[i]) < string(blocked[j])
		})
		return Result{Masked: text, Blocked: blocked}
	}

	// Rebuild text right-to-left so byte offsets stay valid.
	buf := []byte(text)
	for i := len(actions) - 1; i >= 0; i-- {
		a := actions[i]
		switch a.op {
		case types.OperatorToken:
			buf = replaceBytes(buf, a.finding.Start, a.finding.End, []byte(a.tok))
		case types.OperatorIgnore:
			// leave as-is
		}
	}

	return Result{Masked: string(buf)}
}

// replaceBytes replaces buf[start:end] with replacement and returns the new
// slice.
func replaceBytes(buf []byte, start, end int, replacement []byte) []byte {
	result := make([]byte, 0, len(buf)-(end-start)+len(replacement))
	result = append(result, buf[:start]...)
	result = append(result, replacement...)
	result = append(result, buf[end:]...)
	return result
}
