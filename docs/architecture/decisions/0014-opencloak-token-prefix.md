# ADR-0014 - Use OpenCloak token namespace prefix

**Status:** Accepted

## Context

ADR-0003 defined the deterministic reversible token form as `CLK_<TYPE>_<id>`. That shape
proved the core requirements: deterministic HMAC-derived ids, type-aware restore,
identifier-safe output, and a greppable namespace. Before the first public OSS release,
the project needs the visible token namespace to reinforce the OpenCloak product identity
when tokens appear in prompts, logs, demos, and user-facing examples.

The prefix change must not weaken the token contract or rewrite historical acceptance
evidence. Existing Phase 0 and Codex live acceptance records that mention `CLK_` remain
truthful for the builds that were tested at the time.

## Decision

For v0.1.0 and later, OpenCloak uses this token form:

```text
OpenCloak_<TYPE>_<id>
```

This ADR supersedes ADR-0003 only for the namespace prefix. All other ADR-0003 semantics
remain active:

- `<TYPE>` remains an uppercase category code.
- `<id>` remains the first 12 or more lowercase hex characters of
  `HMAC-SHA256(normalize(value), local_key)`, collision-extended if needed.
- Tokens remain deterministic, reversible via scoped local state, type-aware, and
  identifier-safe.
- Restore and residual-token scanners must use the new `OpenCloak_` grammar and must
  tolerate tokens split across streaming byte or event boundaries.

## Alternatives considered

- **Keep `CLK_`.** Rejected: it is short and technical, but it does not carry the product
  name in the most visible runtime artifact.
- **Use `OC_`.** Rejected: shorter but less distinctive and more likely to collide with
  ordinary identifiers than the full product namespace.
- **Support both prefixes indefinitely.** Rejected for v0.1.0: there is no tagged release
  compatibility burden yet, and accepting the old namespace would enlarge the residual
  scanner and idempotent masking surface without a clear user benefit.

## Consequences

- Active code, tests, token specs, SDK docs, and release guides describe
  `OpenCloak_<TYPE>_<id>`.
- Accepted historical ADRs and acceptance reports may still contain `CLK_` examples; those
  are historical records, not the current v0.1.0 token namespace.
- Any final release-candidate live acceptance after this ADR must verify provider-bound
  payloads contain `OpenCloak_` tokens, not `CLK_` tokens.
