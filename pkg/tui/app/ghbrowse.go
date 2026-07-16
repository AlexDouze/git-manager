package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
)

// ghPhase tracks where the browser is in its own little flow.
type ghPhase int

const (
	ghPhaseOwner   ghPhase = iota // typing the owner to list
	ghPhaseLoading                // `gh repo list` in flight
	ghPhaseList                   // choosing repos to clone
	ghPhaseCloning                // clone batch in flight
)

// ghItem is a single row in the GitHub repo list. existing marks a repo already
// cloned on disk (shown as such and non-selectable); selected is the toggle
// state used to build the clone set.
type ghItem struct {
	repo     git.Repository
	selected bool
	existing bool
}

// FilterValue implements list.Item; `/` filters on the repo name.
func (i ghItem) FilterValue() string { return i.repo.Name }

// ghDelegate renders GitHub repo rows: a checkbox, the name, and an
// "already cloned" note for repos present on disk.
type ghDelegate struct {
	styles styles
}

func newGHDelegate(s styles) ghDelegate {
	return ghDelegate{styles: s}
}

func (d ghDelegate) Height() int                             { return 1 }
func (d ghDelegate) Spacing() int                            { return 0 }
func (d ghDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d ghDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(ghItem)
	if !ok {
		return
	}

	box := "[ ] "
	name := it.repo.Name
	switch {
	case it.existing:
		box = "[-] "
		name += d.styles.dim.Render(" (already cloned)")
	case it.selected:
		box = "[✓] "
	}

	line := box + name
	if index == m.Index() {
		fmt.Fprint(w, d.styles.selected.Render("> "+line))
		return
	}
	fmt.Fprint(w, d.styles.normal.Render("  "+line))
}

// ensure the interface is satisfied at compile time.
var _ list.ItemDelegate = ghDelegate{}

// ghScreen is the self-contained GitHub clone browser. It owns an owner-input
// step, an async repo listing, a multi-select list, and an async clone batch.
// It is embedded in the root Model as the screenGHBrowse screen and can also be
// driven standalone (see RunBrowse) by `gitm gh-clone`.
type ghScreen struct {
	ctx  context.Context
	cfg  *config.Config
	keys ghKeyMap

	phase  ghPhase
	owner  string // fixed owner when launched with an argument; "" = prompt
	limit  int
	input  textinput.Model
	list   list.Model
	styles styles

	rootDir string // where clones land (cfg.RootDirectory unless overridden)

	footer    string
	footerErr bool
	err       error

	width  int
	height int
}

// newGHScreen builds a browser. owner, when non-empty, skips the input step and
// lists that owner immediately. rootDir overrides cfg.RootDirectory when set.
func newGHScreen(ctx context.Context, cfg *config.Config, st styles, owner, rootDir string, limit int) ghScreen {
	keys := newGHKeyMap()

	ti := textinput.New()
	ti.Prompt = "Owner: "
	ti.Placeholder = "github org or user (blank = you)"
	ti.SetWidth(40)

	l := list.New(nil, newGHDelegate(st), 0, 0)
	l.Title = "GitHub repositories"
	l.SetShowHelp(true)
	l.SetStatusBarItemName("repo", "repos")
	l.AdditionalShortHelpKeys = keys.shortHelp
	l.AdditionalFullHelpKeys = keys.shortHelp

	root := cfg.RootDirectory
	if rootDir != "" {
		root = rootDir
	}

	s := ghScreen{
		ctx:     ctx,
		cfg:     cfg,
		keys:    keys,
		owner:   owner,
		limit:   limit,
		input:   ti,
		list:    l,
		styles:  st,
		rootDir: root,
	}
	if owner != "" {
		s.phase = ghPhaseLoading
	} else {
		s.phase = ghPhaseOwner
	}
	return s
}

