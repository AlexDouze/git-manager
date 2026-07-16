package app

import "charm.land/lipgloss/v2"

// styles holds the lipgloss styles shared across the app's screens. Colors use
// ANSI 256 codes so they degrade gracefully; when NO_COLOR is set lipgloss drops
// them automatically.
type styles struct {
	selected  lipgloss.Style // the cursor row identity
	normal    lipgloss.Style // non-cursor row identity
	dim       lipgloss.Style // secondary text (path, "loading…")
	ok        lipgloss.Style // clean / success badge
	warn      lipgloss.Style // stale / no-remote badge
	err       lipgloss.Style // behind / gone / dirty badge
	footer    lipgloss.Style // footer status line
	footerErr lipgloss.Style // footer error line
}

func newStyles() styles {
	return styles{
		selected:  lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true),
		normal:    lipgloss.NewStyle(),
		dim:       lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		ok:        lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		warn:      lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		err:       lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		footer:    lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		footerErr: lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	}
}
