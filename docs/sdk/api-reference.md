# SDK API Reference

**Status: v0.1.0 pre-release API.** The text surface, Anthropic Messages wire surface,
OpenAI Responses wire surface, streaming restore, loopback proxy, and maintained
`examples/embed` reference integration are implemented. OpenAI Chat, Gemini, and Phase 1
transform operators are reserved.

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

The zero-value SDK config uses the built-in local policy: `token` by default, with
`PERSON`, `ADDR`, and `DATE` ignored. The standalone CLI can also load a local JSON
policy from `--policy`, `OPENCLOAK_POLICY`, or `~/.opencloak/policy.json` if present.
Embedders can provide any `PolicyProvider` that returns the public `Policy` shape below.

```go
type TransformOperator string

const (
    OperatorToken            TransformOperator = "token"             // default reversible OpenCloak token
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
    RuleSets        []string // Phase 1; Phase 0 rejects non-empty values
}

type PolicyProvider interface {
    Policy(ctx context.Context, scope Scope) (Policy, error)
}
```

Local policy-file shape:

```json
{
  "default_operator": "token",
  "types": {
    "EMAIL": {"operator": "ignore"},
    "SECRET": {"operator": "block"}
  }
}
```

File parsing is strict. Unknown keys fail closed rather than being treated as comments or
metadata. v0.1.0 supports only `token`, `ignore`, and `block`; `redact`,
`format_preserving`, and non-empty `rule_sets` are reserved and rejected.

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
func (e *Engine) NewSSEStreamRestorer(st *State) (*SSEStream, error)

type SSEStream struct { /* opaque */ }
func (s *SSEStream) Event(ctx context.Context, eventData []byte) ([][]byte, error)
func (s *SSEStream) Flush(ctx context.Context) ([][]byte, error)
```

The SDK surface names avoid `L0/L1/L2` because detection uses `L1` for pattern rules and
`L2` for the optional NER layer.

Implemented provider/op pairs:

- `"anthropic"` / `"messages"` for Anthropic Messages `POST /v1/messages`.
- `"openai-responses"` / `"responses"` for OpenAI Responses `POST /v1/responses`.

`"openai-chat"` and `"gemini"` are reserved planned provider tags.
`op` is the endpoint/operation (for example `"messages"` or `"responses"`).
Unsupported provider/op pairs fail closed.

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
    Source     string  // detector or rule id, e.g. "l1:github-pat"
}
```

## Errors & fail-closed

```go
var ErrNotImplemented error
var ErrInvalidState error
var ErrBlocked error
var ErrUnsupportedOperator error
var ErrUnsupportedPolicyFeature error

type BlockedError struct {
    Types []Type
}

type UnsupportedOperatorError struct {
    Type     Type
    Operator TransformOperator
}

type UnsupportedPolicyFeatureError struct {
    Feature string
}
```

`Mask` and `MaskRequest` return an error if detection, policy, key loading, or parsing
fails; callers **must** treat an error as fail-closed (block the request), never forward
the original body.

`Restore`, `RestoreResponse`, and `RestoreSSEEvent` return `ErrInvalidState` for nil or
incomplete `State` handles. `Mask` and `MaskRequest` return `ErrBlocked` or a
`*BlockedError` when a finding's type is configured with `OperatorBlock`, and
`ErrUnsupportedOperator` / `*UnsupportedOperatorError` when policy selects an operator
this build cannot execute. Non-empty `Policy.RuleSets` returns
`ErrUnsupportedPolicyFeature` / `*UnsupportedPolicyFeatureError` in Phase 0. Raw
`RestoreStreamChunk`/`FlushStream` stay error-free hot-path helpers.

`RestoreSSEEvent` is a stateless per-event helper for hosts that already parse SSE and
can tolerate token boundaries inside one complete event payload. `NewSSEStreamRestorer`
is the provider-aware stateful SSE helper: it accepts complete parsed event payloads,
holds back token fragments across adjacent provider events, and emits zero or more
restored event payloads from `Event` plus any final payloads from `Flush`. Use it when a
provider can split assistant text or tool JSON across SSE events.

See the [threat model](../architecture/threat-model.md).
