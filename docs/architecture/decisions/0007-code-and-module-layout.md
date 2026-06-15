# ADR-0007 — Code & module layout

**Status:** Accepted

## Context

With the architecture decided (ADR-0001..0006) and the [open-core boundary](../../product/open-core-boundary.md)
fixed, we need a concrete Go module layout. Inputs confirmed with the maintainer:

- Module path **`github.com/cloakia/opencloak`** (Cloakia is the GitHub org; OpenCloak is
  the open-source repository under it).
- The **public SDK lives at the module root package** (`opencloak`).
- A **single binary with subcommands** (`opencloak proxy|serve|console|mask`).
- **Cloakia is a separate private repository.**
- L1 detection merges **both** rule sources (privacy-filter + gitleaks).
- A **local, single-user web console** belongs in OpenCloak (open); the multi-tenant
  management/audit console belongs in Cloakia.

## Decision

The root package `opencloak` is the public API (the `Engine` + Text/Wire/Stream methods +
the extension interfaces). All implementation lives under `internal/`. Transports and the
CLI live under `cmd/opencloak`.

```
opencloak/                       module github.com/cloakia/opencloak
├── opencloak (root package)     PUBLIC API
│   ├── doc.go  types.go  interfaces.go  engine.go
│   │   └── Engine, New, Text/Wire/Stream methods
│   │       Detector · PolicyProvider · AuditSink · Config · Scope · State
│   │       Type · Finding · TransformOperator · TypePolicy · Policy
├── internal/
│   ├── detect/        detection pipeline (fail-closed)
│   │   ├── l1/        regex + entropy + Luhn + context; rules = privacy-filter + gitleaks
│   │   └── resolver/  Finding merge/de-overlap and precedence
│   ├── mask/          offset-safe replacement + token mapping writes
│   ├── token/         CLK_<TYPE>_<id> + HMAC + key mgmt
│   ├── mapstore/      token<->value reverse map (State); scoped in-memory
│   ├── wire/          internal native-JSON provider adapters; provider-aware restore
│   ├── stream/        streaming restore (chunk-level + SSE-event-level)
│   ├── proxy/         standalone base-URL proxy (localhost; auth pass-through)
│   ├── console/       LOCAL single-user web console (localhost)
│   ├── config/        local PolicyProvider default
│   └── service/       HTTP/gRPC service (Phase 1)
└── cmd/opencloak/     single binary: proxy | serve | console | mask
```

Two extension interfaces are the **seams** between open and commercial code:

- `PolicyProvider` — local-file default in OSS; Cloakia pushes/hot-reloads scoped central
  policy.
- `AuditSink` — no-op/local default in OSS; Cloakia collects minimized audit data.

The **local web console** and Cloakia's **org console** sit on these same two seams (local
vs remote implementations) — so the console split does **not** cross the open-core
boundary.

## Alternatives considered

- **Public API under `pkg/`.** Rejected: a root-package import
  (`github.com/cloakia/opencloak`) is cleaner and idiomatic; `internal/` already hides
  implementation.
- **Multiple binaries.** Rejected: one binary with subcommands is simpler to distribute.
- **Monorepo including Cloakia.** Rejected: commercial code stays out of the Apache-2.0
  repo; Cloakia depends on OpenCloak and implements the two interfaces.
- **Public third-party provider adapter registry in Phase 0.** Rejected for now: the SDK
  contract must be stable first, and OpenCloak should prove the loop with maintained
  adapters before freezing a plugin API. The public contract remains provider-tagged
  native JSON; `internal/wire` is an implementation boundary, not a public extension
  point yet.

## Consequences

- The engine is importable as `github.com/cloakia/opencloak`; gateways embed it directly.
- The OSS repo never imports commercial code; the dependency points one way (Cloakia →
  OpenCloak).
- `internal/` keeps the public surface small and stable — only the root package is API.
- The scaffold is in place and compiles (`go build ./...`, `go vet` clean); method bodies
  are Phase 0 work. Full design: [system-design.md](../system-design.md).
