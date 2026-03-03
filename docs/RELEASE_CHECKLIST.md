# Release checklist

## Validation gates

- `gofmt -w ./...` (or targeted changed files)
- `go test ./...`
- `go vet ./...`
- `golangci-lint run` (if configured in CI/tooling)
- manual checks documented in `docs/LOCKTTY_XTTY_MANUAL_TEST.md`
- session edge-case parser checks via `scripts/ci/session_desktop_edgecases.sh`

## Versioning and changelog

- bump version metadata (if applicable)
- update changelog/release notes with major behavioral deltas
- call out any deprecations and unsupported-platform caveats

## Tagging

```bash
git tag -a v0.1.0 -m "Go-complete port baseline"
git push origin v0.1.0
```
