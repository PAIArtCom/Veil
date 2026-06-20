# Token Specification

**Status:** Accepted (normative). Decision rationale:
[ADR-0003](../architecture/decisions/0003-deterministic-reversible-token-spec.md),
with the namespace prefix updated by
[ADR-0014](../architecture/decisions/0014-opencloak-token-prefix.md).

## Format

```
OpenCloak_<TYPE>_<id>
```

| Part | Meaning |
|---|---|
| `OpenCloak_` | Namespace prefix — greppable, collision-resistant, brand-specific, unmistakable. |
| `<TYPE>` | Uppercase category code (see enum). Lets handling branch and restore classify. |
| `<id>` | First **12** hex chars of `HMAC-SHA256(normalize(value), local_key)`. |

Restore pattern: `OpenCloak_[A-Z0-9]+_[0-9a-f]{12,}`. Implementations should scan for the
fixed `OpenCloak_` structure and validate type/id segments; do not rely solely on
word-boundary regex matching, because a token may sit next to identifier characters.

Example: `sk-live-9f8a7b6c…` → `OpenCloak_SECRET_7f3a9c2e1b8d`

## Type enum (initial)

`SECRET` · `EMAIL` · `PHONE` · `IPV4` · `IPV6` · `CARD` · `ACCT` · `URL` · `DATE` ·
`PERSON` · `ADDR`

`SECRET` covers API keys, tokens, passwords, private keys, connection strings.
`PERSON`/`ADDR` are L2 (semantic) types, off by default — see
[detection layers](detection-layers.md).

## Properties (required)

- **Deterministic.** Same `normalize(value)` → same token for a given local key, with no
  randomness and no request dependence. This keeps prompt caches warm and multi-turn
  context coherent.
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
fake value, deterministically derived) may be used instead of the opaque `OpenCloak_…`
form.
This is an opt-in refinement, not the default, because it complicates a single uniform
restore scanner. The `OpenCloak_` scanner only covers `OperatorToken`; format-preserving
operators need type-specific reverse strategies and fixtures.

`OperatorRedact` is intentionally irreversible and must not write a reversible mapping.
It is useful for policy choices where local restoration is not required, but it cannot
support agent tool execution that needs the real value.
