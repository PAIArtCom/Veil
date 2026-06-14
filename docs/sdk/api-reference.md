# SDK API Reference

**Status: Draft / Planned.** No implementation exists yet. These signatures are the
*proposed* surface derived from the [contract](contract.md); they will change as the engine
is built. Treat this as a design target, not documentation of shipped code.

## Package

```go
import "github.com/opencloak/opencloak"   // module path provisional
```

## Configuration

```go
// Config controls detection and tokenization. Zero value is usable (L1 defaults, key
// loaded from ~/.opencloak/key, generated on first use).
type Config struct {
    KeyPath    string            // HMAC key location; default ~/.opencloak/key
    Types      map[Type]bool     // per-type enable/disable; PERSON/ADDR default false
    RuleSet    string            // L1 rule set name; default "builtin"
    Detector   Detector          // optional L2 detector; nil = L1 only
}

func New(cfg Config) (*Engine, error)
```

## Type

```go
type Type string

const (
    Secret Type = "SECRET"
    Email  Type = "EMAIL"
    Phone  Type = "PHONE"
    IPv4   Type = "IPV4"
    IPv6   Type = "IPV6"
    Card   Type = "CARD"
    Acct   Type = "ACCT"
    URL    Type = "URL"
    Date   Type = "DATE"
    Person Type = "PERSON" // L2
    Addr   Type = "ADDR"   // L2
)
```

## Core operations

```go
// L0 — text
func (e *Engine) Mask(text string) string
func (e *Engine) Restore(text string) string

// L1 — wire-format (native provider JSON)
func (e *Engine) MaskRequest(provider, op string, body []byte) (masked []byte, st *State, err error)
func (e *Engine) RestoreResponse(st *State, body []byte) []byte

// L2 — streaming
func (e *Engine) RestoreStreamChunk(st *State, chunk []byte) []byte
func (e *Engine) FlushStream(st *State) []byte
func (e *Engine) RestoreSSEEvent(st *State, eventData []byte) []byte
```

`provider` ∈ `"anthropic" | "openai-responses" | "openai-chat" | "gemini"`.
`op` is the endpoint/operation (e.g. `"messages"`, `"responses"`).

## State

```go
// State holds the token↔value reverse map for a request/response cycle.
// Obtain it from MaskRequest; pass it to the matching Restore* calls.
// A nil *State uses the engine's process-global store.
type State struct { /* opaque */ }
```

## Detector (L2 extension point)

```go
// Detector finds sensitive spans the L1 patterns cannot. Implement this to plug in a
// local NER model (Phase 1). nil means L1-only.
type Detector interface {
    Detect(ctx context.Context, text string) ([]Span, error)
}

type Span struct {
    Start, End int
    Type       Type
}
```

## Errors & fail-closed

`MaskRequest` returns an error if detection or parsing fails; callers **must** treat an
error as fail-closed (block the request), never forward the original body. See the
[threat model](../architecture/threat-model.md).
