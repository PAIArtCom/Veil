# Guide: Release Checklist

**Status: v0.1.0 release checklist.** This is an operator checklist for cutting or
validating a release. It does not authorize pushing tags or publishing GitHub releases;
those actions require an explicit maintainer decision.

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

Release builds should write the binary into the versioned platform artifact directory and
inject version metadata:

```sh
version=v0.1.0
commit="$(git rev-parse --short HEAD)"
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
out_dir="dist/opencloak_${version}_$(go env GOOS)_$(go env GOARCH)"
bin_name="opencloak"
if [ "$(go env GOOS)" = "windows" ]; then
  bin_name="opencloak.exe"
fi
bin_path="$out_dir/$bin_name"

mkdir -p "$out_dir"
go build -trimpath \
  -ldflags "-X main.version=${version} -X main.commit=${commit} -X main.buildDate=${build_date}" \
  -o "$bin_path" ./cmd/opencloak

"$bin_path" version
shasum -a 256 "$bin_path" > "$bin_path.sha256"
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

If a local tag is created before approval or points at the wrong commit, delete it locally
before pushing anything:

```sh
git tag -d v0.1.0
```

If an artifact is built from the wrong commit or with the wrong version metadata, delete
the affected `dist/opencloak_v0.1.0_*` directory and rebuild from a clean, verified tree.
