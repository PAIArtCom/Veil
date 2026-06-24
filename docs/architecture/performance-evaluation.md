# Performance Evaluation

**Status:** Implemented benchmark framework for v0.1.0 local engine paths.

This document defines the repeatable performance evaluation used for Veil's local
de-identification paths. It measures local CPU and allocation overhead only; provider
network latency, model latency, and live CLI behavior are covered by acceptance tests, not
by these microbenchmarks.

## Scope

Measured surfaces:

- L1 detection only (`internal/detect/l1`).
- Text masking through the public `Engine.Mask` surface.
- Provider-native request masking through `Engine.MaskRequest` for Anthropic Messages and
  OpenAI Responses.

Not measured here:

- Real provider round-trip latency.
- Model generation latency.
- CLI startup or account-auth overhead.
- Persistent mapstore or multi-process cache behavior (not shipped in v0.1.0).

## Workload Tiers

| Tier | Purpose | Shape | Release interpretation |
|---|---|---|---|
| Tier 1: Interactive | Single command or short agent turn | Small mixed prompt/tool payload with secrets, PII, code-looking false positives, and URLs | Should be effectively invisible to users. |
| Tier 2: Agent turn | Normal multi-tool coding turn | Medium transcript with repeated tool results, customer records, credentials, and benign IDs/hashes | Should remain well below provider/model latency. |
| Tier 3: Long context | Large transcript or incident bundle | Long text/tool transcript with many findings and many false-positive controls | Release-relevant for "paste a large business context" workflows. |
| Tier 4: Stress | Scaling probe, not a release gate | Very large local text/L1 payload | Advisory only; use to catch algorithmic blowups. |

The benchmark fixtures use synthetic throwaway values only. They intentionally mix:

- Literal provider secrets and assignment-shaped credentials.
- Emails, phones, cards, IBANs, URLs, IPv4/IPv6 values.
- Business records, support tickets, tool outputs, and migration transcripts.
- False-positive controls such as `process.env.API_KEY`, `${SERVICE_TOKEN}`, UUIDs,
  hashes, static tool schemas, and `.env.example` paths.

## Commands

Focused L1 detector baseline:

```sh
go test -run '^$' \
  -bench 'BenchmarkDetect(Tiered|Complex)|BenchmarkDetect(NoSecret|Mixed|Secret)' \
  -benchmem ./internal/detect/l1
```

Focused public engine and provider-wire baseline:

```sh
go test -run '^$' -bench 'BenchmarkMask(Text|Request)' -benchmem .
```

For a more stable comparison across changes, run several samples and compare with
`benchstat` if available:

```sh
go test -run '^$' -bench 'BenchmarkMask(Text|Request)' -benchmem -count=5 . > /tmp/veil-mask.new
```

These benchmarks are not hard CI timing gates. Wall-clock thresholds are machine-sensitive,
so release review should compare trends, allocation class, and obvious regressions rather
than failing a build because one laptop is slower.

## Reference Run

Reference environment:

- Date: 2026-06-20
- OS/arch: `darwin/arm64`
- CPU: Apple M4
- Go benchmark mode: default `go test -bench`, with `-benchmem`

L1 detector results:

| Benchmark | Time | Throughput | Memory | Allocs |
|---|---:|---:|---:|---:|
| No-secret large code payload | 1.65 ms/op | 4.90 MB/s | 70.8 KB/op | 1,009 |
| Mixed coding payload | 0.423 ms/op | 2.74 MB/s | 15.1 KB/op | 170 |
| Secret-heavy env payload | 0.321 ms/op | 2.58 MB/s | 18.0 KB/op | 170 |
| Tier 1 long context | 1.70 ms/op | 3.40 MB/s | 91.3 KB/op | 1,036 |
| Tier 2 long context | 15.3 ms/op | 2.99 MB/s | 819.8 KB/op | 7,979 |
| Tier 3 long context | 80.9 ms/op | 3.02 MB/s | 4.87 MB/op | 42,262 |
| Tier 4 stress | 168 ms/op | 2.90 MB/s | 9.95 MB/op | 84,336 |
| Complex business payload | 54.6 ms/op | 2.61 MB/s | 2.16 MB/op | 20,688 |

Public engine and wire results:

| Benchmark | Time | Throughput | Memory | Allocs |
|---|---:|---:|---:|---:|
| Mask text Tier 1 | 1.62 ms/op | 3.09 MB/s | 171.3 KB/op | 1,735 |
| Mask text Tier 2 | 15.7 ms/op | 2.51 MB/s | 1.32 MB/op | 13,327 |
| Mask text Tier 3 | 94.6 ms/op | 2.21 MB/s | 7.53 MB/op | 70,613 |
| Mask text Tier 4 | 225 ms/op | 1.86 MB/s | 15.3 MB/op | 140,991 |
| OpenAI Responses request Tier 1 | 1.99 ms/op | 3.65 MB/s | 270.1 KB/op | 2,585 |
| OpenAI Responses request Tier 2 | 11.9 ms/op | 3.47 MB/s | 1.58 MB/op | 14,830 |
| OpenAI Responses request Tier 3 | 47.3 ms/op | 3.45 MB/s | 6.21 MB/op | 58,856 |
| Anthropic request Tier 1 | 1.64 ms/op | 3.95 MB/s | 254.9 KB/op | 2,456 |
| Anthropic request Tier 2 | 11.2 ms/op | 3.40 MB/s | 1.50 MB/op | 14,437 |
| Anthropic request Tier 3 | 49.4 ms/op | 3.07 MB/s | 6.01 MB/op | 57,560 |

## Current Assessment

Tier 1 and Tier 2 results are release-acceptable: the local masking cost is low compared
with normal LLM provider latency, and the allocations are bounded enough for interactive
agent use.

Tier 3 is acceptable for v0.1.0 long-context workflows but should remain visible in
release review. The local wall-clock cost is still sub-100 ms for the measured public
paths on the reference machine. Provider-wire request masking stays in the same memory
class as text-only masking because changed request spans are applied back to provider JSON
with a single range-based string-literal rewrite when byte ranges are available, falling
back to structural path updates only when needed.

Tier 4 is advisory only. It is useful for detecting algorithmic regressions. It should not
be treated as proof that every very large transcript is cheap; it is a scaling smoke test.

## Regression Signals

Treat these as review concerns:

- Tier 1 or Tier 2 time doubles without a clear detector-coverage reason.
- Tier 3 text masking returns to hundreds of MB/op or GB/op allocation behavior.
- Provider-wire Tier 3 allocation returns to tens or hundreds of MB/op after adding new
  JSON walkers.
- L1 throughput falls far below the current ~3 MB/s class for mixed long-context inputs.
- A benchmark fixture has to remove false-positive controls to keep performance stable.

The v0.1.0 release gate remains correctness-first. Performance evidence supports release
readiness, but it does not replace leak tests, fail-closed provider-shape tests, live
acceptance evidence, or Specability validation.
