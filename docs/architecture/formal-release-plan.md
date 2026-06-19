# Formal Release Development Plan

**Status:** Draft release execution plan. Scope source:
[roadmap](../product/roadmap.md), [system design](system-design.md),
[SDK contract](../sdk/contract.md), [threat model](threat-model.md), and the
[Phase 0 acceptance report](phase-0-acceptance.md).

## Release target

The first formal OpenCloak release is **OpenCloak OSS v0.1.0**, a public developer
release for one-person local use.

The release is successful when a developer can install OpenCloak, run it safely with
Claude Code and Codex CLI, embed the SDK in one real gateway path or a maintained
reference integration, configure local policy without a control plane, and reproduce the
security evidence that sensitive values do not cross the LLM provider boundary.

This is not a v1.0 API-freeze release and not the Cloakia commercial control plane.

## Release principles

- **Prove each egress path live.** A provider adapter is not release-ready until a live
  controlled task proves that the provider sees only `CLK_` tokens while local tools and
  files receive restored values.
- **Keep the engine/transport split.** Provider walking, masking, state, and streaming
  restore stay in the SDK path. `cmd/opencloak` and transports only wire them together.
- **Fail closed by default.** Unsupported provider operations, unsupported policy features,
  malformed provider JSON, key errors, and restore uncertainty must not forward plaintext.
- **Ship individual value, defer organizational control.** Local config, local proxy, local
  SDK, and local console/status belong in OpenCloak. Fleet policy, SSO, RBAC, and
  cross-developer audit remain Cloakia.
- **Use module contracts proportionally.** Add structured module READMEs before expanding a
  module's behavior or trust boundary; do not write speculative contracts for modules that
  are still deferred.

## Non-goals for v0.1.0

- Cloakia SaaS, sold-license packaging, SSO/RBAC, central policy push, or fleet audit.
- Full provider coverage for every AI coding tool.
- Remote MCP / remote-tool egress classification.
- A production multi-tenant network service.
- L2 NER as a mandatory default. L2 may be prototyped, but v0.1.0 must remain useful and
  safe with L1-only detection.
- Public third-party provider adapter registration unless a real external adapter requires
  it before release.

## Release-blocking work

| Area | Release expectation |
|---|---|
| Phase 0 baseline | Existing Claude Code proxy path remains accepted, with regression evidence rerun after release changes. |
| Codex CLI | OpenAI Responses provider path works through the standalone proxy with live controlled traffic. |
| SDK embed | One real gateway path or maintained reference integration proves the SDK contract outside the standalone proxy. |
| Local policy | A documented local policy source supports safe per-type behavior for the operators that v0.1.0 claims. Unsupported policy features fail closed and are documented. |
| CLI and packaging | `opencloak proxy`, help/version output, install path, and release artifact instructions are reproducible from a clean checkout. |
| Documentation | README, docs map, guides, API reference, and known limits distinguish shipped behavior from planned behavior. |
| Security evidence | No credential capture, no plaintext protected text/tool-I/O egress, localhost-only proxy, scoped state, residual-token audit, and minimized logs are verified. |

## Deferred after v0.1.0

| Deferred item | Reason |
|---|---|
| OpenAI Chat and Gemini adapters | Valuable ecosystem breadth, but Codex Responses is the next highest-risk coding-agent path. |
| HTTP/gRPC service | Useful for non-Go hosts, but the SDK embed validation should prove the contract before freezing a service API. |
| Local web console beyond status/config inspection | Nice workflow layer; not required to prove safe egress. |
| L2 semantic PII default | Needs model selection, latency bounds, and false-positive controls. Keep optional until proven. |
| `format_preserving` operator | Requires type-specific reversible strategies and separate restore semantics. |
| Remote MCP egress classification | Separate egress channel and threat boundary; should get its own plan/ADR. |

## Milestone sequence

### R0 - Release plan and contract cleanup

**Goal:** make the release target explicit and remove stale Phase 0 planning language before
feature work resumes.

**Packages/docs:** `docs/`, `README.md`, `README.zh-CN.md` if status wording changes.

**Build:**

- Add this release plan and link it from the documentation map and roadmap.
- Update stale Phase 0 status language without changing accepted ADR history.
- Decide the module README gate for each release-blocking module.
- Record which Phase 1 decisions need an ADR before implementation.

**DoD:**