// init returns the command that starts the browser: focus the input when
// prompting, or kick off the listing when an owner was supplied up front.
func (s *ghScreen) init() tea.Cmd {
	if s.phase == ghPhaseLoading {
		return tea.Batch(s.list.StartSpinner(), loadGitHubReposCmd(s.ctx, s.owner, s.limit))
	}
	return s.input.Focus()
}

// setSize resizes the browser's list (leaving room for the help/footer lines).
func (s *ghScreen) setSize(w, h int) {
	s.width, s.height = w, h
	s.list.SetSize(w, h)
	s.input.SetWidth(min(w-len(s.input.Prompt)-1, 60))
}

// update advances the browser. Returned command is nil unless it started async
// work. A returned ghExitMsg (via cmd) is how the browser asks to leave.
func (s ghScreen) update(msg tea.Msg) (ghScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.setSize(msg.Width, msg.Height)
		return s, nil

	case ghReposLoadedMsg:
		s.list.StopSpinner()
		if msg.err != nil {
			s.err = msg.err
			return s, nil
		}
		return s, s.setRepos(msg.repos)

	case ghCloneDoneMsg:
		return s.applyCloneResults(msg.results)

	case tea.KeyPressMsg:
		return s.handleKey(msg)
	}

	// Forward other messages to whichever child owns the current phase.
	return s.forward(msg)
}

// handleKey routes key presses by phase.
func (s ghScreen) handleKey(msg tea.KeyPressMsg) (ghScreen, tea.Cmd) {
	switch s.phase {
	case ghPhaseOwner:
		switch msg.String() {
		case "enter":
			s.owner = strings.TrimSpace(s.input.Value())
			s.phase = ghPhaseLoading
			s.input.Blur()
			return s, tea.Batch(s.list.StartSpinner(), loadGitHubReposCmd(s.ctx, s.owner, s.limit))
		case "esc", "ctrl+c":
			return s, func() tea.Msg { return ghExitMsg{} }
		}
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd

	case ghPhaseList:
		// While filtering, the list owns every key.
		if s.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			s.list, cmd = s.list.Update(msg)
			return s, cmd
		}
		switch {
		case msg.String() == "ctrl+c":
			return s, func() tea.Msg { return ghExitMsg{} }
		case key.Matches(msg, s.keys.Back):
			return s, func() tea.Msg { return ghExitMsg{} }
		case key.Matches(msg, s.keys.Toggle):
			return s.toggleSelected()
		case key.Matches(msg, s.keys.Clone):
			return s.cloneSelected()
		}
		var cmd tea.Cmd
		s.list, cmd = s.list.Update(msg)
		return s, cmd
	}

	// ghPhaseLoading / ghPhaseCloning: ctrl+c aborts back to caller.
	if msg.String() == "ctrl+c" {
		return s, func() tea.Msg { return ghExitMsg{} }
	}
	return s, nil
}

// forward passes non-key messages to the active phase's child component so
// spinners tick and the text input blinks.
func (s ghScreen) forward(msg tea.Msg) (ghScreen, tea.Cmd) {
	var cmd tea.Cmd
	switch s.phase {
	case ghPhaseOwner:
		s.input, cmd = s.input.Update(msg)
	default:
		s.list, cmd = s.list.Update(msg)
	}
	return s, cmd
}

// setRepos populates the list, marking already-cloned repos as non-selectable.
func (s *ghScreen) setRepos(repos []git.Repository) tea.Cmd {
	s.phase = ghPhaseList
	items := make([]list.Item, 0, len(repos))
	for _, r := range repos {
		dir := filepath.Join(s.rootDir, r.Host, r.Organization, r.Name)
		_, statErr := os.Stat(dir)
		items = append(items, ghItem{repo: r, existing: statErr == nil})
	}
	if len(items) == 0 {
		s.footer = "no repositories found"
	}
	return s.list.SetItems(items)
}

// toggleSelected flips the highlighted repo's selection, refusing repos that
// are already cloned.
func (s ghScreen) toggleSelected() (ghScreen, tea.Cmd) {
	idx := s.list.Index()
	items := s.list.Items()
	if idx < 0 || idx >= len(items) {
		return s, nil
	}
	it, ok := items[idx].(ghItem)
	if !ok || it.existing {
		return s, nil
	}
	it.selected = !it.selected
	return s, s.list.SetItem(idx, it)
}

