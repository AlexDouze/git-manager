// Package app implements the interactive Bubble Tea TUI launched by bare `gitm`.
//
// The app follows the Elm architecture: a root Model holds the current screen
// and delegates most navigation/filtering to Bubbles list components. Slow git
// operations never run inside Update; they are dispatched as tea.Cmd closures
// (see commands.go) that return result messages.
package app

import (
	"context"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
)

// screen identifies which list is currently in front.
type screen int

const (
	screenRepos    screen = iota // the top-level repository list
	screenBranches               // the branch list for a drilled-into repo
)

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

	branches   list.Model
	branchKeys branchKeyMap
	activeRepo *git.Repository // the repo whose branches are shown
	branchBusy bool            // async branch load in flight

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
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

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
	}

	return m.updateActiveList(msg)
}

// handleKey routes key presses based on the current screen. Keys are always
// forwarded to the active list afterwards (so navigation/filter still work),
// except when a shortcut fully handles the press.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
			// Ignore refresh while a status load is already running so we don't
			// stack overlapping worker-pool passes over the same repos.
			if m.statusBusy {
				return m, nil
			}
			return m, m.refresh()
		}

	case screenBranches:
		switch {
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
	var content string
	switch {
	case m.err != nil:
		content = "Error: " + m.err.Error() + "\n\nPress q to quit."
	case m.screen == screenBranches:
		content = m.branches.View()
	default:
		content = m.repos.View()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// Run launches the interactive app and blocks until the user quits.
func Run(ctx context.Context, cfg *config.Config, f Filter) error {
	p := tea.NewProgram(New(ctx, cfg, f), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
