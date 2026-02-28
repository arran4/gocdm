# CDM Porting Tasks

- [x] Initialize Repository and Tracking
    - [x] Create `TODO.md`
    - [x] Initialize Go module `github.com/arran4/gocdm`
    - [x] Copy `license.txt`
    - [x] Copy `themes/`
    - [x] Verify structure

- [x] Implement Configuration Loading
    - [x] Create `config/config.go`
    - [x] Implement `LoadConfig()`
    - [x] Write test for `LoadConfig`

- [x] Implement Session Discovery
    - [x] Create `session/discovery.go`
    - [x] Implement `DiscoverSessions()`
    - [x] Write test for `DiscoverSessions`

- [x] Implement UI (Dialog Wrapper)
    - [x] Create `ui/dialog.go`
    - [x] Implement `ShowMenu()`
    - [x] Write test for `ShowMenu`

- [x] Implement X Session Launching Logic
    - [x] Create `x11/x11.go`
    - [x] Implement `FindFreeDisplay()`
    - [x] Implement `GetVT()`
    - [x] Implement `LaunchXSession()`
    - [x] Write unit tests

- [x] Main Application Logic
    - [x] Create `cmd/cdm/main.go`
    - [x] Tie everything together
    - [x] Handle console program execution

- [x] Build System
    - [x] Create `.goreleaser.yaml`
    - [x] Verify `.goreleaser.yaml`

- [x] Cleanup and Verification
    - [x] Remove `temp_cdm`
    - [x] Run `go test ./...`
    - [x] Update `TODO.md`

- [ ] Gap Analysis / Missing Features
    - [ ] Support legacy `dialogrc` themes in `ui/dialog.go`
    - [x] Fully implement ConsoleKit monitoring in `x11/x11.go`

- [ ] Increase Test Coverage
    - [x] `config/state.go` tests (`TestLoadState`, `TestSaveState`)
    - [x] `x11/x11.go` tests (`TestGetVT`, `TestLaunchXSession`)
    - [x] `session/discovery.go` tests (cover `parseDesktopFile` errors)
    - [x] `ui/dialog.go` tests (cover details modal `?` key)
    - [x] `cmd/gocdm/main.go` tests (mock execution and flags)

- [ ] Support C bindings (ideally using github.com/ebitengine/purego instead of cgo if strictly necessary)
