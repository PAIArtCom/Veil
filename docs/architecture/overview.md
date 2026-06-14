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
| **Detector** | Find sensitive spans | Layered: L1 patterns now, L2 NER later. [Spec](../concepts/detection-layers.md) |
| **Tokenizer** | Map value ↔ token, deterministically | `CLK_<TYPE>_<id>`. [Spec](../concepts/token-spec.md) |
| **State** | Hold the token→value reverse map | Process-global in-memory by default; optional explicit handle |
| **Wire adapters** | Walk each provider's request/response JSON | Anthropic Messages, OpenAI Responses/Chat, Gemini — native shapes, no unified IR |
| **Stream restorer** | Restore tokens in streaming responses | Stateful, tolerant of tokens split across byte boundaries |
| **Transports** | Expose the engine | Standalone proxy, embeddable library, HTTP/gRPC service |

## Integration contract

The engine is consumed as a **library of pure functions** with a layered API (text level,
wire-format level, streaming level). It does not assume a host plugin system, a unified
message schema, or that streaming arrives pre-parsed — because real gateways provide none
of those uniformly. The contract and the evidence behind it are in the
[SDK contract](../sdk/contract.md) and the
[gateway integration survey](../research/gateway-integration-survey.md).

## Attach mechanism (standalone proxy)

In-process CLI hooks **cannot** rewrite the prompt sent to the LLM, so the standalone
transport is a base-URL local proxy: `ANTHROPIC_BASE_URL` for Claude Code, a custom
`model_providers` entry for Codex. Credentials pass through unchanged; only the JSON body
is rewritten; the proxy binds to `127.0.0.1` only. Rationale and evidence:
[ADR-0001](decisions/0001-base-url-proxy-over-hooks.md) and
[ADR-0004](decisions/0004-auth-pass-through.md).

## Technology stack

- **Engine + L1:** Go. The L1 layer reuses the proven pattern/entropy approach of the
  `privacy-filter` project.
- **L2 (Phase 1):** a local NER model served via ONNX or a sidecar — pluggable and
  optional, so the L1-only deployment stays small and fast.
