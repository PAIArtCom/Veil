# Contributing to OpenCloak

Thanks for your interest. OpenCloak is the open-source privacy engine for AI coding
tools; it is **documentation-first** while the design stabilizes, then code-first once
the Phase 0 engine lands. Please read this before opening an issue or PR.

## Project status

Pre-implementation. The repository currently contains product and architecture docs (see
[`docs/`](docs/README.md)). The most useful early contributions are: sharpening the
architecture, validating the [SDK contract](docs/sdk/contract.md) against more real
gateways, and refining the [detection](docs/concepts/detection-layers.md) and
[token](docs/concepts/token-spec.md) specs.

## Ground rules

- **Language:** English for all code, comments, and documentation.
- **Decisions go through ADRs.** Significant architectural changes are proposed as a new
  record in [`docs/architecture/decisions/`](docs/architecture/decisions/README.md). ADRs
  are immutable once accepted — supersede, don't rewrite.
- **Docs and code stay in sync.** A change in behavior updates the relevant concept/spec
  doc in the same PR. Don't let docs drift.
- **Security first.** OpenCloak's whole job is to *not* leak data. Any change touching
  detection, masking, restore, or egress must keep the invariants in the
  [threat model](docs/architecture/threat-model.md) — especially **fail-closed** and
  **localhost-only** binding. Never commit real secrets, tokens, or the local key store.

## Workflow

1. Open an issue describing the problem or proposal before large work.
2. Branch from `main`. Keep changes focused.
3. For code (once it exists): format with `gofmt`, pass `golangci-lint`, include tests.
4. Reference the relevant ADR / spec doc in your PR description.
5. Be explicit about what you verified and what you didn't.

## Reporting security issues

Do **not** open a public issue for vulnerabilities. A private disclosure channel will be
listed here once the project publishes releases.

## License

By contributing, you agree that your contributions are licensed under the project's
[Apache-2.0](LICENSE) license.
