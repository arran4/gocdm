package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestBuildLoginForm_TrimSpace(t *testing.T) {
	app := tview.NewApplication()
	var username, password string
	var selectionError error
	var capturedUser, capturedPass string

	authFunc := func(u, p string) error {
		capturedUser = u
		capturedPass = p
		return nil
	}

	errorMsg := tview.NewTextView()
	form := buildLoginForm(app, "Test Login", &username, &password, &selectionError, authFunc, errorMsg)

	// Set inputs with leading/trailing spaces for username, and a trailing space for password
	if u := form.GetFormItemByLabel("Username"); u != nil {
		u.(*tview.InputField).SetText("  arran  ")
	}
	if p := form.GetFormItemByLabel("Password"); p != nil {
		p.(*tview.InputField).SetText("pass ")
	}

	// Trigger the login via "Enter" on Password field
	if p := form.GetFormItemByLabel("Password"); p != nil {
		// invoke the done func to simulate hitting enter
        // it requires reflecting or calling a method.
        // tview.InputField has an InputHandler, but easier: tview fires onChange immediately or we can just send a key event.
        // Wait, tview doesn't expose the done func directly, but we can send an EventKey to it.
        event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
        p.(*tview.InputField).InputHandler()(event, func(p tview.Primitive) {})
	}

    if capturedUser != "arran" {
        t.Errorf("Expected username 'arran', got %q", capturedUser)
    }
    if capturedPass != "pass " {
        t.Errorf("Expected password 'pass ', got %q", capturedPass)
    }
}
