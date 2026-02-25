package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/arran4/gocdm/config"
	"github.com/arran4/gocdm/session"
	"github.com/arran4/gocdm/ui"
	"github.com/arran4/gocdm/x11"
)

func main() {
	var configPath string
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Populate sessions if binlist is empty
	var sessions []session.Session
	if len(cfg.BinList) == 0 {
		sessions, err = session.DiscoverSessions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering sessions: %v\n", err)
			os.Exit(1)
		}
		if len(sessions) == 0 {
			fmt.Fprintln(os.Stderr, "No sessions found.")
			os.Exit(1)
		}
	} else {
		// Convert config lists to sessions
		// Assumes lists are equal length. cfg.BinList, cfg.NameList, cfg.FlagList
		for i := 0; i < len(cfg.BinList); i++ {
			name := cfg.BinList[i]
			if i < len(cfg.NameList) {
				name = cfg.NameList[i]
			}
			flag := "X"
			if i < len(cfg.FlagList) {
				flag = cfg.FlagList[i]
			}
			sessions = append(sessions, session.Session{
				Name: name,
				Exec: cfg.BinList[i],
				Type: flag,
			})
		}
	}

	var selectedIdx int
	if len(sessions) == 1 {
		selectedIdx = 0
	} else {
		optionNames := make([]string, len(sessions))
		for i, s := range sessions {
			optionNames[i] = s.Name
		}

		idx, err := ui.ShowMenu("Console Display Manager", optionNames, cfg.CountFrom, cfg.DialogRC)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Selection cancelled or error: %v\n", err)
			os.Exit(2)
		}
		selectedIdx = idx
	}

	if selectedIdx < 0 || selectedIdx >= len(sessions) {
		fmt.Fprintf(os.Stderr, "Invalid selection index: %d\n", selectedIdx)
		os.Exit(1)
	}

	selectedSession := sessions[selectedIdx]

	// Normalize flag
	flag := strings.ToUpper(selectedSession.Type)
	// gocdm script: [Cc] -> Console, [Xx] -> X.
	if strings.HasPrefix(flag, "C") {
		flag = "C"
	} else if strings.HasPrefix(flag, "X") {
		flag = "X"
	}

	switch flag {
	case "C":
		// Console program
		parts := strings.Fields(selectedSession.Exec)
		if len(parts) == 0 {
			fmt.Fprintln(os.Stderr, "Empty command")
			os.Exit(1)
		}
		bin := parts[0]
		args := parts[1:]

		binary, err := exec.LookPath(bin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command not found: %s\n", bin)
			os.Exit(1)
		}

		env := os.Environ()
		env = append(env, fmt.Sprintf("GOCDM_SPAWN=%d", os.Getpid()))

		if err := syscall.Exec(binary, append([]string{bin}, args...), env); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to exec: %v\n", err)
			os.Exit(1)
		}

	case "X":
		// X program
		display := cfg.Display // Start from 0 effectively since FindFreeDisplay starts at 0

		// Find free display
		disp, err := x11.FindFreeDisplay()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to find free display: %v\n", err)
			os.Exit(1)
		}
		display = disp

		vt, err := x11.GetVT(cfg.XTTY, display)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get VT: %v\n", err)
			os.Exit(1)
		}

		parts := strings.Fields(selectedSession.Exec)
		err = x11.LaunchXSession(parts, display, vt, cfg.ConsoleKit, cfg.CKTimeout, cfg.AltStartX, cfg.StartXLog, cfg.ServerArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch X session: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown session type: %s\n", selectedSession.Type)
		os.Exit(1)
	}
}
