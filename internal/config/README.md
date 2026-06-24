# internal/config

`internal/config` is the local policy source for the Veil OSS binary.

## Purpose

This module loads a single-user local policy file, validates it fail-closed, and
exposes it as the `PolicyProvider` used by `cmd/veil`. It is the OSS default
configuration path for per-type operator choices without any central control plane.

## Principles

- MUST: Reject unknown keys, unsupported operators, unsupported rule-set choices, and malformed files before they can become plaintext pass-through.
- MUST: Support only the v0.1.0 shipped operators: `token`, `ignore`, and `block`.
- MUST: Keep `redact`, `format_preserving`, and non-empty `rule_sets` reserved and fail-closed until their ADRs and tests exist.
- MUST: Treat labels, comments, metadata, provider tags, analytics tags, raw references, and secret-looking extra fields as unsupported keys rather than hints.
- SHOULD: Keep the file format small, deterministic, and easy to inspect in a text editor.

## Boundaries

- Does NOT handle: Central policy push, fleet policy, hot reload, tenant policy merge, RBAC, SSO, or audit export (see: ../../docs/product/open-core-boundary.md).
- Does NOT handle: Detection rule-pack authoring or configurable rule-set execution (see: ../../docs/architecture/decisions/0012-phase-0-l1-rule-sourcing.md).
- Does NOT handle: Transport startup, provider routing, or credential forwarding (see: ../../cmd/veil/README.md and ../proxy/README.md).
- Does NOT handle: `redact` or `format_preserving` semantics for v0.1.0 (see: ../../docs/architecture/formal-release-plan.md).

## Adversarial Surfaces

- **Unknown-key policy drift**: Extra keys such as `metadata`, `comment`, `label`, `provider`, or `raw_payload` could falsely imply supported behavior. Verified by: policy_test.go.
- **Unsupported operator pass-through**: Deferred operators must fail before request handling, not silently degrade to tokenization or ignore. Verified by: policy_test.go.
- **Rule-set ambiguity**: Non-empty `rule_sets` cannot be ignored because that would make users believe additional detectors are active. Verified by: policy_test.go.
- **Startup fallback confusion**: A configured but invalid policy file must block startup; only an absent default file falls back to the built-in policy. Verified by: policy_test.go and ../../cmd/veil/main_test.go.

## Open Questions

- [ ] Should v0.1.x add explicit schema versioning before the first non-breaking file-format extension? (open since: 2026-06)
- [ ] Should a later local console provide a structured editor for this file instead of asking users to edit JSON directly? (open since: 2026-06)
