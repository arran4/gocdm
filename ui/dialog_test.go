package ui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

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
