package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// confirmState is an orthogonal overlay that intercepts keys for a yes/no
// decision before a destructive action runs. When present on the Model it takes
// priority over the active screen's keymap. onConfirm is the command to run when
// the user accepts (y); pressing n/esc dismisses it without running anything.
type confirmState struct {
	prompt    string
	onConfirm tea.Cmd
}

// confirmView renders the prompt centered over the given area as a bordered box.
func confirmView(s styles, cs confirmState, width, height int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(1, 3).
		Render(cs.prompt + "\n\n" + s.dim.Render("y confirm · n/esc cancel"))

	if width <= 0 || height <= 0 {
		return box
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
