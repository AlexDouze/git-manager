// Package app implements the interactive Bubble Tea TUI launched by bare `gitm`.
//
// The app follows the Elm architecture: a root Model holds the current screen
// and delegates most navigation/filtering to Bubbles list components. Slow git
// operations never run inside Update; they are dispatched as tea.Cmd closures
// (see commands.go) that return result messages.
package app

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
)

// screen identifies which list is currently in front.
type screen int

const (
	screenRepos    screen = iota // the top-level repository list
	screenBranches               // the branch list for a drilled-into repo
	screenGHBrowse               // the GitHub clone browser
)

// ghBrowseLimit caps how many repos the in-app clone browser lists per owner,
// matching the `gh-clone --limit` default.
const ghBrowseLimit = 1000

// Filter scopes which repositories the app loads. It mirrors the shared
// cmd.FilterFlags so `gitm --org foo` can open the app pre-scoped.
type Filter struct {
	Host string
	Org  string
	Repo string
	Path string
}

// Model is the root Bubble Tea model.
type Model struct {
	ctx    context.Context // program context; git ops honor its cancellation
	cfg    *config.Config
	filter Filter

	screen screen

	repos      list.Model
	repoKeys   repoKeyMap
	byPath     map[string]int // repo path -> index in the current item slice
	statusBusy bool           // async status load in flight
	bulkBusy   bool           // an update-all/prune-all pass is in flight

	branches   list.Model
	branchKeys branchKeyMap
	activeRepo *git.Repository // the repo whose branches are shown
	branchBusy bool            // async branch load in flight

	gh *ghScreen // GitHub clone browser; non-nil only while screenGHBrowse

	confirm   *confirmState // orthogonal yes/no overlay; intercepts keys when set
	footer    string        // last op result shown in the footer line
	footerErr bool          // render the footer as an error (red) when true

	styles styles

	width  int
	height int

	loading bool // initial repository walk in flight
	err     error
}

// New builds the root model.
func New(ctx context.Context, cfg *config.Config, f Filter) Model {
	st := newStyles()
	repoKeys := newRepoKeyMap()
	branchKeys := newBranchKeyMap()

	repos := list.New(nil, newRepoDelegate(st), 0, 0)
	repos.Title = "Repositories"
	repos.SetShowHelp(true)
	repos.SetStatusBarItemName("repo", "repos")
	repos.AdditionalShortHelpKeys = repoKeys.shortHelp
	repos.AdditionalFullHelpKeys = repoKeys.shortHelp

	branches := list.New(nil, newBranchDelegate(st), 0, 0)
	branches.SetShowHelp(true)
	branches.SetStatusBarItemName("branch", "branches")
	branches.AdditionalShortHelpKeys = branchKeys.shortHelp
	branches.AdditionalFullHelpKeys = branchKeys.shortHelp

	return Model{
		ctx:        ctx,
		cfg:        cfg,
		filter:     f,
		screen:     screenRepos,
		repos:      repos,
		repoKeys:   repoKeys,
		branches:   branches,
		branchKeys: branchKeys,
		styles:     st,
		byPath:     map[string]int{},
		loading:    true,
	}
}

// Init kicks off the initial repository load.
func (m Model) Init() tea.Cmd {
	return loadReposCmd(m.cfg, m.filter)
}

// Update handles incoming messages and returns the next model + command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.repos.SetSize(msg.Width, msg.Height)
		m.branches.SetSize(msg.Width, msg.Height)
		if m.gh != nil {
			m.gh.setSize(msg.Width, msg.Height)
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case ghExitMsg:
		return m.closeGHBrowse()

	case reposLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, m.setRepos(msg.repos)

	case statusesLoadedMsg:
		m.statusBusy = false
		m.repos.StopSpinner()
		return m, m.applyStatuses(msg.results)

	case branchesLoadedMsg:
		m.branchBusy = false
		m.branches.StopSpinner()
		// Ignore results for a repo we already navigated away from.
		if m.activeRepo == nil || msg.path != m.activeRepo.Path {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		return m, m.setBranches(msg.branches)

	case opDoneMsg:
		return m.handleOpDone(msg)

	case bulkOpDoneMsg:
		return m.handleBulkOpDone(msg)

	case ghReposLoadedMsg, ghCloneDoneMsg:
		return m.updateGH(msg)
	}

	return m.updateActiveList(msg)
}

