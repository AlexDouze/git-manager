// pkg/git/repository.go
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repository represents a Git repository with its metadata
type Repository struct {
	Host         string // Host (e.g., github.com)
	Organization string // Organization or user (e.g., octocat)
	Name         string // Repository name
	Path         string // Local filesystem path
}

// ParseURL parses a git URL and extracts host, organization, and repository name
func ParseURL(url string) (*Repository, error) {
	repo := &Repository{}

	// Remove trailing .git if present
	url = strings.TrimSuffix(url, ".git")

	// Handle SSH URLs (git@github.com:org/repo)
	if strings.HasPrefix(url, "git@") {
		parts := strings.Split(url, ":")
		if len(parts) != 2 {
			return nil, errors.New("invalid SSH git URL format")
		}

		repo.Host = strings.TrimPrefix(parts[0], "git@")
		pathParts := strings.Split(parts[1], "/")

		if len(pathParts) < 2 {
			return nil, errors.New("invalid repository path in SSH URL")
		}

		repo.Organization = pathParts[0]
		repo.Name = pathParts[len(pathParts)-1]
		return repo, nil
	}

	// Handle HTTPS URLs (https://github.com/org/repo)
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		// Remove protocol prefix
		urlWithoutProtocol := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
		parts := strings.Split(urlWithoutProtocol, "/")

		if len(parts) < 3 {
			return nil, errors.New("invalid HTTPS git URL format")
		}

		repo.Host = parts[0]
		repo.Organization = parts[1]
		repo.Name = parts[len(parts)-1]
		return repo, nil
	}

	return nil, errors.New("unsupported git URL format")
}

// execGitCommand executes a git command with the given arguments
// If stdout is true, command output is connected to os.Stdout and os.Stderr
func (r *Repository) execGitCommand(stdout bool, args ...string) ([]byte, error) {
	// Insert the repository path argument if provided
	if r.Path != "" {
		if len(args) > 0 {
			if args[0] == "clone" {
				args = append([]string{"-C", filepath.Dir(r.Path)}, args...)
			} else {
				args = append([]string{"-C", r.Path}, args...)
			}
		}
	}

	cmd := exec.Command("git", args...)

	if stdout {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		return nil, err
	}

	return cmd.Output()
}

// Clone clones a repository to the specified root directory
func (r *Repository) Clone(rootDir, url string, options []string) error {
	r.Path = filepath.Join(rootDir, r.Host, r.Organization, r.Name)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(r.Path), 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Check if repository already exists
	if _, err := os.Stat(r.Path); err == nil {
		return fmt.Errorf("repository already exists at %s", r.Path)
	}

	// Prepare git clone command
	args := append([]string{"clone"}, options...)
	args = append(args, url, r.Path)

	_, err := r.execGitCommand(true, args...)
	return err
}

// Status gets the status of the repository
func (r *Repository) Status() (*RepositoryStatus, error) {
	status := &RepositoryStatus{
		Repository: r,
	}

	// Check if path exists
	if _, err := os.Stat(r.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", r.Path)
	}

	// Get uncommitted changes
	if err := r.getUncommittedChanges(status); err != nil {
		return nil, fmt.Errorf("failed to get uncommitted changes: %w", err)
	}

	// Get branch information
	if err := r.getBranchInformation(status); err != nil {
		return nil, fmt.Errorf("failed to get branch information: %w", err)
	}

	// Check for stashes
	if err := r.getStashInformation(status); err != nil {
		return nil, fmt.Errorf("failed to get stash information: %w", err)
	}

	return status, nil
}

// getUncommittedChanges populates the uncommitted changes information
func (r *Repository) getUncommittedChanges(status *RepositoryStatus) error {
	output, err := r.execGitCommand(false, "status", "--porcelain")
	if err != nil {
		return err
	}

	if len(output) > 0 {
		status.HasUncommittedChanges = true
		status.UncommittedChanges = strings.Split(strings.TrimSpace(string(output)), "\n")
	}

	return nil
}

