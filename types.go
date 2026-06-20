package opencloak

import (
	"errors"
	"strings"

	"github.com/cloakia/opencloak/internal/stream"
	"github.com/cloakia/opencloak/internal/types"
)

// Type is a category of sensitive data. It is embedded in every token as
// OpenCloak_<TYPE>_<id> so that handling and restore can branch on the category.
// See docs/concepts/token-spec.md.
type Type = types.Type

const (
	TypeSecret Type = types.TypeSecret
	TypeEmail  Type = types.TypeEmail
	TypePhone  Type = types.TypePhone
	TypeIPv4   Type = types.TypeIPv4
	TypeIPv6   Type = types.TypeIPv6
	TypeCard   Type = types.TypeCard
	TypeAcct   Type = types.TypeAcct
	TypeURL    Type = types.TypeURL
	TypeDate   Type = types.TypeDate
	TypePerson Type = types.TypePerson
	TypeAddr   Type = types.TypeAddr
)

// Finding is a detected sensitive region within a piece of text, as UTF-8 byte
// offsets [Start, End). Score is normalized to 0..1 and Source names the detector
// or rule that produced the finding, for example "l1:gitleaks:github-pat".
// Findings are resolved for overlaps before masking; see docs/architecture/decisions/0008.
type Finding = types.Finding

// Scope selects the mapstore namespace for a request/stream lifecycle. The zero
// value is the single-user local scope. Multi-user embedders should set Tenant;
// long-lived agent workflows should set Session and, when useful, Project.
type Scope = types.Scope

// State holds token<->value reverse mappings for a masked text or wire request and the
// matching restore lifecycle. It records the Scope and, for wire calls, the provider/op
// selected by MaskRequest so buffered and SSE-event restores can dispatch to the same
// provider walker. Obtain it from Mask or MaskRequest and pass it to the matching
// Restore* calls. See docs/architecture/decisions/0009.
//
// Skeleton: the internal fields are owned by the mapstore implementation.
type State struct {
	scope    Scope
	provider string
	op       string

	// stream is the lazily-initialized holdback restorer backing the raw
	// streaming surface (RestoreStreamChunk/FlushStream). It is created on the
	// first RestoreStreamChunk call and is single-writer: one stream per State,
	// driven by one relay goroutine.
	stream *stream.Restorer
}

// Scope returns the mapstore namespace associated with st. A nil State returns the
// zero single-user local scope.
func (st *State) Scope() Scope {
	if st == nil {
		return Scope{}
	}
	return st.scope
}

// Provider returns the provider tag associated with st.
func (st *State) Provider() string {
	if st == nil {
		return ""
	}
	return st.provider
}

// Op returns the provider operation associated with st.
func (st *State) Op() string {
	if st == nil {
		return ""
	}
	return st.op
}

// ErrNotImplemented is reserved for planned operations that are not built yet.
// Callers MUST treat it as fail-closed: block the request, never forward plaintext.
var ErrNotImplemented = errors.New("opencloak: not implemented")

// ErrInvalidState is returned when a restore path receives nil State or, for provider
// shaped responses, a State without provider/op metadata.
var ErrInvalidState = errors.New("opencloak: invalid state")

// ErrBlocked is returned by Mask or MaskRequest when a finding's type is configured
// with OperatorBlock: the request is refused rather than masked, so a transport can map
// it to a blocked-by-policy response.
var ErrBlocked = errors.New("opencloak: blocked by policy")

// ErrUnsupportedOperator is returned when Policy selects a transform operator
// that this build cannot execute. Phase 0 supports token/block/ignore only;
// format_preserving and redact are reserved for Phase 1 and fail closed here.
var ErrUnsupportedOperator = errors.New("opencloak: unsupported transform operator")

// ErrUnsupportedPolicyFeature is returned when Policy uses a non-operator feature
// this build cannot execute. Phase 0 rejects RuleSets instead of silently ignoring
// them, so caller policy uncertainty cannot become plaintext pass-through.
var ErrUnsupportedPolicyFeature = errors.New("opencloak: unsupported policy feature")

// BlockedError reports which sensitive types caused an OperatorBlock decision. It wraps
// ErrBlocked for errors.Is checks.
type BlockedError struct {
	Types []Type
}

func (e *BlockedError) Error() string {
	if len(e.Types) == 0 {
		return ErrBlocked.Error()
	}
	names := make([]string, len(e.Types))
	for i, t := range e.Types {
		names[i] = string(t)
	}
	return ErrBlocked.Error() + ": " + strings.Join(names, ", ")
}

func (e *BlockedError) Is(target error) bool {
	return target == ErrBlocked
}

// UnsupportedOperatorError reports the unsupported operator selected by policy.
// It wraps ErrUnsupportedOperator for errors.Is checks.
type UnsupportedOperatorError struct {
	Type     Type
	Operator TransformOperator
}

func (e *UnsupportedOperatorError) Error() string {
	if e.Type == "" {
		return ErrUnsupportedOperator.Error() + ": default=" + string(e.Operator)
	}
	return ErrUnsupportedOperator.Error() + ": " + string(e.Type) + "=" + string(e.Operator)
}

func (e *UnsupportedOperatorError) Is(target error) bool {
	return target == ErrUnsupportedOperator
}

// UnsupportedPolicyFeatureError reports the unsupported Policy feature selected
// by the caller. It wraps ErrUnsupportedPolicyFeature for errors.Is checks.
type UnsupportedPolicyFeatureError struct {
	Feature string
}

func (e *UnsupportedPolicyFeatureError) Error() string {
	if e.Feature == "" {
		return ErrUnsupportedPolicyFeature.Error()
	}
	return ErrUnsupportedPolicyFeature.Error() + ": " + e.Feature
}

func (e *UnsupportedPolicyFeatureError) Is(target error) bool {
	return target == ErrUnsupportedPolicyFeature
}
