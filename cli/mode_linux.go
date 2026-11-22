//go:build linux

package cli

import (
	"os"
	"strconv"
	"strings"
)

func readSystemLoginUID() (uint32, bool) {
	data, err := os.ReadFile("/proc/self/loginuid")
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return 0, false
	}
	val, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(val), true
}
