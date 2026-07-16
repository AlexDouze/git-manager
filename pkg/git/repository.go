// pkg/git/repository.go
package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitCommandExecutor defines an interface for executing git commands
type GitCommandExecutor interface {
	Execute(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error)
}

// DefaultGitCommandExecutor is the default implementation of GitCommandExecutor
type DefaultGitCommandExecutor struct{}

// Execute executes a git command with the given arguments.
// If stdout is true, the command streams output directly to the terminal (used for interactive commands like clone).
// If stdout is false, stdout and stderr are both captured; on error the output is included in the ExitError.
// The command is bound to ctx, so cancelling ctx terminates the underlying git process.
func (e *DefaultGitCommandExecutor) Execute(ctx context.Context, repoPath string, stdout bool, args ...string) ([]byte, error) {
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

	cmd := exec.CommandContext(ctx, "git", args...)

	if stdout {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		return nil, err
	}

	out, err := cmd.CombinedOutput()
	if err != nil && len(out) > 0 {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, err
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
func (r *Repository) execGitCommand(ctx context.Context, stdout bool, args ...string) ([]byte, error) {
	// Initialize with default executor if not set
	if r.gitExecutor == nil {
		r.gitExecutor = &DefaultGitCommandExecutor{}
	}

	return r.gitExecutor.Execute(ctx, r.Path, stdout, args...)
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
func (r *Repository) Clone(ctx context.Context, rootDir, url string, options []string) error {
	r.Path = filepath.Join(rootDir, r.Host, r.Organization, r.Name)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(r.Path), 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Check if repository already exists
	if _, err := os.Stat(r.Path); err == nil {
		return fmt.Errorf("repository already exists at %s", r.Path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check repository path: %w", err)
	}

	// Prepare git clone command
	args := append([]string{"clone"}, options...)
	args = append(args, url, r.Path)

	_, err := r.execGitCommand(ctx, true, args...)
	return err
}

// Status gets the status of the repository
func (r *Repository) Status(ctx context.Context) (*RepositoryStatus, error) {
	status := &RepositoryStatus{
		Repository: r,
	}

	// Check if path exists
	if _, err := os.Stat(r.Path); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository path does not exist: %s", r.Path)
	}

	// Get uncommitted changes
	if err := r.getUncommittedChanges(ctx, status); err != nil {
		return nil, fmt.Errorf("failed to get uncommitted changes: %w", err)
	}

	// Get branch information
	if err := r.getBranchInformation(ctx, status); err != nil {
		return nil, fmt.Errorf("failed to get branch information: %w", err)
	}

	// Check for stashes
	if err := r.getStashInformation(ctx, status); err != nil {
		return nil, fmt.Errorf("failed to get stash information: %w", err)
	}

	return status, nil
}

// getUncommittedChanges populates the uncommitted changes information
func (r *Repository) getUncommittedChanges(ctx context.Context, status *RepositoryStatus) error {
	output, err := r.execGitCommand(ctx, false, "status", "--porcelain")
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
func (r *Repository) getBranchInformation(ctx context.Context, status *RepositoryStatus) error {
	branches, err := r.ListBranches(ctx)
	if err != nil {
		return err
	}

	for _, branch := range branches {
		status.Branches = append(status.Branches, branch)

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

	return nil
}

// getStashInformation populates the stash information
func (r *Repository) getStashInformation(ctx context.Context, status *RepositoryStatus) error {
	output, err := r.execGitCommand(ctx, false, "stash", "list")
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
func (r *Repository) Update(ctx context.Context, fetchOnly, prune bool) (*UpdateResult, error) {
	// Save the current branch to restore it at the end
	originalBranch, err := r.GetCurrentBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	// Detached HEAD: GetCurrentBranch reports "HEAD". Resolve the commit SHA so
	// we can restore the exact commit after checking out branches to pull them.
	if originalBranch == "HEAD" {
		sha, err := r.execGitCommand(ctx, false, "rev-parse", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("failed to resolve detached HEAD: %w", err)
		}
		originalBranch = strings.TrimSpace(string(sha))
	}

	// Fetch from all remotes
	fetchArgs := []string{"fetch", "--all"}
	if prune {
		fetchArgs = append(fetchArgs, "--prune")
	}

	_, err = r.execGitCommand(ctx, false, fetchArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}

	results := make(map[string]BranchUpdateResult)
	hasError := false

	if !fetchOnly {
		// Check for uncommitted changes before pulling
		status, err := r.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository status: %w", err)
		}
		if status.HasUncommittedChanges {
			return nil, errors.New("cannot update: repository has uncommitted changes")
		}

		branches, err := r.ListBranches(ctx)
		if err != nil {
			return nil, err
		}

		// Process each branch sequentially (can't do parallel checkouts)
		for i := range branches {
			branch := branches[i]
			if !branch.NoRemoteTracking && !branch.RemoteGone {
				// Skip branches that are not behind
				if branch.Behind <= 0 {
					results[branch.Name] = BranchUpdateResult{
						Branch: &branch,
						Err:    nil,
					}
					continue
				}

				var err error

				// Checkout the branch
				if branch.Name != originalBranch {
					err = r.Checkout(ctx, branch.Name)
					if err != nil {
						results[branch.Name] = BranchUpdateResult{
							Branch: &branch,
							Err:    fmt.Errorf("failed to checkout branch: %w", err),
						}
						hasError = true
						continue
					}
				}

				// Pull changes for the branch
				_, err = r.execGitCommand(ctx, false, "pull", "--rebase")

				results[branch.Name] = BranchUpdateResult{
					Branch: &branch,
					Err:    err,
				}

				if err != nil {
					hasError = true
				}
			}
		}

		// Restore the original branch
		if originalBranch != "" {
			err = r.Checkout(ctx, originalBranch)
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
// By default, it will prune the current branch if its remote is gone by checking out the default branch first
// Set noPruneCurrent to true to disable pruning the current branch
func (r *Repository) PruneBranches(ctx context.Context, goneOnly, mergedOnly bool, dryRun bool, noPruneCurrent bool) ([]string, error) {
	var branchesToPrune []string

	// Get branch information
	status, err := r.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository status: %w", err)
	}

	// Determine which branches to prune (by default allow pruning current branch)
	branchesToPrune, err = r.identifyBranchesToPrune(ctx, status, goneOnly, mergedOnly, !noPruneCurrent)
	if err != nil {
		return nil, err
	}

	// Actually delete the branches if not a dry run
	if !dryRun && len(branchesToPrune) > 0 {
		// Check if we need to checkout a different branch first
		if !noPruneCurrent {
			currentBranch := status.CurrentBranch
			// Check if current branch is in the list to prune
			currentBranchToPrune := false
			for _, branch := range branchesToPrune {
				if branch == currentBranch {
					currentBranchToPrune = true
					break
				}
			}

			// If current branch needs to be pruned, checkout default branch first
			if currentBranchToPrune {
				defaultBranch, err := r.GetDefaultBranch(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to determine default branch: %w", err)
				}

				// Don't prune the default branch if it's the only branch we have
				if defaultBranch == currentBranch {
					return nil, fmt.Errorf("cannot prune current branch '%s' because it is also the default branch", currentBranch)
				}

				// Checkout the default branch
				err = r.Checkout(ctx, defaultBranch)
				if err != nil {
					return nil, fmt.Errorf("failed to checkout default branch '%s' before pruning current branch: %w", defaultBranch, err)
				}
			}
		}

		// Now delete the branches
		for _, branch := range branchesToPrune {
			_, err := r.execGitCommand(ctx, false, "branch", "-D", branch)
			if err != nil {
				return branchesToPrune, fmt.Errorf("failed to delete branch %s: %w", branch, err)
			}
		}
	}

	return branchesToPrune, nil
}

// GetDefaultBranch returns the default branch name (main or master)
func (r *Repository) GetDefaultBranch(ctx context.Context) (string, error) {
	// First check if main branch exists
	_, err := r.execGitCommand(ctx, false, "show-ref", "--verify", "--quiet", "refs/heads/main")
	if err == nil {
		return "main", nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return "", fmt.Errorf("failed to check for main branch: %w", err)
	}

	// Then check if master branch exists
	_, err = r.execGitCommand(ctx, false, "show-ref", "--verify", "--quiet", "refs/heads/master")
	if err == nil {
		return "master", nil
	}
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		return "", fmt.Errorf("failed to check for master branch: %w", err)
	}

	// If neither exists, return the current branch as a fallback
	return r.GetCurrentBranch(ctx)
}

// MarkStaleBranches marks branches as stale based on the given threshold.
// Commit dates come from ListBranches (already populated on status.Branches when
// Status was called), so this needs no extra git call to fetch dates. The default
// branch is excluded. For each stale branch, it also computes the number of commits
// it is behind the default branch.
func (r *Repository) MarkStaleBranches(ctx context.Context, status *RepositoryStatus, threshold time.Duration) error {
	status.StaleBranchThreshold = threshold

	defaultBranch, err := r.GetDefaultBranch(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default branch: %w", err)
	}
	cutoff := time.Now().Add(-threshold)

	for i, branch := range status.Branches {
		if branch.Name == defaultBranch {
			continue
		}
		if branch.LastCommitDate.IsZero() || !branch.LastCommitDate.Before(cutoff) {
			continue
		}
		status.HasStaleBranches = true
		status.Branches[i].Stale = true

		// Count commits behind the default branch
		out, err := r.execGitCommand(ctx, false, "rev-list", "--count", branch.Name+".."+defaultBranch)
		if err == nil {
			count := strings.TrimSpace(string(out))
			n := 0
			fmt.Sscanf(count, "%d", &n)
			status.Branches[i].CommitsBehindDefault = n
		}
	}

	return nil
}

// identifyBranchesToPrune determines which branches should be pruned based on criteria
func (r *Repository) identifyBranchesToPrune(ctx context.Context, status *RepositoryStatus, goneOnly, mergedOnly bool, pruneCurrent bool) ([]string, error) {
	var branchesToPrune []string

	// Get the default branch
	defaultBranch, err := r.GetDefaultBranch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to determine default branch: %w", err)
	}

	// Build merged branch set once (avoid calling git per branch)
	mergedBranchSet := make(map[string]bool)
	if mergedOnly {
		output, err := r.execGitCommand(ctx, false, "branch", "--merged", defaultBranch)
		if err == nil {
			for _, mb := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				mb = strings.TrimSpace(mb)
				mb = strings.TrimPrefix(mb, "* ")
				if mb != "" {
					mergedBranchSet[mb] = true
				}
			}
		}
	}

	for _, branch := range status.Branches {
		// Always skip default branch
		if branch.Name == defaultBranch {
			continue
		}

		// Skip current branch if pruneCurrent is false
		if branch.Current && !pruneCurrent {
			continue
		}

		shouldPrune := false

		// Check if remote is gone
		if goneOnly && branch.RemoteGone {
			shouldPrune = true
		}

		// Check if branch is merged
		if mergedOnly && mergedBranchSet[branch.Name] {
			shouldPrune = true
		}

		if shouldPrune {
			branchesToPrune = append(branchesToPrune, branch.Name)
		}
	}

	return branchesToPrune, nil
}

// BranchInfo contains information about a git branch
type BranchInfo struct {
	Name                 string    // Branch name
	Current              bool      // Whether this is the current branch
	RemoteTracking       string    // Remote tracking branch (e.g., "origin/main")
	NoRemoteTracking     bool      // Whether this branch has no remote tracking
	RemoteGone           bool      // Whether the remote tracking branch is gone
	Ahead                int       // Number of commits ahead of remote
	Behind               int       // Number of commits behind remote
	LastCommitDate       time.Time // Date of the last commit on this branch
	CommitsBehindDefault int       // Number of commits behind the default branch
	Stale                bool      // Whether this branch is considered stale
	WorktreePath         string    // Path of the worktree that has this branch checked out, if any
}

// RepositoryStatus contains the status information of a repository
type RepositoryStatus struct {
	Repository                *Repository   // Reference to the repository
	HasUncommittedChanges     bool          // Whether there are uncommitted changes
	UncommittedChanges        []string      // List of uncommitted changes
	Branches                  []BranchInfo  // List of branches
	CurrentBranch             string        // Name of the current branch
	HasBranchesWithoutRemote  bool          // Whether there are branches without remote tracking
	HasBranchesWithRemoteGone bool          // Whether there are branches with remote gone
	HasBranchesBehindRemote   bool          // Whether there are branches behind remote
	StashCount                int           // Number of stashes
	HasStaleBranches          bool          // Whether there are stale branches
	StaleBranchThreshold      time.Duration // Threshold used for stale detection
}

func (s RepositoryStatus) HasIssues() bool {
	return s.HasUncommittedChanges || s.HasBranchesWithoutRemote || s.HasBranchesWithRemoteGone || s.HasBranchesBehindRemote || s.HasStaleBranches
}

// branchRefFormat is the for-each-ref format used by ListBranches. Fields are
// separated by NUL (%00) because that byte is illegal in git refnames, so it can
// never collide with branch names, upstream names, or worktree paths (unlike ' '
// or '|', which are all legal in refnames). for-each-ref interpolates %00 into an
// actual NUL byte. Fields, in order:
//
//	refname:short, HEAD marker, upstream:short, upstream:track, committerdate, worktreepath
const branchRefFormat = "%(refname:short)%00%(HEAD)%00%(upstream:short)%00%(upstream:track)%00%(committerdate:iso8601-strict)%00%(worktreepath)"

// ListBranches returns information about every local branch using a single
// for-each-ref call. This replaces parsing "git branch -vv" porcelain, whose
// output is ambiguous: a commit subject containing brackets (e.g. "[fix] ..."
// or the word "gone") could be misread as tracking info and wrongly flag a
// branch as RemoteGone — a data-loss hazard when pruning. for-each-ref exposes
// each attribute in its own field, so no such ambiguity exists.
func (r *Repository) ListBranches(ctx context.Context) ([]BranchInfo, error) {
	output, err := r.execGitCommand(ctx, false, "for-each-ref",
		"--format="+branchRefFormat, "refs/heads/")
	if err != nil {
		return nil, err
	}

	var branches []BranchInfo
	for _, line := range strings.Split(string(output), "\n") {
		branch := parseBranchRefLine(line)
		if branch == nil {
			continue
		}
		branches = append(branches, *branch)
	}

	return branches, nil
}

// parseBranchRefLine parses a single NUL-separated line produced by
// branchRefFormat. It is pure and table-testable. Returns nil for blank or
// malformed lines.
func parseBranchRefLine(line string) *BranchInfo {
	if strings.TrimSpace(line) == "" {
		return nil
	}

	fields := strings.Split(line, "\x00")
	if len(fields) < 6 {
		return nil
	}

	name := fields[0]
	if name == "" {
		return nil
	}

	branch := &BranchInfo{
		Name:           name,
		Current:        fields[1] == "*",
		RemoteTracking: fields[2],
		WorktreePath:   fields[5],
	}

	// upstream:track is one of: "" (in sync), "[gone]", "[ahead N]",
	// "[behind M]", or "[ahead N, behind M]".
	track := fields[3]
	if strings.Contains(track, "gone") {
		branch.RemoteGone = true
	} else {
		if idx := strings.Index(track, "ahead "); idx != -1 {
			fmt.Sscanf(track[idx:], "ahead %d", &branch.Ahead)
		}
		if idx := strings.Index(track, "behind "); idx != -1 {
			fmt.Sscanf(track[idx:], "behind %d", &branch.Behind)
		}
	}

	// A branch with no configured upstream has an empty upstream:short field.
	if branch.RemoteTracking == "" {
		branch.NoRemoteTracking = true
	}

	// committerdate:iso8601-strict is RFC3339-compatible.
	if fields[4] != "" {
		if t, err := time.Parse(time.RFC3339, fields[4]); err == nil {
			branch.LastCommitDate = t
		}
	}

	return branch
}

// GetCurrentBranch gets the current branch name
func (r *Repository) GetCurrentBranch(ctx context.Context) (string, error) {
	output, err := r.execGitCommand(ctx, false, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// Checkout checks out a branch
func (r *Repository) Checkout(ctx context.Context, branchOrArgs ...string) error {
	args := append([]string{"checkout"}, branchOrArgs...)
	_, err := r.execGitCommand(ctx, false, args...)
	if err != nil {
		return fmt.Errorf("failed to checkout: %w", err)
	}
	return nil
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
func FindRepositories(rootDir, host, org, repo, path string) ([]*Repository, error) {
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
