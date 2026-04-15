package dialog

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func applyLoginTheme(app *tview.Application, form *tview.Form, flex *tview.Flex, headerText, clockText *tview.TextView, rc *DialogRC) {
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
			headerText.SetTextColor(MapColor(attr.Foreground))
			clockText.SetTextColor(MapColor(attr.Foreground))
		} else {
			headerText.SetTextColor(tcell.ColorWhite)
			clockText.SetTextColor(tcell.ColorWhite)
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
		if attr, ok := rc.Attributes["screen_color"]; ok {
			flex.SetBackgroundColor(MapColor(attr.Background))
			app.SetBeforeDrawFunc(func(s tcell.Screen) bool {
				s.Fill(' ', tcell.StyleDefault.Background(MapColor(attr.Background)))
				return false
			})
		}
	} else {
		// Default styling
		form.SetLabelColor(tcell.ColorWhite)
		form.SetFieldTextColor(tcell.ColorWhite)
		form.SetFieldBackgroundColor(tcell.ColorDarkBlue)
		form.SetButtonBackgroundColor(tcell.ColorDarkBlue)
		form.SetButtonTextColor(tcell.ColorWhite)

		headerText.SetTextColor(tcell.ColorWhite)
		clockText.SetTextColor(tcell.ColorWhite)
	}
}

func buildLoginForm(app *tview.Application, title string, username, password *string, selectionError *error, authFunc func(string, string) error, errorMsg *tview.TextView) *tview.Form {
	form := tview.NewForm()

	loginFunc := func() {
		// Try to read directly in case the on-change didn't fire yet
		if u := form.GetFormItemByLabel("Username"); u != nil {
			*username = strings.TrimSpace(u.(*tview.InputField).GetText())
		}
		if p := form.GetFormItemByLabel("Password"); p != nil {
			*password = p.(*tview.InputField).GetText()
		}

		if *username != "" {
			*selectionError = nil
			if authFunc != nil {
				err := authFunc(*username, *password)
				if err != nil {
					errorMsg.SetText(fmt.Sprintf("Login failed: %v", err))
					*password = ""
					if p := form.GetFormItemByLabel("Password"); p != nil {
						p.(*tview.InputField).SetText("")
					}
					app.SetFocus(form.GetFormItemByLabel("Password"))
					return
				}
			}
			app.Stop()
		}
	}

	form.AddInputField("Username", "", 30, nil, func(text string) {
		*username = strings.TrimSpace(text)
	}).
		AddPasswordField("Password", "", 30, '*', func(text string) {
			*password = text
		}).
		AddButton("Login", loginFunc).
		AddButton("Cancel", func() {
			*selectionError = fmt.Errorf("cancelled")
			app.Stop()
		})

	if p := form.GetFormItemByLabel("Password"); p != nil {
		p.(*tview.InputField).SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				loginFunc()
			}
		})
	}

	form.SetTitle(fmt.Sprintf(" %s ", title)).SetTitleAlign(tview.AlignCenter)
	form.SetBorder(true)
	return form
}

func buildLoginLayout(form *tview.Form, headerText, clockText, errorMsg *tview.TextView) *tview.Flex {
	// Calculate height
	height := 11
	width := 50

	// Center the form
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(headerText, 1, 1, false).
			AddItem(clockText, 1, 1, false).
			AddItem(nil, 1, 1, false).
			AddItem(form, height, 1, true).
			AddItem(errorMsg, 2, 1, false).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}

func startClock(app *tview.Application, clockText *tview.TextView) {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			app.QueueUpdateDraw(func() {
				clockText.SetText(time.Now().Format("15:04:05 - Jan 02, 2006"))
			})
		}
	}()
}

// ShowLogin displays a login form using tview.
// Returns the username, password, and error (if cancelled or failed).
func ShowLogin(title string, theme string, authFunc func(string, string) error) (string, string, error) {
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

	errorMsg := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetTextColor(tcell.ColorRed)

	form := buildLoginForm(app, title, &username, &password, &selectionError, authFunc, errorMsg)

	// Add Hostname and Clock
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	headerText := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText(hostname)

	clockText := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	startClock(app, clockText)

	flex := buildLoginLayout(form, headerText, clockText, errorMsg)

	applyLoginTheme(app, form, flex, headerText, clockText, rc)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			selectionError = fmt.Errorf("cancelled")
			app.Stop()
			return nil
		}
		return event
	})

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if _, ok := <-sigs; ok {
			selectionError = fmt.Errorf("cancelled")
			app.Stop()
		}
	}()

	err = app.SetRoot(flex, true).Run()
	signal.Stop(sigs)
	close(sigs)

	if err != nil {
		return "", "", err
	}

	if selectionError != nil {
		return "", "", selectionError
	}

	return username, password, nil
}
