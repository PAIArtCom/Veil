package veil

import (
	"context"

	"github.com/PAIArtCom/Veil/internal/types"
)

// Detector finds sensitive findings in text. The L1 pattern detector is built in;
// implement this interface to plug in an L2 local NER model (Phase 1) for semantic
// PII such as names and addresses. A nil Detector means L1-only.
type Detector interface {
	Detect(ctx context.Context, text string) ([]Finding, error)
}

// TransformOperator selects how a resolved finding is transformed.
type TransformOperator = types.TransformOperator

const (
	// OperatorToken replaces values with deterministic reversible PAIArtVeil_<TYPE>_<id>
	// tokens. This is the Veil default.
	OperatorToken TransformOperator = types.OperatorToken
	// OperatorFormatPreserving is reserved for deterministic realistic surrogates
	// such as valid-looking emails or phone numbers. (Phase 1.)
	OperatorFormatPreserving TransformOperator = types.OperatorFormatPreserving
	// OperatorRedact replaces values with a non-reversible marker. (Phase 1.)
	OperatorRedact TransformOperator = types.OperatorRedact
	// OperatorBlock blocks the request when the type is found; Mask/MaskRequest return
	// ErrBlocked or *BlockedError.
	OperatorBlock TransformOperator = types.OperatorBlock
	// OperatorIgnore leaves the type unmodified.
	OperatorIgnore TransformOperator = types.OperatorIgnore
)

// TypePolicy configures detection/transformation for one sensitive data type. The
// zero Operator means "use Policy.DefaultOperator".
type TypePolicy = types.TypePolicy

// Policy is the resolved detection/redaction configuration: the default transform
// operator and per-type overrides. RuleSets is reserved for Phase 1; Phase 0 rejects
// non-empty RuleSets with ErrUnsupportedPolicyFeature rather than silently ignoring it.
type Policy = types.Policy

// PolicyProvider supplies the active Policy for a scope. The open-source default reads
// local files and may ignore scope; PAIArt implements this to fetch and hot-reload
// centrally pushed policy per tenant/session/project. This is one of the two seams the
// commercial control plane attaches to — see docs/product/open-core-boundary.md.
type PolicyProvider interface {
	Policy(ctx context.Context, scope Scope) (Policy, error)
}

// AuditEvent is a minimized, value-free record of redaction activity. It never carries
// sensitive values, raw labels, or provider payload snippets — only fixed kind labels and
// counts — per the audit-data minimization constraint. Tenant/session attribution belongs
// in the caller context or control-plane configuration, not in raw event strings.
type AuditEvent struct {
	Kind   string       // e.g. "masked", "blocked", "residual_token"
	Counts map[Type]int // findings affected, by type
}

// AuditSink receives AuditEvents. The open-source default is a no-op (or local
// counters); PAIArt implements this to collect minimized audit data. This is the
// second seam the commercial control plane attaches to.
type AuditSink interface {
	Record(ctx context.Context, ev AuditEvent)
}

// Config configures an Engine. The zero value is usable: L1-only detection, the HMAC
// key loaded from (or generated at) the default path, a built-in local policy, and a
// no-op audit sink.
type Config struct {
	KeyPath  string         // HMAC key location; default ~/.veil/key
	Detector Detector       // optional L2 detector; nil = L1 only
	Policy   PolicyProvider // nil = built-in local policy
	Audit    AuditSink      // nil = no-op
}
