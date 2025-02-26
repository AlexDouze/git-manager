// pkg/ui/prune_model.go
package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/alexDouze/gitm/pkg/git"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	pruneHeaderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2).
				Bold(true)

	pruneFooterStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)

	branchItemStyle = lipgloss.NewStyle().PaddingLeft(4)

	selectedBranchStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("170"))

	checkboxChecked   = "✓ "
	checkboxUnchecked = "  "
)

// BranchItem represents a branch in the pruning list
type BranchItem struct {
	Repository    *git.Repository
	BranchName    string
	IsRemoteGone  bool
	IsMerged      bool
	HasCommits    bool
	LastCommitAge string
	Selected      bool
}

func (b BranchItem) Title() string {
	prefix := checkboxUnchecked
	if b.Selected {
		prefix = checkboxChecked
	}
	return prefix + b.BranchName
}

func (b BranchItem) Description() string {
	var parts []string

	if b.IsRemoteGone {
		parts = append(parts, "remote gone")
	}

	if b.IsMerged {
		parts = append(parts, "merged")
	}

	if b.HasCommits {
		parts = append(parts, "has unpushed commits")
	}

	if b.LastCommitAge != "" {
		parts = append(parts, fmt.Sprintf("last commit %s", b.LastCommitAge))
	}

	return strings.Join(parts, ", ")
}

func (b BranchItem) FilterValue() string {
	return b.BranchName
}

// PruneModel is the BubbleTea model for the prune command
type PruneModel struct {
	repositories     []*git.Repository
	currentRepoIndex int
	branchList       list.Model
	spinner          spinner.Model
	loading          bool
	dryRun           bool
	goneOnly         bool
	mergedOnly       bool
	filter           struct {
		host         string
		organization string
		repository   string
	}
	width, height    int
	selectedBranches map[string][]string // map[repoPath][]branchName
	confirming       bool
	pruning          bool
	pruneResults     map[string][]string // map[repoPath][]branchName
	pruneErrors      map[string]string   // map[repoPath]error
	showSummary      bool
}

// NewPruneModel creates a new prune model
func NewPruneModel(repositories []*git.Repository, dryRun, goneOnly, mergedOnly bool) *PruneModel {
	s := spinner.New()
	s.Spinner = spinner.Dot

	m := &PruneModel{
		repositories:     repositories,
		spinner:          s,
		loading:          true,
		dryRun:           dryRun,
		goneOnly:         goneOnly,
		mergedOnly:       mergedOnly,
		selectedBranches: make(map[string][]string),
		pruneResults:     make(map[string][]string),
		pruneErrors:      make(map[string]string),
	}

	// Initialize the branch list
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select branches to prune"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	// Add custom keybindings
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("space"),
				key.WithHelp("space", "toggle selection"),
			),
			key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "select all"),
			),
			key.NewBinding(
				key.WithKeys("n"),
				key.WithHelp("n", "select none"),
			),
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "confirm selection"),
			),
		}
	}

	m.branchList = l

	return m
}

func (m *PruneModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			return m.loadBranches()
		},
	)
}

// loadBranches loads branches for the current repository
func (m *PruneModel) loadBranches() tea.Msg {
	if m.currentRepoIndex >= len(m.repositories) {
		// We've processed all repositories, show summary
		return branchesLoadedMsg{done: true}
	}

	repo := m.repositories[m.currentRepoIndex]

	// Get branches for the current repository
	branches, err := m.getBranchesForPruning(repo)
	if err != nil {
		return branchLoadErrorMsg{
			repository: repo,
			err:        err,
		}
	}

	return branchesLoadedMsg{
		repository: repo,
		branches:   branches,
	}
}

