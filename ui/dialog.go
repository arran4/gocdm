package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var execCommand = exec.Command

// ShowMenu displays a menu using the dialog command.
// options is a slice of option names.
// startIdx is the index to start numbering options.
// theme is the path to the dialogrc file.
// Returns the index of the selected option (0-based relative to options), or error.
func ShowMenu(title string, options []string, startIdx int, theme string) (int, error) {
	// Construct the menu arguments
	menuArgs := []string{}
	for i, opt := range options {
		menuArgs = append(menuArgs, strconv.Itoa(i+startIdx), opt)
	}

	args := []string{
		"--colors", "--stdout",
		"--backtitle", title,
		"--ok-label", " Select ",
		"--cancel-label", " Exit ",
		"--menu", "Select session", "0", "0", "0",
	}
	args = append(args, menuArgs...)

	cmd := execCommand("dialog", args...)
	cmd.Stderr = os.Stderr // dialog uses stderr for the TUI

	if theme != "" {
		cmd.Env = append(os.Environ(), "DIALOGRC="+theme)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// dialog returns 1 on Cancel/Esc
			if exitError.ExitCode() == 1 {
				return -1, fmt.Errorf("cancelled")
			}
		}
		return -1, fmt.Errorf("dialog command failed: %w", err)
	}

	selectedStr := strings.TrimSpace(string(output))
	selectedIdx, err := strconv.Atoi(selectedStr)
	if err != nil {
		return -1, fmt.Errorf("invalid output from dialog: %s", selectedStr)
	}

	return selectedIdx - startIdx, nil
}
