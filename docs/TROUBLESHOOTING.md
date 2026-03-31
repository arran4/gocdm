# Troubleshooting

## `gocdm must be launched from an interactive TTY`

Cause: launched from non-interactive shell/session.

Fix: run from a real VT/getty login or use `-dry-run` for non-interactive validation.

## `Failed to find free display`

Cause: All 7 X displays (0-6) are currently in use, or there is an error probing local X11 unix sockets.

Fix: Ensure there is at least one free display or check `/tmp/.X11-unix` socket directory permissions.

## `Failed to launch X session: required tool "startx" not found in PATH`

Cause: missing `startx` / xinit package.

Fix: install distro package providing `startx` and re-run.

## `Authentication failed: ...` in `-login`

Cause: wrong credentials, PAM policy rejection, or missing PAM build support.

Fix:

- verify `-tags nopam` build was not used,
- verify `-pam-service` target exists in `/etc/pam.d/`,
- inspect PAM logs (`journalctl`, auth logs) for policy denial detail.

## Session env fallback warning

Message: `Warning: failed to build secure session environment: ...`

Cause: passwd/pam_env parsing issue.

Fix: validate `/etc/passwd` entry and `/etc/security/pam_env.conf` syntax.