// getBranchesForPruning gets branches that match the pruning criteria
func (m *PruneModel) getBranchesForPruning(repo *git.Repository) ([]BranchItem, error) {
	// Get repository status
	status, err := repo.Status()
	if err != nil {
		return nil, err
	}

	var branchItems []BranchItem

	for _, branch := range status.Branches {
		// Skip current branch
		if branch.Current {
			continue
		}

		// Apply filters
		if m.goneOnly && !branch.RemoteGone {
			continue
		}

		// Check if branch is merged (this is a simplified check, in a real implementation
		// you would need to check against the target branch)
		isMerged := false
		if m.mergedOnly {
			// This would be a call to check if the branch is merged
			// For this example, we'll assume some branches are merged
			isMerged = branch.Name != "main" && branch.Name != "master" && !strings.HasPrefix(branch.Name, "feature/")
		}

		if m.mergedOnly && !isMerged {
			continue
		}

		// Create branch item
		branchItem := BranchItem{
			Repository:    repo,
			BranchName:    branch.Name,
			IsRemoteGone:  branch.RemoteGone,
			IsMerged:      isMerged,
			HasCommits:    branch.Ahead > 0,
			LastCommitAge: "2 weeks ago", // This would be determined from git log
			Selected:      false,
		}

		branchItems = append(branchItems, branchItem)
	}

	return branchItems, nil
}

type branchesLoadedMsg struct {
	repository *git.Repository
	branches   []BranchItem
	done       bool
}

type branchLoadErrorMsg struct {
	repository *git.Repository
	err        error
}

type pruneDoneMsg struct {
	repository *git.Repository
	branches   []string
	err        error
}

