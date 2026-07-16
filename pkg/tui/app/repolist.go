package app

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alexDouze/gitm/pkg/git"
)

// repoItem is a single row in the repository list.
type repoItem struct {
	repo *git.Repository
}

// title is the "host/org/name" identity used for display and filtering.
func (i repoItem) title() string {
	return fmt.Sprintf("%s/%s/%s", i.repo.Host, i.repo.Organization, i.repo.Name)
}

// FilterValue implements list.Item; the identity string is what `/` filters on.
func (i repoItem) FilterValue() string { return i.title() }

// repoDelegate renders repository rows. For the P0 skeleton it shows only the
// identity; later phases add status badges.
type repoDelegate struct {
	selected lipgloss.Style
	normal   lipgloss.Style
}

func newRepoDelegate() repoDelegate {
	return repoDelegate{
		selected: lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true),
		normal:   lipgloss.NewStyle(),
	}
}

func (d repoDelegate) Height() int                             { return 1 }
func (d repoDelegate) Spacing() int                            { return 0 }
func (d repoDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d repoDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(repoItem)
	if !ok {
		return
	}
	line := it.title()
	if index == m.Index() {
		fmt.Fprint(w, d.selected.Render("> "+line))
		return
	}
	fmt.Fprint(w, d.normal.Render("  "+line))
}

// ensure the interface is satisfied at compile time.
var _ list.ItemDelegate = repoDelegate{}