// getBranchInformation populates the branch information
func (r *Repository) getBranchInformation(status *RepositoryStatus) error {
	output, err := r.execGitCommand(false, "branch", "-vv")
	if err != nil {
		return err
	}

	// Parse branch output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		branch := parseBranchInfo(line)
		if branch != nil {
			status.Branches = append(status.Branches, *branch)

			if branch.Current {
				status.CurrentBranch = branch.Name
			}

			if branch.RemoteGone {
				status.HasBranchesWithRemoteGone = true
			}

			if branch.NoRemoteTracking {
				status.HasBranchesWithoutRemote = true
			}

			if branch.Behind > 0 {
				status.HasBranchesBehindRemote = true
			}
		}
	}

	return nil
}

// getStashInformation populates the stash information
func (r *Repository) getStashInformation(status *RepositoryStatus) error {
	output, err := r.execGitCommand(false, "stash", "list")
	if err != nil {
		return err
	}

	stashes := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(stashes) > 0 && stashes[0] != "" {
		status.StashCount = len(stashes)
	}

	return nil
}

// Update updates the repository (fetch and optionally pull)
func (r *Repository) Update(fetchOnly, prune bool) error {
	// Fetch from all remotes
	fetchArgs := []string{"fetch"}
	if prune {
		fetchArgs = append(fetchArgs, "--prune")
	}

	_, err := r.execGitCommand(true, fetchArgs...)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if !fetchOnly {
		// Check if there are uncommitted changes
		status, err := r.Status()
		if err != nil {
			return fmt.Errorf("failed to get repository status: %w", err)
		}

		if status.HasUncommittedChanges {
			return errors.New("cannot update: repository has uncommitted changes")
		}

		// Pull changes for current branch
		_, err = r.execGitCommand(true, "pull", "--rebase")
		if err != nil {
			return fmt.Errorf("failed to pull: %w", err)
		}
	}

	return nil
}

// PruneBranches prunes branches based on criteria
func (r *Repository) PruneBranches(goneOnly, mergedOnly bool, dryRun bool) ([]string, error) {
	var branchesToPrune []string

	// Get branch information
	status, err := r.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository status: %w", err)
	}

	// Determine which branches to prune
	branchesToPrune, err = r.identifyBranchesToPrune(status, goneOnly, mergedOnly)
	if err != nil {
		return nil, err
	}

	// Actually delete the branches if not a dry run
	if !dryRun && len(branchesToPrune) > 0 {
		for _, branch := range branchesToPrune {
			_, err := r.execGitCommand(false, "branch", "-D", branch)
			if err != nil {
				return branchesToPrune, fmt.Errorf("failed to delete branch %s: %w", branch, err)
			}
		}
	}

	return branchesToPrune, nil
}

// GetDefaultBranch returns the default branch name (main or master)
func (r *Repository) GetDefaultBranch() (string, error) {
	// First check if main branch exists
	_, err := r.execGitCommand(false, "show-ref", "--verify", "--quiet", "refs/heads/main")
	if err == nil {
		return "main", nil
	}

	// Then check if master branch exists
	_, err = r.execGitCommand(false, "show-ref", "--verify", "--quiet", "refs/heads/master")
	if err == nil {
		return "master", nil
	}

	// If neither exists, return the current branch as a fallback
	return r.GetCurrentBranch()
}

// identifyBranchesToPrune determines which branches should be pruned based on criteria
func (r *Repository) identifyBranchesToPrune(status *RepositoryStatus, goneOnly, mergedOnly bool) ([]string, error) {
	var branchesToPrune []string

	// Get the default branch
	defaultBranch, err := r.GetDefaultBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to determine default branch: %w", err)
	}

	for _, branch := range status.Branches {
		// Skip current branch and default branch
		if branch.Current || branch.Name == defaultBranch {
			continue
		}

		shouldPrune := false

		// Check if remote is gone
		if goneOnly && branch.RemoteGone {
			shouldPrune = true
		}

		// Check if branch is merged
		if mergedOnly {
			// Use the already determined default branch
			// Check if branch is merged into the default branch
			output, err := r.execGitCommand(false, "branch", "--merged", defaultBranch)
			if err == nil {
				// Check if branch name is in the merged branches output
				mergedBranches := strings.Split(strings.TrimSpace(string(output)), "\n")
				for _, mb := range mergedBranches {
					// Remove leading spaces and asterisk for current branch
					mb = strings.TrimSpace(mb)
					mb = strings.TrimPrefix(mb, "* ")

					if mb == branch.Name {
						shouldPrune = true
						break
					}
				}
			}
		}

		if shouldPrune {
			branchesToPrune = append(branchesToPrune, branch.Name)
		}
	}

	return branchesToPrune, nil
}