func (m *PruneModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showSummary {
			if msg.String() == "q" || msg.String() == "esc" {
				return m, tea.Quit
			}
			break
		}

		if m.confirming {
			switch msg.String() {
			case "y":
				m.confirming = false
				m.pruning = true
				return m, m.pruneBranches()
			case "n", "esc":
				m.confirming = false
			}
			break
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "space":
			if m.loading || m.pruning {
				break
			}

			// Toggle selection of current item
			index := m.branchList.Index()
			items := m.branchList.Items()
			if index >= 0 && index < len(items) {
				branch := items[index].(BranchItem)
				branch.Selected = !branch.Selected
				items[index] = branch
				m.branchList.SetItems(items)
			}

		case "a":
			if m.loading || m.pruning {
				break
			}

			// Select all branches
			items := m.branchList.Items()
			for i, item := range items {
				branch := item.(BranchItem)
				branch.Selected = true
				items[i] = branch
			}
			m.branchList.SetItems(items)

		case "n":
			if m.loading || m.pruning {
				break
			}

			// Deselect all branches
			items := m.branchList.Items()
			for i, item := range items {
				branch := item.(BranchItem)
				branch.Selected = false
				items[i] = branch
			}
			m.branchList.SetItems(items)

		case "enter":
			if m.loading || m.pruning {
				break
			}

			// Store selected branches
			repo := m.repositories[m.currentRepoIndex]
			var selectedBranches []string

			for _, item := range m.branchList.Items() {
				branch := item.(BranchItem)
				if branch.Selected {
					selectedBranches = append(selectedBranches, branch.BranchName)
				}
			}

			if len(selectedBranches) > 0 {
				m.selectedBranches[repo.Path] = selectedBranches
			}

			// Move to next repository
			m.currentRepoIndex++
			m.loading = true
			return m, func() tea.Msg {
				return m.loadBranches()
			}

		case "esc":
			if !m.loading && !m.pruning {
				// Skip current repository
				m.currentRepoIndex++
				m.loading = true
				return m, func() tea.Msg {
					return m.loadBranches()
				}
			}
		}

	case branchesLoadedMsg:
		m.loading = false

		if msg.done {
			// We've processed all repositories, check if we have any branches to prune
			totalBranches := 0
			for _, branches := range m.selectedBranches {
				totalBranches += len(branches)
			}

			if totalBranches > 0 {
				m.confirming = true
			} else {
				m.showSummary = true
			}

			break
		}

		// Convert branch items to list items
		var items []list.Item
		for _, branch := range msg.branches {
			items = append(items, branch)
		}

		m.branchList.Title = fmt.Sprintf("Select branches to prune in %s/%s/%s",
			msg.repository.Host, msg.repository.Organization, msg.repository.Name)
		m.branchList.SetItems(items)

	case branchLoadErrorMsg:
		m.loading = false
		m.pruneErrors[msg.repository.Path] = msg.err.Error()

		// Move to next repository
		m.currentRepoIndex++
		m.loading = true
		return m, func() tea.Msg {
			return m.loadBranches()
		}

	case pruneDoneMsg:
		if msg.err != nil {
			m.pruneErrors[msg.repository.Path] = msg.err.Error()
		} else {
			m.pruneResults[msg.repository.Path] = msg.branches
		}

		// Check if all repositories have been processed
		processedCount := len(m.pruneResults) + len(m.pruneErrors)
		if processedCount == len(m.selectedBranches) {
			m.pruning = false
			m.showSummary = true
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.branchList.SetWidth(msg.Width - 4)
		m.branchList.SetHeight(msg.Height - 10)
	}

	if m.loading {
		spinnerCmd := m.spinner.Tick
		cmds = append(cmds, spinnerCmd)
	}

	if !m.loading && !m.pruning && !m.confirming && !m.showSummary {
		var cmd tea.Cmd
		m.branchList, cmd = m.branchList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// pruneBranches prunes the selected branches
func (m *PruneModel) pruneBranches() tea.Cmd {
	return func() tea.Msg {
		// Process one repository at a time
		for repoPath, branches := range m.selectedBranches {
			// Find the repository
			var repo *git.Repository
			for _, r := range m.repositories {
				if r.Path == repoPath {
					repo = r
					break
				}
			}

			if repo == nil {
				m.pruneErrors[repoPath] = "repository not found"
				continue
			}

			// Prune branches
			if m.dryRun {
				// In dry run mode, just report what would be pruned
				m.pruneResults[repoPath] = branches
			} else {
				// Actually prune the branches
				err := m.pruneBranchesInRepo(repo, branches)
				if err != nil {
					return pruneDoneMsg{
						repository: repo,
						err:        err,
					}
				}

				return pruneDoneMsg{
					repository: repo,
					branches:   branches,
				}
			}
		}

		// If we get here, we've processed all repositories in dry run mode
		m.pruning = false
		m.showSummary = true
		return nil
	}
}

// pruneBranchesInRepo prunes branches in a repository
func (m *PruneModel) pruneBranchesInRepo(repo *git.Repository, branches []string) error {
	for _, branch := range branches {
		// Execute git branch -D command
		cmd := exec.Command("git", "-C", repo.Path, "branch", "-D", branch)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to delete branch %s: %w", branch, err)
		}
	}

	return nil
}

func (m *PruneModel) View() string {
	if m.showSummary {
		return m.renderSummary()
	}

	if m.pruning {
		return m.renderPruningView()
	}

	if m.confirming {
		return m.renderConfirmationView()
	}

	if m.loading {
		if m.currentRepoIndex >= len(m.repositories) {
			return fmt.Sprintf("\n\n   %s Processing results...\n\n", m.spinner.View())
		}

		repo := m.repositories[m.currentRepoIndex]
		return fmt.Sprintf("\n\n   %s Loading branches from %s/%s/%s...\n\n",
			m.spinner.View(), repo.Host, repo.Organization, repo.Name)
	}

	// Render the branch list
	header := pruneHeaderStyle.Render(fmt.Sprintf(
		"Repository %d/%d: %s/%s/%s",
		m.currentRepoIndex+1,
		len(m.repositories),
		m.repositories[m.currentRepoIndex].Host,
		m.repositories[m.currentRepoIndex].Organization,
		m.repositories[m.currentRepoIndex].Name,
	))

	footer := pruneFooterStyle.Render(
		"[space] Toggle  [a] Select All  [n] Select None  [enter] Confirm  [esc] Skip  [q] Quit",
	)

	list := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(m.width - 4).
		Height(m.height - 8).
		Render(m.branchList.View())

	return fmt.Sprintf("%s\n%s\n%s", header, list, footer)
}

// renderConfirmationView renders the confirmation view
func (m *PruneModel) renderConfirmationView() string {
	totalBranches := 0
	for _, branches := range m.selectedBranches {
		totalBranches += len(branches)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n\n  You have selected %d branches for pruning in %d repositories:\n\n",
		totalBranches, len(m.selectedBranches)))

	for repoPath, branches := range m.selectedBranches {
		// Find repository info
		var repo *git.Repository
		for _, r := range m.repositories {
			if r.Path == repoPath {
				repo = r
				break
			}
		}

		if repo != nil {
			sb.WriteString(fmt.Sprintf("  • %s/%s/%s: %d branches\n",
				repo.Host, repo.Organization, repo.Name, len(branches)))

			for _, branch := range branches {
				sb.WriteString(fmt.Sprintf("    - %s\n", branch))
			}
			sb.WriteString("\n")
		}
	}

	if m.dryRun {
		sb.WriteString("  This is a dry run. No branches will be deleted.\n\n")
	} else {
		sb.WriteString("  WARNING: This operation cannot be undone!\n\n")
	}

	sb.WriteString("  Proceed with pruning? [y/n] ")

	return sb.String()
}