// updateGH forwards a message to the GitHub browser sub-model, if present.
func (m Model) updateGH(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.gh == nil {
		return m, nil
	}
	updated, cmd := m.gh.update(msg)
	m.gh = &updated
	return m, cmd
}

// handleOpDone folds a completed mutating action back into the model: it sets
// the footer summary and, on success, refreshes the affected view. A safe
// branch delete that was refused as "not fully merged" is turned into a
// force-delete confirm overlay instead of an error.
func (m Model) handleOpDone(msg opDoneMsg) (tea.Model, tea.Cmd) {
	if msg.notFullyMerged {
		r := m.repoByPath(msg.path)
		if r == nil {
			r = m.activeRepo
		}
		if r != nil {
			m.confirm = &confirmState{
				prompt:    msg.branch + " is not fully merged. Force delete?",
				onConfirm: deleteBranchCmd(m.ctx, r, msg.branch, true),
			}
			return m, nil
		}
	}

	m.footer = msg.summary
	m.footerErr = msg.err != nil

	// The action is done either way, so the row's busy indicator (if this was a
	// single-repo update/prune) no longer applies.
	clearCmd := m.clearRepoBusy(msg.path)
	if msg.err != nil {
		return m, clearCmd
	}

	// On success, re-read the affected repo so its rows/badges reflect the
	// change. A branch mutation also refreshes the open branch list.
	cmds := []tea.Cmd{clearCmd}
	if c := m.refreshRepo(msg.path); c != nil {
		cmds = append(cmds, c)
	}
	if m.screen == screenBranches && m.activeRepo != nil && m.activeRepo.Path == msg.path && !m.branchBusy {
		m.branchBusy = true
		cmds = append(cmds, m.branches.StartSpinner(), loadBranchesCmd(m.ctx, m.activeRepo))
	}
	return m, tea.Batch(cmds...)
}

// handleBulkOpDone folds the result of an update-all/prune-all pass back into
// the model: clears the bulk-busy guard and every row's busy indicator, sets
// an aggregate footer summary, and reloads every repo's status so rows
// reflect the change.
func (m Model) handleBulkOpDone(msg bulkOpDoneMsg) (tea.Model, tea.Cmd) {
	m.bulkBusy = false
	m.footer = msg.summary
	m.footerErr = false
	for _, r := range msg.results {
		if r.err != nil {
			m.footerErr = true
			break
		}
	}

	items := m.repos.Items()
	cmds := make([]tea.Cmd, 0, len(items)+1)
	for i, li := range items {
		it, ok := li.(repoItem)
		if !ok || !it.busy {
			continue
		}
		it.busy = false
		it.busyLabel = ""
		cmds = append(cmds, m.repos.SetItem(i, it))
	}

	if c := m.refresh(); c != nil {
		cmds = append(cmds, c)
	}
	return m, tea.Batch(cmds...)
}

