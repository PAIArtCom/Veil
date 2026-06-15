# System Design

**Status:** Accepted (layout & responsibilities). Implementation is Phase 0.

This is the concrete software design that realizes the [architecture overview](overview.md)
in Go. The layout decision and its rationale are [ADR-0007](decisions/0007-code-and-module-layout.md).

## Module tree (scaffold vs planned)

```
opencloak/                       module github.com/cloakia/opencloak
│
├── doc.go types.go interfaces.go engine.go    [scaffold] PUBLIC API (root package)
│
├── internal/
│   ├── detect/            [scaffold] detection pipeline, fail-closed orchestration
│   │   ├── l1/            [scaffold] regex + entropy + Luhn + context keywords
│   │   │                                  rules merge: privacy-filter + gitleaks (go:embed)
│   │   └── resolver/      [scaffold] Finding merge/de-overlap and precedence
│   ├── mask/              [scaffold] offset-safe replacement + token mapping writes
│   ├── token/            [scaffold] CLK_<TYPE>_<id>, HMAC, normalize, local key
│   ├── mapstore/         [scaffold] token<->value reverse map (State), scoped in-mem
│   ├── wire/             [scaffold] internal provider-native JSON adapters
│   │   ├── anthropic/    [scaffold] /v1/messages  (Phase 0)
│   │   ├── openairesponses/   [planned]  /v1/responses (Codex, Phase 1)
│   │   ├── openaichat/        [planned]  /v1/chat/completions (Phase 1)
│   │   └── gemini/            [planned]  generateContent (Phase 1)
│   ├── stream/           [scaffold] chunk-level (byte-split tolerant) + SSE-event
│   ├── proxy/            [scaffold] standalone base-URL proxy (localhost, pass-through)
│   ├── console/          [scaffold] LOCAL single-user web console (localhost)
│   ├── config/           [scaffold] local PolicyProvider default
│   └── service/          [phase1 scaffold] HTTP/gRPC service (Phase 1)
│
├── cmd/opencloak/        [scaffold] single binary: proxy | serve | console | mask
└── examples/embed/       [planned]          minimal "embed the engine" example
```

"Scaffold" means the package boundary exists and compiles, but does not claim behavior is
implemented. Most files are package docs or fail-closed placeholders. Phase 0 is complete
only after the exit criteria in the roadmap pass against a real agent flow.

## Component responsibilities

| Package | Responsibility |
|---|---|
| `opencloak` (root) | Public API: `Engine`, `New`, Text/Wire/Stream methods, `Scope`/`State`, `Finding`, policy operators, and extension interfaces. The only package external code imports. |
| `internal/detect` | Run the configured detector layers; emit `Finding` values; enforce fail-closed. |
| `internal/detect/l1` | Pattern detection: regex rule sets (privacy-filter + gitleaks), Shannon entropy + context keywords/bare fallback, checksums (Luhn). |
| `internal/detect/resolver` | Merge same-type overlaps, resolve cross-type conflicts, and emit non-overlapping findings. |
| `internal/mask` | Consume resolved findings, perform offset-safe replacement, call token strategy, write token mappings into `State`, and scan final text/wire buffers for residual tokens. |
| `internal/token` | The `CLK_<TYPE>_<id>` format, HMAC derivation, `normalize`, and the local key. |
| `internal/mapstore` | The token↔value reverse map behind `State`; in-memory, scoped by request/stream and optional session/tenant namespace. |
| `internal/wire` | Internal provider adapters that extract/apply text spans for each provider's native request/response JSON. No unified schema. |
| `internal/stream` | Restore tokens in streaming responses (chunk-level + SSE-event-level) and scan raw streams for residual tokens at flush/end-of-stream. |
| `internal/proxy` | Standalone base-URL proxy transport (localhost-only, credential pass-through). |
| `internal/console` | Local single-user web console (localhost-only). |
| `internal/config` | Default local `PolicyProvider`. |
| `internal/service` | Network service (HTTP/gRPC) for non-Go hosts (Phase 1). |
| `cmd/opencloak` | The CLI binary; wires transports. |

