package dialog

import (
	"path/filepath"
	"testing"
)

func TestParseDialogRCSimple(t *testing.T) {
	rcPath := filepath.Join("testdata", "todo", "cdm.dialogrc")
	rc, err := ParseDialogRC(rcPath)
	if err != nil {
		t.Fatalf("Failed to parse %s: %v", rcPath, err)
	}

	// Spot check an attribute
	attr, ok := rc.Attributes["screen_color"]
	if !ok {
		t.Errorf("Expected screen_color attribute")
	} else {
		if attr.Foreground != "BLUE" {
			t.Errorf("Expected screen_color Foreground BLUE, got %s", attr.Foreground)
		}
		if attr.Background != "BLACK" {
			t.Errorf("Expected screen_color Background BLACK, got %s", attr.Background)
		}
		if attr.Highlight != true {
			t.Errorf("Expected screen_color Highlight ON")
		}
	}
}

func TestParseDialogRCDefault(t *testing.T) {
	rcPath := filepath.Join("testdata", "todo", "default.dialogrc")
	rc, err := ParseDialogRC(rcPath)
	if err != nil {
		t.Fatalf("Failed to parse %s: %v", rcPath, err)
	}

	// Spot check a number/string
	if val := rc.Numbers["aspect"]; val != 0 {
		t.Errorf("Expected aspect to be 0, got %v", val)
	}

	// Spot check a boolean
	if val, ok := rc.Booleans["use_shadow"]; !ok || val != false {
		t.Errorf("Expected use_shadow to be false")
	}

	// Check another attribute style
	attr, ok := rc.Attributes["button_key_active_color"]
	if !ok {
		t.Errorf("Expected button_key_active_color attribute")
	} else {
		if attr.Foreground != "CYAN" {
			t.Errorf("Expected button_key_active_color Foreground CYAN, got %s", attr.Foreground)
		}
		if attr.Background != "BLACK" {
			t.Errorf("Expected button_key_active_color Background BLACK, got %s", attr.Background)
		}
		if attr.Highlight != true {
			t.Errorf("Expected button_key_active_color Highlight ON")
		}
	}
}
