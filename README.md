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

## Customisation

See `/etc/cdmrc` for examples.