// handleKey routes key presses based on the current screen. Keys are always
// forwarded to the active list afterwards (so navigation/filter still work),
// except when a shortcut fully handles the press.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// A confirm overlay is orthogonal: while it is up it owns every key. y runs
	// the pending command; n/esc dismisses it.
	if m.confirm != nil {
		switch msg.String() {
		case "y":
			cs := m.confirm
			m.confirm = nil
			var acceptCmd tea.Cmd
			if cs.onAccept != nil {
				acceptCmd = cs.onAccept(&m)
			}
			return m, tea.Batch(acceptCmd, cs.onConfirm)
		case "n", "esc", "ctrl+c":
			m.confirm = nil
			return m, nil
		}
		return m, nil
	}

	// The GitHub browser is a self-contained sub-model that owns all of its
	// keys (including its own filter state), so route to it before anything.
	if m.screen == screenGHBrowse {
		return m.updateGH(msg)
	}

	// While filtering, the active list owns every key (typing into the filter
	// box, esc to cancel), so no app shortcut fires.
	if m.activeList().FilterState() == list.Filtering {
		return m.updateActiveList(msg)
	}

	// ctrl+c always quits, regardless of screen.
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case screenRepos:
		switch {
		case msg.String() == "q":
			return m, tea.Quit
		case key.Matches(msg, m.repoKeys.Enter):
			return m.drillIn()
		case key.Matches(msg, m.repoKeys.Refresh):
			// Ignore refresh while a status load or a bulk pass is already
			// running so we don't stack overlapping worker-pool passes over the
			// same repos.
			if m.statusBusy || m.bulkBusy {
				return m, nil
			}
			return m, m.refresh()
		case key.Matches(msg, m.repoKeys.Update):
			if m.bulkBusy {
				return m, nil
			}
			return m.updateSelectedRepo()
		case key.Matches(msg, m.repoKeys.Prune):
			if m.bulkBusy {
				return m, nil
			}
			return m.pruneSelectedRepo()
		case key.Matches(msg, m.repoKeys.UpdateAll):
			if m.bulkBusy {
				return m, nil
			}
			return m.updateAllRepos()
		case key.Matches(msg, m.repoKeys.PruneAll):
			if m.bulkBusy {
				return m, nil
			}
			return m.pruneAllRepos()
		case key.Matches(msg, m.repoKeys.Clone):
			return m.openGHBrowse()
		}

	case screenBranches:
		switch {
		case key.Matches(msg, m.branchKeys.Checkout):
			return m.checkoutSelectedBranch()
		case key.Matches(msg, m.branchKeys.Delete):
			return m.deleteSelectedBranch()
		case key.Matches(msg, m.branchKeys.Update):
			return m.updateActiveRepo()
		case key.Matches(msg, m.branchKeys.Back):
			return m.back()
		case msg.String() == "q":
			return m, tea.Quit
		}
	}

	return m.updateActiveList(msg)
}

// updateActiveList forwards a message to whichever list is in front.
func (m Model) updateActiveList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.screen {
	case screenBranches:
		m.branches, cmd = m.branches.Update(msg)
	default:
		m.repos, cmd = m.repos.Update(msg)
	}
	return m, cmd
}

// activeList returns the list backing the current screen.
func (m Model) activeList() list.Model {
	if m.screen == screenBranches {
		return m.branches
	}
	return m.repos
}

// drillIn opens the branch list for the currently selected repository.
func (m Model) drillIn() (tea.Model, tea.Cmd) {
	sel, ok := m.repos.SelectedItem().(repoItem)
	if !ok {
		return m, nil
	}
	m.screen = screenBranches
	m.activeRepo = sel.repo
	m.branches.ResetFilter()
	m.branches.Title = sel.title()
	m.branchBusy = true
	return m, tea.Batch(
		m.branches.SetItems(nil),
		m.branches.StartSpinner(),
		loadBranchesCmd(m.ctx, sel.repo),
	)
}

// back returns to the repository list.
func (m Model) back() (tea.Model, tea.Cmd) {
	m.screen = screenRepos
	m.activeRepo = nil
	return m, nil
}

// openGHBrowse creates a GitHub clone browser and switches to it. The browser
// prompts for an owner, so it launches without a fixed owner or root override.
func (m Model) openGHBrowse() (tea.Model, tea.Cmd) {
	gh := newGHScreen(m.ctx, m.cfg, m.styles, "", "", ghBrowseLimit)
	gh.setSize(m.width, m.height)
	m.gh = &gh
	m.screen = screenGHBrowse
	return m, m.gh.init()
}

