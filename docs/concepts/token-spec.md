# Token Specification

**Status:** Accepted (normative). Decision rationale: [ADR-0003](../architecture/decisions/0003-deterministic-reversible-token-spec.md).

## Format

```
CLK_<TYPE>_<id>
```

| Part | Meaning |
|---|---|
| `CLK_` | Namespace prefix — greppable, collision-resistant, unmistakable. |
| `<TYPE>` | Uppercase category code (see enum). Lets handling branch and restore classify. |
| `<id>` | First **12** hex chars of `HMAC-SHA256(normalize(value), local_key)`. |

Restore matches: `\bCLK_[A-Z]+_[0-9a-f]{12}\b`

Example: `sk-live-9f8a7b6c…` → `CLK_SECRET_7f3a9c2e1b8d`

## Type enum (initial)

`SECRET` · `EMAIL` · `PHONE` · `IPV4` · `IPV6` · `CARD` · `ACCT` · `URL` · `DATE` ·
`PERSON` · `ADDR`

`SECRET` covers API keys, tokens, passwords, private keys, connection strings.
`PERSON`/`ADDR` are L2 (semantic) types, off by default — see
[detection layers](detection-layers.md).

## Properties (required)

- **Deterministic.** Same `normalize(value)` → same token, globally, with no randomness and
  no session dependence. This keeps prompt caches warm and multi-turn context coherent.
- **Type-aware.** The embedded `TYPE` allows per-type handling (e.g. default-off for
  `PERSON`) and classification on restore.
- **Bijective.** `token → value` resolves via the in-memory map. Truncation to 12 hex (48
  bits) keeps collisions negligible; on the rare insert-time collision (different value,
  same id) the engine extends the id length for that entry.
- **Identifier-safe.** The token matches `[A-Za-z_][A-Za-z0-9_]*`, so if it ever lands in
  generated code it is a valid identifier — it will not break syntax. Models also preserve
  such opaque identifiers reliably across a round trip.

## `normalize(value)`

A conservative, type-specific normalization so trivially-different representations of the
same value map consistently (e.g. trim surrounding whitespace; lowercase the domain part
of an email). Normalization must never be aggressive enough to merge *distinct* secrets.

## The local key

`<id>` is keyed by a **local secret** (`local_key`), generated once per install and stored
locally (e.g. `~/.opencloak/key`). It is:

- the **root of determinism** — it must be stable across restarts, or tokens would change;
- **defense in depth** — an observer of tokens alone cannot recover values without it
  (though the real protection is that values never leave the machine).

## Optional: format-preserving variant

For structured types where the model benefits from a realistic shape (e.g. validating a
phone or email format), a per-type *format-preserving* replacement (a valid-looking but
fake value, deterministically derived) may be used instead of the opaque `CLK_…` form.
This is an opt-in refinement, not the default, because it complicates a single uniform
restore regex.
