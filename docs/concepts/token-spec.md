# Token Specification

**Status:** Accepted (normative). Decision rationale:
[ADR-0003](../architecture/decisions/0003-deterministic-reversible-token-spec.md),
with the namespace prefix updated by
[ADR-0014](../architecture/decisions/0014-veil-token-prefix.md).

## Format

Most sensitive types use the opaque token format:

```
PAIArtVeil_<TYPE>_<id>
```

| Part | Meaning |
|---|---|
| `PAIArtVeil_` | Namespace prefix — greppable, collision-resistant, brand-specific, unmistakable. |
| `<TYPE>` | Uppercase category code (see enum). Lets handling branch and restore classify. |
| `<id>` | First **12** hex chars of `HMAC-SHA256(normalize(value), local_key)`. |

Restore pattern: `PAIArtVeil_[A-Z0-9]+_[0-9a-f]{12,}`. Implementations should scan for the
fixed `PAIArtVeil_` structure and validate type/id segments; do not rely solely on
word-boundary regex matching, because a token may sit next to identifier characters.
Because the id suffix is hex and collision extension can lengthen it, restore
implementations also check the local reverse map for the longest known token prefix. If a
known token is immediately followed by additional hex text, only the known prefix is
restored as a token; the suffix remains ordinary text and is eligible for masking on a
later outbound pass.
Unknown token-shaped text is not trusted as store-resident beyond the minimal residual
prefix on outbound masking. A substantial lowercase-hex suffix after an unknown
`PAIArtVeil_`-shaped prefix is treated as candidate `SECRET` text rather than as protected
token id space.

Example: `sk-live-9f8a7b6c…` → `PAIArtVeil_SECRET_7f3a9c2e1b8d`

EMAIL findings and sensitive URL findings use deterministic format-preserving surrogates
under the `veil.paiart.com` domain family so the model still sees URL/email grammar.
Sensitive URLs include database/cache connection strings and HTTP(S) URLs with userinfo
or sensitive query keys; ordinary public/reference HTTP(S) links are not masked by
default:

```
user-<id>@veil.paiart.com
https://api-<id>.veil.paiart.com/...
postgresql://user-<id>:password-<id>@db-<id>.veil.paiart.com:5432/...
```

IPv4 and IPv6 findings use IP-shaped surrogates instead of opaque tokens so the model
still sees address grammar:

```
10.<id-byte>.<id-byte>.<host>       # original 10/8 private IPv4
192.168.<id-byte>.<host>           # original 192.168/16 private IPv4
203.0.113.<host>                   # original public/other IPv4
fd00:<id-hex>:<id-hex>::<id-hex>   # original private IPv6
fe80::<id-hex>:<id-hex>:<id-hex>   # original link-local IPv6
2001:db8:<id-hex>:<id-hex>::<id-hex> # original public/other IPv6
```

The `<id>` is derived from the same local-keyed HMAC material as the opaque token. These
surrogates are still reversible only through the scoped local reverse map; the real email,
host, userinfo, IP address, and sensitive query values do not cross provider egress.
If a generated format-preserving surrogate is already owned by a different value in the
same scope, Veil falls back to the opaque `PAIArtVeil_<TYPE>_<id>` token for that value
rather than overwriting the existing reverse mapping.

## Type enum (initial)

`SECRET` · `EMAIL` · `PHONE` · `IPV4` · `IPV6` · `CARD` · `ACCT` · `URL` · `DATE` ·
`PERSON` · `ADDR`

`SECRET` covers API keys, tokens, passwords, private keys, connection strings.
`PERSON`/`ADDR` are L2 (semantic) types, off by default — see
[detection layers](detection-layers.md).

## Properties (required)

- **Deterministic.** Same `normalize(value)` → same placeholder for a given local key,
  with no randomness and no request dependence. This keeps prompt caches warm and
  multi-turn context coherent.
- **Type-aware.** The embedded `TYPE` allows per-type handling (e.g. default-off for
  `PERSON`) and classification on restore. Format-preserving surrogates carry type through
  their grammar and map entry rather than through a `PAIArtVeil_` prefix.
- **Bijective.** `placeholder → value` resolves via the in-memory map. Truncation to 12
  hex (48 bits) keeps collisions negligible; on the rare insert-time collision (different
  value, same id) the engine extends the id length for that entry.
- **Identifier-safe.** The token matches `[A-Za-z_][A-Za-z0-9_]*`, so if it ever lands in
  generated code it is a valid identifier — it will not break syntax. Models also preserve
  such opaque identifiers reliably across a round trip.

## `normalize(value)`

A conservative, type-specific normalization so trivially-different representations of the
same value map consistently (e.g. trim surrounding whitespace; lowercase the domain part
of an email). Normalization must never be aggressive enough to merge *distinct* secrets.

## The local key

`<id>` is keyed by a **local secret** (`local_key`), generated once per install and stored
locally (e.g. `~/.veil/key`). It is:

- the **root of determinism** — it must be stable across restarts, or tokens would change;
- **defense in depth** — an observer of tokens alone cannot recover values without it
  (though the real protection is that values never leave the machine).

## Format-preserving variants

For EMAIL, IPv4, IPv6, and sensitive URL findings, Veil's default `OperatorToken` emits
the format-preserving surrogates above instead of opaque `PAIArtVeil_…` tokens. This
keeps model-visible text closer to the original grammar while preserving the same local
reversibility boundary. Ordinary public/reference HTTP(S) links are not findings under
the default L1 URL rule.

Additional per-type format-preserving policy operators remain reserved for Phase 1. They
need type-specific reverse strategies, stream holdback rules, fixtures, and policy
contracts before they can be exposed as configurable behavior.

`OperatorRedact` is intentionally irreversible and must not write a reversible mapping.
It is useful for policy choices where local restoration is not required, but it cannot
support agent tool execution that needs the real value.
