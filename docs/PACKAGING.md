# Packaging and installation notes

## Build variants

### Standard binary

```bash
go build -o gocdm ./cmd/gocdm
```

### PAM-enabled binary

```bash
go build -tags pam -o gocdm ./cmd/gocdm
```

## Suggested install paths

```bash
sudo install -m 0755 gocdm /usr/local/bin/gocdm
```

## Distro guidance (baseline)

- Debian/Ubuntu: package as `gocdm`, include optional PAM-enabled build variant.
- Fedora/RHEL: package as `gocdm`, wire service docs for `getty@tty1` replacement.
- Arch Linux: provide PKGBUILD variants or split package (`gocdm`, `gocdm-pam`).

For all distros, document:

- required runtime deps (`dialog`, `startx` if X path used, PAM libs for PAM builds),
- installed docs/scripts under `/usr/share/doc/gocdm/`.