// closeGHBrowse leaves the browser and returns to the repo list, reloading it
// so any freshly cloned repositories appear.
func (m Model) closeGHBrowse() (tea.Model, tea.Cmd) {
	m.gh = nil
	m.screen = screenRepos
	m.loading = true
	return m, loadReposCmd(m.cfg, m.filter)
}

// updateSelectedRepo fetches+pulls the repo highlighted in the repo list.
func (m Model) updateSelectedRepo() (tea.Model, tea.Cmd) {
	sel, ok := m.repos.SelectedItem().(repoItem)
	if !ok {
		return m, nil
	}
	m.footer = "updating " + sel.repo.Name + "…"
	m.footerErr = false
	setCmd := m.setRepoBusy(sel.repo.Path, "updating…")
	return m, tea.Batch(setCmd, updateCmd(m.ctx, sel.repo))
}

// pruneSelectedRepo asks for confirmation, then prunes gone branches in the
// repo highlighted in the repo list.
func (m Model) pruneSelectedRepo() (tea.Model, tea.Cmd) {
	sel, ok := m.repos.SelectedItem().(repoItem)
	if !ok {
		return m, nil
	}
	path := sel.repo.Path
	m.confirm = &confirmState{
		prompt: "Prune gone branches in " + sel.repo.Name + "?",
		onAccept: func(m *Model) tea.Cmd {
			return m.setRepoBusy(path, "pruning…")
		},
		onConfirm: pruneGoneCmd(m.ctx, sel.repo),
	}
	return m, nil
}

// updateAllRepos fetches+pulls every repository in the list, in parallel via
// the worker pool. Unlike prune-all this needs no confirmation: fetch+pull
// mirrors the single-repo `u` action and mutates nothing destructively.
func (m Model) updateAllRepos() (tea.Model, tea.Cmd) {
	repos := m.allRepos()
	if len(repos) == 0 {
		return m, nil
	}
	m.bulkBusy = true
	m.footer = fmt.Sprintf("updating %d repositories…", len(repos))
	m.footerErr = false
	busyCmd := m.setAllRepoBusy("updating…")
	return m, tea.Batch(busyCmd, updateAllCmd(m.ctx, repos))
}

// pruneAllRepos asks for confirmation, then prunes gone branches across every
// repository in the list.
func (m Model) pruneAllRepos() (tea.Model, tea.Cmd) {
	repos := m.allRepos()
	if len(repos) == 0 {
		return m, nil
	}
	m.confirm = &confirmState{
		prompt: fmt.Sprintf("Prune gone branches in %d repositories?", len(repos)),
		onAccept: func(m *Model) tea.Cmd {
			m.bulkBusy = true
			m.footer = fmt.Sprintf("pruning %d repositories…", len(repos))
			m.footerErr = false
			return m.setAllRepoBusy("pruning…")
		},
		onConfirm: pruneAllCmd(m.ctx, repos),
	}
	return m, nil
}

// allRepos returns the repository behind every current row in the repo list.
func (m Model) allRepos() []*git.Repository {
	items := m.repos.Items()
	repos := make([]*git.Repository, 0, len(items))
	for _, li := range items {
		if it, ok := li.(repoItem); ok {
			repos = append(repos, it.repo)
		}
	}
	return repos
}

// setRepoBusy marks a single row (by repo path) busy with the given label.
// Returns nil if the path is not in the list.
func (m *Model) setRepoBusy(path, label string) tea.Cmd {
	idx, ok := m.byPath[path]
	if !ok {
		return nil
	}
	items := m.repos.Items()
	if idx >= len(items) {
		return nil
	}
	it, ok := items[idx].(repoItem)
	if !ok {
		return nil
	}
	it.busy = true
	it.busyLabel = label
	return m.repos.SetItem(idx, it)
}

