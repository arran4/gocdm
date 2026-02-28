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

func TestParseDialogRCEdgeCases(t *testing.T) {
	rcPath := filepath.Join("testdata", "todo", "edge_cases.dialogrc")
	rc, err := ParseDialogRC(rcPath)
	if err != nil {
		t.Fatalf("Failed to parse %s: %v", rcPath, err)
	}

	// Empty value should be ignored (or empty string/not break anything)
	if val, ok := rc.Strings["empty_val"]; ok && val != "" {
		t.Errorf("Expected empty_val to be empty string or not present, got %s", val)
	}

	// Whitespace handling
	attr, ok := rc.Attributes["padded_key"]
	if !ok {
		t.Errorf("Expected padded_key attribute")
	} else {
		if attr.Foreground != "RED" || attr.Background != "GREEN" || attr.Highlight != true {
			t.Errorf("padded_key parsing failed: %v", attr)
		}
	}

	// Incomplete attributes
	attrOne := rc.Attributes["attr_one"]
	if attrOne.Foreground != "RED" || attrOne.Background != "" || attrOne.Highlight != false {
		t.Errorf("attr_one parsing failed: %v", attrOne)
	}

	attrTwo := rc.Attributes["attr_two"]
	if attrTwo.Foreground != "RED" || attrTwo.Background != "BLUE" || attrTwo.Highlight != false {
		t.Errorf("attr_two parsing failed: %v", attrTwo)
	}

	attrEmpty := rc.Attributes["attr_empty"]
	if attrEmpty.Foreground != "" || attrEmpty.Background != "" || attrEmpty.Highlight != false {
		t.Errorf("attr_empty parsing failed: %v", attrEmpty)
	}

	// Invalid boolean fallback to string
	if val := rc.Strings["bad_bool"]; val != "MAYBE" {
		t.Errorf("Expected bad_bool to fallback to string MAYBE, got %s", val)
	}

	// Invalid number fallback to string
	if val := rc.Strings["bad_number"]; val != "123a" {
		t.Errorf("Expected bad_number to fallback to string 123a, got %s", val)
	}

	// Missing trailing parenthesis fallback to string
	if val := rc.Strings["unclosed_attr"]; val != "(RED,BLUE,ON" {
		t.Errorf("Expected unclosed_attr to fallback to string, got %s", val)
	}

	// Quoted string with whitespace
	if val := rc.Strings["string_val"]; val != "  hello world  " {
		t.Errorf("Expected string_val to be '  hello world  ', got '%s'", val)
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
