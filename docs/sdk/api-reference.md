# SDK API Reference

**Status: Draft / scaffold.** A compiling scaffold exists, but behavior is not implemented
yet. These signatures are the proposed surface derived from the [contract](contract.md);
they may change before the engine ships.

## Package

```go
import (
    "context"

    "github.com/cloakia/opencloak"
)
```

## Configuration

```go
// Config controls detection and tokenization. Zero value is usable (L1 defaults, key
// loaded from ~/.opencloak/key, generated on first use).
type Config struct {
    KeyPath  string         // HMAC key location; default ~/.opencloak/key
    Detector Detector       // optional L2 detector; nil = L1 only
    Policy   PolicyProvider // nil = built-in local policy
    Audit    AuditSink      // nil = no-op
}

func New(cfg Config) (*Engine, error)
```

## Scope

```go
// Scope selects the mapstore namespace. Zero value is single-user local use.
type Scope struct {
    Tenant  string // required for multi-user embedding
    Session string // stable agent/session id when available
    Project string // optional project/workspace namespace
}
```

## Type

```go
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
    TypePerson Type = "PERSON" // L2
    TypeAddr   Type = "ADDR"   // L2
)
```

## Policy

```go
type TransformOperator string

const (
    OperatorToken            TransformOperator = "token"             // default reversible CLK token
    OperatorFormatPreserving TransformOperator = "format_preserving" // Phase 1; type-specific reverse strategy
    OperatorRedact           TransformOperator = "redact"            // Phase 1; irreversible
    OperatorBlock            TransformOperator = "block"
    OperatorIgnore           TransformOperator = "ignore"
)

type TypePolicy struct {
    Operator TransformOperator // empty = Policy.DefaultOperator
}

type Policy struct {
    DefaultOperator TransformOperator // empty = OperatorToken
    Types           map[Type]TypePolicy
    RuleSets        []string
}

type PolicyProvider interface {
    Policy(ctx context.Context, scope Scope) (Policy, error)
}
```

## Core operations

```go
// Text surface
func (e *Engine) Mask(ctx context.Context, scope Scope, text string) (masked string, st *State, err error)
func (e *Engine) Restore(ctx context.Context, st *State, text string) (string, error)

// Wire surface (native provider JSON)
func (e *Engine) MaskRequest(ctx context.Context, scope Scope, provider, op string, body []byte) (masked []byte, st *State, err error)
func (e *Engine) RestoreResponse(ctx context.Context, st *State, body []byte) ([]byte, error)

// Stream surface
func (e *Engine) RestoreStreamChunk(st *State, chunk []byte) []byte
func (e *Engine) FlushStream(st *State) []byte
func (e *Engine) RestoreSSEEvent(ctx context.Context, st *State, eventData []byte) ([]byte, error)
```

The SDK surface names avoid `L0/L1/L2` because detection uses `L1` for pattern rules and
`L2` for the optional NER layer.

Phase 0 implements `"anthropic"`. `"openai-responses"`, `"openai-chat"`, and `"gemini"`
are reserved planned provider tags.
`op` is the endpoint/operation (e.g. `"messages"`, `"responses"`).

## State

```go
// State holds the token↔value reverse map for a text or wire request/restore cycle and
// records the Scope plus provider/op for wire calls.
type State struct { /* opaque */ }

func (st *State) Scope() Scope
func (st *State) Provider() string
func (st *State) Op() string
```

## Detector (L2 extension point)

```go
// Detector finds sensitive findings the L1 patterns cannot. Implement this to plug in a
// local NER model (Phase 1). nil means L1-only.
type Detector interface {
    Detect(ctx context.Context, text string) ([]Finding, error)
}

type Finding struct {
    Start, End int     // UTF-8 byte offsets [Start, End)
    Type       Type
    Score      float64 // normalized 0..1 confidence
    Source     string  // detector or rule id, e.g. "l1:gitleaks:github-pat"
}
```

## Errors & fail-closed

```go
var ErrNotImplemented error
var ErrInvalidState error
var ErrBlocked error

type BlockedError struct {
    Types []Type
}
```

`Mask` and `MaskRequest` return an error if detection, policy, key loading, or parsing
fails; callers **must** treat an error as fail-closed (block the request), never forward
the original body.

`Restore`, `RestoreResponse`, and `RestoreSSEEvent` return `ErrInvalidState` for nil or
incomplete `State` handles. `Mask` and `MaskRequest` return `ErrBlocked` or a
`*BlockedError` when a finding's type is configured with `OperatorBlock`. Raw
`RestoreStreamChunk`/`FlushStream` stay error-free hot-path helpers.

See the [threat model](../architecture/threat-model.md).
