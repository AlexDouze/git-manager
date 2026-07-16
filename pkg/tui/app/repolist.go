package app

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/alexDouze/gitm/pkg/git"
)

// repoItem is a single row in the repository list. status is nil until the
// async status load completes; loaded distinguishes "still loading" from
// "loaded, no status because it errored". busy/busyLabel track a mutating
// action (checkout/update/prune, single or bulk) actively running against this
// repo, so the row can show e.g. "updating…" instead of stale badges.
type repoItem struct {
	repo    *git.Repository
	status  *git.RepositoryStatus
	loaded  bool
	loadErr error

	busy      bool
	busyLabel string
}

// title is the "host/org/name" identity used for display and filtering.
func (i repoItem) title() string {
	return fmt.Sprintf("%s/%s/%s", i.repo.Host, i.repo.Organization, i.repo.Name)
}

// FilterValue implements list.Item; the identity string is what `/` filters on.
func (i repoItem) FilterValue() string { return i.title() }

// statusBadges renders a compact, colored summary of the repository's status.
// It returns the empty string while the status is still loading (the caller
// shows a placeholder instead).
func (i repoItem) statusBadges(s styles) string {
	if !i.loaded {
		return ""
	}
	if i.loadErr != nil || i.status == nil {
		return s.err.Render("!error")
	}
	st := i.status

	var badges []string
	if st.HasUncommittedChanges {
		badges = append(badges, s.err.Render("dirty"))
	}
	if st.HasBranchesBehindRemote {
		badges = append(badges, s.err.Render("↓behind"))
	}
	if st.HasBranchesWithRemoteGone {
		badges = append(badges, s.err.Render("gone"))
	}
	if st.HasBranchesWithoutRemote {
		badges = append(badges, s.warn.Render("no-remote"))
	}
	if st.HasStaleBranches {
		badges = append(badges, s.warn.Render("stale"))
	}
	if st.StashCount > 0 {
		badges = append(badges, s.dim.Render(fmt.Sprintf("📦%d", st.StashCount)))
	}
	if len(badges) == 0 {
		return s.ok.Render("✓ clean")
	}
	return strings.Join(badges, " ")
}

// repoDelegate renders repository rows: the "host/org/name" identity followed by
// status badges (or a loading placeholder).
type repoDelegate struct {
	styles styles
}

func newRepoDelegate(s styles) repoDelegate {
	return repoDelegate{styles: s}
}

func (d repoDelegate) Height() int                             { return 1 }
func (d repoDelegate) Spacing() int                            { return 0 }
func (d repoDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d repoDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(repoItem)
	if !ok {
		return
	}

	identity := it.title()
	var badges string
	switch {
	case it.busy:
		badges = d.styles.busy.Render(it.busyLabel)
	default:
		badges = it.statusBadges(d.styles)
		if badges == "" {
			badges = d.styles.dim.Render("loading…")
		}
	}

	prefix := "  "
	styled := d.styles.normal.Render(prefix + identity)
	if index == m.Index() {
		styled = d.styles.selected.Render("> " + identity)
	}
	fmt.Fprint(w, styled+"  "+badges)
}

// ensure the interface is satisfied at compile time.
var _ list.ItemDelegate = repoDelegate{}
