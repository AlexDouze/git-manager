// pkg/ui/status_model.go
package ui

import (
	"fmt"
	"strings"

	"github.com/alexDouze/gitm/pkg/git"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	statusBarStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#626262",
	})
)

type repositoryItem struct {
	repo   *git.Repository
	status *git.RepositoryStatus
}

func (r repositoryItem) Title() string {
	prefix := "✓"
	if r.status != nil {
		if r.status.HasUncommittedChanges || r.status.HasBranchesWithRemoteGone || 
		   r.status.HasBranchesWithoutRemote || r.status.HasBranchesBehindRemote || 
		   r.status.StashCount > 0 {
			prefix = "⚠"
		}
	}
	return fmt.Sprintf("%s %s/%s/%s", prefix, r.repo.Host, r.repo.Organization, r.repo.Name)
}

func (r repositoryItem) Description() string {
	if r.status == nil {
		return "Status not available"
	}
	
	var issues []string
	
	if r.status.HasUncommittedChanges {
		issues = append(issues, fmt.Sprintf("%d uncommitted changes", len(r.status.UncommittedChanges)))
	}
	
	if r.status.HasBranchesWithRemoteGone {
		issues = append(issues, "branches with remote gone")
	}
	
	if r.status.HasBranchesWithoutRemote {
		issues = append(issues, "branches without remote")
	}
	
	if r.status.HasBranchesBehindRemote {
		issues = append(issues, "branches behind remote")
	}
	
	if r.status.StashCount > 0 {
		issues = append(issues, fmt.Sprintf("%d stashed changes", r.status.StashCount))
	}
	
	if len(issues) == 0 {
		return "No issues"
	}
	
	return strings.Join(issues, ", ")
}

func (r repositoryItem) FilterValue() string {
	return fmt.Sprintf("%s/%s/%s", r.repo.Host, r.repo.Organization, r.repo.Name)
}

type StatusModel struct {
	repositories []repositoryItem
	list         list.Model
	loading      bool
	spinner      spinner.Model
	filter       struct {
		host         string
		organization string
		repository   string
	}
	selectedRepository *repositoryItem
	detailView         viewport.Model
	showDetails        bool
	width, height      int
}

func NewStatusModel() *StatusModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	
	m := &StatusModel{
		spinner: s,
		loading: true,
	}
	
	// Initialize the list
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Repositories"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	
	m.list = l
	m.detailView = viewport.New(0, 0)
	
	return m
}

func (m *StatusModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadRepositories,
	)
}

func (m *StatusModel) loadRepositories() tea.Msg {
	// This would be replaced with actual repository loading logic
	// For now, we'll use dummy data
	repos := []repositoryItem{
		{
			repo: &git.Repository{
				Host:         "github.com",
				Organization: "organization",
				Name:         "repo1",
				Path:         "/path/to/repo1",
			},
			status: &git.RepositoryStatus{
				HasUncommittedChanges: false,
			},
		},
		{
			repo: &git.Repository{
				Host:         "github.com",
				Organization: "username",
				Name:         "repo2",
				Path:         "/path/to/repo2",
			},
			status: &git.RepositoryStatus{
				HasUncommittedChanges:    true,
				UncommittedChanges:       []string{"M src/main.go", "?? docs/new-feature.md"},
				HasBranchesBehindRemote:  true,
				HasBranchesWithRemoteGone: true,
			},
		},
		{
			repo: &git.Repository{
				Host:         "gitlab.com",
				Organization: "organization",
				Name:         "repo3",
				Path:         "/path/to/repo3",
			},
			status: &git.RepositoryStatus{
				StashCount: 2,
			},
		},
	}

	// Convert to list items
	var items []list.Item
	for _, r := range repos {
		items = append(items, r)
	}
	
	return loadedRepositoriesMsg{items: items}
}

type loadedRepositoriesMsg struct {
	items []list.Item
}

func (m *StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if !m.showDetails && !m.loading {
				i, ok := m.list.SelectedItem().(repositoryItem)
				if ok {
					m.selectedRepository = &i
					m.showDetails = true
					m.updateDetailView()
				}
			}
		case "esc":
			if m.showDetails {
				m.showDetails = false
			}
		case "f":
			// Toggle filter view (not implemented in this example)
		}
	
	case loadedRepositoriesMsg:
		m.loading = false
		m.repositories = make([]repositoryItem, len(msg.items))
		for i, item := range msg.items {
			m.repositories[i] = item.(repositoryItem)
		}
		m.list.SetItems(msg.items)
	
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		
		if m.showDetails {
			m.detailView.Width = msg.Width - 4
			m.detailView.Height = msg.Height - 6
		} else {
			m.list.SetWidth(msg.Width)
			m.list.SetHeight(msg.Height - 6)
		}
	}

	if m.loading {
		spinnerCmd := m.spinner.Tick
		cmds = append(cmds, spinnerCmd)
	}

	if m.showDetails {
		var cmd tea.Cmd
		m.detailView, cmd = m.detailView.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *StatusModel) updateDetailView() {
	if m.selectedRepository == nil {
		return
	}
	
	repo := m.selectedRepository.repo
	status := m.selectedRepository.status
	
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("Repository Details: %s/%s/%s\n\n", 
		repo.Host, repo.Organization, repo.Name))
	
	if status.HasUncommittedChanges || status.HasBranchesWithRemoteGone || 
	   status.HasBranchesWithoutRemote || status.HasBranchesBehindRemote || 
	   status.StashCount > 0 {
		sb.WriteString("Status: ⚠ Issues detected\n\n")
	} else {
		sb.WriteString("Status: ✓ No issues\n\n")
	}
	
	if status.HasUncommittedChanges {
		sb.WriteString("Uncommitted Changes:\n")
		for _, change := range status.UncommittedChanges {
			sb.WriteString(fmt.Sprintf("  %s\n", change))
		}
		sb.WriteString("\n")
	}
	
	sb.WriteString("Branches:\n")
	for _, branch := range status.Branches {
		branchStatus := "✓ Up to date"
		
		if branch.RemoteGone {
			branchStatus = "⚠ Remote gone"
		} else if branch.NoRemoteTracking {
			branchStatus = "⚠ No remote tracking"
		} else if branch.Ahead > 0 || branch.Behind > 0 {
			branchStatus = ""
			if branch.Ahead > 0 {
				branchStatus += fmt.Sprintf("%d commits ahead", branch.Ahead)
			}
			if branch.Behind > 0 {
				if branchStatus != "" {
					branchStatus += ", "
				}
				branchStatus += fmt.Sprintf("%d commits behind", branch.Behind)
			}
			branchStatus = "⚠ " + branchStatus
		}
		
		prefix := "  "
		if branch.Current {
			prefix = "* "
		}
		
		sb.WriteString(fmt.Sprintf("%s%s: %s\n", prefix, branch.Name, branchStatus))
	}
	sb.WriteString("\n")
	
	sb.WriteString(fmt.Sprintf("Stashes: %d\n\n", status.StashCount))
	
	sb.WriteString("[u] Update Repository  [p] Prune Branches  [Esc] Back")
	
	m.detailView.SetContent(sb.String())
}

func (m *StatusModel) View() string {
	if m.loading {
		return fmt.Sprintf("\n\n   %s Loading repositories...\n\n", m.spinner.View())
	}
	
	if m.showDetails && m.selectedRepository != nil {
		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(m.width - 4).
			Render(m.detailView.View())
	}
	
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(m.width - 4).
		Render(m.list.View())
}
