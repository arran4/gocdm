# Wayland integration notes

GoCDM launches Wayland entries by executing the configured `Exec` command directly (session type `W`).
Unlike the X path, it does **not** run through `startx`, VT switching, or display probing.

## Key differences from X launch assumptions

- No `startx` wrapper is used.
- No X11 socket probing is performed to find free displays.
- No `chvt` handoff is performed by GoCDM.
- `XDG_SESSION_TYPE=wayland` is appended to the launched environment.

## Compositor startup expectations

Your Wayland `Exec` command should be self-contained and start the compositor/session directly, for example:

- `sway`
- `weston-launch` (where applicable)
- `dbus-run-session sway` (if your distro/session requires dbus bootstrap)

If your compositor needs additional environment setup (`XDG_RUNTIME_DIR`, seat/session helpers, policykit/dbus agents), place that logic in the command itself or a wrapper script referenced by `binlist`.

## Recommended wrapper pattern

For compositor-specific setup, use a wrapper script and reference it in `binlist`:

```sh
#!/bin/sh
export XDG_CURRENT_DESKTOP=sway
exec dbus-run-session sway
```

Then set `flaglist` entry to `W`.

## Login-mode interaction

In `-login` mode, GoCDM authenticates via PAM, assembles session environment, drops privileges, then executes the Wayland command.
Any compositor prerequisites not provided by PAM/system services should be handled by your wrapper.
