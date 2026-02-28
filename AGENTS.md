## Testing Guidelines
* No Go-based test should perform actual OS tests (e.g., creating real files/devices, or spawning real terminals) outside of a temporary isolated environment. Use `tcell.NewSimulationScreen` for UI, mock `exec.Command` for OS commands, or `os.MkdirTemp` for files.
* You may produce bash test scripts for actual OS tests, but those should be clearly documented for manual user execution.
* If a test requires multi-line string contents (such as testing `.desktop` file parsing), use Go's `embed` package rather than writing strings inline, and store the test payload files in a `testdata/` directory relative to the package.
