// pkg/git/repository.go
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// GitCommandExecutor defines an interface for executing git commands
type GitCommandExecutor interface {
	Execute(repoPath string, stdout bool, args ...string) ([]byte, error)
}

// DefaultGitCommandExecutor is the default implementation of GitCommandExecutor
type DefaultGitCommandExecutor struct{}

// Execute executes a git command with the given arguments
// If stdout is true, command output is connected to os.Stdout and os.Stderr
func (e *DefaultGitCommandExecutor) Execute(repoPath string, stdout bool, args ...string) ([]byte, error) {
	// Insert the repository path argument if provided
	if repoPath != "" {
		if len(args) > 0 {
			if args[0] == "clone" {
				args = append([]string{"-C", filepath.Dir(repoPath)}, args...)
			} else {
				args = append([]string{"-C", repoPath}, args...)
			}
		}
	}

	cmd := exec.Command("git", args...)

	if stdout {
		err := cmd.Run()
		return nil, err
	}

	return cmd.Output()
}

// Repository represents a Git repository with its metadata
type Repository struct {
	Host         string             // Host (e.g., github.com)
	Organization string             // Organization or user (e.g., octocat)
	Name         string             // Repository name
	Path         string             // Local filesystem path
	gitExecutor  GitCommandExecutor // Git command executor
}

// NewRepository creates a new Repository with default GitCommandExecutor
func NewRepository() *Repository {
	return &Repository{
		gitExecutor: &DefaultGitCommandExecutor{},
	}
}

// SetGitCommandExecutor sets a custom GitCommandExecutor (useful for testing)
func (r *Repository) SetGitCommandExecutor(executor GitCommandExecutor) {
	r.gitExecutor = executor
}

// execGitCommand is a helper method that uses the GitCommandExecutor
func (r *Repository) execGitCommand(stdout bool, args ...string) ([]byte, error) {
	// Initialize with default executor if not set
	if r.gitExecutor == nil {
		r.gitExecutor = &DefaultGitCommandExecutor{}
	}

	return r.gitExecutor.Execute(r.Path, stdout, args...)
}

