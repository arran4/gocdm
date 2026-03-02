# GoCDM Port Completion TODO

This checklist captures the remaining work needed to call the Go port feature-complete, production-ready, and maintainable.

## 1) Reach behavior parity with the original CDM script

- [ ] Audit the original `cdm` shell script behavior option-by-option and session-path-by-session-path against `cli.Run`, then document any intentional differences.
- [ ] Implement and/or remove dead config paths currently parsed but not fully implemented:
  - [ ] `altstartx` behavior is currently parsed but not used in X launch flow.
  - [ ] `cktimeout` behavior is currently parsed but not used for any ConsoleKit timeout/monitoring logic.
- [ ] Verify `locktty` and `xtty=keep` behavior on real TTYs across common Linux setups (systemd getty, OpenRC, etc.) and align with legacy behavior via a dedicated manual bash test script plus testing-instructions markdown.
- [ ] Validate session discovery parity for `.desktop` edge cases (quoted values, escaped values, localization fields, unusual whitespace/order) with a CI script and companion documentation.

## 2) Harden login-manager mode for real deployments

- [ ] Complete threat-model review for `-login` mode (credential handling lifetime, env sanitization boundaries, privilege transitions).
- [ ] Confirm PAM behavior with common stacks (`login`, distro-specific services) and document required PAM config.
- [ ] Add integration checks for privilege drop + environment assembly + session exec path under a temporary test user harness.
- [ ] Validate failure modes and UX for authentication errors, missing PAM support, and non-interactive TTY launch.

## 3) Improve portability and platform strategy

- [ ] Decide and document platform support matrix (Linux primary, BSD secondary, all other platforms best-effort/unsupported).
- [ ] For unsupported platforms, provide explicit UX and docs (current Windows code path returns "not supported" for exec/login features).
- [ ] Split Linux-specific behavior behind build tags/interfaces where appropriate to keep non-Linux builds clean and intentional.

## 4) Make config loading safer and more robust

- [ ] Replace shell-sourcing based config parsing with a native parser (or explicitly constrain/trust model in docs if sourcing is retained).
- [ ] Add validation diagnostics for malformed arrays and mismatched `binlist`/`namelist`/`flaglist` lengths.
- [ ] Define and enforce precedence rules across positional config path, `-config`, and auto-discovery sources.

## 5) Complete X/Wayland launch-path robustness

- [ ] Implement or remove `altStartX` branch in `x11.LaunchXSession`.
- [ ] Implement or remove `ckTimeout` semantics (actual timeout handling or deprecate setting).
- [ ] Add explicit handling for missing external tools (`startx`, `xdpyinfo`, `chvt`, `ck-launch-session`) with actionable error messages.
- [ ] Add integration documentation for Wayland sessions where compositor startup differs from X session assumptions.

## 6) Expand test coverage to release confidence

- [ ] Add end-to-end CLI tests for:
  - [ ] Session auto-selection vs. forced menu behavior.
  - [ ] `-dry-run`, `-version`, and argument precedence.
  - [ ] Login-mode happy path and failure path with mocked PAM/auth plumbing.
- [ ] Add higher-fidelity integration tests (in temp-isolated env) for X session command construction.
- [ ] Add golden/fixture-driven tests for config parsing edge cases.
- [ ] Add regression tests for state persistence and default-session selection interaction.

## 7) Ship-readiness tasks

- [ ] Add a "port status" section in `README.md` that clearly states what is complete, partial, and intentionally out-of-scope.
- [ ] Add packaging/install docs for major distros (binary install + PAM-enabled build variants).
- [ ] Add operational troubleshooting guide (TTY errors, X display allocation failures, PAM failures).
- [ ] Establish release checklist (lint, unit tests, integration/manual checks, versioning, changelog).

## 8) Nice-to-have (post-parity)

- [ ] Keep a strict no-telemetry policy while adding local-only debug logging mode for easier field diagnosis.
- [ ] Add optional structured output mode for wrappers/integrators.
- [ ] Expand bindings/examples for embedding use cases.
