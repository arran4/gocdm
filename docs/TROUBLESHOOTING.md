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

## `Error: failed to build secure session environment: ...`

Cause: NSS/passwd account lookup or `/etc/security/pam_env.conf` parsing failed in `-login` mode. GoCDM fails closed rather than inheriting the privileged getty environment.

Fix: validate the authenticated user's `/etc/passwd` (or NSS) entry and `/etc/security/pam_env.conf` syntax.

## Legacy root-owned `state.json` or symlink refusal (`owned by uid ...`)

Cause: `SafeSaveStateAt` found an existing `~/.config/gocdm` directory or `state.json` owned by another user (for example `uid 0` created by GoCDM `<= 0.0.26`), or encountered a symlink in the state path. GoCDM refuses to overwrite or `chown` user trees automatically.

Fix: inspect `~/.config/gocdm` and remove or fix ownership manually as the user or administrator (`rm -rf ~/.config/gocdm` or `chown -R <user>:<group> ~/.config/gocdm` after verifying there are no symlinks).

## `Warning: cannot change directory to ... falling back to /`

Cause: the authenticated user's home directory does not exist or is not accessible after dropping privileges.

Fix: ensure the account's home directory exists and is owned/accessible by the authenticated user.