// BranchInfo contains information about a git branch
type BranchInfo struct {
	Name             string // Branch name
	Current          bool   // Whether this is the current branch
	RemoteTracking   string // Remote tracking branch (e.g., "origin/main")
	NoRemoteTracking bool   // Whether this branch has no remote tracking
	RemoteGone       bool   // Whether the remote tracking branch is gone
	Ahead            int    // Number of commits ahead of remote
	Behind           int    // Number of commits behind remote
}

// RepositoryStatus contains the status information of a repository
type RepositoryStatus struct {
	Repository                *Repository  // Reference to the repository
	HasUncommittedChanges     bool         // Whether there are uncommitted changes
	UncommittedChanges        []string     // List of uncommitted changes
	Branches                  []BranchInfo // List of branches
	CurrentBranch             string       // Name of the current branch
	HasBranchesWithoutRemote  bool         // Whether there are branches without remote tracking
	HasBranchesWithRemoteGone bool         // Whether there are branches with remote gone
	HasBranchesBehindRemote   bool         // Whether there are branches behind remote
	StashCount                int          // Number of stashes
}

// parseBranchInfo parses a line from git branch -vv output
func parseBranchInfo(line string) *BranchInfo {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	branch := &BranchInfo{}

	// Check if this is the current branch
	if strings.HasPrefix(line, "* ") {
		branch.Current = true
		line = strings.TrimPrefix(line, "* ")
	} else {
		line = strings.TrimPrefix(line, "  ")
	}

	// Parse branch name and tracking info
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil
	}

	branch.Name = parts[0]

	// Check for tracking information
	trackingInfoFound := false
	for _, part := range parts {
		if strings.HasPrefix(part, "[") && strings.Contains(part, "]") {
			trackingInfoFound = true
			trackInfo := strings.Trim(part, "[]")

			if strings.Contains(trackInfo, "gone") {
				branch.RemoteGone = true
				continue
			}

			// Extract remote tracking branch
			remoteParts := strings.Split(trackInfo, ":")
			branch.RemoteTracking = remoteParts[0]

			// Check for ahead/behind
			if len(remoteParts) > 1 {
				statusParts := strings.Fields(remoteParts[1])
				for _, status := range statusParts {
					if strings.HasPrefix(status, "ahead") {
						fmt.Sscanf(status, "ahead %d", &branch.Ahead)
					} else if strings.HasPrefix(status, "behind") {
						fmt.Sscanf(status, "behind %d", &branch.Behind)
					}
				}
			}

			break
		}
	}

	// If no tracking info was found
	if !trackingInfoFound {
		branch.NoRemoteTracking = true
	}

	return branch
}

// Fetch fetches from remotes
func (r *Repository) Fetch(args []string) error {
	fetchArgs := append([]string{"fetch"}, args...)
	_, err := r.execGitCommand(true, fetchArgs...)
	return err
}

// GetCurrentBranch gets the current branch name
func (r *Repository) GetCurrentBranch() (string, error) {
	output, err := r.execGitCommand(false, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// Checkout checks out a branch
func (r *Repository) Checkout(branchOrArgs ...string) error {
	args := append([]string{"checkout"}, branchOrArgs...)
	_, err := r.execGitCommand(true, args...)
	if err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}
	return nil
}

// Pull pulls changes
func (r *Repository) Pull(args []string) error {
	pullArgs := append([]string{"pull"}, args...)
	_, err := r.execGitCommand(true, pullArgs...)
	if err != nil {
		return fmt.Errorf("failed to pull: %w", err)
	}
	return nil
}

// GetRemoteBranches gets a list of remote branches
func (r *Repository) GetRemoteBranches() ([]string, error) {
	output, err := r.execGitCommand(false, "branch", "-r")
	if err != nil {
		return nil, fmt.Errorf("failed to get remote branches: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var branches []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip HEAD entry
		if strings.Contains(line, "HEAD") {
			continue
		}

		// Extract branch name from "origin/branch-name"
		parts := strings.SplitN(line, "/", 2)
		if len(parts) == 2 {
			branches = append(branches, parts[1])
		}
	}

	return branches, nil
}
