# GoCDM TODO

The original porting checklist is complete.

## Remaining work

- [ ] Standalone `getty` replacement support
  - [ ] Integrate PAM for native user authentication (username/password prompting)
  - [ ] Manage TTY allocation and session handoffs directly (replacing `login`/`agetty`)

## Next milestone candidate

- [ ] Define an incremental implementation plan for PAM + TTY ownership lifecycle
- [ ] Add integration tests for non-interactive PAM/auth failure paths in an isolated test harness
