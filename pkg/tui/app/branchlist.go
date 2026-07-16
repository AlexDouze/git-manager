package app

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/alexDouze/gitm/pkg/git"
)

// branchItem is a single row in the branch list.
type branchItem struct {
	branch git.BranchInfo
}

// FilterValue implements list.Item; `/` filters on the branch name.
func (i branchItem) FilterValue() string { return i.branch.Name }

// badges renders a compact, colored summary of the branch's state.
func (i branchItem) badges(s styles) string {
	b := i.branch
	var badges []string
	if b.Ahead > 0 {
		badges = append(badges, s.ok.Render(fmt.Sprintf("↑%d", b.Ahead)))
	}
	if b.Behind > 0 {
		badges = append(badges, s.err.Render(fmt.Sprintf("↓%d", b.Behind)))
	}
	if b.RemoteGone {
		badges = append(badges, s.err.Render("gone"))
	} else if b.NoRemoteTracking {
		badges = append(badges, s.warn.Render("no-remote"))
	}
	if b.Stale {
		badges = append(badges, s.warn.Render("stale"))
	}
	if b.WorktreePath != "" {
		badges = append(badges, s.dim.Render("worktree"))
	}
	if len(badges) == 0 {
		return s.ok.Render("✓")
	}
	return strings.Join(badges, " ")
}

// branchDelegate renders branch rows: a current-branch marker, the name, then
// state badges.
type branchDelegate struct {
	styles styles
}

func newBranchDelegate(s styles) branchDelegate {
	return branchDelegate{styles: s}
}

func (d branchDelegate) Height() int                             { return 1 }
func (d branchDelegate) Spacing() int                            { return 0 }
func (d branchDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d branchDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(branchItem)
	if !ok {
		return
	}

	name := it.branch.Name
	marker := "  "
	if it.branch.Current {
		marker = "* "
	}

	badges := it.badges(d.styles)

	if index == m.Index() {
		fmt.Fprint(w, d.styles.selected.Render("> "+marker+name)+"  "+badges)
		return
	}
	fmt.Fprint(w, d.styles.normal.Render("  "+marker+name)+"  "+badges)
}

// ensure the interface is satisfied at compile time.
var _ list.ItemDelegate = branchDelegate{}
