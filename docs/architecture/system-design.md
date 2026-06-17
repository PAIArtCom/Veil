# System Design

**Status:** Accepted (layout & responsibilities). Phase 0 is accepted; Phase 1+ surfaces
remain planned until implemented and verified.

This is the concrete software design that realizes the [architecture overview](overview.md)
in Go. The layout decision and its rationale are [ADR-0007](decisions/0007-code-and-module-layout.md).

## Module tree (implemented vs planned)

```
opencloak/                       module github.com/cloakia/opencloak
│
├── doc.go types.go interfaces.go engine.go    [implemented] PUBLIC API (root package)
│
├── internal/
│   ├── types/             [implemented] shared data types (Finding/Scope/Type/Policy/operators); root re-exports as transparent aliases
│   ├── detect/            [implemented] detection pipeline, fail-closed orchestration
│   │   ├── l1/            [implemented] regex + entropy + validators + context keywords
│   │   └── resolver/      [implemented] Finding merge/de-overlap and precedence
│   ├── mask/              [implemented] offset-safe replacement + token mapping writes
│   ├── token/            [implemented] CLK_<TYPE>_<id>, HMAC, normalize, local key
│   ├── mapstore/         [implemented] token<->value reverse map (State), scoped in-mem
│   ├── wire/             [implemented] internal provider-native JSON adapters
│   │   ├── anthropic/    [implemented] /v1/messages  (Phase 0)
│   │   ├── openairesponses/   [implemented] /v1/responses (Codex Responses; live acceptance pending)
│   │   ├── openaichat/        [planned]  /v1/chat/completions (Phase 1)
│   │   └── gemini/            [planned]  generateContent (Phase 1)
│   ├── stream/           [implemented] chunk-level (byte-split tolerant) + SSE-event
│   ├── proxy/            [implemented] standalone base-URL proxy (localhost, pass-through)
│   ├── console/          [scaffold] LOCAL single-user web console (localhost)
│   ├── config/           [scaffold] local PolicyProvider default
│   └── service/          [phase1 scaffold] HTTP/gRPC service (Phase 1)
│
├── cmd/opencloak/        [partial] proxy implemented; serve | console | mask are placeholders
└── examples/embed/       [implemented]      maintained SDK reference integration
```

`implemented` means behavior exists with automated fixtures; `partial` means only the named
subcommands are live; `scaffold` means the package boundary exists but does not yet claim
behavior. Phase 0 is accepted for the standalone Claude Code proxy after the exit criteria
in the roadmap pass against a real Claude Code flow.

## Component responsibilities

| Package | Responsibility |
|---|---|
| `opencloak` (root) | Public API: `Engine`, `New`, Text/Wire/Stream methods, `Scope`/`State`, `Finding`, policy operators, and extension interfaces. The only package external code imports. |
| `internal/types` | Shared data types (`Finding`,`Scope`,`Type`,`Policy`,operators); the root package re-exports them as transparent aliases — breaks the root↔internal import cycle while keeping the public API byte-identical. |
| `internal/detect` | Run the configured detector layers; emit `Finding` values; enforce fail-closed. |
| `internal/detect/l1` | Pattern detection: built-in regex rules, Shannon entropy + context keywords, checksums/validators (Luhn, IBAN, date parsing). |
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

`internal/wire` is not a public plugin API in v0.1.0. It is an internal boundary that
keeps provider-native JSON walking out of the root package while OpenCloak proves the loop
with maintained Anthropic Messages and OpenAI Responses adapters. Phase 1+ may add
OpenAI Chat and Gemini. A public third-party adapter registration API should be added only
when there is a real external adapter use case; until then, the stable public contract is
`MaskRequest(ctx, scope, provider, op, body)` plus the documented provider tags.

## Phase 0 cut

**Implement:** `token`, scoped `mapstore`/`State`, `detect/l1` (with built-in starter
rules), finding conflict resolution, per-type transform operators, `mask`,
`wire/anthropic`, provider-aware buffered/SSE restore, `stream` (raw chunk-level), the root
façade methods, `internal/proxy` (Claude Code endpoint), and `opencloak proxy`.
Validate the standalone proxy against the end-to-end task in the
[roadmap](../product/roadmap.md). The real-gateway embed validation is Phase 1 hardening.

**Defer (Phase 1):** L2 detector, the non-Anthropic `wire` providers, `service`
(HTTP/gRPC), and the local `console` beyond a minimal status view.

## Build status

Phase 0 is accepted for the standalone Claude Code proxy path: `gofmt` clean,
`go vet ./...` clean, `go build ./...` ok, `go test ./...` and `go test -race ./...`
pass, the binary help path runs, and the live Claude Code acceptance report is recorded in
[phase-0-acceptance.md](phase-0-acceptance.md). Implemented scope covers the text engine,
Anthropic Messages buffered wire, streaming restore, and loopback proxy. R2 release
hardening adds the maintained `examples/embed` SDK reference integration outside the
standalone proxy. R3 adds offline-verified OpenAI Responses provider support for Codex;
the live Codex acceptance run remains a release gate. OpenAI Chat, Gemini, service, and
console remain Phase 1+.
