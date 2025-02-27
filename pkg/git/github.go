package git

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

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
	// Prepare the GitHub CLI command
	args := []string{"repo", "list"}
	if owner != "" {
		args = append(args, owner)
	}
	args = append(args, "--json", "name,owner,url")
	args = append(args, "--limit", fmt.Sprintf("%d", 1000))

	// Execute the GitHub CLI command
	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
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
