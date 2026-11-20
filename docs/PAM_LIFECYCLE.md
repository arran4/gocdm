# PAM Session Lifecycle and Supervision Architecture

## Session-Ownership Alternatives Evaluated

When GoCDM runs as a direct `getty` replacement (`ModeGettyLogin`), it opens a PAM transaction (`pam_open_session`), which in modern Linux distributions invokes `pam_systemd` to register a logind session tied to the calling process lifetime and controlling TTY.

Three architectures were evaluated:

1. **Direct `syscall.Exec` replacement (prior `<= 0.0.26` behavior for `C`/`W`)**:
   - *Pros:* Zero parent process remains in memory.
   - *Cons:* The process image is replaced while `pam_close_session`, `pam_setcred(PAM_DELETE_CRED)`, and `pam_end` have not run. Logind session teardown relies only on session-leader exit heuristics rather than clean PAM close/credential cleanup, TTY ownership/mode cannot be restored cleanly on exit, and a crashed or signaled child never triggers PAM session cleanup.
2. **Persistent privileged system daemon**:
   - *Pros:* Centralizes session tracking across VTs.
   - *Cons:* Introduces a long-lived privileged background service, IPC surface, and systemd unit redesign unnecessary for a per-VT console display manager.
3. **Single-session per-VT parent/child supervisor (selected for `ModeGettyLogin`)**:
   - *Pros:* Keeps zero persistent daemon beyond the active VT login instance, matches `login(1)` / `agetty` / display-manager session-leader semantics, retains the open PAM handle across the child lifetime, runs `pam_close_session` -> `pam_setcred(PAM_DELETE_CRED)` -> `pam_end` unconditionally on child exit or signal, restores TTY ownership/permissions and foreground process group, and keeps `ModeUserSession` (`-mode=user`) completely free of any supervisor process (using direct `exec`).
   - *Cons:* One small per-VT supervisor process remains while that VT session is active.

## Privilege Boundary and Child Launch Policy

- **PAM transaction ordering:**
  1. `pam_start` and `PAM_TTY` setup
  2. `pam_authenticate` (retaining notices)
  3. `pam_acct_mgmt` (retaining notices)
  4. Canonical `PAM_USER` check (`pam_get_item`) -> OS account lookup (`AccountInfo`)
  5. `pam_putenv` metadata (`XDG_SESSION_TYPE`, `XDG_SESSION_CLASS`, `XDG_VTNR`, `XDG_SEAT`, and `XDG_SESSION_DESKTOP` for graphical sessions)
  6. `pam_setcred(PAM_ESTABLISH_CRED)` followed by `pam_open_session` (with automatic `PAM_DELETE_CRED` + `pam_end` rollback if `pam_open_session` fails)
  7. Delivery of accumulated `pam.TextInfo` / `pam.ErrorMsg` notices once
  8. `pam_getenvlist` merged into sanitized session environment
  9. TTY ownership/permission handoff to authenticated UID / `tty` group (`0620`)
  10. Unprivileged state persistence (`SaveStateAsUser`, spawning a short-lived unprivileged self-helper via `GOCDM_INTERNAL_SAVE_STATE=1` when `euid == 0` for non-root users)
  11. Child session launch with `SysProcAttr.Credential` dropping `UID`, `GID`, and supplementary groups inside the child fork (`Setpgid: true`, `Foreground: true` / `Ctty` when stdin owns the controlling TTY)
  12. Child wait -> reclaim foreground process group (`TIOCSPGRP`) -> restore original TTY ownership/mode -> `pam_close_session` -> `pam_setcred(PAM_DELETE_CRED)` -> `pam_end` -> propagate child exit status (`128 + signal` when terminated by signal)

- **Signal handling in the supervisor:**
  - `SIGINT`, `SIGQUIT`, `SIGTSTP`, `SIGTTIN`, and `SIGTTOU` are ignored in the supervisor while waiting so interactive terminal signals reach the foreground child process group rather than killing the supervisor mid-session.
  - `SIGTERM` and `SIGHUP` received by the supervisor are forwarded to the child's process group (`kill(-childPid, sig)`); the supervisor waits for child teardown before closing PAM.

## PAM Notices and MOTD Policy

- All `pam.TextInfo` and `pam.ErrorMsg` messages from authentication, account management, credential establishment, and session opening are accumulated in order and flushed **once** (`Info` -> `stdout`, `Error` -> `stderr`) immediately after `OpenSession` succeeds, preserving indentation/ASCII art.
- GoCDM **never** reads or prints `/etc/motd` directly on top of PAM; MOTD display follows the configured `/etc/pam.d/login` (or `-pam-service`) policy (such as `pam_motd.so`), avoiding duplicate MOTDs.

## Opt-In Real-VT Validation (Gentoo / systemd)

Perform real-TTY verification on a spare VT (e.g., `tty2`) while leaving `tty1` or another VT untouched as a recovery console. Never weaken `/etc/pam.d/login` or edit real user dotfiles during automated testing.

1. Configure a spare VT service drop-in (`systemctl edit getty@tty2.service`):
   ```ini
   [Service]
   ExecStart=
   ExecStart=-/usr/local/bin/gocdm -login
   StandardInput=tty
   StandardOutput=tty
   StandardError=tty
   TTYPath=/dev/%I
   TTYReset=yes
   TTYVHangup=yes
   TTYVTDisallocate=yes
   ```
2. Switch to `tty2` (`chvt 2`), authenticate with a test account, and verify:
   - Configured PAM notices / MOTD appear once after login without extra duplication.
   - `loginctl session-status` shows an active `user` class session on `seat0` / `tty2` with expected type (`tty`, `x11`, or `wayland`).
   - Printed environment variables remain free of root secrets (redact any tokens before sharing logs).
3. Exit the session (`logout` / compositor quit) and check from the recovery VT (`loginctl list-sessions` / `systemctl status getty@tty2.service`) that the prior logind session closed cleanly (`Closing` -> removed) and `tty2` presents a fresh login prompt.
4. Log in a second time on `tty2` and confirm a new logind session ID and single MOTD presentation.
