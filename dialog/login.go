package dialog

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowLogin displays a login form using tview.
// Returns the username, password, and error (if cancelled or failed).
func ShowLogin(title string, theme string) (string, string, error) {
	app := tview.NewApplication()
	if testScreen != nil {
		app.SetScreen(testScreen)
	}

	var rc *DialogRC
	if theme != "" {
		parsed, err := ParseDialogRC(theme)
		if err == nil {
			rc = parsed
		}
	}

	var username, password string
	var selectionError error = fmt.Errorf("cancelled")

	form := tview.NewForm().
		AddInputField("Username", "", 30, nil, func(text string) {
			username = text
		}).
		AddPasswordField("Password", "", 30, '*', func(text string) {
			password = text
		}).
		AddButton("Login", func() {
			if username != "" {
				selectionError = nil
				app.Stop()
			}
		}).
		AddButton("Cancel", func() {
			selectionError = fmt.Errorf("cancelled")
			app.Stop()
		})

	form.SetTitle(fmt.Sprintf(" %s ", title)).SetTitleAlign(tview.AlignCenter)
	form.SetBorder(true)

	if rc != nil {
		if attr, ok := rc.Attributes["menubox_border_color"]; ok {
			form.SetBorderColor(MapColor(attr.Foreground))
		} else if attr, ok := rc.Attributes["border_color"]; ok {
			form.SetBorderColor(MapColor(attr.Foreground))
		}
		if attr, ok := rc.Attributes["menubox_color"]; ok {
			form.SetBackgroundColor(MapColor(attr.Background))
		} else if attr, ok := rc.Attributes["dialog_color"]; ok {
			form.SetBackgroundColor(MapColor(attr.Background))
		}
		if attr, ok := rc.Attributes["title_color"]; ok {
			form.SetTitleColor(MapColor(attr.Foreground))
		}
		if attr, ok := rc.Attributes["item_color"]; ok {
			form.SetFieldTextColor(MapColor(attr.Foreground))
			form.SetLabelColor(MapColor(attr.Foreground))
			form.SetButtonTextColor(MapColor(attr.Foreground))
		}
		if attr, ok := rc.Attributes["item_selected_color"]; ok {
			form.SetButtonBackgroundColor(MapColor(attr.Background))
			// Can set field background too if needed
			form.SetFieldBackgroundColor(MapColor(attr.Background))
		}
	} else {
		// Default styling
		form.SetLabelColor(tcell.ColorWhite)
		form.SetFieldTextColor(tcell.ColorWhite)
		form.SetFieldBackgroundColor(tcell.ColorDarkBlue)
		form.SetButtonBackgroundColor(tcell.ColorDarkBlue)
		form.SetButtonTextColor(tcell.ColorWhite)
	}

	// Calculate height
	height := 11
	width := 50

	// Center the form
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	if rc != nil {
		if attr, ok := rc.Attributes["screen_color"]; ok {
			flex.SetBackgroundColor(MapColor(attr.Background))
			app.SetBeforeDrawFunc(func(s tcell.Screen) bool {
				s.Fill(' ', tcell.StyleDefault.Background(MapColor(attr.Background)))
				return false
			})
		}
	}

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			selectionError = fmt.Errorf("cancelled")
			app.Stop()
			return nil
		}
		return event
	})

	if err := app.SetRoot(flex, true).Run(); err != nil {
		return "", "", err
	}

	if selectionError != nil {
		return "", "", selectionError
	}

	return username, password, nil
}