- `specability validate .`, `specability validate docs`, and `specability scan --json`
  pass with no new drift.
- The plan names release blockers, non-goals, verification gates, and acceptance evidence.

### R1 - Baseline hardening and release plumbing

**Goal:** turn the accepted Phase 0 code into a releasable baseline before adding new
provider behavior.

**Packages/docs:** root package, `cmd/opencloak`, `internal/proxy`, `internal/stream`,
`internal/mask`, `docs/guides/claude-code.md`, release docs.

**Build:**

- Add or update module READMEs for `cmd/opencloak` and `internal/proxy` because they are
  user-facing trust-boundary modules.
- Add `Adversarial Surfaces` sections to existing structured READMEs where they describe
  active security boundaries.
- Ensure the CLI exposes stable help and version output.
- Add CI or documented release verification that runs the full local gate.
- Document install and release artifact paths.
- Preserve the Claude Code live acceptance runbook as a regression gate.

**DoD:**

- Clean clone can build `opencloak` and run `opencloak proxy --help` and version output.
- Logs and errors contain no credentials, raw secrets, or captured provider bodies.
- Claude Code Phase 0 acceptance can be rerun without changing the documented procedure.

**Fixtures/checks:**

- `go test -count=1 ./...`
- `go test -race -count=1 ./...`
- `go vet ./...`
- `go build ./...`
- `gofmt -l .`
- `git diff --check`
- `specability validate .`
- `specability validate docs`
- `specability scan --json`
- `specability reconcile . --json`
- `specability reconcile docs --json`

### R2 - SDK embed validation

**Goal:** close the Phase 0 secondary validation gap by proving the SDK contract in a real
gateway-style integration.

**Packages/docs:** `examples/embed/`, root SDK docs, `docs/sdk/contract.md`,
`docs/sdk/integration-guide.md`, selected gateway notes.

**Build:**

- Choose the validation target before implementation. Candidate: `clipal`, because it has
  hard-coded seams and a raw streaming path that match the SDK's lowest common denominator.
- Add an `examples/embed/README.md` contract before writing the example or integration.
- Wire `MaskRequest`, `RestoreResponse`, `RestoreStreamChunk`, `FlushStream`, and
  `RestoreSSEEvent` at the target's real request/response seams.
- Prove scope/state lifecycle across retries and stream completion.
- Keep provider credentials outside the engine.

**DoD:**

- The integration forwards only masked provider payloads.
- Local tool execution or gateway-local response handling receives restored real values.
- Raw-stream restore tolerates arbitrary byte splits in the integration path.
- Cross-scope restore fails visibly or leaves a residual token; it never restores from
  another namespace.
- The SDK docs are updated only for behavior proven by the integration.

**Fixtures/checks:**

- Unit tests for integration glue where it lives in this repo.
- A recorded sanitized runbook with no committed raw credentials or secret captures.
- `specability reconcile docs --json` confirms docs do not overclaim.

### R3 - Codex CLI and OpenAI Responses provider

**Goal:** support the second high-value AI coding path through the standalone proxy.

**Packages/docs:** `internal/wire/openairesponses/`, `internal/wire`, `internal/proxy`,
`cmd/opencloak`, `docs/guides/codex.md`, SDK API reference.

**Build:**

- Add structured READMEs for `internal/wire/openairesponses` and any expanded
  `internal/wire` boundary before implementation.
- Verify current Codex CLI base-url behavior and capture real provider-native request,
  response, streaming, tool-call, and tool-result shapes before writing the walker.
- Implement request extraction and application over native OpenAI Responses JSON, including
  agentic tool I/O fields discovered in the capture.
- Implement provider-aware buffered and parsed-SSE restore for Responses events.
- Extend the standalone proxy routing without weakening Anthropic behavior.
- Update `docs/guides/codex.md` from planned to shipped only after live acceptance passes.

**DoD:**

- A controlled Codex CLI task with a throwaway DSN proves:
  1. provider-bound payloads contain `CLK_` tokens and no real DSN;
  2. overlapping findings produce one correct token;
  3. tool-call arguments and tool results restore locally;
  4. streamed tokens survive arbitrary byte and event splits;
  5. errors are visible and fail closed;
  6. files on disk contain no `CLK_`;
  7. a repeated turn keeps deterministic masked prefixes where provider caching exposes it.
