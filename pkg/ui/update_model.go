// pkg/ui/update_model.go
package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/alexDouze/gitm/pkg/git"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	updateHeaderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2).
				Bold(true)

	updateProgressStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)

	updateLogStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Height(10)

	updateFooterStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)
)

// UpdateOperation represents a repository update operation
type UpdateOperation struct {
	Repository *git.Repository
	Status     string
	Log        []string
	Error      string
	Complete   bool
}

// UpdateModel is the BubbleTea model for the update command
type UpdateModel struct {
	repositories []*git.Repository
	operations   map[string]*UpdateOperation
	progress     progress.Model
	spinner      spinner.Model
	currentOp    *UpdateOperation
	fetchOnly    bool
	prune        bool
	filter       struct {
		host         string
		organization string
		repository   string
	}
	width, height int
	completed     int
	skipped       int
	failed        int
	total         int
	updating      bool
	done          bool
	logMutex      sync.Mutex
}

// NewUpdateModel creates a new update model
func NewUpdateModel(repositories []*git.Repository, fetchOnly, prune bool) *UpdateModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	s := spinner.New()
	s.Spinner = spinner.Dot

	operations := make(map[string]*UpdateOperation)
	for _, repo := range repositories {
		operations[repo.Path] = &UpdateOperation{
			Repository: repo,
			Status:     "Pending",
			Log:        []string{},
		}
	}

	return &UpdateModel{
		repositories: repositories,
		operations:   operations,
		progress:     p,
		spinner:      s,
		fetchOnly:    fetchOnly,
		prune:        prune,
		total:        len(repositories),
		updating:     true,
	}
}

func (m *UpdateModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.startUpdating,
	)
}

// startUpdating begins the update process
func (m *UpdateModel) startUpdating() tea.Msg {
	// Start a goroutine to update repositories
	go func() {
		for _, repo := range m.repositories {
			op := m.operations[repo.Path]
			m.currentOp = op

			// Update status
			op.Status = "Fetching"
			m.updateLog(op, "Fetching from remotes...")

			// Fetch from remotes
			err := m.fetchRepository(repo)
			if err != nil {
				op.Status = "Failed"
				op.Error = fmt.Sprintf("Fetch failed: %v", err)
				op.Complete = true
				m.failed++
				m.completed++
				continue
			}

			m.updateLog(op, "Fetch completed successfully")

			// If fetch-only, skip pulling
			if m.fetchOnly {
				op.Status = "Completed"
				op.Complete = true
				m.completed++
				continue
			}

			// Get repository status
			status, err := repo.Status()
			if err != nil {
				op.Status = "Failed"
				op.Error = fmt.Sprintf("Status check failed: %v", err)
				op.Complete = true
				m.failed++
				m.completed++
				continue
			}

			// Skip if there are uncommitted changes
			if status.HasUncommittedChanges {
				op.Status = "Skipped"
				op.Error = "Uncommitted changes present"
				op.Complete = true
				m.skipped++
				m.completed++
				continue
			}

			// Update branches that are behind
			updatedBranches := 0
			for _, branch := range status.Branches {
				if branch.Behind > 0 && !branch.RemoteGone {
					op.Status = fmt.Sprintf("Updating %s", branch.Name)
					m.updateLog(op, fmt.Sprintf("Branch %s is %d commits behind %s",
						branch.Name, branch.Behind, branch.RemoteTracking))

					// Update branch
					err := m.updateBranch(repo, branch.Name)
					if err != nil {
						m.updateLog(op, fmt.Sprintf("Failed to update branch %s: %v", branch.Name, err))
						continue
					}

					m.updateLog(op, fmt.Sprintf("Successfully updated branch %s", branch.Name))
					updatedBranches++
				}
			}

			// Check for new remote branches to checkout
			if err := m.checkoutNewBranches(repo, status); err != nil {
				m.updateLog(op, fmt.Sprintf("Failed to checkout new branches: %v", err))
			}

			op.Status = "Completed"
			op.Complete = true
			m.completed++
		}

		m.currentOp = nil
		m.updating = false
		m.done = true
	}()

	return nil
}

// fetchRepository fetches from all remotes
func (m *UpdateModel) fetchRepository(repo *git.Repository) error {
	op := m.operations[repo.Path]

	// Fetch from all remotes
	args := []string{"--all"}
	if m.prune {
		args = append(args, "--prune")
		m.updateLog(op, "Pruning remote-tracking branches...")
	}

	return repo.Fetch(args)
}

