# Detection Layers

**Status:** Accepted (L1 normative; L2 planned). MVP scope: [ADR-0006](../architecture/decisions/0006-l1-only-mvp-defer-l2.md).

Detection finds sensitive `Finding` values that the resolver de-overlaps before the
[tokenizer](token-spec.md) masks them. It is layered so the common, high-value case needs
no model.

## L1 — Patterns (MVP)

Fast, deterministic, no model. Catches data with **structure**.

| Technique | Catches |
|---|---|
| Built-in regex rules (gitleaks-style) | API keys, provider tokens, private keys |
| Shannon entropy + context keywords | High-entropy secrets near `password`/`token`/`key` |
| Checksums (e.g. Luhn) | Credit-card numbers, validated identifiers |
| Structured patterns | Emails, phones, IPv4/IPv6, connection strings, URLs |

Context keywords reduce false positives: in Phase 0, a high-entropy blob is only treated
as a secret when a keyword (`secret`, `token`, `apikey`, …) sits nearby. Isolated
high-entropy strings are not flagged; strict bare high-entropy fallback is Phase 1.

**Categories → token TYPE:** `SECRET`, `EMAIL`, `PHONE`, `IPV4`/`IPV6`, `CARD`, `ACCT`,
`URL`, `DATE`.

**Phase 0 coverage notes (deliberate limits):**

- **IPV6** is intentionally *partial*. A bare compressed form like `::1` or `a::b` is textually
  identical to language scope/path syntax (`std::vector`, `crate::module`, `a::b`), so masking
  it would corrupt source code — the traffic this tool protects. Phase 0 masks an IPv6 literal
  only when it has a hextet of ≥3 hex digits (e.g. `2001:db8::1`, `fe80::1`); tiny ambiguous
  forms are left untouched. Context-aware IPv6 is Phase 1.
- **ACCT** covers checksum-validated **IBAN** (ISO 13616 mod-97) only; other account-number
  formats are Phase 1.
- **DATE** is detected but **ignored by the default policy** — most dates are not sensitive and
  masking them all hurts model utility. A caller/Cloakia policy can opt in per type (the same
  way `PERSON`/`ADDR` are off by default).

Every L1 detector emits `Finding{Start, End, Type, Score, Source}`. Candidates that require
validation (for example Luhn checks) are dropped if validation fails. Entropy can validate
or rank a regex candidate, and it can also originate a finding for an otherwise-unmatched
high-entropy value when contextual controls pass. Overlaps are resolved centrally before
masking; individual detectors do not get to decide global precedence.

## L2 — Local NER (planned, Phase 1, opt-in)

Catches **unstructured** PII — `PERSON`, `ADDR`, organizations — which no regex matches and
which require a model that reads context. Because it is a neural network on the request
path, it is **opt-in** and runs only on each turn's *new* content to bound latency. Person
detection is **default-off** and user-configurable.

Candidate models (to evaluate): GLiNER (small, fast on CPU — pragmatic default); the
OpenAI Privacy Filter token-classifier (exact category fit, higher accuracy, heavier);
spaCy (lightweight baseline).

### Consistency hazard (L2-specific)

A model may catch "Jane Doe" on turn 1 but miss it on turn 5, which would both leak and
break that entity's cache. Resolution: **the model only *discovers* entities; the
deterministic map *enforces* a stable pseudonym thereafter** — once an entity is masked,
every later occurrence is replaced by deterministic string match regardless of whether the
model re-detects it.

## Pluggable by design

The detector is an interface from day one. L1 is the built-in implementation; L2 attaches
as an optional detector without touching the core. Coverage is always an explicit,
documented trade-off (per-type on/off; configurable rule sets are Phase 1) — never a
silent guarantee. Detection runs **fail-closed**: on engine error, block rather than
forward ([threat model](../architecture/threat-model.md)).
