# ADR-0008 — Finding model and conflict resolution

**Status:** Accepted

## Context

OpenCloak masks by replacing detected byte ranges with reversible tokens. The original
public model exposed a `Span{Start, End, Type}`. That is enough for a single detector, but
it is not enough for the actual Phase 0 detector stack:

- L1 combines structured regexes, gitleaks-style rules, entropy heuristics, and checksum
  validation.
- Multiple rules can match the same value, or partially-overlapping values, with different
  confidence.
- L2 later adds semantic detections whose confidence must be compared with L1 findings.

Reference projects confirm that this is a correctness boundary, not a cosmetic detail:
Presidio and PasteGuard both have explicit conflict-resolution logic, while
privacy-filter merges and de-overlaps spans before rebuilding text. For OpenCloak this is
even more important because the token map is bijective. Overlapping findings must not
create two tokens for one original value or one token for a sliced value.

## Decision

The detector contract produces `Finding`, not a bare span:

```go
type Finding struct {
    Start  int
    End    int
    Type   Type
    Score  float64
    Source string
}
```

`Start` and `End` are UTF-8 byte offsets. `Score` is normalized to `0..1`. Exact
validators such as Luhn-confirmed cards can use `1.0`. Provider-specific or
gitleaks-style regexes should use high confidence. Entropy/context findings use lower
confidence because they are heuristic. `Source` names the detector/rule family, for
example `l1:gitleaks:github-pat`, `l1:entropy:contextual`, `l1:entropy:strict`,
`l1:luhn`, or `l2:gliner`.

Before any masking occurs, all findings pass through a resolver:

1. Drop invalid ranges.
2. Merge overlapping findings of the same type when they clearly describe the same value.
3. Resolve cross-type conflicts by score, then by longer range, then by earlier start.
4. Emit non-overlapping findings in ascending byte-offset order.

Checksum checks are validators: a candidate that fails validation is dropped, not emitted
with a low score. Entropy is both a validator/ranker for regex candidates and an
originator for otherwise-unmatched high-entropy strings. Originated entropy findings must
pass context/false-positive suppression; contextual entropy findings start at medium
confidence, and bare high-entropy findings require a stricter threshold and lower score.

## Alternatives considered

- **Keep `Span` and make each detector avoid overlaps.** Rejected: this pushes global
  ordering and precedence into individual detectors, making combined L1/L2 behavior hard
  to reason about and test.
- **Let the tokenizer replace overlaps in whatever order it receives.** Rejected: this can
  corrupt byte offsets and produce non-bijective mappings.
- **Use entropy only as a validator.** Rejected: it would miss bare high-entropy secrets
  that have no provider-specific prefix.
- **Adopt Presidio's full analyzer result model.** Rejected: useful concepts are score,
  source, and conflict resolution. The rest of Presidio's model is larger than OpenCloak's
  Phase 0 needs.

## Consequences

- `Detector.Detect` returns `[]Finding`.
- `internal/detect` owns detector orchestration and resolver policy.
- `internal/mask` consumes already-resolved findings and performs offset-safe replacement.
- Tests must cover overlap, containment, same-type merge, cross-type precedence, invalid
  ranges, entropy false positives, and checksum validators.
- Phase 0 cannot be considered complete without resolver fixtures.
