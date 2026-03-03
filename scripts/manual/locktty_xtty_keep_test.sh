#!/usr/bin/env bash
set -euo pipefail

# Manual integration script for validating locktty + xtty=keep behavior on real TTYs.
# Run as a real user session on tty1/tty2 (not from SSH/non-interactive shell).

GOCDM_BIN=${GOCDM_BIN:-gocdm}
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

cat >"$TMPDIR/cdmrc" <<'RC'
binlist=("/bin/true")
namelist=("TTY Keep Test")
flaglist=("X")
locktty=yes
xtty=keep
display=0
RC

echo "[1/4] Sanity check: show resolved tty"
tty

echo "[2/4] Dry-run X launch path"
"$GOCDM_BIN" -config "$TMPDIR/cdmrc" -dry-run || true

echo "[3/4] If an X session already exists on display :0, dry-run should print VT derived from current tty"
echo "      Expected pattern: 'Dry run: would switch to existing X session on display :0 VT<current-tty-number>'"

echo "[4/4] Capture output for notes"
"$GOCDM_BIN" -config "$TMPDIR/cdmrc" -dry-run 2>&1 | tee "$TMPDIR/output.log" || true

echo "Saved output: $TMPDIR/output.log"
