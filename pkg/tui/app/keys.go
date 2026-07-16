package app

import "charm.land/bubbles/v2/key"

// repoKeyMap holds the shortcuts active on the repository-list screen. The
// list component already owns navigation (arrows, j/k), filtering (/), and quit;
// these are the app-specific actions layered on top and surfaced in the help bar
// via the list's AdditionalShortHelpKeys hook.
type repoKeyMap struct {
	Enter   key.Binding
	Refresh key.Binding
}

func newRepoKeyMap() repoKeyMap {
	return repoKeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "branches"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
	}
}

// shortHelp returns the app-specific bindings appended to the list's built-in
// help (navigation/filter/quit).
func (k repoKeyMap) shortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Refresh}
}

// branchKeyMap holds the shortcuts active on the branch-list screen. Navigation,
// filtering, and quit come from the list component; Back returns to the repo
// list.
type branchKeyMap struct {
	Back key.Binding
}

func newBranchKeyMap() branchKeyMap {
	return branchKeyMap{
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

func (k branchKeyMap) shortHelp() []key.Binding {
	return []key.Binding{k.Back}
}