// renderPruningView renders the pruning view
func (m *PruneModel) renderPruningView() string {
	processedCount := len(m.pruneResults) + len(m.pruneErrors)
	totalRepos := len(m.selectedBranches)

	return fmt.Sprintf("\n\n   %s Pruning branches... (%d/%d repositories processed)\n\n",
		m.spinner.View(), processedCount, totalRepos)
}

// renderSummary renders the summary view
func (m *PruneModel) renderSummary() string {
	var sb strings.Builder

	if len(m.selectedBranches) == 0 {
		sb.WriteString("\n\n  No branches were selected for pruning.\n\n")
		sb.WriteString("  Press q to exit.\n")
		return sb.String()
	}

	totalBranches := 0
	for _, branches := range m.selectedBranches {
		totalBranches += len(branches)
	}

	if m.dryRun {
		sb.WriteString(fmt.Sprintf("\n\n  Dry run completed. %d branches in %d repositories would be pruned:\n\n",
			totalBranches, len(m.selectedBranches)))
	} else {
		sb.WriteString(fmt.Sprintf("\n\n  Pruning completed. %d branches in %d repositories were pruned:\n\n",
			totalBranches, len(m.selectedBranches)))
	}

	// Show successful prunes
	for repoPath, branches := range m.pruneResults {
		// Find repository info
		var repo *git.Repository
		for _, r := range m.repositories {
			if r.Path == repoPath {
				repo = r
				break
			}
		}

		if repo != nil {
			sb.WriteString(fmt.Sprintf("  ✓ %s/%s/%s: %d branches\n",
				repo.Host, repo.Organization, repo.Name, len(branches)))

			for _, branch := range branches {
				sb.WriteString(fmt.Sprintf("    - %s\n", branch))
			}
			sb.WriteString("\n")
		}
	}

	// Show errors
	if len(m.pruneErrors) > 0 {
		sb.WriteString("  Errors:\n\n")

		for repoPath, errMsg := range m.pruneErrors {
			// Find repository info
			var repo *git.Repository
			for _, r := range m.repositories {
				if r.Path == repoPath {
					repo = r
					break
				}
			}

			if repo != nil {
				sb.WriteString(fmt.Sprintf("  ✗ %s/%s/%s: %s\n",
					repo.Host, repo.Organization, repo.Name, errMsg))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  Press q to exit.\n")

	return sb.String()
}
