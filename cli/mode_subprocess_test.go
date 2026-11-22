package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildModeTestBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "gocdm-test")
	cmd := exec.Command("go", "build", "-tags", "nopam", "-o", binPath, "../cmd/gocdm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build -tags nopam ../cmd/gocdm failed: %v\n%s", err, string(out))
	}
	return binPath
}

func cleanSubprocessEnv(homeDir string) []string {
	var env []string
	for _, k := range []string{"PATH", "TMPDIR", "TEMP", "TMP", "SYSTEMROOT"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	env = append(env, "HOME="+homeDir)
	return env
}

func runCompiledGoCDM(t *testing.T, bin string, homeDir string, extraEnv []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(cleanSubprocessEnv(homeDir), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to execute %s %v: %v", bin, args, err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

func TestCompiledSubprocessOperatingModes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("non-root subprocess privilege checks require non-root test execution")
	}

	bin := buildModeTestBinary(t)
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "cdmrc")
	if err := os.WriteFile(cfgPath, []byte("binlist=(\"echo ok\")\nnamelist=(\"Echo\")\nflaglist=(\"C\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("non-root dry-run without DISPLAY resolves user mode", func(t *testing.T) {
		stdout, stderr, code := runCompiledGoCDM(t, bin, tmpDir, []string{"DISPLAY="}, "-config", cfgPath, "-dry-run")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "Dry run: would execute console program: echo [ok]") {
			t.Fatalf("unexpected stdout: %q (stderr=%q)", stdout, stderr)
		}
	})

	t.Run("explicit user mode dry-run", func(t *testing.T) {
		stdout, stderr, code := runCompiledGoCDM(t, bin, tmpDir, nil, "-config", cfgPath, "-mode=user", "-dry-run")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "Dry run: would execute console program: echo [ok]") {
			t.Fatalf("unexpected stdout: %q (stderr=%q)", stdout, stderr)
		}
	})

	t.Run("non-root explicit -login without dry-run fails fast", func(t *testing.T) {
		stdout, stderr, code := runCompiledGoCDM(t, bin, tmpDir, nil, "-config", cfgPath, "-login")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "requires root privileges") {
			t.Fatalf("expected root privileges diagnostic on stderr, got %q", stderr)
		}
	})

	t.Run("non-root explicit -mode=login without dry-run fails fast", func(t *testing.T) {
		stdout, stderr, code := runCompiledGoCDM(t, bin, tmpDir, nil, "-config", cfgPath, "-mode=login")
		if code != 1 {
			t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "requires root privileges") {
			t.Fatalf("expected root privileges diagnostic on stderr, got %q", stderr)
		}
	})

	t.Run("conflicting -login and -mode=user exits 2", func(t *testing.T) {
		stdout, stderr, code := runCompiledGoCDM(t, bin, tmpDir, nil, "-login", "-mode=user")
		if code != 2 {
			t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "conflicting mode flags") {
			t.Fatalf("expected conflict diagnostic on stderr, got %q", stderr)
		}
	})

	t.Run("invalid -mode exits 2", func(t *testing.T) {
		stdout, stderr, code := runCompiledGoCDM(t, bin, tmpDir, nil, "-mode=bogus")
		if code != 2 {
			t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "invalid -mode") {
			t.Fatalf("expected invalid mode diagnostic on stderr, got %q", stderr)
		}
	})

	t.Run("-version without TTY exits 0", func(t *testing.T) {
		stdout, stderr, code := runCompiledGoCDM(t, bin, tmpDir, nil, "-version")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "gocdm version ") {
			t.Fatalf("expected version output, got stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("-help without TTY exits 0", func(t *testing.T) {
		stdout, stderr, code := runCompiledGoCDM(t, bin, tmpDir, nil, "-help")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "-mode") {
			t.Fatalf("expected -mode in help output, got stdout=%q stderr=%q", stdout, stderr)
		}
	})
}
