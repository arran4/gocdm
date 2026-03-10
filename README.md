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

## Using GoCDM as a getty replacement (Login Manager)

GoCDM can run in login-manager mode with native username/password prompts and PAM authentication.

### Installation

Build and install the standard binary:

```bash
go build -o gocdm ./cmd/gocdm
sudo install -m 0755 gocdm /usr/local/bin/gocdm
```

Build and install the PAM-enabled binary:

```bash
go build -tags pam -o gocdm-pam ./cmd/gocdm
sudo install -m 0755 gocdm-pam /usr/local/bin/gocdm
```

In login mode, GoCDM authenticates credentials through PAM, prepares a sanitized session environment (`HOME`, `SHELL`, `USER`, `LOGNAME`, plus `/etc/security/pam_env.conf`), and drops privileges to the authenticated account before launching the selected session.

### RC/profile autostart setup (user session)

If you want GoCDM to start after a normal shell login, append this to `~/.profile` (or `~/.bash_profile`):

```sh
if [ -z "${DISPLAY:-}" ] && [ "$(tty)" = "/dev/tty1" ]; then
  exec /usr/local/bin/gocdm
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
TTYPath=/dev/tty1
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
* On successful authentication, GoCDM derives a session environment from user identity (`/etc/passwd`) and `/etc/security/pam_env.conf` values.
* Privilege transitions are explicit: `-login` uses PAM auth first, then drops privileges to the authenticated account before launching session executables.
* `-login` should be run from a real TTY (the binary rejects non-interactive launch unless `-dry-run` is used).

#### PAM requirements

GoCDM requires a PAM-enabled build for `-login` mode:

```bash
go build -tags pam -o gocdm ./cmd/gocdm
```

Default PAM service is `login` (override with `-pam-service`). Typical stacks:

* Debian/Ubuntu: `login` service in `/etc/pam.d/login`
* Fedora/RHEL: `login` service in `/etc/pam.d/login`
* Arch Linux: `login` service in `/etc/pam.d/login`

If PAM support is missing at build time, authentication fails with a clear error indicating to rebuild with `-tags pam`.

#### Failure modes and UX

* Authentication prompt failure (stdin/stdout not usable): exits with error `Authentication prompt failed: ...`.
* PAM authentication failure (wrong password/policy denial/service failure): exits with error `Authentication failed: ...`.
* Non-interactive launch without `-dry-run`: exits with `gocdm must be launched from an interactive TTY`.
* If secure session environment assembly fails, GoCDM prints a warning and falls back to inherited environment.

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

- Console (`C`) and Wayland (`W`) entries execute configured commands directly.
- X (`X`) supports both existing-session VT handoff (`locktty`) and new-session launch (`startx` path).

### Intentional differences from legacy shell flow

- Deprecated config knobs now fail explicitly instead of behaving as silent/no-ops:
  - `altstartx`
  - non-default `cktimeout` in ConsoleKit path
- External X tooling is validated up-front with actionable errors (`xdpyinfo`, `chvt`, `startx`, `ck-launch-session`).
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