// ParseURL parses a git URL and extracts host, organization, and repository name
func ParseURL(url string) (*Repository, error) {
	repo := NewRepository()

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
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, line := range lines {
		wg.Add(1)
		go func(line string) {
			defer wg.Done()
			branch := parseBranchInfo(line)
			if branch != nil {
				mu.Lock()
				defer mu.Unlock()
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
		}(line)
	}

	wg.Wait()
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

type UpdateResult struct {
	Repository          *Repository
	BranchUpdateResults map[string]BranchUpdateResult
	HasErrors           bool
}
type BranchUpdateResult struct {
	Branch *BranchInfo
	Err    error
}

// PruneResult represents the result of a branch pruning operation
type PruneResult struct {
	Repository     *Repository
	PrunedBranches []string
	Error          error
}

// Update updates the repository (fetch and optionally pull)
func (r *Repository) Update(fetchOnly, prune bool) (*UpdateResult, error) {
	// Save the current branch to restore it at the end
	originalBranch, err := r.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Fetch from all remotes
	fetchArgs := []string{"fetch"}
	if prune {
		fetchArgs = append(fetchArgs, "--prune")
	}

	_, err = r.execGitCommand(true, fetchArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}

	// Check if there are uncommitted changes
	status, err := r.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get repository status: %w", err)
	}

	if status.HasUncommittedChanges {
		return nil, errors.New("cannot update: repository has uncommitted changes")
	}

	results := make(map[string]BranchUpdateResult)
	hasError := false

	if !fetchOnly {
		output, err := r.execGitCommand(false, "branch", "-vv")
		if err != nil {
			return nil, err
		}

		// Parse branch output
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")

		// Process each branch sequentially (can't do parallel checkouts)
		for _, line := range lines {
			branch := parseBranchInfo(line)
			if branch != nil && !branch.NoRemoteTracking && !branch.RemoteGone {
				// Skip branches that are not behind
				if branch.Behind <= 0 {
					results[branch.Name] = BranchUpdateResult{
						Branch: branch,
						Err:    nil,
					}
					continue
				}
				
				var err error

				// Checkout the branch
				if branch.Name != originalBranch {
					err = r.Checkout(branch.Name)
					if err != nil {
						results[branch.Name] = BranchUpdateResult{
							Branch: branch,
							Err:    fmt.Errorf("failed to checkout branch: %w", err),
						}
						hasError = true
						continue
					}
				}

				// Pull changes for the branch
				_, err = r.execGitCommand(true, "pull", "--rebase")

				results[branch.Name] = BranchUpdateResult{
					Branch: branch,
					Err:    err,
				}

				if err != nil {
					hasError = true
				}
			}
		}

		// Restore the original branch
		if originalBranch != "" {
			err = r.Checkout(originalBranch)
			if err != nil {
				return &UpdateResult{
					Repository:          r,
					BranchUpdateResults: results,
					HasErrors:           true,
				}, fmt.Errorf("failed to restore original branch %s: %w", originalBranch, err)
			}
		}
	}

	return &UpdateResult{
		Repository:          r,
		BranchUpdateResults: results,
		HasErrors:           hasError,
	}, nil
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

func (s RepositoryStatus) HasIssues() bool {
	return s.HasUncommittedChanges || s.HasBranchesWithoutRemote || s.HasBranchesWithRemoteGone || s.HasBranchesBehindRemote
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

	// Extract branch name (first word)
	spaceIndex := strings.Index(line, " ")
	if spaceIndex == -1 {
		return nil
	}

	branch.Name = line[:spaceIndex]
	line = strings.TrimSpace(line[spaceIndex:])

	// Look for tracking information between square brackets
	trackingInfoFound := false
	startIdx := strings.Index(line, "[")
	endIdx := strings.Index(line, "]")

	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		trackingInfoFound = true
		trackInfo := line[startIdx+1 : endIdx]

		if strings.Contains(trackInfo, "gone") {
			branch.RemoteGone = true
		} else {
			// Extract remote tracking branch
			colonIndex := strings.Index(trackInfo, ":")
			if colonIndex != -1 {
				branch.RemoteTracking = strings.TrimSpace(trackInfo[:colonIndex])

				// Parse ahead/behind information
				statusInfo := trackInfo[colonIndex+1:]

				// Check for ahead
				aheadIdx := strings.Index(statusInfo, "ahead")
				if aheadIdx != -1 {
					fmt.Sscanf(statusInfo[aheadIdx:], "ahead %d", &branch.Ahead)
				}

				// Check for behind
				behindIdx := strings.Index(statusInfo, "behind")
				if behindIdx != -1 {
					fmt.Sscanf(statusInfo[behindIdx:], "behind %d", &branch.Behind)
				}
			} else {
				branch.RemoteTracking = strings.TrimSpace(trackInfo)
			}
		}
	}

	// If no tracking info was found
	if !trackingInfoFound && branch.RemoteTracking == "" {
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

// FilterRepositories filters repositories based on host, org, and repo
func FilterRepositories(repositories []*Repository, host, org, repo string) []*Repository {
	if host == "" && org == "" && repo == "" {
		return repositories
	}

	var filtered []*Repository

	for _, r := range repositories {
		if host != "" && r.Host != host {
			continue
		}

		if org != "" && r.Organization != org {
			continue
		}

		if repo != "" && r.Name != repo {
			continue
		}

		filtered = append(filtered, r)
	}

	return filtered
}

// IsGitRepo checks if a directory is a git repository
func IsGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

// CreateRepositoryFromPath creates a Repository object from a path
func CreateRepositoryFromPath(path string) (*Repository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Extract host, organization, and name from path
	// This is a simplified approach and may need to be adjusted
	parts := strings.Split(absPath, string(os.PathSeparator))

	// We need at least 3 parts for host/org/repo
	if len(parts) < 3 {
		repo := NewRepository()
		repo.Path = absPath
		repo.Name = filepath.Base(absPath)
		return repo, nil
	}

	// Try to extract host, org, repo from path
	name := parts[len(parts)-1]
	org := parts[len(parts)-2]
	host := parts[len(parts)-3]

	// Validate that host looks like a domain
	if !strings.Contains(host, ".") {
		repo := NewRepository()
		repo.Path = absPath
		repo.Name = name
		return repo, nil
	}

	repo := NewRepository()
	repo.Host = host
	repo.Organization = org
	repo.Name = name
	repo.Path = absPath
	return repo, nil
}

// FindRepositories finds repositories based on filters
func FindRepositories(rootDir, host, org, repo, path string, all bool) ([]*Repository, error) {
	var repositories []*Repository

	// If path is specified, only check that path
	if path != "" {
		// Check if path is a git repository
		if IsGitRepo(path) {
			repository, err := CreateRepositoryFromPath(path)
			if err != nil {
				return nil, err
			}
			repositories = append(repositories, repository)
		} else {
			// Check if path contains git repositories
			err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info.IsDir() && IsGitRepo(p) {
					repository, err := CreateRepositoryFromPath(p)
					if err != nil {
						return err
					}
					repositories = append(repositories, repository)
					return filepath.SkipDir
				}

				return nil
			})

			if err != nil {
				return nil, err
			}
		}

		return FilterRepositories(repositories, host, org, repo), nil
	}

	// Always walk through the rootDir from config
	err := filepath.Walk(rootDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && IsGitRepo(p) {
			repository, err := CreateRepositoryFromPath(p)
			if err != nil {
				return err
			}
			repositories = append(repositories, repository)
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return FilterRepositories(repositories, host, org, repo), nil
}
