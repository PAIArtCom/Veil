# ADR-0012 — Phase 0 L1 rule sourcing and RuleSets behavior

**Status:** Accepted

## Context

ADR-0007 assumed Phase 0 would merge rule sources from privacy-filter and gitleaks into
`internal/detect/l1`. During Phase 0 implementation and audit, that proved too broad for the
first acceptance loop: the real blocker was proving mask → forward → restore over Anthropic
Messages and streaming tool I/O, while reducing false positives that corrupt source code.

The implemented detector now ships a small built-in starter set: provider/API key patterns,
PEM private-key headers, email/phone/IP/URL/connection-string rules, Luhn cards, IBAN, ISO
dates, and contextual entropy. It also intentionally avoids bare high-entropy matching in
Phase 0 because isolated hashes/base64/code identifiers produce too many false positives.

The public `Policy` shape already contains `RuleSets []string`, but no configurable rule-set
loader exists yet.

## Decision

Phase 0 L1 uses built-in starter rules only. Configurable rule sets are a Phase 1 feature.

The engine must not silently ignore `Policy.RuleSets`: a non-empty `RuleSets` value returns
`ErrUnsupportedPolicyFeature` / `*UnsupportedPolicyFeatureError` before detection or masking.
This preserves fail-closed behavior when a caller thinks a specific rule set is active.

Bare high-entropy fallback is also deferred to Phase 1. Phase 0 entropy findings require
nearby context keywords.

This ADR partially supersedes ADR-0007 only for the L1 rule-source claim. ADR-0007 remains
accepted for module layout and public/internal package boundaries.

## Alternatives considered

- **Silently ignore `RuleSets` until Phase 1.** Rejected: caller policy uncertainty would look
  successful while the requested coverage was not actually active.
- **Implement full configurable rule sets now.** Rejected: out of scope for the Phase 0
  acceptance loop and likely to expand the false-positive surface before live validation.
- **Enable bare high-entropy fallback immediately.** Rejected for Phase 0: without contextual
  controls it masks too many hashes, generated IDs, and code tokens.

## Consequences

- Public API compatibility is preserved: `RuleSets` remains in the shape for Phase 1.
- Phase 0 callers get an explicit error for unsupported rule-set policy instead of false
  confidence.
- Documentation must describe L1 coverage as built-in starter rules plus contextual entropy,
  not privacy-filter/gitleaks merged rule sets or strict bare entropy fallback.
