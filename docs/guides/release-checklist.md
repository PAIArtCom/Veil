# Guide: Release Checklist

**Status: v0.1.0 release checklist.** This is an operator checklist for cutting or
validating a release. It does not authorize pushing tags; pushing a `v*` tag is the
explicit maintainer release action and triggers automatic GitHub Release publication.

## Preconditions

- The working tree is clean.
- R1-R7 runtime evidence is recorded through Specability.
- Claude Code live acceptance remains reproducible from
  [Guide: Claude Code](claude-code.md).
- Codex/OpenAI Responses live acceptance is recorded with sanitized evidence. The local
  Codex CLI Responses run is the v0.1.0 OpenAI Responses protocol evidence; a separate
  direct `api.openai.com` official-service run is not part of this release gate.
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
- Codex guide presents the local Codex CLI live-accepted path as the OpenAI Responses
  protocol evidence and avoids claiming a separate direct `api.openai.com` official-service
  run.
- Deployment guide documents clean-checkout build, release artifact shape, local policy,
  and verification commands.
- Planned behavior remains labeled planned or reserved.

## Artifact Instructions

Release builds should produce the full v0.1.0 platform matrix and checksum manifest:

```sh
VERSION=v0.1.0 ./scripts/build-release.sh
./scripts/gen-checksums.sh dist/release > dist/release/checksums.txt
local_bin="dist/release/veil-v0.1.0-$(go env GOOS)-$(go env GOARCH)"
if [ "$(go env GOOS)" = "windows" ]; then
  local_bin="${local_bin}.exe"
fi
"$local_bin" version
```

Expected release assets:

| Platform | Artifact path |
|---|---|
| macOS | `dist/release/veil-<version>-darwin-amd64`, `dist/release/veil-<version>-darwin-arm64` |
| Linux | `dist/release/veil-<version>-linux-amd64`, `dist/release/veil-<version>-linux-arm64` |
| Windows | `dist/release/veil-<version>-windows-amd64.exe`, `dist/release/veil-<version>-windows-arm64.exe` |
| Checksums | `dist/release/checksums.txt` |

## Release Cut

Only after all gates pass and maintainers approve:

1. Record the final release report
   ([v0.1.0 release report](../architecture/v0.1.0-release-report.md)).
2. Create the release commit or tag instructions.
3. Prepare release notes from the current version section of CHANGELOG.md.
4. Do not push tags from this checklist alone. Pushing a `v*` tag triggers automatic
   GitHub Release publication.

If a local tag is created before approval or points at the wrong commit, delete it locally
before pushing anything:

```sh
git tag -d v0.1.0
```

The tag-triggered GitHub Actions workflow runs the Go release gate including race tests,
builds the same assets, extracts the current changelog version section as release notes,
and publishes the GitHub Release automatically. If an artifact is built from the wrong
commit or with the wrong version metadata, delete `dist/release` and rebuild from a clean,
verified tree.
