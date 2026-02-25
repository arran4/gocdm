#!/bin/bash
# To avoid potential situation where gocdm(1) crashes on every TTY, here we
# default to execute gocdm(1) on tty1 only, and leave other TTYs untouched.
if [ "$(tty)" = '/dev/tty1' ]; then
    [ -n "$GOCDM_SPAWN" ] && return
    # Avoid executing gocdm(1) when X11 has already been started.
    [ -z "$DISPLAY$SSH_TTY$(pgrep xinit)" ] && exec gocdm
fi
