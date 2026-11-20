# GoCDM

The Console Display Manager (Go Port)

## Invocation

To run GoCDM, use `gocdm [RCFILE]`.

GoCDM tries to source configuration files in the following order, and uses
the first found configuration:

* [RCFILE] (direct input)
* `$HOME/.cdmrc`
* `$XDG_CONFIG_HOME/cdm/cdmrc`
* `/etc/cdmrc`

To autostart gocdm when you log in your account, copy the content of
/usr/share/doc/gocdm/profile.sh to the tail of your shell profile (~/.profile,
etc.).

## Operating modes

GoCDM supports two distinct operating modes:

1. **Direct getty replacement (`-login` or `-mode=login` / `-mode=getty`)**: GoCDM runs as a root-owned login manager directly on a virtual terminal, authenticates credentials once via PAM, and drops privileges to the authenticated user before launching the chosen session.
2. **Post-login user autostart (`-mode=user` / `-mode=autostart` or `-login=false`)**: An already-authenticated user launches GoCDM after a normal shell login (for example from `~/.profile` or `profile.sh`). GoCDM displays the session menu and launches the chosen session without prompting for PAM authentication again.

### Mode selection and auto-detection (`-mode`)

* `-mode=auto` (default): Selects direct getty login mode only when GoCDM is running with real and effective UID `0`, both stdin and stdout are attached to the same `/dev/ttyN` character device, Linux audit `loginuid` is unset (when available), and no active session markers are set (`DISPLAY`, `WAYLAND_DISPLAY`, `SSH_TTY`, `SSH_CONNECTION`, `SSH_CLIENT`, `XDG_SESSION_ID`, `GOCDM_SPAWN`, `SHLVL`, `USER`, `LOGNAME`, `SUDO_USER`, `DOAS_USER`). Otherwise it defaults to already-authenticated user session mode.
* `-mode=login` (or `-mode=getty`, `-login`): Forces direct getty login mode. Requires `euid == 0` (unless `-dry-run` is specified) and refuses to run inside an existing graphical (`DISPLAY`/`WAYLAND_DISPLAY`) or SSH session.
* `-mode=user` (or `-mode=autostart`, `-login=false`): Forces already-authenticated user session mode with no PAM authentication prompt.
* Combining conflicting explicit flags such as `-login -mode=user` or `-login=false -mode=login` fails with exit status `2`.

> **Migration note (from v0.0.26):** Earlier releases inferred `-login` whenever `DISPLAY` was unset and a `/dev/ttyN` descriptor was open, which could mistakenly prompt already-authenticated users for PAM credentials again during `~/.profile` autostart on `tty1`. In `-mode=auto`, post-`login(1)` shells on `tty1` now stay in user session mode. Passing `-login` explicitly in `getty@tty1.service` and `-mode=user` (or bare `gocdm`) in profile autostart is recommended.

## Using GoCDM as a getty replacement (Login Manager)

GoCDM can run in login-manager mode with native username/password prompts and PAM authentication.

### Installation

Build and install the standard binary:

```bash
go build -o gocdm ./cmd/gocdm
sudo install -m 0755 gocdm /usr/local/bin/gocdm
```

Build and install the binary without PAM support:

```bash
go build -tags nopam -o gocdm-nopam ./cmd/gocdm
sudo install -m 0755 gocdm-nopam /usr/local/bin/gocdm
```

In login mode, GoCDM authenticates credentials through PAM, resolves the canonical account via NSS/`/etc/passwd` before loading user configuration/sessions/state, constructs a minimal sanitized session environment (`USER`, `LOGNAME`, `HOME`, `SHELL`, safe `PATH`, allowlisted `TERM`/locale, `/etc/security/pam_env.conf`, and authoritative PAM-supplied environment such as `pam_systemd` `XDG_RUNTIME_DIR` without fabricating missing runtime/D-Bus values), drops privileges to the authenticated account, changes the working directory to the user's home (or `/` with a warning if unavailable), and writes `~/.config/gocdm/state.json` safely as the authenticated user before launching the selected session.

### RC/profile autostart setup (user session)

If you want GoCDM to start after a normal shell login, append this to `~/.profile` (or `~/.bash_profile`):

```sh
if [ -z "${DISPLAY:-}" ] && [ "$(tty)" = "/dev/tty1" ]; then
  exec /usr/local/bin/gocdm -mode=user
fi
```

### systemd getty replacement setup (login-manager mode)

To run GoCDM directly from tty1 under systemd, override `getty@tty1.service`:

```bash
sudo systemctl edit getty@tty1.service
```

