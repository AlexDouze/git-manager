package app

import "charm.land/bubbles/v2/key"

// repoKeyMap holds the shortcuts active on the repository-list screen. The
// list component already owns navigation (arrows, j/k), filtering (/), and quit;
// these are the app-specific actions layered on top and surfaced in the help bar
// via the list's AdditionalShortHelpKeys hook.
type repoKeyMap struct {
	Enter   key.Binding
	Refresh key.Binding
	Update  key.Binding
	Prune   key.Binding
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
		Update: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "update"),
		),
		Prune: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prune gone"),
		),
	}
}

// shortHelp returns the app-specific bindings appended to the list's built-in
// help (navigation/filter/quit).
func (k repoKeyMap) shortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Refresh, k.Update, k.Prune}
}

// branchKeyMap holds the shortcuts active on the branch-list screen. Navigation,
// filtering, and quit come from the list component; the rest act on the selected
// branch or the whole repo.
type branchKeyMap struct {
	Checkout key.Binding
	Delete   key.Binding
	Update   key.Binding
	Back     key.Binding
}

func newBranchKeyMap() branchKeyMap {
	return branchKeyMap{
		Checkout: key.NewBinding(
			key.WithKeys("enter", "c"),
			key.WithHelp("enter/c", "checkout"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Update: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "update"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

func (k branchKeyMap) shortHelp() []key.Binding {
	return []key.Binding{k.Checkout, k.Delete, k.Update, k.Back}
}