// setAllRepoBusy marks every row in the repo list busy with the given label.
func (m *Model) setAllRepoBusy(label string) tea.Cmd {
	items := m.repos.Items()
	cmds := make([]tea.Cmd, 0, len(items))
	for i, li := range items {
		it, ok := li.(repoItem)
		if !ok {
			continue
		}
		it.busy = true
		it.busyLabel = label
		cmds = append(cmds, m.repos.SetItem(i, it))
	}
	return tea.Batch(cmds...)
}

// clearRepoBusy clears a single row's busy indicator (by repo path), if set.
// Returns nil if the path is not in the list or the row wasn't busy.
func (m *Model) clearRepoBusy(path string) tea.Cmd {
	idx, ok := m.byPath[path]
	if !ok {
		return nil
	}
	items := m.repos.Items()
	if idx >= len(items) {
		return nil
	}
	it, ok := items[idx].(repoItem)
	if !ok || !it.busy {
		return nil
	}
	it.busy = false
	it.busyLabel = ""
	return m.repos.SetItem(idx, it)
}

// updateActiveRepo fetches+pulls the repo whose branches are being shown.
func (m Model) updateActiveRepo() (tea.Model, tea.Cmd) {
	if m.activeRepo == nil {
		return m, nil
	}
	m.footer = "updating " + m.activeRepo.Name + "…"
	m.footerErr = false
	return m, updateCmd(m.ctx, m.activeRepo)
}

// checkoutSelectedBranch checks out the branch highlighted in the branch list.
func (m Model) checkoutSelectedBranch() (tea.Model, tea.Cmd) {
	sel, ok := m.branches.SelectedItem().(branchItem)
	if !ok || m.activeRepo == nil {
		return m, nil
	}
	return m, checkoutCmd(m.ctx, m.activeRepo, sel.branch.Name)
}

// deleteSelectedBranch attempts a safe delete of the highlighted branch. A
// branch checked out in a worktree is skipped with a footer note; an unmerged
// branch surfaces a force-delete confirm via the resulting opDoneMsg.
func (m Model) deleteSelectedBranch() (tea.Model, tea.Cmd) {
	sel, ok := m.branches.SelectedItem().(branchItem)
	if !ok || m.activeRepo == nil {
		return m, nil
	}
	if sel.branch.WorktreePath != "" {
		m.footer = sel.branch.Name + " is checked out in a worktree; skipped"
		m.footerErr = true
		return m, nil
	}
	return m, deleteBranchCmd(m.ctx, m.activeRepo, sel.branch.Name, false)
}

// repoByPath returns the loaded repository with the given path, or nil.
func (m Model) repoByPath(path string) *git.Repository {
	idx, ok := m.byPath[path]
	if !ok {
		return nil
	}
	items := m.repos.Items()
	if idx >= len(items) {
		return nil
	}
	it, ok := items[idx].(repoItem)
	if !ok {
		return nil
	}
	return it.repo
}

// refreshRepo re-reads a single repository's status locally (no fetch) and
// folds it back into its row. Returns nil if the path is not in the list.
func (m *Model) refreshRepo(path string) tea.Cmd {
	idx, ok := m.byPath[path]
	if !ok {
		return nil
	}
	items := m.repos.Items()
	if idx >= len(items) {
		return nil
	}
	it, ok := items[idx].(repoItem)
	if !ok {
		return nil
	}
	r := it.repo
	it.loaded = false
	it.status = nil
	it.loadErr = nil
	return tea.Batch(
		m.repos.SetItem(idx, it),
		loadStatusesCmd(m.ctx, []*git.Repository{r}),
	)
}

// setBranches populates the branch list from loaded branch info.
func (m *Model) setBranches(branches []git.BranchInfo) tea.Cmd {
	items := make([]list.Item, 0, len(branches))
	for _, b := range branches {
		items = append(items, branchItem{branch: b})
	}
	return m.branches.SetItems(items)
}