Use this override:

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

Then reload and restart:

```bash
sudo systemctl daemon-reload
sudo systemctl restart getty@tty1.service
```

## Customisation

See `/etc/cdmrc` for examples.


## C bindings

GoCDM now provides a `c-shared` entrypoint at `cmd/gocdm-bindings` for embedding core capabilities in native applications.

Build the shared library and generated header with:

```bash
go build -buildmode=c-shared -o libgocdm.so ./cmd/gocdm-bindings
```

The generated header exposes:

* `GoCDMDefaultConfigJSON()`
* `GoCDMLoadConfigJSON(const char* path)`
* `GoCDMDiscoverSessionsJSON(const char* home_dir)`
* `GoCDMFreeCString(char* s)`

Each function returns a heap-allocated JSON response envelope with this shape:

```json
{"ok":true,"data":{...}}
```

or on failure:

```json
{"ok":false,"error":"..."}
```

Always release returned strings with `GoCDMFreeCString`.

### Login-mode security and operations notes

#### Threat model summary (`-login`)

* Credentials are read only at interactive prompt time and sent directly to PAM for validation.
* Pre-login configuration only reads explicit `-config`/`RCFILE` or `/etc/cdmrc` (never the launching root user's `~/.cdmrc`). User config, session discovery, and last-session state are resolved only after authentication against the authenticated account.
* On successful authentication, GoCDM constructs the session environment from a minimal allowlisted base (`TERM` and locale/TZ variables) rather than inheriting the privileged launcher environment wholesale, then applies canonical identity (`USER`, `LOGNAME`, `HOME`, `SHELL`), safe `PATH`, `/etc/security/pam_env.conf`, and authoritative PAM session variables (such as `pam_systemd` values) without fabricating `XDG_RUNTIME_DIR` or `DBUS_SESSION_BUS_ADDRESS`.
* Privilege transitions are explicit: `-login` follows the full PAM lifecycle (`pam_authenticate` -> `pam_acct_mgmt` -> `pam_setcred(ESTABLISH)` -> `pam_open_session`), flushes PAM notices/MOTD once, persists state via an unprivileged helper, supervises the child session with credential drop (`UID`/`GID`/supplementary groups) and TTY handoff, and restores TTY state + runs `pam_close_session` / `pam_setcred(DELETE)` / `pam_end` on logout. See `docs/PAM_LIFECYCLE.md` for architecture and signal details.
* `-login` should be run from a real TTY (the binary rejects non-interactive launch unless `-dry-run` is used).

#### PAM requirements

GoCDM requires a PAM-enabled build for `-login` mode:

```bash
go build -o gocdm ./cmd/gocdm
```

Default PAM service is `login` (override with `-pam-service`). Typical stacks:

* Debian/Ubuntu: `login` service in `/etc/pam.d/login`
* Fedora/RHEL: `login` service in `/etc/pam.d/login`
* Arch Linux: `login` service in `/etc/pam.d/login`

If PAM support is missing at build time, authentication fails with a clear error indicating to rebuild without `-tags nopam`.

#### Failure modes and UX

* Authentication prompt failure (stdin/stdout not usable): exits with error `Authentication prompt failed: ...`.
* PAM authentication failure (wrong password/policy denial/service failure): exits with error `Authentication failed: ...`.
* Non-interactive launch without `-dry-run`: exits with `gocdm must be launched from an interactive TTY`.
* If secure session environment assembly fails in `-login` mode, GoCDM closes the PAM session and exits with `Error: failed to build secure session environment: ...` instead of falling back to the privileged launcher environment.
* If the authenticated user's home directory is missing or inaccessible, GoCDM warns and falls back to `/` as the working directory. State persistence under `~/.config/gocdm/state.json` refuses symlinks and reports legacy root-owned state files diagnostically without calling `chown`.

## Platform support matrix

| Platform | Support tier | `-login` mode | X session launch path |
|---|---|---|---|
| Linux | Primary | Supported | Supported |
| FreeBSD / OpenBSD / NetBSD / DragonFly BSD | Secondary (best-effort) | Supported | Supported (best-effort) |
| Windows | Unsupported | Not supported | Not supported |
| Other platforms (e.g. macOS) | Unsupported | Not supported | Not supported |

### Unsupported-platform UX

GoCDM now emits explicit runtime errors for unsupported platform features:

* `-login is not supported on <GOOS> (support tier: unsupported)`
* `X session launch is not supported on <GOOS> (support tier: unsupported)`

This keeps behavior intentional on non-Linux/BSD targets instead of failing later in the launch flow.


## Port parity summary

### Option/session behavior status

- Config path resolution (`RCFILE`, `$HOME/.cdmrc`, `$XDG_CONFIG_HOME/cdm/cdmrc`, `/etc/cdmrc`) is implemented in that order.
- `-menu`, `-dry-run`, and `-version` behavior are implemented.
- `-login` prompt/auth + privilege-transition flow is implemented.
- Non-interactive startup is blocked unless `-dry-run` is used.

### Session path status

- Console (`C`) entries distinguish registered login shells (`/etc/shells`, matching `IsShell` or symlink-resolved bare shell paths) from arbitrary console commands or commands with explicit arguments:
  - In direct getty mode (`-login`), selected bare shells are invoked with login-shell `argv[0]` (`-<basename>`, e.g. `-zsh`, `-bash`) in the authenticated user's home with `SHELL` set to the selected shell so normal system/user login initialization runs while PAM delivers MOTD once.
  - In post-login autostart mode (`-mode=user`), selected shells launch without a leading `-` to avoid duplicating login profile startup.
  - Arbitrary console executables (and shell invocations with explicit arguments) execute directly with shell-quoted argument splitting and do not source user shell profiles.
- Wayland (`W`) entries execute configured commands directly.
- X (`X`) supports both existing-session VT handoff (`locktty`) and new-session launch (`startx` path).

### Intentional differences from legacy shell flow

- Deprecated config knobs now fail explicitly instead of behaving as silent/no-ops:
  - `altstartx`
  - non-default `cktimeout` in ConsoleKit path
- External X tooling is validated up-front with actionable errors (`chvt`, `startx`, `ck-launch-session`).
- Platform support policy is explicit (Linux primary, BSD secondary, other OS unsupported).

### Remaining parity work

- Real-TTY/manual validation matrix for `locktty` + `xtty=keep` across init systems.
- Additional `.desktop` discovery parity expansion as new edge cases are identified.

## Manual/CI validation helpers

- Manual real-TTY validation for `locktty` + `xtty=keep`:
  - script: `scripts/manual/locktty_xtty_keep_test.sh`
  - guide: `docs/LOCKTTY_XTTY_MANUAL_TEST.md`
- CI parser edge-case checks for `.desktop` discovery:
  - script: `scripts/ci/session_desktop_edgecases.sh`
  - guide: `docs/SESSION_DISCOVERY_EDGECASES.md`


## Wayland integration

For compositor/session startup guidance where Wayland behavior differs from X-session assumptions, see `docs/WAYLAND_INTEGRATION.md`.


## Config parsing and precedence

GoCDM uses a native config parser (no shell-sourcing) for `cdmrc` values.

Validation rules:

- Array fields (`binlist`, `namelist`, `flaglist`, `serverargs`) must use parenthesized shell-style list syntax.
- If `namelist` or `flaglist` is provided, its length must match `binlist` length.

Config source precedence is enforced as:

1. `-config <path>`
2. positional config path (`gocdm <path>`)
3. auto-discovery (`$HOME/.cdmrc`, `$XDG_CONFIG_HOME/cdm/cdmrc`, `/etc/cdmrc`)

Providing both `-config` and a positional config path is treated as an error.

## Port status

### Complete

- Native Go CLI session dispatcher (`C`/`W`/`X`) with config/state handling.
- Native config parser with validation and explicit precedence rules.
- Login-manager mode with PAM-aware flow, privilege drop path, and failure-mode UX.
- X launch hardening (tool checks, explicit deprecation handling for unsupported legacy knobs).
- Platform support policy with explicit unsupported-platform UX.

### Partial / requires manual environment verification

- Real-TTY behavior validation across distributions/init systems (`locktty`, `xtty=keep`) via manual script.
- Deployment-specific Wayland compositor startup wrappers and distro integration details.

### Intentionally out of scope (for now)

- Full runtime parity for deprecated legacy knobs (`altstartx`, non-default `cktimeout`).
- Non-Linux/BSD first-class support (other platforms remain unsupported/best-effort only).

## Packaging and distro installation

See `docs/PACKAGING.md` for distro-oriented install guidance (standard and PAM-enabled builds).

## Troubleshooting

See `docs/TROUBLESHOOTING.md` for common operational errors and remediation steps.

## Release checklist

See `docs/RELEASE_CHECKLIST.md` for pre-release gates and tagging/changelog steps.

## Embedding examples

See `examples/embedding/c/README.md` for a minimal C embedding workflow using `cmd/gocdm-bindings`.
