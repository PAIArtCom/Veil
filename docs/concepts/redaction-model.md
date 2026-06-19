# Redaction Model

**Status:** Accepted (normative)

This is the conceptual contract for *what* OpenCloak transforms and *where*. It follows
directly from the [threat model](../architecture/threat-model.md): the protected text and
tool-I/O surface crosses network egress to the LLM, while everything local is trusted.
Opaque media/document payloads and provider thinking/control traces preserve
provider-native semantics and are not part of the v0.1.0 replacement surface.

## Two transformation points

```
                 ┌──────────── local (trusted) ────────────┐   ┌─ network (untrusted) ─┐
 dev tool ──────▶│                                          │   │                       │
                 │   protected text/tool fields             │   │                       │
                 │   with real secrets & PII                │   │                       │
                 │                  │                       │   │                       │
                 │                  ▼  [MASK]   value→token │   │                       │
                 │                  └──────────────────────────▶│  LLM provider         │
                 │                                          │   │   (protected fields   │
                 │                                          │   │    contain tokens)    │
                 │                  ┌──────────────────────────◀│                       │
                 │                  ▼  [RESTORE] token→value │   │                       │
 dev tool ◀──────│   response & tool-calls with real values │   │                       │
                 └──────────────────────────────────────────┘   └───────────────────────┘
```

- **MASK** on supported text/tool-I/O payloads going *to* the LLM: initial prompt text,
  later text turns, and **tool results fed back** (a tool's output may contain a new
  secret).
- **RESTORE** on supported payloads coming *from* the LLM: assistant text and tool-call
  arguments.
- Local actions — tool execution, file writes, terminal display — run with **real**
  values. That is intended: the data is the user's own, on the user's own machine, within
  the existing trust boundary.

## Why reversible (not just redacted)

Irreversible redaction is safer but breaks agents: the agent can no longer execute the
command that needs the real value. Reversibility — via a deterministic, type-aware
[token](token-spec.md) — is what lets agentic tool-use *and* prompt caching survive
de-identification. The cost (a local reverse map) stays entirely within the trusted local
boundary.

## Restore is easy where it must be exact, hard where it is cosmetic

- **Tool-call arguments** are accumulated to a *complete* value before the agent executes
  the tool (verified in both Claude Code and Codex). So restoring tokens in tool args is a
  **buffered, full-string replace** — exact and simple, and it is the correctness-critical
  path.
- **Streamed assistant text** is the only true streaming case. We restore it to real
  values (decided), which requires a **hold-back buffer** so a token split across two
  stream chunks is not emitted half-restored. This path is about display quality, not
  correctness.

## The completeness guarantee

A response can only reference tokens for values that were present in the request that
produced it. The engine created those token→value entries while masking that same request,
so the matching live `State` should contain what that response needs. This guarantee is
scoped: restore must use the same request/stream state and namespace. Across another
tenant/session/project scope, or after a restart without an explicit persistent cache,
restore may leave a residual token or surface an error; it must never consult another
scope to make restoration succeed ([ADR-0009](../architecture/decisions/0009-state-lifecycle-and-scope.md)).

## Orphan tokens

If the model mangles a token (splits it, re-encodes it) restore may miss it, leaving a
`CLK_…` literal in output. Two mitigations: the [token form](token-spec.md) is
identifier-safe (it does not break code syntax if it lands), and residual-token scans flag
missed restores so the failure is visible, not silent.
