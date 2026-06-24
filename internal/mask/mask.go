package mask

import (
	"fmt"
	"sort"

	"github.com/PAIArtCom/Veil/internal/mapstore"
	"github.com/PAIArtCom/Veil/internal/token"
	"github.com/PAIArtCom/Veil/internal/types"
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
) (Result, error) {
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
		switch op {
		case types.OperatorToken, types.OperatorIgnore:
			// handled after the full action list is validated
		case types.OperatorBlock:
			blockedSet[f.Type] = struct{}{}
		default:
			return Result{}, fmt.Errorf("mask: unsupported transform operator %q for type %s", op, f.Type)
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
		return Result{Masked: text, Blocked: blocked}, nil
	}

	for i := range actions {
		if actions[i].op != types.OperatorToken {
			continue
		}
		f := actions[i].finding
		value := text[f.Start:f.End]
		tok := keyer.Derive(f.Type, value, collisions)
		store.Put(scope, tok, value)
		actions[i].tok = tok
	}

	outLen := len(text)
	for _, a := range actions {
		if a.op == types.OperatorToken {
			outLen += len(a.tok) - (a.finding.End - a.finding.Start)
		}
	}

	buf := make([]byte, 0, outLen)
	cursor := 0
	for _, a := range actions {
		f := a.finding
		if cursor < f.Start {
			buf = append(buf, text[cursor:f.Start]...)
		}
		switch a.op {
		case types.OperatorToken:
			buf = append(buf, a.tok...)
		case types.OperatorIgnore:
			buf = append(buf, text[f.Start:f.End]...)
		}
		cursor = f.End
	}
	if cursor < len(text) {
		buf = append(buf, text[cursor:]...)
	}

	return Result{Masked: string(buf)}, nil
}