// setRepos replaces the list items with fresh repoItems (status pending) and
// returns a batch that populates the list and kicks off the async status load.
func (m *Model) setRepos(repos []*git.Repository) tea.Cmd {
	items := make([]list.Item, 0, len(repos))
	m.byPath = make(map[string]int, len(repos))
	for i, r := range repos {
		items = append(items, repoItem{repo: r})
		m.byPath[r.Path] = i
	}
	m.statusBusy = true
	return tea.Batch(
		m.repos.SetItems(items),
		m.repos.StartSpinner(),
		loadStatusesCmd(m.ctx, repos),
	)
}

// applyStatuses folds the async status results back into the matching list rows.
func (m *Model) applyStatuses(results []repoStatusResult) tea.Cmd {
	var cmds []tea.Cmd
	items := m.repos.Items()
	for _, res := range results {
		idx, ok := m.byPath[res.path]
		if !ok || idx >= len(items) {
			continue
		}
		it, ok := items[idx].(repoItem)
		if !ok {
			continue
		}
		it.loaded = true
		it.status = res.status
		it.loadErr = res.err
		cmds = append(cmds, m.repos.SetItem(idx, it))
	}
	return tea.Batch(cmds...)
}

// refresh re-reads every repository's status locally (no fetch). Rows revert to
// the "loading…" placeholder until the new statuses arrive.
func (m *Model) refresh() tea.Cmd {
	items := m.repos.Items()
	repos := make([]*git.Repository, 0, len(items))
	var cmds []tea.Cmd
	for i, li := range items {
		it, ok := li.(repoItem)
		if !ok {
			continue
		}
		repos = append(repos, it.repo)
		it.loaded = false
		it.status = nil
		it.loadErr = nil
		cmds = append(cmds, m.repos.SetItem(i, it))
	}
	if len(repos) == 0 {
		return nil
	}
	m.statusBusy = true
	cmds = append(cmds, m.repos.StartSpinner(), loadStatusesCmd(m.ctx, repos))
	return tea.Batch(cmds...)
}

// View renders the current screen. AltScreen is set on the returned view so
// the app takes over the full terminal and restores it on quit (v2 replaced the
// WithAltScreen program option with this field).
func (m Model) View() tea.View {
	// A confirm overlay, when present, replaces the whole view — it is a modal
	// decision the user must resolve before anything else.
	if m.confirm != nil {
		v := tea.NewView(confirmView(m.styles, *m.confirm, m.width, m.height))
		v.AltScreen = true
		return v
	}

	var content string
	switch {
	case m.err != nil:
		content = "Error: " + m.err.Error() + "\n\nPress q to quit."
	case m.screen == screenGHBrowse && m.gh != nil:
		content = m.gh.view()
	case m.screen == screenBranches:
		content = m.branches.View()
	default:
		content = m.repos.View()
	}
	if m.footer != "" {
		style := m.styles.footer
		if m.footerErr {
			style = m.styles.footerErr
		}
		content += "\n" + style.Render(m.footer)
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// Run launches the interactive app and blocks until the user quits. noColor
// forces the ASCII color profile so styling is stripped (mirrors --no-color).
func Run(ctx context.Context, cfg *config.Config, f Filter, noColor bool) error {
	p := tea.NewProgram(New(ctx, cfg, f), programOpts(ctx, noColor)...)
	_, err := p.Run()
	return err
}

// programOpts builds the shared Bubble Tea program options: context binding and,
// when noColor is set, a forced ASCII color profile.
func programOpts(ctx context.Context, noColor bool) []tea.ProgramOption {
	opts := []tea.ProgramOption{tea.WithContext(ctx)}
	if noColor {
		opts = append(opts, tea.WithColorProfile(colorprofile.Ascii))
	}
	return opts
}
