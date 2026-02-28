package dialog

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// testScreen is used for testing to inject a simulation screen.
var testScreen tcell.Screen

// MapColor maps a string color name to its tcell.Color equivalent.
func MapColor(colorName string) tcell.Color {
	switch strings.ToUpper(strings.TrimSpace(colorName)) {
	case "BLACK":
		return tcell.ColorBlack
	case "RED":
		return tcell.ColorRed
	case "GREEN":
		return tcell.ColorGreen
	case "YELLOW":
		return tcell.ColorYellow
	case "BLUE":
		return tcell.ColorBlue
	case "MAGENTA":
		return tcell.ColorPurple // or tcell.ColorFuchsia
	case "CYAN":
		return tcell.ColorTeal // or tcell.ColorAqua
	case "WHITE":
		return tcell.ColorWhite
	default:
		return tcell.ColorDefault
	}
}

// ShowMenu displays a menu using tview.
// options is a slice of option names.
// details is a slice of detail strings corresponding to options (optional, can be nil).
// startIdx is the index to start numbering options.
// defaultIdx is the index of the option to pre-select.
// theme is the path to the dialogrc file.
// Returns the index of the selected option (0-based relative to options), or error.
func ShowMenu(title string, options []string, details []string, startIdx int, defaultIdx int, theme string) (int, error) {
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

	list := tview.NewList()
	list.ShowSecondaryText(false).
		SetSelectedBackgroundColor(tcell.ColorDarkBlue).
		SetSelectedTextColor(tcell.ColorWhite)
	list.SetTitle(fmt.Sprintf(" %s ", title)).
		SetTitleAlign(tview.AlignCenter)

	list.SetBorder(true)

	if rc != nil {
		if attr, ok := rc.Attributes["item_selected_color"]; ok {
			list.SetSelectedBackgroundColor(MapColor(attr.Background))
			list.SetSelectedTextColor(MapColor(attr.Foreground))
		}
		if attr, ok := rc.Attributes["item_color"]; ok {
			list.SetMainTextColor(MapColor(attr.Foreground))
		}
		if attr, ok := rc.Attributes["title_color"]; ok {
			list.SetTitleColor(MapColor(attr.Foreground))
		}
		if attr, ok := rc.Attributes["menubox_border_color"]; ok {
			list.SetBorderColor(MapColor(attr.Foreground))
		} else if attr, ok := rc.Attributes["border_color"]; ok {
			list.SetBorderColor(MapColor(attr.Foreground))
		}
		if attr, ok := rc.Attributes["menubox_color"]; ok {
			list.SetBackgroundColor(MapColor(attr.Background))
		} else if attr, ok := rc.Attributes["dialog_color"]; ok {
			list.SetBackgroundColor(MapColor(attr.Background))
		}
	}

	for i, opt := range options {
		// Prepend index to match dialog look
		itemText := fmt.Sprintf("%d %s", i+startIdx, opt)
		list.AddItem(itemText, "", 0, nil)
	}

	if defaultIdx >= 0 && defaultIdx < len(options) {
		list.SetCurrentItem(defaultIdx)
	}

	// Calculate height based on options, max out at 20 or screen height - padding
	height := len(options) + 4 // +2 for border, +2 for margin? Just +2 for border usually.
	if height > 20 {
		height = 20
	}
	width := 60

	// Center the list
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, height, 1, true).
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

	var selectedIdx = -1
	var selectionError error

	list.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		selectedIdx = index
		app.Stop()
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			selectionError = fmt.Errorf("cancelled")
			app.Stop()
			return nil
		}
		if event.Rune() == '?' {
			idx := list.GetCurrentItem()
			if details != nil && idx >= 0 && idx < len(details) {
				detailText := details[idx]
				modal := tview.NewModal().
					SetText(detailText).
					AddButtons([]string{"OK"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						app.SetRoot(flex, true)
					})
				app.SetRoot(modal, false)
				return nil
			}
		}
		return event
	})

	if err := app.SetRoot(flex, true).Run(); err != nil {
		return -1, err
	}

	if selectionError != nil {
		return -1, selectionError
	}

	if selectedIdx == -1 {
		return -1, fmt.Errorf("cancelled")
	}

	return selectedIdx, nil
}
