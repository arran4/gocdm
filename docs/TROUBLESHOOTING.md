# Troubleshooting

## `direct getty login mode (-login) requires root privileges`

Cause: `-login` or `-mode=login` was invoked by an unprivileged user without `-dry-run`.

Fix: omit `-login` or pass `-mode=user` when launching GoCDM from an already-authenticated user shell (`~/.profile` / `profile.sh`). Use `-login` only when GoCDM replaces `getty` as root (e.g., `getty@tty1.service`).

## `direct getty login mode (-login) cannot run inside an active graphical session` / `inside an SSH session`

Cause: `-login` or `-mode=login` was invoked when `DISPLAY`, `WAYLAND_DISPLAY`, `SSH_TTY`, `SSH_CONNECTION`, or `SSH_CLIENT` was set.

Fix: use `-mode=user` (or omit `-login`) inside existing sessions; reserve `-login` for direct console/getty ownership.

## `conflicting mode flags` / `invalid -mode`

Cause: contradictory flags were supplied (such as `-login -mode=user` or `-login=false -mode=login`) or `-mode` was given an unrecognized value.

Fix: specify one consistent mode (`-mode=auto`, `-mode=login`/`getty`, or `-mode=user`/`autostart`).

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
