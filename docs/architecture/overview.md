# Architecture Overview

**Status:** Accepted

## Core principle: engine / transport split

"Gateway filtering" and "an SDK for other gateways" are **the same engine in different
shells.** The engine is transport-agnostic, pure logic. A standalone proxy is just one
adapter on top of it.

```
        ┌───────────────────────────────────────────────────────┐
        │   OpenCloak engine  (open core · transport-agnostic)    │
        │   detect (L1/L2) · deterministic token · mask/restore   │
        │   · state                                                │
        └───────────────────────────────────────────────────────┘
            ▲ import        ▲ import        ▲ plugin      ▲ HTTP/gRPC
        ┌───┴────────┐  ┌────┴────────┐  ┌──┴────────┐  ┌─┴──────────┐
        │ standalone │  │ your gateway │  │ CLIProxy- │  │ non-Go     │
        │ proxy      │  │ (clipal /    │  │ API       │  │ gateway    │
        │ (Claude/   │  │  Orbit / …)  │  │ (plugin)  │  │ (sidecar)  │
        │  Codex)    │  │              │  │           │  │            │
        └────────────┘  └─────────────┘  └───────────┘  └────────────┘
```

This answers the central product question — "do we build a gateway or an SDK?" — with
*both, from one core.* See [ADR-0002](decisions/0002-engine-transport-split.md).

## The data flow

Two transformation points; everything local is untouched. See the
[redaction model](../concepts/redaction-model.md) for the full reasoning.

```
dev tool ──request(real)──▶ [MASK outbound] ──request(tokens)──▶ LLM
dev tool ◀──response(real)── [RESTORE inbound] ◀──response(tokens)── LLM
```

- **Mask** runs on every payload going *to* the LLM (initial prompt, later turns, tool
  results fed back).
- **Restore** runs on every payload coming *from* the LLM (assistant text, tool-call
  arguments).
- Tool execution, file writes, and terminal display happen locally with **real** values —
  by design, and within the user's existing trust boundary.

## Components

| Component | Responsibility | Notes |
|---|---|---|
| **Detector** | Find sensitive findings | Layered: L1 patterns now, L2 NER later. Findings include confidence and source. [Spec](../concepts/detection-layers.md) |
| **Resolver** | Merge and de-overlap findings | Same-type merge; cross-type precedence before any replacement. [ADR-0008](decisions/0008-finding-model-and-conflict-resolution.md) |
| **Tokenizer** | Map value ↔ token, deterministically | `CLK_<TYPE>_<id>`. [Spec](../concepts/token-spec.md) |
| **Masker** | Apply token strategy to resolved findings | Offset-safe replacement and token→value mapping writes |
| **State** | Hold token→value reverse mappings for restore | Explicit request/stream handle with scoped in-memory namespaces. [ADR-0009](decisions/0009-state-lifecycle-and-scope.md) |
| **Wire adapters** | Walk each provider's request/response JSON | OpenCloak-maintained internal adapters at first: Anthropic Messages live-accepted; OpenAI Responses offline-verified for Codex with live acceptance pending. OpenAI Chat and Gemini are later. Native shapes, no unified IR; buffered/SSE restore is provider-aware. |
| **Stream restorer** | Restore tokens in raw streaming responses | Provider-agnostic byte holdback for chunks split across token boundaries |
| **Transports** | Expose the engine | Standalone proxy, embeddable library, HTTP/gRPC service, local web console |
| **Seams** | Extension points the commercial control plane attaches to | `PolicyProvider` (config/rules), `AuditSink` (minimized audit) — local defaults in the OSS engine |

For the concrete Go module layout, package responsibilities, and dependency direction, see
the [system design](system-design.md) and [ADR-0007](decisions/0007-code-and-module-layout.md).

## Integration contract

The engine is consumed as a **small library API** with three API surfaces (Text, Wire,
Stream). It does not assume a host plugin system, a unified
message schema, or that streaming arrives pre-parsed — because real gateways provide none
of those uniformly. The contract and the evidence behind it are in the
[SDK contract](../sdk/contract.md) and the
[gateway integration survey](../research/gateway-integration-survey.md).

The Text/Wire/Stream names are separate from the detection-layer names: L1 remains the
pattern detector and L2 remains the optional local NER layer.

## Attach mechanism (standalone proxy)

In-process CLI hooks **cannot** rewrite the prompt sent to the LLM, so the standalone
transport is a base-URL local proxy: `ANTHROPIC_BASE_URL` for Claude Code and a custom
`model_providers` entry for Codex/OpenAI Responses.
Credentials pass through unchanged; only the JSON body is rewritten; the proxy binds to
`127.0.0.1` only. Rationale and evidence:
[ADR-0001](decisions/0001-base-url-proxy-over-hooks.md) and
[ADR-0004](decisions/0004-auth-pass-through.md).

## Technology stack

- **Engine + L1:** Go. The L1 layer reuses the proven pattern/entropy approach of the
  `privacy-filter` project, with explicit finding conflict resolution before masking.
- **Local policy:** strict JSON file loading for `token`, `ignore`, and `block`; reserved
  operators and non-empty rule sets fail closed.
- **L2 (Phase 1):** a local NER model served via ONNX or a sidecar — pluggable and
  optional, so the L1-only deployment stays small and fast.
