package cli

import (
	"os"
	"testing"
)

func TestIsSecureTTYPath(t *testing.T) {
	originalOsStat := osStat
	defer func() { osStat = originalOsStat }()

	osStat = func(name string) (os.FileInfo, error) {
		if name == "/dev/tty1" || name == "/dev/tty7" {
			return mockFileInfo{mode: os.ModeCharDevice}, nil
		}
		if name == "/dev/tty/1" || name == "/dev/tty_hacked" || name == "/dev/ttyA" {
			return mockFileInfo{mode: 0}, nil
		}
		return nil, os.ErrNotExist
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Valid tty1", "/dev/tty1", true},
		{"Valid tty7", "/dev/tty7", true},
		{"Invalid prefix", "/tmp/tty1", false},
		{"Invalid subdirectory", "/dev/tty/1", false},
		{"Invalid letters", "/dev/ttyA", false},
		{"Invalid hack payload", "/dev/tty_hacked", false},
		{"Missing numbers", "/dev/tty", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isSecureTTYPath(tc.path)
			if result != tc.expected {
				t.Errorf("expected %v for %s, got %v", tc.expected, tc.path, result)
			}
		})
	}
}

type mockFileInfo struct {
	os.FileInfo
	mode os.FileMode
}

func (m mockFileInfo) Mode() os.FileMode {
	return m.mode
}
