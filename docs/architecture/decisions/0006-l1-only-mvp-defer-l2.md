# ADR-0006 — L1-only MVP; defer the L2 NER model

**Status:** Accepted

## Context

Detection is layered ([spec](../../concepts/detection-layers.md)):

- **L1** — patterns: regex + entropy + checksums (Luhn) + context keywords. Catches
  structured data (API keys, tokens, connection strings, emails, phones, cards, IPs).
  Microsecond-scale, deterministic, no model.
- **L2** — a local NER model for *unstructured* PII (names, addresses) that no regex can
  match. Requires running a neural network on every turn's prompt.

L2 has real costs: a larger install, per-turn inference latency added to the request path
(hundreds of ms to seconds for a 1.5B model over a long prompt on CPU), and NER
imprecision. It also introduces a consistency hazard (a name caught on turn 1 but missed on
turn 5). Meanwhile, the highest-value targets — secrets and structured PII — are entirely
in L1, and unstructured names are low-value on the model side and reasonably excluded by
default.

## Decision

**The MVP ships L1 only.** L2 is a **pluggable, opt-in layer deferred to Phase 1.** When
added, it runs only on per-turn *new* content to bound latency, with person detection
default-off and user-configurable. Semantic-PII consistency is enforced by the same
deterministic map ("model discovers entities; the map enforces stable pseudonyms").

## Alternatives considered

- **Include L2 in the MVP.** Rejected: adds latency and install weight while proving
  nothing about the core thesis (the mask/restore loop). It would slow the MVP and dilute
  the "lossless masking" sweet spot that L1 occupies.

## Consequences

- The MVP stays small, fast, deterministic, and model-free — the credible shippable core.
- L2 candidates (to evaluate in Phase 1): a small generalist NER like GLiNER (fast on CPU)
  as the pragmatic default; the OpenAI Privacy Filter model as the exact-fit high-accuracy
  option; spaCy as a lightweight baseline.
- The detector interface must be pluggable from day one so L2 attaches without touching the
  core.
