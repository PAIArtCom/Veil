# ADR-0003 — Deterministic, reversible, type-aware token spec

**Status:** Accepted

## Context

What replaces a sensitive value in the outbound payload determines whether agents and
prompt caches survive de-identification:

- **Irreversible redaction** (delete the value) is safe but breaks agentic tool-use — the
  agent can no longer run the command that needs the real value.
- **Random per-occurrence tokens** break prompt caching and multi-turn coherence, because
  the same value yields different bytes each time.

We need replacement that is reversible, cache-stable, distinguishable by type (so handling
can branch and restore can find tokens), and robust if it accidentally lands in code.

## Decision

Use the token form **`CLK_<TYPE>_<id>`**:

- `CLK_` namespace prefix — greppable, collision-resistant, unmistakable.
- `<TYPE>` ∈ `SECRET | EMAIL | PHONE | IPV4 | IPV6 | CARD | PERSON | ADDR | URL | DATE | ACCT | …`
- `<id>` = first 12 hex of `HMAC-SHA256(normalize(value), local_key)`.

Properties: **deterministic** (same value → same token, no session dependence → caches
stay warm), **type-aware**, **bijective** (token→value via the local map; collisions
avoided by length and a check-and-extend on insert), **identifier-safe** (matches
`[A-Za-z_][A-Za-z0-9_]*`, so it survives landing in source without breaking syntax and is
preserved well by models). Restore scans for the fixed `CLK_[A-Z0-9]+_[0-9a-f]{12}`
structure and validates type/id segments; it must not rely solely on word-boundary regex
matching. Full normative detail in the [token spec](../../concepts/token-spec.md).

## Alternatives considered

- **Random/UUID tokens.** Rejected: break prompt caching and consistency.
- **Type-only labels** (`<EMAIL>`, Presidio-style). Rejected: not bijective — collapses
  distinct values, so restore cannot recover the original.
- **Format-preserving replacement** (swap a phone for another valid-looking phone).
  Retained as an *optional per-type* refinement (better for the model's format
  validation), not the default — it complicates a single uniform restore regex.

## Consequences

- The forward direction is stateless (pure HMAC); only the reverse map needs storage,
  enabling the scoped in-memory mapstore in [ADR-0009](0009-state-lifecycle-and-scope.md).
- A stable **`local_key`** (e.g. `~/.veil/key`, generated once) is the root of
  determinism and must persist across restarts.
- The identifier-safe form bounds the blast radius of an orphaned token (valid syntax, not
  broken output); residual-token scans still flag leftover `CLK_` tokens.
