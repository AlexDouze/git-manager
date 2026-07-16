package app

import "charm.land/bubbles/v2/key"

// repoKeyMap holds the shortcuts active on the repository-list screen. The
// list component already owns navigation (arrows, j/k), filtering (/), and quit;
// these are the app-specific actions layered on top and surfaced in the help bar
// via the list's AdditionalShortHelpKeys hook.
type repoKeyMap struct {
	Enter     key.Binding
	Refresh   key.Binding
	Update    key.Binding
	Prune     key.Binding
	UpdateAll key.Binding
	PruneAll  key.Binding
	Clone     key.Binding
	Open      key.Binding
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
		UpdateAll: key.NewBinding(
			// Bubble Tea v2 reports a shifted letter with Text set to the
			// uppercase rune, so String() is "U" — distinct from "u" and safe to
			// bind separately (same reasoning as the ghKeyMap space fix).
			key.WithKeys("U"),
			key.WithHelp("U", "update all"),
		),
		PruneAll: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "prune all"),
		),
		Clone: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clone"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in editor"),
		),
	}
}

// shortHelp returns the app-specific bindings appended to the list's built-in
// help (navigation/filter/quit).
func (k repoKeyMap) shortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Refresh, k.Update, k.Prune, k.UpdateAll, k.PruneAll, k.Clone, k.Open}
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

// ghKeyMap holds the shortcuts active on the GitHub clone browser's list phase.
// Navigation and filtering come from the list component; these act on the
// selection set or the browser as a whole.
type ghKeyMap struct {
	Toggle key.Binding
	Clone  key.Binding
	Back   key.Binding
}

func newGHKeyMap() ghKeyMap {
	return ghKeyMap{
		Toggle: key.NewBinding(
			// Bubble Tea v2 renders the space key's String() as "space", not " ",
			// so the binding must use "space" for key.Matches to fire.
			key.WithKeys("space"),
			key.WithHelp("space", "select"),
		),
		Clone: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "clone"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

func (k ghKeyMap) shortHelp() []key.Binding {
	return []key.Binding{k.Toggle, k.Clone, k.Back}
}