- Unsupported Responses shapes fail closed rather than being forwarded as plaintext.
- Claude Code acceptance still passes after proxy routing changes.

**Fixtures/checks:**

- Sanitized provider-native request/response/SSE fixtures from the capture.
- Unit tests for tool-call arguments, tool results, output text, malformed JSON, and
  unsupported operation paths.
- Regression tests for Anthropic routing after adding OpenAI Responses.

### R4 - Local policy configuration

**Goal:** make local single-user policy real enough for a public release without adding
central management.

**Packages/docs:** `internal/config`, root policy types, `cmd/opencloak`, API reference,
deployment guide.

**Build:**

- Add `internal/config/README.md` before implementing behavior.
- Define the local policy file format, search path, validation errors, and precedence.
- Support the operators v0.1.0 claims. At minimum: `token`, `ignore`, and `block`.
- Decide whether `redact` ships in v0.1.0. If it ships, add an ADR for irreversible
  semantics. If it does not ship, keep it explicitly reserved and fail closed.
- Keep `format_preserving` reserved unless a type-specific reversible strategy is designed
  and tested.
- Keep non-empty `RuleSets` fail-closed until configurable rule packs are implemented.

**DoD:**

- Invalid policy blocks startup or request handling visibly; it never silently falls back to
  plaintext forwarding.
- Per-type policy affects masking exactly as documented.
- Config parsing does not allow unknown unsafe keys to imply supported behavior.
- Deployment docs show the default path and a minimal safe config.

**Fixtures/checks:**

- Valid config, unknown key, bad operator, unsupported ruleset, per-type block, per-type
  ignore, and default-token cases.
- Negative controls with secret-looking values in labels, comments, unknown keys, and nested
  metadata.

### R5 - Release documentation and distribution

**Goal:** make the project understandable and installable by someone who was not present for
development.

**Packages/docs:** README, docs map, guides, SDK docs, release notes, repository metadata.

**Build:**

- Update README and README.zh-CN status to match shipped release scope.
- Add a release checklist and changelog entry.
- Add `SECURITY.md` and contribution guidance if absent before public announcement.
- Make Claude Code and Codex guides copy-paste runnable for the shipped release.
- Keep planned docs marked planned; do not present service, console, L2, Gemini, or remote
  MCP as shipped.

**DoD:**

- A new user can follow one quickstart from a clean clone or release artifact.
- Docs have no stale "planned" wording for shipped v0.1.0 behavior.
- Docs have no shipped wording for deferred behavior.

**Fixtures/checks:**

- Manual clean-checkout quickstart.
- `specability scan --json` and `specability reconcile docs --json`.

### R6 - Release candidate hardening

**Goal:** prove the release under the same kinds of failures real users hit.

**Packages/docs:** all release-blocking packages.

**Build:**

- Run a security review against the threat model invariants.
- Run an impact review for public API, config format, CLI flags, and docs.
- Run a test strategy review to confirm the smallest credible release gate.
- Add missing regression tests found by those reviews.
- Freeze the public v0.1.0 behavior and mark any remaining work deferred.

**DoD:**

- No blocker findings remain open.
- Any accepted residual risk is documented as a release note or known limit.
- Verification evidence is fresh after the last code or docs change.

