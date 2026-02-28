package dialog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestMapColor(t *testing.T) {
	tests := []struct {
		input    string
		expected tcell.Color
	}{
		{"BLACK", tcell.ColorBlack},
		{"RED", tcell.ColorRed},
		{"GREEN", tcell.ColorGreen},
		{"YELLOW", tcell.ColorYellow},
		{"BLUE", tcell.ColorBlue},
		{"MAGENTA", tcell.ColorPurple},
		{"CYAN", tcell.ColorTeal},
		{"WHITE", tcell.ColorWhite},
		{"unknown", tcell.ColorDefault},
		{"   blue  ", tcell.ColorBlue},
		{"red", tcell.ColorRed},
	}

	for _, tt := range tests {
		actual := MapColor(tt.input)
		if actual != tt.expected {
			t.Errorf("MapColor(%q): expected %v, got %v", tt.input, tt.expected, actual)
		}
	}
}

func TestShowMenu(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	// Inject the simulation screen into ShowMenu
	testScreen = s
	defer func() { testScreen = nil }()

	// Because ShowMenu blocks, we need to inject key presses concurrently
	go func() {
		// Give the app a moment to start
		time.Sleep(100 * time.Millisecond)

		// Simulate pressing Enter to select the first item
		s.InjectKey(tcell.KeyEnter, ' ', tcell.ModNone)
	}()

	// Call ShowMenu
	// Options: "Option A", "Option B"
	// StartIdx: 0
	// DefaultIdx: 0
	// Theme: ""
	idx, err := ShowMenu("Test Menu", []string{"Option A", "Option B"}, nil, 0, 0, "")
	if err != nil {
		t.Fatalf("ShowMenu returned error: %v", err)
	}

	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}
}

func TestShowMenuWithTheme(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	testScreen = s
	defer func() { testScreen = nil }()

	// Create a temporary theme file
	tmpDir := t.TempDir()
	themeFile := filepath.Join(tmpDir, "test.dialogrc")
	themeContent := `
screen_color = (YELLOW,BLACK,ON)
item_color = (RED,BLACK,OFF)
item_selected_color = (GREEN,BLACK,ON)
`
	if err := os.WriteFile(themeFile, []byte(themeContent), 0644); err != nil {
		t.Fatalf("Failed to write temp theme: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		s.InjectKey(tcell.KeyEnter, ' ', tcell.ModNone)
	}()

	idx, err := ShowMenu("Test Menu", []string{"Option A", "Option B"}, nil, 0, 0, themeFile)
	if err != nil {
		t.Fatalf("ShowMenu returned error: %v", err)
	}

	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}
}

func TestShowMenuDefaultSelection(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	testScreen = s
	defer func() { testScreen = nil }()

	go func() {
		time.Sleep(100 * time.Millisecond)
		// We expect the second item (index 1) to be selected by default.
		// So hitting Enter immediately should return 1.
		s.InjectKey(tcell.KeyEnter, ' ', tcell.ModNone)
	}()

	// DefaultIdx: 1
	idx, err := ShowMenu("Test Menu", []string{"Option A", "Option B"}, nil, 0, 1, "")
	if err != nil {
		t.Fatalf("ShowMenu returned error: %v", err)
	}

	if idx != 1 {
		t.Errorf("Expected index 1 (default selection), got %d", idx)
	}
}

func TestShowMenuCancel(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	testScreen = s
	defer func() { testScreen = nil }()

	go func() {
		time.Sleep(100 * time.Millisecond)
		s.InjectKey(tcell.KeyEscape, ' ', tcell.ModNone)
	}()

	_, err := ShowMenu("Test Menu", []string{"Option A", "Option B"}, nil, 0, 0, "")
	if err == nil {
		t.Fatal("Expected error for cancelled menu, got nil")
	}
	if err.Error() != "cancelled" {
		t.Errorf("Expected 'cancelled' error, got '%v'", err)
	}
}

func TestShowMenuDetailsModal(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatalf("Failed to init simulation screen: %v", err)
	}
	testScreen = s
	defer func() { testScreen = nil }()

	go func() {
		time.Sleep(100 * time.Millisecond)
		// Press '?' to show modal
		s.InjectKey(tcell.KeyRune, '?', tcell.ModNone)

		time.Sleep(100 * time.Millisecond)
		// Press 'Enter' to close modal
		s.InjectKey(tcell.KeyEnter, ' ', tcell.ModNone)

		time.Sleep(100 * time.Millisecond)
		// Press 'Enter' again to select the item and exit
		s.InjectKey(tcell.KeyEnter, ' ', tcell.ModNone)
	}()

	idx, err := ShowMenu("Test Menu", []string{"Option A", "Option B"}, []string{"Detail A", "Detail B"}, 0, 0, "")
	if err != nil {
		t.Fatalf("ShowMenu returned error: %v", err)
	}

	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}
}