// cloneSelected kicks off the clone batch for every selected repo.
func (s ghScreen) cloneSelected() (ghScreen, tea.Cmd) {
	var chosen []git.Repository
	for _, li := range s.list.Items() {
		if it, ok := li.(ghItem); ok && it.selected {
			chosen = append(chosen, it.repo)
		}
	}
	if len(chosen) == 0 {
		s.footer = "nothing selected"
		s.footerErr = true
		return s, nil
	}

	var options []string
	if s.cfg.Clone.DefaultOptions != "" {
		options = strings.Fields(s.cfg.Clone.DefaultOptions)
	}
	s.phase = ghPhaseCloning
	s.footer = fmt.Sprintf("cloning %d repositories…", len(chosen))
	s.footerErr = false
	return s, tea.Batch(s.list.StartSpinner(), cloneReposCmd(s.ctx, chosen, s.rootDir, options))
}

// applyCloneResults summarizes a finished clone batch and returns to the list
// so successfully-cloned repos now show as already cloned.
func (s ghScreen) applyCloneResults(results []cloneResult) (ghScreen, tea.Cmd) {
	s.list.StopSpinner()
	var ok, failed int
	for _, r := range results {
		if r.err != nil {
			failed++
		} else {
			ok++
		}
	}
	s.footer = fmt.Sprintf("cloned %d, failed %d", ok, failed)
	s.footerErr = failed > 0
	s.phase = ghPhaseList

	// Re-mark rows: a just-cloned repo is now existing and deselected.
	var cmds []tea.Cmd
	cloned := make(map[string]bool, ok)
	for _, r := range results {
		if r.err == nil {
			cloned[r.name] = true
		}
	}
	for i, li := range s.list.Items() {
		it, isItem := li.(ghItem)
		if !isItem {
			continue
		}
		if cloned[it.repo.Name] {
			it.existing = true
			it.selected = false
			cmds = append(cmds, s.list.SetItem(i, it))
		}
	}
	return s, tea.Batch(cmds...)
}

// view renders the browser for the current phase.
func (s ghScreen) view() string {
	if s.err != nil {
		return "Error: " + s.err.Error() + "\n\nPress esc to go back."
	}

	var content string
	switch s.phase {
	case ghPhaseOwner:
		content = "List a GitHub owner's repositories to clone.\n\n" +
			s.input.View() + "\n\n" +
			s.styles.dim.Render("enter list · esc cancel")
	default:
		content = s.list.View()
	}
	if s.footer != "" {
		style := s.styles.footer
		if s.footerErr {
			style = s.styles.footerErr
		}
		content += "\n" + style.Render(s.footer)
	}
	return content
}

// ghProgram is a minimal root model that runs the ghScreen standalone (used by
// `gitm gh-clone`). It maps the browser's ghExitMsg to a program quit.
type ghProgram struct {
	gh ghScreen
}

func (p ghProgram) Init() tea.Cmd { return p.gh.init() }

func (p ghProgram) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(ghExitMsg); ok {
		return p, tea.Quit
	}
	var cmd tea.Cmd
	p.gh, cmd = p.gh.update(msg)
	return p, cmd
}

func (p ghProgram) View() tea.View {
	v := tea.NewView(p.gh.view())
	v.AltScreen = true
	return v
}

// RunBrowse launches the GitHub clone browser standalone. owner, when
// non-empty, skips the input prompt and lists that owner immediately. rootDir
// overrides cfg.RootDirectory as the clone destination when set.
func RunBrowse(ctx context.Context, cfg *config.Config, owner, rootDir string, limit int) error {
	st := newStyles()
	gh := newGHScreen(ctx, cfg, st, owner, rootDir, limit)
	p := tea.NewProgram(ghProgram{gh: gh}, tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
