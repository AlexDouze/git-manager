package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GithubCommandExecutor defines an interface for executing GitHub CLI commands
type GithubCommandExecutor interface {
	Execute(ctx context.Context, args ...string) ([]byte, error)
}

// DefaultGithubCommandExecutor is the default implementation of GithubCommandExecutor
type DefaultGithubCommandExecutor struct{}

// Execute executes a GitHub CLI command with the given arguments.
// The command is bound to ctx, so cancelling ctx terminates the underlying gh process.
func (e *DefaultGithubCommandExecutor) Execute(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	return cmd.Output()
}

// Repository represents a GitHub repository
type githubRepository struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// ListGitHubRepositories lists repositories from a GitHub organization or username,
// returning at most limit repositories.
func ListGitHubRepositories(ctx context.Context, owner string, limit int) ([]Repository, error) {
	return ListGitHubRepositoriesWithExecutor(ctx, owner, limit, &DefaultGithubCommandExecutor{})
}

// ListGitHubRepositoriesWithExecutor lists repositories using a custom executor (useful for testing)
func ListGitHubRepositoriesWithExecutor(ctx context.Context, owner string, limit int, executor GithubCommandExecutor) ([]Repository, error) {
	// Prepare the GitHub CLI command
	args := []string{"repo", "list"}
	if owner != "" {
		args = append(args, owner)
	}
	args = append(args, "--json", "name,owner")
	args = append(args, "--limit", fmt.Sprintf("%d", limit))

	// Execute the GitHub CLI command
	output, err := executor.Execute(ctx, args...)
	if err != nil {
		// gh writes the useful diagnostic to stderr; surface it.
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("failed to execute GitHub CLI: %w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("failed to execute GitHub CLI: %w", err)
	}

	// Parse the JSON output
	var ghRepos []githubRepository
	err = json.Unmarshal(output, &ghRepos)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GitHub CLI output: %w", err)
	}

	var repos []Repository
	for _, repo := range ghRepos {
		repos = append(repos, Repository{
			Host:         "github.com",
			Organization: repo.Owner.Login,
			Name:         repo.Name,
		})
	}

	return repos, nil
}
