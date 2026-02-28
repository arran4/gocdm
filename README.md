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

Currently, GoCDM operates as a display manager *after* a user has already logged in to a TTY (by sourcing it in `.profile` or `.bash_profile`). It does not yet include PAM integration or user authentication out of the box to fully replace `getty` or `login`.

When launching sessions, GoCDM now derives identity environment variables (`HOME`, `SHELL`, `USER`, `LOGNAME`) from `/etc/passwd` and applies values from `/etc/security/pam_env.conf` (`DEFAULT=` and `OVERRIDE=` entries) before exec'ing the selected session.

However, you can achieve a pseudo-getty autologin setup by configuring your init system (e.g., systemd) to auto-login your user on a specific TTY, and then have your shell profile immediately launch GoCDM.

1. Create a drop-in systemd override for `getty@tty1.service`:
   ```bash
   sudo systemctl edit getty@tty1.service
   ```
2. Add the following lines to autologin your user (replace `username`):
   ```ini
   [Service]
   ExecStart=
   ExecStart=-/sbin/agetty -o '-p -f -- \\u' --noclear --autologin username %I $TERM
   ```
3. Ensure GoCDM is invoked in your user's `~/.profile` or `~/.bash_profile`.

*Note: Fully replacing `getty` standalone by integrating PAM directly into GoCDM is currently tracked as a future feature in `TODO.md`.*

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
