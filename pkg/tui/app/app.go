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

	repos  list.Model
	keys   repoKeyMap
	styles styles
	byPath map[string]int // repo path -> index in the current item slice

	width  int
	height int

	loading    bool // initial repository walk in flight
	statusBusy bool // async status load in flight
	err        error
}

// New builds the root model.
func New(ctx context.Context, cfg *config.Config, f Filter) Model {
	st := newStyles()
	keys := newRepoKeyMap()

	l := list.New(nil, newRepoDelegate(st), 0, 0)
	l.Title = "Repositories"
	l.SetShowHelp(true)
	l.SetStatusBarItemName("repo", "repos")
	l.AdditionalShortHelpKeys = keys.shortHelp
	l.AdditionalFullHelpKeys = keys.shortHelp

	return Model{
		ctx:     ctx,
		cfg:     cfg,
		filter:  f,
		repos:   l,
		keys:    keys,
		styles:  st,
		byPath:  map[string]int{},
		loading: true,
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
		return m, nil

	case tea.KeyPressMsg:
		// While filtering, let the list consume every key (so "q" types into
		// the filter box instead of quitting, and "r" doesn't refresh).
		if m.repos.FilterState() == list.Filtering {
			break
		}
		switch {
		case msg.String() == "q", msg.String() == "ctrl+c":
			return m, tea.Quit
		case key.Matches(msg, m.keys.Refresh):
			// Ignore refresh while a status load is already running so we don't
			// stack overlapping worker-pool passes over the same repos.
			if m.statusBusy {
				break
			}
			return m, m.refresh()
		}

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
	}

	var cmd tea.Cmd
	m.repos, cmd = m.repos.Update(msg)
	return m, cmd
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
	if m.err != nil {
		content = "Error: " + m.err.Error() + "\n\nPress q to quit."
	} else {
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
