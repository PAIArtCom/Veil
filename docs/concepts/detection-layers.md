# Detection Layers

**Status:** Accepted (L1 normative; L2 planned). MVP scope: [ADR-0006](../architecture/decisions/0006-l1-only-mvp-defer-l2.md).

Detection finds the sensitive spans that the [tokenizer](token-spec.md) then masks. It is
layered so the common, high-value case needs no model.

## L1 — Patterns (MVP)

Fast, deterministic, no model. Catches data with **structure**.

| Technique | Catches |
|---|---|
| Regex rule sets (gitleaks-style) | API keys, provider tokens, private keys |
| Shannon entropy + context keywords | High-entropy secrets near `password`/`token`/`key` |
| Checksums (e.g. Luhn) | Credit-card numbers, validated identifiers |
| Structured patterns | Emails, phones, IPv4/IPv6, connection strings, URLs |

Context keywords reduce false positives: a high-entropy blob is only treated as a secret
when a keyword (`secret`, `token`, `apikey`, …) sits nearby; an isolated high-entropy
string is down-weighted. L1 reuses the proven approach of the `privacy-filter` project.

**Categories → token TYPE:** `SECRET`, `EMAIL`, `PHONE`, `IPV4`/`IPV6`, `CARD`, `ACCT`,
`URL`, `DATE`.

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
break that span's cache. Resolution: **the model only *discovers* entities; the
deterministic map *enforces* a stable pseudonym thereafter** — once a span is masked, every
later occurrence is replaced by deterministic string match regardless of whether the model
re-detects it.

## Pluggable by design

The detector is an interface from day one. L1 is the built-in implementation; L2 attaches
as an optional detector without touching the core. Coverage is always an explicit,
documented trade-off (per-type on/off, rule sets) — never a silent guarantee. Detection
runs **fail-closed**: on engine error, block rather than forward
([threat model](../architecture/threat-model.md)).