**Required release gate:**

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
gofmt -l .
git diff --check
specability validate .
specability validate docs
specability validate docs/architecture/decisions
specability scan --json
specability reconcile . --json
specability reconcile docs --json
specability reconcile docs/architecture/decisions --json
```

**Manual/live acceptance:**

- Claude Code live acceptance rerun.
- Codex CLI live acceptance run.
- SDK embed validation run.
- Clean install/quickstart run.

### R7 - v0.1.0 release cut

**Goal:** publish only after release evidence is complete and reproducible.

**Build:**

- Create the final release report under `docs/architecture/`.
- Tag the release and attach release artifacts or installation instructions.
- Record the release gate outputs and live acceptance summaries.
- Update roadmap status from "planned" to "released" only for shipped behavior.

**DoD:**

- `v0.1.0` tag points at the verified commit.
- Release notes list shipped support, known limits, upgrade notes, and security posture.
- Any deferred release-blocking candidate is either removed from the release claim or has a
  follow-up issue/plan.

## Module README strategy

Structured module READMEs are required before release-blocking work starts in a module when
any of these are true:

- the module owns a trust boundary;
- the module changes public SDK, CLI, config, provider, or wire behavior;
- the module introduces a new egress path;
- the module is a planned package becoming implemented behavior.

Required before v0.1.0 implementation continues:

| Module | README timing | Why |
|---|---|---|
| `cmd/opencloak` | R1 | User-visible CLI and release surface. |
| `internal/proxy` | R1 | Local network boundary, credential pass-through, fail-closed transport. |
| `internal/wire` | R3 | Provider adapter boundary expands beyond Anthropic. |
| `internal/wire/openairesponses` | R3 | New provider-native JSON walker and tool I/O coverage. |
| `internal/config` | R4 | Local policy source and fail-closed config validation. |
| `examples/embed` | R2 | SDK contract proof and integration guidance. |

Do not create full structured READMEs yet for `internal/service`, `internal/console`,
`internal/wire/openaichat`, `internal/wire/gemini`, or L2 model packages unless their work
is pulled into the v0.1.0 release scope.

## ADRs and decisions required before coding

| Decision | Needed before | Notes |
|---|---|---|
| OpenAI Responses provider contract | R3 | Capture-driven paths, tool I/O coverage, streaming event restore, unsupported operation behavior. |
| Provider adapter public boundary | Before third-party adapter registration | Phase 0 keeps `internal/wire` internal; do not freeze a plugin API prematurely. |
| Local config and policy source semantics | R4 | File format, precedence, unknown-key behavior, and fail-closed rules. |
| `redact` operator semantics | Before shipping `redact` | Irreversible behavior must not be confused with reversible local tool execution. |
| `format_preserving` semantics | Before shipping the operator | Requires per-type reverse strategy and residual-token scanning implications. |
| Release compatibility policy | R6 | Clarify v0.1 pre-1.0 compatibility and what is allowed to break before v1.0. |

## Acceptance matrix for v0.1.0

| # | Claim | Evidence required |
|---|---|---|
| 1 | Claude Code path remains safe | Live acceptance rerun plus automated proxy and stream tests. |
| 2 | Codex CLI path is safe | Live controlled Codex run with sanitized capture and fixtures. |
| 3 | SDK contract is embeddable | Real gateway path or maintained reference integration passes outbound, buffered restore, and streaming restore checks. |
| 4 | Provider never receives real sensitive values | Upstream captures for live runs show tokens only; no real DSN/secret/PII. |
| 5 | Local tools and files receive restored real values | Controlled tool task writes real values locally and no `CLK_` tokens. |
| 6 | Fail-closed behavior is preserved | Tests cover bad key, bad config, malformed JSON, unsupported provider/op, unsupported policy feature, and detector errors. |
| 7 | Scope isolation holds | Same token/value cannot restore across tenant/session/project scopes. |
| 8 | Logs and audit are minimized | Logs contain no credential headers, raw secrets, or provider bodies; residual-token audit records token metadata only. |
| 9 | Installation is reproducible | Clean checkout or release artifact quickstart succeeds. |
| 10 | Docs match behavior | Specability validation/reconciliation passes and status markers are accurate. |

## Release risk register

| Risk | Mitigation |
|---|---|
| Codex provider shapes drift before implementation | Treat live capture and current docs verification as an R3 entry gate; fail closed on unknown shapes. |
| Proxy routing change regresses Claude Code | Keep Claude live acceptance and Anthropic regression fixtures in R3/R6. |
| Local config creates a fail-open path | Validate before use, reject unknown unsupported behavior, and test negative controls. |
| Public API freezes too early | Keep `internal/wire` internal and label v0.1.0 pre-1.0 compatibility honestly. |
| Module READMEs become speculative | Add READMEs only for modules entering release-blocking work. |
| Logs/captures leak secrets during acceptance | Use throwaway values, strip credentials, inspect raw captures locally, and commit only sanitized summaries. |

## Working protocol

Each milestone should close with:

1. a bounded Specability task frame;
2. module README contracts for affected trust-boundary modules;
3. implementation and tests;
4. independent review when the milestone touches egress, provider JSON, policy, state, or
   release claims;
5. fresh verification commands;
6. a short milestone outcome report;
7. a commit checkpoint only after evidence passes.

Tiny documentation-only corrections may stay direct, but release-blocking code milestones
should be recorded and independently evaluated.
