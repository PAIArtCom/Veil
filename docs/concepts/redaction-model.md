# Redaction Model

**Status:** Accepted (normative)

This is the conceptual contract for *what* OpenCloak transforms and *where*. It follows
directly from the [threat model](../architecture/threat-model.md): only the network egress
to the LLM matters; everything local is trusted.

## Two transformation points

```
                 ┌──────────── local (trusted) ────────────┐   ┌─ network (untrusted) ─┐
 dev tool ──────▶│                                          │   │                       │
                 │   request with real secrets & PII        │   │                       │
                 │                  │                       │   │                       │
                 │                  ▼  [MASK]   value→token │   │                       │
                 │                  └──────────────────────────▶│  LLM provider         │
                 │                                          │   │   (sees only tokens)  │
                 │                  ┌──────────────────────────◀│                       │
                 │                  ▼  [RESTORE] token→value │   │                       │
 dev tool ◀──────│   response & tool-calls with real values │   │                       │
                 └──────────────────────────────────────────┘   └───────────────────────┘
```

- **MASK** on every payload going *to* the LLM: the initial prompt, every later turn, and
  **tool results fed back** (a tool's output may contain a new secret).
- **RESTORE** on every payload coming *from* the LLM: assistant text and tool-call
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
produced it. The engine created those token→value entries while masking that same request.
Therefore the reverse map always contains what a response needs — independent of scope or
restarts. This is the property that lets the map be a simple in-memory cache
([ADR-0005](../architecture/decisions/0005-global-in-memory-scope.md)).

## Orphan tokens

If the model mangles a token (splits it, re-encodes it) restore may miss it, leaving a
`CLK_…` literal in output. Two mitigations: the [token form](token-spec.md) is
identifier-safe (it does not break code syntax if it lands), and an egress scan flags any
residual tokens so the failure is visible, not silent.