## Dependency direction (one way)

```
        cmd/opencloak  (proxy | serve | console | mask)
                  │ wires
   internal/proxy · internal/console · internal/service
                  │ depend on
        opencloak  (root: public façade + interfaces)
                  │ delegates to
   detect(+l1+resolver) · mask · token · mapstore · wire(+providers) · stream · config
                  ▲ implemented externally
   Detector (L2, Phase 1)        PolicyProvider / AuditSink (Cloakia, separate repo)
```

Rules: the **core** (`detect`/`mask`/`token`/`mapstore`/`wire`/`stream`) depends only on
the standard library plus small JSON helpers — never on a transport. **Transports** depend
on the engine. **Nothing** depends on a transport except `cmd`. This puts the
[engine/transport split](overview.md) and the [SDK contract](../sdk/contract.md) into the
package graph.

## The two seams (open ↔ commercial)

The commercial control plane attaches through exactly two interfaces, both defined in the
open root package with local defaults:

| Interface | OSS default | Cloakia (separate repo) |
|---|---|---|
| `PolicyProvider` | local file (`internal/config`), scope may be ignored | fetch + hot-reload centrally pushed policy by tenant/session/project |
| `AuditSink` | no-op / local counters | collect minimized audit data |

**Console symmetry.** The local web console (`internal/console`, open) and Cloakia's
multi-tenant console (commercial) consume the *same* two seams — local implementations
versus remote ones. That is why a console can exist on both sides of the open-core boundary
without crossing it.

## Public API surfaces -> packages

| API surface | Method(s) | Backed by |
|---|---|---|
| Text | `Mask` / `Restore` | `detect` + `resolver` + `mask` + `token` + `mapstore`; returns `State`/errors |
| Wire | `MaskRequest` / `RestoreResponse` | `wire/<provider>` + Text; `State` records scope/provider/op |
| Stream | `RestoreStreamChunk` / `FlushStream` / `RestoreSSEEvent` | `stream` + `mapstore` |

These API surfaces are intentionally named Text/Wire/Stream rather than L0/L1/L2. Detection
already uses L1 for pattern rules and L2 for optional NER, and the two dimensions should
not be conflated.

## Provider adapter boundary

`internal/wire` is not a public plugin API in Phase 0. It is an internal boundary that
keeps provider-native JSON walking out of the root package while OpenCloak proves the loop
with one maintained Anthropic adapter. Phase 1 adds maintained adapters for OpenAI
Responses, OpenAI Chat, and Gemini. A public third-party adapter registration API should
be added only when there is a real external adapter use case; until then, the stable
public contract is `MaskRequest(ctx, scope, provider, op, body)` plus the documented
provider tags.

## Phase 0 cut

**Implement:** `token`, scoped `mapstore`/`State`, `detect/l1` (with a starter merged rule
set), finding conflict resolution, per-type transform operators, `mask`,
`wire/anthropic`, provider-aware buffered/SSE restore, `stream` (raw chunk-level), the root
façade methods, `internal/proxy` (Claude Code endpoint), and `opencloak proxy`.
Validate by embedding the engine in one real gateway plus the standalone proxy, against
the end-to-end task in the [roadmap](../product/roadmap.md).

**Defer (Phase 1):** L2 detector, the non-Anthropic `wire` providers, `service`
(HTTP/gRPC), and the local `console` beyond a minimal status view.

## Build status

The scaffold compiles today: `gofmt` clean, `go vet ./...` clean, `go build ./...` ok, the
binary help path runs. Most method bodies are pass-through placeholders; `MaskRequest`,
`RestoreResponse`, and `RestoreSSEEvent` return `ErrNotImplemented` so any premature
integration fails closed instead of silently forwarding plaintext or claiming restore.
