package git

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// GithubCommandExecutor defines an interface for executing GitHub CLI commands
type GithubCommandExecutor interface {
	Execute(args ...string) ([]byte, error)
}

// DefaultGithubCommandExecutor is the default implementation of GithubCommandExecutor
type DefaultGithubCommandExecutor struct{}

// Execute executes a GitHub CLI command with the given arguments
func (e *DefaultGithubCommandExecutor) Execute(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	return cmd.Output()
}

// Repository represents a GitHub repository
type githubRepository struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
	URL string `json:"url"`
}

// ListGitHubRepositories lists repositories from a GitHub organization or username
func ListGitHubRepositories(owner string) ([]Repository, error) {
	return ListGitHubRepositoriesWithExecutor(owner, &DefaultGithubCommandExecutor{})
}

// ListGitHubRepositoriesWithExecutor lists repositories using a custom executor (useful for testing)
func ListGitHubRepositoriesWithExecutor(owner string, executor GithubCommandExecutor) ([]Repository, error) {
	// Prepare the GitHub CLI command
	args := []string{"repo", "list"}
	if owner != "" {
		args = append(args, owner)
	}
	args = append(args, "--json", "name,owner,url")
	args = append(args, "--limit", fmt.Sprintf("%d", 1000))

	// Execute the GitHub CLI command
	output, err := executor.Execute(args...)
	if err != nil {
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
