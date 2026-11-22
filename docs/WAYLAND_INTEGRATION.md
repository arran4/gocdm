# Wayland and Graphical Session Integration

GoCDM launches graphical sessions directly via shell-free `execve` semantics:

- **Wayland (`W`)**: Executes the parsed `Exec=` argv directly under PAM session supervision (in `-login` mode) or via direct process replacement (`exec` in `-mode=user`).
- **X11 (`X`)**: Launches `startx` with quoted desktop `Exec=` tokens preserved, dropping privileges to the authenticated user before starting X when running in `-login` mode.

## Desktop & XDG Session Metadata

When discovering `.desktop` files under `/usr/share/wayland-sessions`, `~/.config/wayland-sessions`, `/usr/share/xsessions`, or `~/.local/share/xsessions`:

- Keys are scoped to `[Desktop Entry]`; `[Desktop Action ...]` sections cannot override session properties, and `Hidden=true` or `NoDisplay=true` entries are excluded.
- Before opening the PAM session (`-login`), GoCDM sets:
  - `XDG_SESSION_TYPE` (`wayland` or `x11`)
  - `XDG_SESSION_CLASS=user`
  - `XDG_SESSION_DESKTOP` and `DESKTOP_SESSION` from the `.desktop` filename stem (falling back to session display `Name` for manual `cdmrc` `binlist` entries)
  - `XDG_CURRENT_DESKTOP` from `DesktopNames=` (joined with `:`, falling back to `XDG_SESSION_DESKTOP`)
  - `XDG_VTNR` and `XDG_SEAT=seat0`
- GoCDM **never fabricates** `XDG_RUNTIME_DIR`, `DBUS_SESSION_BUS_ADDRESS`, `WAYLAND_DISPLAY`, or `XAUTHORITY`. Those values come from PAM/logind (`pam_systemd` / `pam_elogind`) or the active desktop session stack.

## KDE Plasma Wayland Wrapper (`plasma-dbus-run-session-if-needed`)

On KDE Plasma, `session/wayland_wrappers.go` checks whether a discovered `startplasma-wayland` command already includes `plasma-dbus-run-session-if-needed` or `dbus-run-session`:

- **Plasma 6 desktop files** often include `/usr/libexec/plasma-dbus-run-session-if-needed` in `Exec=` already; GoCDM preserves it without double-wrapping.
- **Unwrapped entries** look up `plasma-dbus-run-session-if-needed` across `/usr/libexec/`, `/usr/lib64/libexec/`, `/usr/lib/`, and `$PATH`, prepending it only when available.
- **Is it needed?** On systemd-logind setups where `pam_systemd` / systemd user D-Bus already sets `DBUS_SESSION_BUS_ADDRESS` (or `$XDG_RUNTIME_DIR/bus`), `plasma-dbus-run-session-if-needed` is effectively a pass-through. On non-systemd setups (such as OpenRC without a user D-Bus daemon), it transparently starts `dbus-run-session`. Preserving without double-wrapping keeps both Gentoo systemd and OpenRC profiles working cleanly.

## Shell-Free Policy

Graphical entries (`W` and `X`) do **not** wrap launches in `/bin/sh -l` or source interactive user dotfiles (`~/.bashrc`, `~/.profile`, `~/.zshrc`, `~/.zprofile`) by default. Graphical applications that rely on interactive shell startup files will intentionally not run those scripts automatically.

## X11 Modern Logind / ConsoleKit & `xtty=keep`

- `consolekit` defaults to legacy auto behavior only when unset (`ConsoleKitExplicit == false`): if PAM/logind sets `XDG_SESSION_ID` or `ck-launch-session` is absent from `$PATH`, GoCDM skips `ck-launch-session` automatically so `startx` works out of the box on systemd/elogind Linux.
- Explicit `consolekit=yes` or `consolekit=no` in `cdmrc` is strictly honored.
- When `xtty=keep` is configured and GoCDM is already running on the target VT (`CurrentVT == VT`), GoCDM skips invoking a redundant `chvt`.
- Non-`/dev/null` `startxlog` paths reject path traversal, symlinks (`O_NOFOLLOW`), and non-regular files.

## Opt-In Real-VT Manual Testing (Spare VT, Gentoo / systemd)

Do **not** replace `getty@tty1.service`, overwrite PAM configuration, or modify real user dotfiles while testing. Test safely on a spare VT (for example `tty2` or `tty3`) using a dedicated test user or temporary overlay unit:

1. Switch to an unused recovery VT (`Ctrl+Alt+F2`) or stop/test only on `tty2`/`tty3`.
2. Inspect the redacted session environment before launching:
   ```sh
   sudo /usr/local/bin/gocdm -login -debug-env -dry-run
   ```
3. Launch KDE Plasma Wayland (`plasma.desktop`) or an X11 session interactively from a test unit on `tty2`:
   ```sh
   # Verify PAM/logind state inside the resulting session:
   loginctl show-session "$XDG_SESSION_ID" -p Type -p Class -p Seat -p VTNr -p State -p Desktop
   printf 'XDG_SESSION_TYPE=%s XDG_SESSION_DESKTOP=%s XDG_CURRENT_DESKTOP=%s\n' \
     "$XDG_SESSION_TYPE" "$XDG_SESSION_DESKTOP" "$XDG_CURRENT_DESKTOP"
   test -d "$XDG_RUNTIME_DIR" && test -n "$DBUS_SESSION_BUS_ADDRESS" && echo "runtime+dbus OK"
   ```
4. Log out cleanly from Plasma / X11 and verify PAM teardown and clean re-login on the same VT:
   ```sh
   loginctl list-sessions
   ```
5. For post-login autostart parity (`-mode=user`), log into a TTY test shell first, confirm `XDG_RUNTIME_DIR` and `DBUS_SESSION_BUS_ADDRESS` exist, and run `/usr/local/bin/gocdm -mode=user`.
