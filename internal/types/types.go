// Package types holds the shared data types used by both the public opencloak
// package and the internal implementation packages. Having them here breaks
// the import cycle that would otherwise arise between the root package and its
// internal sub-packages.
//
// The root opencloak package re-exports these as type aliases so callers see
// them as opencloak.Finding, opencloak.Scope, etc. — the public API is
// unchanged.
package types

// Type is a category of sensitive data. It is embedded in every token as
// CLK_<TYPE>_<id> so that handling and restore can branch on the category.
type Type string

const (
	TypeSecret Type = "SECRET"
	TypeEmail  Type = "EMAIL"
	TypePhone  Type = "PHONE"
	TypeIPv4   Type = "IPV4"
	TypeIPv6   Type = "IPV6"
	TypeCard   Type = "CARD"
	TypeAcct   Type = "ACCT"
	TypeURL    Type = "URL"
	TypeDate   Type = "DATE"
	TypePerson Type = "PERSON"
	TypeAddr   Type = "ADDR"
)

// Finding is a detected sensitive region within a piece of text, as UTF-8
// byte offsets [Start, End).
type Finding struct {
	Start  int
	End    int
	Type   Type
	Score  float64
	Source string
}

// Scope selects the mapstore namespace for a request/stream lifecycle.
type Scope struct {
	Tenant  string
	Session string
	Project string
}

// TransformOperator selects how a resolved finding is transformed.
type TransformOperator string

const (
	OperatorToken            TransformOperator = "token"
	OperatorFormatPreserving TransformOperator = "format_preserving"
	OperatorRedact           TransformOperator = "redact"
	OperatorBlock            TransformOperator = "block"
	OperatorIgnore           TransformOperator = "ignore"
)

// TypePolicy configures detection/transformation for one sensitive data type.
type TypePolicy struct {
	Operator TransformOperator
}

// Policy is the resolved detection/redaction configuration.
type Policy struct {
	DefaultOperator TransformOperator
	Types           map[Type]TypePolicy
	RuleSets        []string
}
