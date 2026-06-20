# Guide: Release Checklist

**Status: v0.1.0 release-candidate checklist.** This is an operator checklist for preparing
a release cut. It does not authorize pushing tags or publishing GitHub releases; those
actions require an explicit maintainer decision.

## Preconditions

- The working tree is clean.
- R1-R7 runtime evidence is recorded through Specability.
- Claude Code live acceptance remains reproducible from
  [Guide: Claude Code](claude-code.md).
- Codex/OpenAI Responses live acceptance is recorded with sanitized evidence. If the
  release claim requires direct `api.openai.com` upstream evidence, rerun the Codex
  acceptance task with a valid OpenAI API key first.
- No raw provider captures, credentials, local key files, or real secrets are staged.

## Local Gate

Run from the repository root after the last code or documentation change:

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

`gofmt -l .` must print no paths. `specability scan --json` and reconcile commands must
not report drift or invalid modules.

## Performance Baseline

Performance checks are advisory release evidence, not hard CI timing gates. Run them when
detector coverage, masking, provider walkers, or long-context handling changes:

```sh
go test -run '^$' \
  -bench 'BenchmarkDetect(Tiered|Complex)|BenchmarkDetect(NoSecret|Mixed|Secret)' \
  -benchmem ./internal/detect/l1

go test -run '^$' -bench 'BenchmarkMask(Text|Request)' -benchmem .
```

Compare the result against the reference bands in
[Performance evaluation](../architecture/performance-evaluation.md).

## Documentation Gate

- README and README.zh-CN describe shipped scope and known limits.
- Changelog has a v0.1.0-ready entry.
- SECURITY.md has private-reporting guidance and known security limits.
- Claude Code guide is copy-paste runnable for the live-accepted path.
- Codex guide distinguishes the local Codex CLI live-accepted path from unclaimed direct
  `api.openai.com` upstream acceptance.
- Deployment guide documents clean-checkout build, release artifact shape, local policy,
  and verification commands.
- Planned behavior remains labeled planned or reserved.

## Artifact Instructions

Release builds should inject version metadata:

```sh
go build \
  -ldflags "-X main.version=v0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o ./dist/opencloak ./cmd/opencloak
```

Expected binary paths:

| Platform | Artifact path |
|---|---|
| macOS/Linux | `dist/opencloak_<version>_<os>_<arch>/opencloak` |
| Windows | `dist/opencloak_<version>_windows_<arch>/opencloak.exe` |

## Release Cut

Only after all gates pass and maintainers approve:

1. Record the final release report
   ([v0.1.0 release report](../architecture/v0.1.0-release-report.md)).
2. Create the release commit or tag instructions.
3. Prepare release notes from CHANGELOG.md.
4. Do not push tags or publish a GitHub release from this checklist alone.
