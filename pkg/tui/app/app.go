// Package app implements the interactive Bubble Tea TUI launched by bare `gitm`.
//
// The app follows the Elm architecture: a root Model holds the current screen
// and delegates most navigation/filtering to Bubbles list components. Slow git
// operations never run inside Update; they are dispatched as tea.Cmd closures
// (see commands.go) that return result messages.
package app

import (
	"context"

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
	cfg    *config.Config
	filter Filter

	repos list.Model

	width  int
	height int

	loading bool
	err     error
}

// New builds the root model.
func New(cfg *config.Config, f Filter) Model {
	l := list.New(nil, newRepoDelegate(), 0, 0)
	l.Title = "Repositories"
	l.SetShowHelp(true)

	return Model{
		cfg:     cfg,
		filter:  f,
		repos:   l,
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
		// the filter box instead of quitting).
		if m.repos.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case reposLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.repos))
		for _, r := range msg.repos {
			items = append(items, repoItem{repo: r})
		}
		return m, m.repos.SetItems(items)
	}

	var cmd tea.Cmd
	m.repos, cmd = m.repos.Update(msg)
	return m, cmd
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
	p := tea.NewProgram(New(cfg, f), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

// loadReposCmd finds repositories matching the filter. This is fast (a
// filesystem walk, no git subprocesses) so it runs as the initial command.
func loadReposCmd(cfg *config.Config, f Filter) tea.Cmd {
	return func() tea.Msg {
		repos, err := git.FindRepositories(cfg.RootDirectory, f.Host, f.Org, f.Repo, f.Path)
		return reposLoadedMsg{repos: repos, err: err}
	}
}

// reposLoadedMsg carries the result of loadReposCmd.
type reposLoadedMsg struct {
	repos []*git.Repository
	err   error
}
