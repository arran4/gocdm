package config

import (
	"os"
	"path/filepath"
	"testing"
)

func DiscoverConfigPathOriginal() string {
	home, _ := os.UserHomeDir()
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	paths := []string{
		filepath.Join(home, ".cdmrc"),
		filepath.Join(xdgConfig, "cdm", "cdmrc"),
		"/etc/cdmrc",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func BenchmarkDiscoverConfigPathOriginal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DiscoverConfigPathOriginal()
	}
}

func BenchmarkDiscoverConfigPathOptimized(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DiscoverConfigPath()
	}
}

func BenchmarkDiscoverConfigPathOriginalWithFile(b *testing.B) {
	home := b.TempDir()
	b.Setenv("HOME", home)
	b.Setenv("USERPROFILE", home)
	f := filepath.Join(home, ".cdmrc")
	os.WriteFile(f, []byte(""), 0644)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscoverConfigPathOriginal()
	}
}

func BenchmarkDiscoverConfigPathOptimizedWithFile(b *testing.B) {
	home := b.TempDir()
	b.Setenv("HOME", home)
	b.Setenv("USERPROFILE", home)
	f := filepath.Join(home, ".cdmrc")
	os.WriteFile(f, []byte(""), 0644)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscoverConfigPath()
	}
}