// updateBranch updates a branch using pull --rebase
func (m *UpdateModel) updateBranch(repo *git.Repository, branch string) error {
	op := m.operations[repo.Path]

	// Save current branch
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return err
	}

	// Checkout the branch to update
	if currentBranch != branch {
		m.updateLog(op, fmt.Sprintf("Checking out branch %s...", branch))
		if err := repo.Checkout(branch); err != nil {
			return err
		}
	}

	// Pull with rebase
	m.updateLog(op, fmt.Sprintf("Pulling with rebase on branch %s...", branch))
	if err := repo.Pull([]string{"--rebase"}); err != nil {
		// Try to restore original branch if we changed it
		if currentBranch != branch {
			checkoutErr := repo.Checkout(currentBranch)
			if checkoutErr != nil {
				m.updateLog(op, fmt.Sprintf("Warning: Failed to restore original branch %s: %v", currentBranch, checkoutErr))
				// Return the original pull error, but log the checkout error
			}
		}
		return err
	}

	// Restore original branch if we changed it
	if currentBranch != branch {
		m.updateLog(op, fmt.Sprintf("Checking out original branch %s...", currentBranch))
		if err := repo.Checkout(currentBranch); err != nil {
			return err
		}
	}

	return nil
}

// checkoutNewBranches checks out new remote branches
func (m *UpdateModel) checkoutNewBranches(repo *git.Repository, status *git.RepositoryStatus) error {
	op := m.operations[repo.Path]

	// Get list of remote branches
	remoteBranches, err := repo.GetRemoteBranches()
	if err != nil {
		return err
	}

	// Get list of local branches
	localBranches := make(map[string]bool)
	for _, branch := range status.Branches {
		localBranches[branch.Name] = true
	}

	// Check for new branches to checkout
	for _, remoteBranch := range remoteBranches {
		// Skip if already exists locally
		if localBranches[remoteBranch] {
			continue
		}

		// Skip special branches like HEAD
		if remoteBranch == "HEAD" {
			continue
		}

		m.updateLog(op, fmt.Sprintf("Checking out new branch %s...", remoteBranch))

		// Note: GetRemoteBranches already extracts the branch name without the remote prefix
		// We'll assume origin as the remote for now, but this could be enhanced to handle multiple remotes
		remoteName := "origin"
		branchName := remoteBranch

		// Checkout the new branch
		if err := repo.Checkout("-b", branchName, "--track", remoteName+"/"+branchName); err != nil {
			m.updateLog(op, fmt.Sprintf("Failed to checkout branch %s: %v", branchName, err))
			continue
		}

		m.updateLog(op, fmt.Sprintf("Successfully checked out new branch %s", remoteBranch))
	}

	return nil
}

// updateLog adds a log entry to an operation
func (m *UpdateModel) updateLog(op *UpdateOperation, message string) {
	m.logMutex.Lock()
	defer m.logMutex.Unlock()

	op.Log = append(op.Log, message)

	// Keep log at a reasonable size
	if len(op.Log) > 100 {
		op.Log = op.Log[len(op.Log)-100:]
	}
}

func (m *UpdateModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress.Width = msg.Width - 20
	}

	if m.updating {
		spinnerCmd := m.spinner.Tick
		cmds = append(cmds, spinnerCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *UpdateModel) View() string {
	if m.total == 0 {
		return "No repositories to update."
	}

	// Calculate progress
	progress := float64(m.completed) / float64(m.total)

	// Build header
	header := updateHeaderStyle.Render(fmt.Sprintf(
		"Updating repositories: %d/%d completed", m.completed, m.total))

	// Build progress bar
	progressBar := updateProgressStyle.Render(fmt.Sprintf(
		"Progress: %s", m.progress.ViewAs(progress)))

	// Build status
	var statusText string
	if m.done {
		statusText = fmt.Sprintf("Completed: %d  Skipped: %d  Failed: %d",
			m.completed-m.skipped-m.failed, m.skipped, m.failed)
	} else if m.currentOp != nil {
		statusText = fmt.Sprintf("Current: %s/%s/%s - %s",
			m.currentOp.Repository.Host,
			m.currentOp.Repository.Organization,
			m.currentOp.Repository.Name,
			m.currentOp.Status)
	} else {
		statusText = "Initializing..."
	}

	status := updateProgressStyle.Render(statusText)

	// Build log
	var logContent string
	if m.currentOp != nil {
		m.logMutex.Lock()
		logs := m.currentOp.Log
		if len(logs) > 10 {
			logs = logs[len(logs)-10:]
		}
		m.logMutex.Unlock()

		logContent = strings.Join(logs, "\n")
	} else {
		logContent = "No current operation"
	}

	log := updateLogStyle.Render(logContent)

	// Build footer
	var footer string
	if m.done {
		footer = updateFooterStyle.Render("Update completed. Press q to exit.")
	} else {
		footer = updateFooterStyle.Render("Press q to cancel")
	}

	// Combine all parts
	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s\n\n%s",
		header, progressBar, status, log, footer)
}
