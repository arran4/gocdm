# Manual test instructions: `locktty` + `xtty=keep`

This test validates real-TTY behavior and should be run manually on Linux hosts.

## Preconditions

- Run from a real virtual terminal (e.g. tty1 via systemd getty/OpenRC getty), not SSH.
- `gocdm` installed and available in `$PATH`.
- Optional: an active X session on `:0` to verify lock/handoff path.

## Script

Use:

```bash
scripts/manual/locktty_xtty_keep_test.sh
```

You can override binary path:

```bash
GOCDM_BIN=/usr/local/bin/gocdm scripts/manual/locktty_xtty_keep_test.sh
```

## What to verify

1. The script prints the current tty (`/dev/ttyN`).
2. Dry-run output includes either:
   - existing-X lock handoff path:
     `Dry run: would switch to existing X session on display :0 VTN`
   - or new-session launch path if no active X on `:0`.
3. For lock handoff, `VTN` should match the tty number in step 1 when `xtty=keep`.

## Cross-init matrix

Run the same script on:

- systemd-based distro (e.g. Debian/Fedora)
- OpenRC-based distro (e.g. Alpine/Gentoo)

Record outputs and differences in your deployment notes.
