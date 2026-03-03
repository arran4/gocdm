# Troubleshooting

## `gocdm must be launched from an interactive TTY`

Cause: launched from non-interactive shell/session.

Fix: run from a real VT/getty login or use `-dry-run` for non-interactive validation.

## `Failed to find free display` / `required tool "xdpyinfo" not found in PATH`

Cause: missing X tooling or no usable display probe path.

Fix: install X utilities and ensure `xdpyinfo` is in `PATH`.

## `Failed to launch X session: required tool "startx" not found in PATH`

Cause: missing `startx` / xinit package.

Fix: install distro package providing `startx` and re-run.

## `Authentication failed: ...` in `-login`

Cause: wrong credentials, PAM policy rejection, or missing PAM build support.

Fix:

- verify `-tags pam` build was used,
- verify `-pam-service` target exists in `/etc/pam.d/`,
- inspect PAM logs (`journalctl`, auth logs) for policy denial detail.

## Session env fallback warning

Message: `Warning: failed to build secure session environment: ...`

Cause: passwd/pam_env parsing issue.

Fix: validate `/etc/passwd` entry and `/etc/security/pam_env.conf` syntax.
