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
ExecStart=-/usr/local/bin/gocdm -login -pam-service login
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
