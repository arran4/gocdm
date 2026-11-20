package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/arran4/gocdm/config"
)

type childContextProbeResult struct {
	CWD                   string `json:"cwd"`
	Home                  string `json:"home"`
	User                  string `json:"user"`
	Logname               string `json:"logname"`
	Shell                 string `json:"shell"`
	PWD                   string `json:"pwd"`
	Path                  string `json:"path"`
	XDGRuntimeDir         string `json:"xdg_runtime_dir"`
	HasXDGRuntimeDir      bool   `json:"has_xdg_runtime_dir"`
	DBusAddress           string `json:"dbus_address"`
	HasDBusAddress        bool   `json:"has_dbus_address"`
	HasLeakedSecret       bool   `json:"has_leaked_secret"`
	HasLeakedLDPreload    bool   `json:"has_leaked_ld_preload"`
	HasLeakedInvocationID bool   `json:"has_leaked_invocation_id"`
}

func TestContextChildProbe(t *testing.T) {
	if os.Getenv("GO_WANT_CONTEXT_PROBE") != "1" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		os.Exit(2)
	}
	xdgRuntime, hasXDG := os.LookupEnv("XDG_RUNTIME_DIR")
	dbusAddr, hasDBus := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS")
	_, hasSecret := os.LookupEnv("LEAKED_ROOT_SECRET")
	_, hasLD := os.LookupEnv("LD_PRELOAD")
	_, hasInv := os.LookupEnv("INVOCATION_ID")

	res := childContextProbeResult{
		CWD:                   cwd,
		Home:                  os.Getenv("HOME"),
		User:                  os.Getenv("USER"),
		Logname:               os.Getenv("LOGNAME"),
		Shell:                 os.Getenv("SHELL"),
		PWD:                   os.Getenv("PWD"),
		Path:                  os.Getenv("PATH"),
		XDGRuntimeDir:         xdgRuntime,
		HasXDGRuntimeDir:      hasXDG,
		DBusAddress:           dbusAddr,
		HasDBusAddress:        hasDBus,
		HasLeakedSecret:       hasSecret,
		HasLeakedLDPreload:    hasLD,
		HasLeakedInvocationID: hasInv,
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(res); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestCompiledChildSessionContextAndState(t *testing.T) {
	tmpDir := t.TempDir()
	aliceHome := filepath.Join(tmpDir, "home", "alice")
	if err := os.MkdirAll(aliceHome, 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedAliceHome, err := filepath.EvalSymlinks(aliceHome)
	if err != nil {
		t.Fatal(err)
	}

	pamEnvPath := filepath.Join(tmpDir, "pam_env.conf")
	if err := os.WriteFile(pamEnvPath, []byte("EDITOR DEFAULT=vim\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	acct := &config.AccountInfo{
		Username: "alice",
		UID:      1001,
		GID:      1001,
		HomeDir:  resolvedAliceHome,
		Shell:    "/bin/zsh",
	}

	launcherEnv := []string{
		"USER=root",
		"HOME=/root",
		"TERM=xterm-256color",
		"LANG=en_AU.UTF-8",
		"LEAKED_ROOT_SECRET=do-not-leak",
		"LD_PRELOAD=/tmp/preload.so",
		"INVOCATION_ID=getty-service-id",
		"XDG_RUNTIME_DIR=/run/user/0",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/0/bus",
	}

	runProbe := func(pamSessionEnv []string) childContextProbeResult {
		t.Helper()
		sessionCwd, err := resolveAndChdir(acct.HomeDir, func(dir string) error {
			_, statErr := os.Stat(dir)
			return statErr
		})
		if err != nil {
			t.Fatalf("resolveAndChdir failed: %v", err)
		}

		env, err := config.BuildGettySessionEnv(config.GettyEnvOptions{
			LauncherEnv:   launcherEnv,
			Account:       acct,
			PamEnvFile:    pamEnvPath,
			PamSessionEnv: pamSessionEnv,
			SessionType:   "C",
			SessionName:   "zsh",
			WorkingDir:    sessionCwd,
			SpawnPID:      os.Getpid(),
		})
		if err != nil {
			t.Fatalf("BuildGettySessionEnv failed: %v", err)
		}
		env = append(env, "GO_WANT_CONTEXT_PROBE=1")

		cmd := exec.Command(os.Args[0], "-test.run=^TestContextChildProbe$")
		cmd.Dir = sessionCwd
		cmd.Env = env
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("child probe failed: %v", err)
		}
		var res childContextProbeResult
		if err := json.Unmarshal(out, &res); err != nil {
			t.Fatalf("decode child probe output %q: %v", string(out), err)
		}
		return res
	}

	t.Run("with authoritative PAM runtime and dbus env", func(t *testing.T) {
		res := runProbe([]string{
			"XDG_RUNTIME_DIR=/run/user/1001",
			"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus",
		})
		if res.CWD != resolvedAliceHome {
			t.Fatalf("child CWD = %q, want %q", res.CWD, resolvedAliceHome)
		}
		if res.Home != resolvedAliceHome || res.User != "alice" || res.Logname != "alice" || res.Shell != "/bin/zsh" || res.PWD != resolvedAliceHome {
			t.Fatalf("unexpected child identity/pwd: %+v", res)
		}
		if !res.HasXDGRuntimeDir || res.XDGRuntimeDir != "/run/user/1001" {
			t.Fatalf("expected PAM XDG_RUNTIME_DIR, got %+v", res)
		}
		if !res.HasDBusAddress || res.DBusAddress != "unix:path=/run/user/1001/bus" {
			t.Fatalf("expected PAM DBUS_SESSION_BUS_ADDRESS, got %+v", res)
		}
		if res.HasLeakedSecret || res.HasLeakedLDPreload || res.HasLeakedInvocationID {
			t.Fatalf("child inherited secret/launcher env vars: %+v", res)
		}
	})

	t.Run("without PAM runtime env does not fabricate runtime or dbus", func(t *testing.T) {
		res := runProbe(nil)
		if res.HasXDGRuntimeDir || res.HasDBusAddress {
			t.Fatalf("must not fabricate XDG_RUNTIME_DIR or DBUS_SESSION_BUS_ADDRESS: %+v", res)
		}
		if res.HasLeakedSecret || res.HasLeakedLDPreload || res.HasLeakedInvocationID {
			t.Fatalf("child inherited secret/launcher env vars: %+v", res)
		}
	})

	t.Run("state save and symlink rejection in alice home", func(t *testing.T) {
		if err := config.SafeSaveStateAt(resolvedAliceHome, "zsh", os.Geteuid()); err != nil {
			t.Fatalf("SafeSaveStateAt failed: %v", err)
		}
		loaded, err := config.LoadStateForUser(resolvedAliceHome, os.Geteuid())
		if err != nil || loaded.LastSession != "zsh" {
			t.Fatalf("LoadStateForUser got (%+v, %v), want LastSession=zsh", loaded, err)
		}

		// Replace state.json with symlink and verify refusal.
		statePath := filepath.Join(resolvedAliceHome, ".config", "gocdm", "state.json")
		victim := filepath.Join(tmpDir, "victim")
		if err := os.WriteFile(victim, []byte("untouched"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(statePath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(victim, statePath); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		if err := config.SafeSaveStateAt(resolvedAliceHome, "bash", os.Geteuid()); err == nil {
			t.Fatal("expected symlink refusal when overwriting state.json")
		}
		data, _ := os.ReadFile(victim)
		if string(data) != "untouched" {
			t.Fatalf("victim file altered: %q", string(data))
		}
	})
}
