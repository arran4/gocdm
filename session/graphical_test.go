package session

import (
	"embed"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

//go:embed testdata/graphical/*.desktop
var graphicalDesktopFiles embed.FS

func writeGraphicalFixture(t *testing.T, dir, filename string) string {
	t.Helper()
	data, err := graphicalDesktopFiles.ReadFile("testdata/graphical/" + filename)
	if err != nil {
		t.Fatalf("read embedded fixture %s: %v", filename, err)
	}
	target := filepath.Join(dir, filename)
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", filename, err)
	}
	return target
}

func TestParseGraphicalDesktopMetadataAndSections(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("parses Plasma Wayland DesktopID and DesktopNames and scopes out Desktop Action", func(t *testing.T) {
		path := writeGraphicalFixture(t, tmpDir, "plasma.desktop")
		d := NewDiscoverer()
		d.ExecLookPath = func(file string) (string, error) {
			if file == "startplasma-wayland" {
				return "/usr/bin/startplasma-wayland", nil
			}
			if file == "/usr/libexec/plasma-dbus-run-session-if-needed" {
				return "/usr/libexec/plasma-dbus-run-session-if-needed", nil
			}
			return "", os.ErrNotExist
		}

		s, err := d.parseDesktopFile(path, "W")
		if err != nil {
			t.Fatalf("parseDesktopFile failed: %v", err)
		}
		if s.Name != "Plasma (Wayland)" {
			t.Fatalf("unexpected Name=%q (should not be overwritten by [Desktop Action])", s.Name)
		}
		if s.DesktopID != "plasma" {
			t.Fatalf("unexpected DesktopID=%q, want plasma", s.DesktopID)
		}
		if !reflect.DeepEqual(s.DesktopNames, []string{"KDE"}) {
			t.Fatalf("unexpected DesktopNames=%v, want [KDE]", s.DesktopNames)
		}
		if s.Exec != "/usr/libexec/plasma-dbus-run-session-if-needed startplasma-wayland" {
			t.Fatalf("unexpected wrapped Exec=%q", s.Exec)
		}
	})

	t.Run("does not double-wrap Plasma 6 entry already containing plasma-dbus-run-session-if-needed", func(t *testing.T) {
		path := writeGraphicalFixture(t, tmpDir, "plasma_wrapped.desktop")
		d := NewDiscoverer()
		d.ExecLookPath = func(file string) (string, error) {
			return file, nil
		}

		s, err := d.parseDesktopFile(path, "W")
		if err != nil {
			t.Fatalf("parseDesktopFile failed: %v", err)
		}
		wantExec := "/usr/libexec/plasma-dbus-run-session-if-needed /usr/bin/startplasma-wayland"
		if s.Exec != wantExec {
			t.Fatalf("got Exec=%q, want %q (double wrapped!)", s.Exec, wantExec)
		}
		if s.DesktopID != "plasma_wrapped" {
			t.Fatalf("unexpected DesktopID=%q", s.DesktopID)
		}
		if !reflect.DeepEqual(s.DesktopNames, []string{"KDE"}) {
			t.Fatalf("unexpected DesktopNames=%v, want [KDE]", s.DesktopNames)
		}
	})

	t.Run("parses X11 multi DesktopNames and quoted Exec without breaking quoting", func(t *testing.T) {
		path := writeGraphicalFixture(t, tmpDir, "x11_quoted.desktop")
		d := NewDiscoverer()
		d.ExecLookPath = func(file string) (string, error) {
			if file == "startxfce4" {
				return "/usr/bin/startxfce4", nil
			}
			return "", os.ErrNotExist
		}

		s, err := d.parseDesktopFile(path, "X")
		if err != nil {
			t.Fatalf("parseDesktopFile failed: %v", err)
		}
		if s.DesktopID != "x11_quoted" {
			t.Fatalf("unexpected DesktopID=%q", s.DesktopID)
		}
		if !reflect.DeepEqual(s.DesktopNames, []string{"XFCE", "GTK"}) {
			t.Fatalf("unexpected DesktopNames=%v, want [XFCE GTK]", s.DesktopNames)
		}
		wantExec := `startxfce4 --with-arg "two words"`
		if s.Exec != wantExec {
			t.Fatalf("got Exec=%q, want %q", s.Exec, wantExec)
		}
	})

	t.Run("skips Hidden=true and NoDisplay=true entries", func(t *testing.T) {
		d := newMockDiscoverer()
		for _, fname := range []string{"hidden.desktop", "nodisplay.desktop"} {
			p := writeGraphicalFixture(t, tmpDir, fname)
			if _, err := d.parseDesktopFile(p, "W"); err == nil {
				t.Fatalf("expected hidden/nodisplay desktop file %s to be skipped", fname)
			}
		}
	})
}

func TestApplyWaylandWrappersCandidatesAndIdempotency(t *testing.T) {
	origWrapper := PlasmaWaylandWrapperPath
	t.Cleanup(func() { PlasmaWaylandWrapperPath = origWrapper })
	PlasmaWaylandWrapperPath = "/usr/libexec/plasma-dbus-run-session-if-needed"

	t.Run("falls back to /usr/lib64/libexec when primary missing", func(t *testing.T) {
		d := NewDiscoverer()
		d.ExecLookPath = func(file string) (string, error) {
			if file == "/usr/lib64/libexec/plasma-dbus-run-session-if-needed" {
				return file, nil
			}
			return "", os.ErrNotExist
		}
		got := ApplyWaylandWrappers("/usr/bin/startplasma-wayland", d)
		want := "/usr/lib64/libexec/plasma-dbus-run-session-if-needed /usr/bin/startplasma-wayland"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("skips when dbus-run-session already present or non-plasma", func(t *testing.T) {
		d := newMockDiscoverer()
		for _, in := range []string{
			"dbus-run-session /usr/bin/startplasma-wayland",
			"/usr/libexec/plasma-dbus-run-session-if-needed /usr/bin/startplasma-wayland",
			"sway",
		} {
			if got := ApplyWaylandWrappers(in, d); got != in {
				t.Fatalf("ApplyWaylandWrappers(%q) = %q, want unchanged", in, got)
			}
		}
	})
}
