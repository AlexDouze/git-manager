// cmd/github.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	githubOwner string
	rootDirFlag string
)

// Repository represents a GitHub repository
type Repository struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
	URL string `json:"url"`
}

var githubCmd = &cobra.Command{
	Use:   "github [owner]",
	Short: "List and clone GitHub repositories",
	Long: `List repositories from a GitHub organization or username,
select repositories from a filterable list, and clone the selected repositories.

Examples:
  # List repositories from a GitHub organization
  gitm github octocat

  # List repositories from a GitHub organization with a limit
  gitm github octocat --limit 10

  # List repositories from a GitHub organization and clone to a specific directory
  gitm github octocat --root-dir ~/projects`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Set owner from args if provided
		if len(args) > 0 {
			githubOwner = args[0]
		}

		// If no owner is provided, use the authenticated user
		if githubOwner == "" {
			fmt.Println("No owner provided. Using authenticated user.")
		}

		// Load configuration
		// cfg, err := config.LoadConfig()
		// if err != nil {
		// 	fmt.Printf("Failed to load configuration: %s\n", err)
		// 	return
		// }

		// Use specified root directory or default from config
		// targetDir := cfg.RootDirectory
		// if rootDirFlag != "" {
		// 	targetDir = rootDirFlag
		// }

		// List repositories using GitHub CLI
		repos, err := listGitHubRepositories(githubOwner)
		if err != nil {
			fmt.Printf("Failed to list repositories: %s\n", err)
			return
		}

		if len(repos) == 0 {
			fmt.Println("No repositories found.")
			return
		}

		for i, repo := range repos {
			fmt.Print(i+1, ". ", repo.Owner.Login, "/", repo.Name, "\n")
		}
	},
}

// listGitHubRepositories lists repositories from a GitHub organization or username
func listGitHubRepositories(owner string) ([]Repository, error) {
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
	var repos []Repository
	err = json.Unmarshal(output, &repos)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GitHub CLI output: %w", err)
	}

	return repos, nil
}

func init() {
	rootCmd.AddCommand(githubCmd)
	githubCmd.Flags().StringVar(&githubOwner, "owner", "", "GitHub organization or username")
	githubCmd.Flags().StringVar(&rootDirFlag, "root-dir", "", "Root directory for cloning repositories")
}
