package tui

import (
	"strings"

	"github.com/alexDouze/gitm/pkg/git"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Item struct {
	Repo     git.Repository
	Selected bool
}

func SelectGithubReposRender(repos []git.Repository) ([]git.Repository, error) {
	var items []Item
	for _, repo := range repos {
		items = append(items, Item{Repo: repo, Selected: false})
	}

	app := tview.NewApplication()
	list := tview.NewList()
	list.SetBorder(true).SetTitle("Select Items (Space to select, / to filter, Enter to confirm)")
	// Create a text field for filtering
	filterInput := tview.NewInputField().
		SetLabel("Filter: ").
		SetFieldWidth(30)

	// Track if we're in filter mode
	// filterMode := false
	filterText := ""

	// Function to update the displayed list based on current filter
	updateList := func() {
		list.Clear()
		for _, item := range items {
			// Skip items that don't match the filter
			if filterText != "" && !strings.Contains(strings.ToLower(item.Repo.Name), strings.ToLower(filterText)) {
				continue
			}

			// Display selection status
			prefix := "[ ] "
			if item.Selected {
				prefix = "[✓] "
			}

			// Add the item to the list without alphabetical shortcut
			list.AddItem(prefix+item.Repo.Name, "", 0, nil)
		}
	}

	// Initial list population
	updateList()

	// Set up filter input handling
	filterInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			filterText = filterInput.GetText()
			// filterMode = false
			updateList()
			app.SetFocus(list)
		} else if key == tcell.KeyEsc {
			// filterMode = false
			app.SetFocus(list)
		}
	})

	// Handle list navigation and selection
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Get current selection
		currentIndex := list.GetCurrentItem()

		// Handle key presses
		switch {
		case event.Key() == tcell.KeyRune && event.Rune() == 'j':
			// Move down (vim style)
			if currentIndex < list.GetItemCount()-1 {
				list.SetCurrentItem(currentIndex + 1)
			}
			return nil
		case event.Key() == tcell.KeyRune && event.Rune() == 'k':
			// Move up (vim style)
			if currentIndex > 0 {
				list.SetCurrentItem(currentIndex - 1)
			}
			return nil
		case event.Key() == tcell.KeyRune:
			if event.Rune() == '/' {
				// Enter filter mode
				// filterMode = true
				filterInput.SetText("")
				app.SetFocus(filterInput)
				return nil
			} else if event.Rune() == ' ' {
				// Toggle selection of current item
				// Map display index back to actual item index
				actualIndex := -1
				count := 0
				for i, item := range items {
					if filterText == "" || strings.Contains(strings.ToLower(item.Repo.Name), strings.ToLower(filterText)) {
						if count == currentIndex {
							actualIndex = i
							break
						}
						count++
					}
				}

				if actualIndex >= 0 {
					items[actualIndex].Selected = !items[actualIndex].Selected
					updateList()
					list.SetCurrentItem(currentIndex) // Maintain selection position
				}
				return nil
			}
		case event.Key() == tcell.KeyEnter:
			// Confirm selection and exit
			app.Stop()
			return nil
		}
		return event
	})

	// Create layout
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 1, true).
		AddItem(filterInput, 1, 0, false)

	// Start the application
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}

	var res []git.Repository
	for _, item := range items {
		if item.Selected {
			res = append(res, item.Repo)
		}
	}

	return res, nil
}
