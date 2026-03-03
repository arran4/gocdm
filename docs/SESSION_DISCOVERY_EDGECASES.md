# Session discovery edge-case validation

GoCDM includes targeted parser tests for `.desktop` edge cases:

- unusual whitespace around `key = value`
- localized `Name[locale]` fallback when plain `Name=` is absent
- quoted `Exec=` command fragments with Freedesktop `%` token stripping

## CI command

```bash
scripts/ci/session_desktop_edgecases.sh
```

## Full package command

```bash
go test ./session
```

The edge-case fixtures live under `session/testdata/edgecases/` and are embedded by tests.
