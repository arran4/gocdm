package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type probeReport struct {
	Arg0 string   `json:"arg0"`
	Args []string `json:"args"`
	Cwd  string   `json:"cwd"`
	Env  []string `json:"env"`
	UID  int      `json:"uid"`
	EUID int      `json:"euid"`
	GID  int      `json:"gid"`
	EGID int      `json:"egid"`
}

func main() {
	base := filepath.Base(os.Args[0])
	cwd, _ := os.Getwd()
	rep := probeReport{
		Arg0: os.Args[0],
		Args: os.Args,
		Cwd:  cwd,
		Env:  os.Environ(),
		UID:  os.Getuid(),
		EUID: os.Geteuid(),
		GID:  os.Getgid(),
		EGID: os.Getegid(),
	}

	if outDir := os.Getenv("GOCDM_PROBE_DIR"); outDir != "" {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal probe: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(outDir, base+".json"), data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write probe: %v\n", err)
			os.Exit(1)
		}
	}

	if failCmd := os.Getenv("GOCDM_PROBE_FAIL_CMD"); failCmd != "" && failCmd == base {
		code := 1
		if raw := os.Getenv("GOCDM_PROBE_EXIT"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				code = parsed
			}
		}
		fmt.Fprintf(os.Stderr, "%s simulated failure (%d)\n", base, code)
		os.Exit(code)
	}

	if base == "plasma-dbus-run-session-if-needed" && len(os.Args) > 1 {
		nextBin, err := exec.LookPath(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "wrapper failed to resolve %s: %v\n", os.Args[1], err)
			os.Exit(127)
		}
		cmd := exec.Command(nextBin, os.Args[2:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			fmt.Fprintf(os.Stderr, "wrapper run error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if raw := os.Getenv("GOCDM_PROBE_EXIT"); raw != "" {
		if code, err := strconv.Atoi(raw); err == nil {
			os.Exit(code)
		}
	}
}
