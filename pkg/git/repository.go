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

type Repository struct {
	Host         string
	Organization string
	Name         string
	Path         string
}

// ParseURL parses a git URL and extracts host, organization, and repository name
func ParseURL(url string) (*Repository, error) {
	repo := &Repository{}

	// Handle SSH URLs (git@github.com:org/repo.git)
	if strings.HasPrefix(url, "git@") {
		parts := strings.Split(url, ":")
		if len(parts) != 2 {
			return nil, errors.New("invalid SSH git URL format")
		}

		repo.Host = strings.TrimPrefix(parts[0], "git@")
		pathParts := strings.Split(strings.TrimSuffix(parts[1], ".git"), "/")

		if len(pathParts) != 2 {
			return nil, errors.New("invalid repository path in SSH URL")
		}

		repo.Organization = pathParts[0]
		repo.Name = pathParts[1]
		return repo, nil
	}

	// Handle HTTPS URLs (https://github.com/org/repo.git)
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		url = strings.TrimSuffix(url, ".git")
		parts := strings.Split(strings.TrimPrefix(url, "https://"), "/")
		parts = append(parts, strings.Split(strings.TrimPrefix(url, "http://"), "/")...)

		if len(parts) < 3 {
			return nil, errors.New("invalid HTTPS git URL format")
		}

		repo.Host = parts[0]
		repo.Organization = parts[1]
		repo.Name = parts[2]
		return repo, nil
	}

	return nil, errors.New("unsupported git URL format")
}

// Clone clones a repository to the specified root directory
func (r *Repository) Clone(rootDir, url string, options []string) error {
	r.Path = filepath.Join(rootDir, r.Host, r.Organization, r.Name)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(r.Path), 0755); err != nil {
		return err
	}

	// Check if repository already exists
	if _, err := os.Stat(r.Path); err == nil {
		return fmt.Errorf("repository already exists at %s", r.Path)
	}

	// Prepare git clone command
	args := append([]string{"clone"}, options...)
	args = append(args, url, r.Path)

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
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
	cmd := exec.Command("git", "-C", r.Path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if len(output) > 0 {
		status.HasUncommittedChanges = true
		status.UncommittedChanges = strings.Split(strings.TrimSpace(string(output)), "\n")
	}

	// Get branch information
	cmd = exec.Command("git", "-C", r.Path, "branch", "-vv")
	output, err = cmd.Output()
	if err != nil {
		return nil, err
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

	// Check for stashes
	cmd = exec.Command("git", "-C", r.Path, "stash", "list")
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	stashes := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(stashes) > 0 && stashes[0] != "" {
		status.StashCount = len(stashes)
	}

	return status, nil
}

// Update updates the repository (fetch and optionally pull)
func (r *Repository) Update(fetchOnly, prune bool) error {
	// Fetch from all remotes
	args := []string{"-C", r.Path, "fetch"}
	if prune {
		args = append(args, "--prune")
	}

	cmd := exec.Command("git", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if !fetchOnly {
		// Check if there are uncommitted changes
		status, err := r.Status()
		if err != nil {
			return err
		}

		if status.HasUncommittedChanges {
			return errors.New("cannot update: repository has uncommitted changes")
		}

		// Pull changes for current branch
		cmd = exec.Command("git", "-C", r.Path, "pull", "--rebase")
		return cmd.Run()
	}

	return nil
}

// PruneBranches prunes branches based on criteria
func (r *Repository) PruneBranches(goneOnly, mergedOnly bool, dryRun bool) ([]string, error) {
	var branchesToPrune []string

	// Get branch information
	status, err := r.Status()
	if err != nil {
		return nil, err
	}

	for _, branch := range status.Branches {
		shouldPrune := false

		if goneOnly && branch.RemoteGone {
			shouldPrune = true
		}

		if mergedOnly {
			// Check if branch is merged
			cmd := exec.Command("git", "-C", r.Path, "branch", "--merged", "main")
			output, err := cmd.Output()
			if err == nil && strings.Contains(string(output), branch.Name) {
				shouldPrune = true
			}
		}

		if shouldPrune && !branch.Current {
			branchesToPrune = append(branchesToPrune, branch.Name)
		}
	}

	if !dryRun && len(branchesToPrune) > 0 {
		for _, branch := range branchesToPrune {
			cmd := exec.Command("git", "-C", r.Path, "branch", "-D", branch)
			if err := cmd.Run(); err != nil {
				return branchesToPrune, fmt.Errorf("failed to delete branch %s: %w", branch, err)
			}
		}
	}

	return branchesToPrune, nil
}

type BranchInfo struct {
	Name             string
	Current          bool
	RemoteTracking   string
	NoRemoteTracking bool
	RemoteGone       bool
	Ahead            int
	Behind           int
}

type RepositoryStatus struct {
	Repository                *Repository
	HasUncommittedChanges     bool
	UncommittedChanges        []string
	Branches                  []BranchInfo
	CurrentBranch             string
	HasBranchesWithoutRemote  bool
	HasBranchesWithRemoteGone bool
	HasBranchesBehindRemote   bool
	StashCount                int
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
	for _, part := range parts {
		if strings.HasPrefix(part, "[") && strings.Contains(part, "]") {
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
	if branch.RemoteTracking == "" {
		branch.NoRemoteTracking = true
	}

	return branch
}
