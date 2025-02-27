// cmd/github.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	githubOwner string
	rootDirFlag string
)

var githubCmd = &cobra.Command{
	Use:   "github [owner]",
	Short: "List and clone GitHub repositories",
	Long: `List repositories from a GitHub organization or username,
select repositories from a filterable list, and clone the selected repositories.

Examples:
  # List repositories from a GitHub organization
  gitm github alexdouze

  # List repositories from a GitHub organization with a limit
  gitm github alexdouze --limit 10

  # List repositories from a GitHub organization and clone to a specific directory
  gitm github alexdouze --root-dir ~/projects`,
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
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Failed to load configuration: %s\n", err)
			return
		}

		// Use specified root directory or default from config
		targetDir := cfg.RootDirectory
		if rootDirFlag != "" {
			targetDir = rootDirFlag
		}

		// List repositories using GitHub CLI
		repos, err := git.ListGitHubRepositories(githubOwner)
		if err != nil {
			fmt.Printf("Failed to list repositories: %s\n", err)
			return
		}

		if len(repos) == 0 {
			fmt.Println("No repositories found.")
			return
		}
		selectedRepos, err := tui.SelectGithubReposRender(repos)
		if err != nil {
			fmt.Printf("Failed to select repositories: %s\n", err)
			return
		}

		var cloneOptions []string
		if cfg.Clone.DefaultOptions != "" {
			cloneOptions = strings.Fields(cfg.Clone.DefaultOptions)
		}

		for _, repo := range selectedRepos {
			url := fmt.Sprintf("git@github.com:%s/%s.git", repo.Organization, repo.Name)
			err = repo.Clone(targetDir, url, cloneOptions)
			if err != nil {
				fmt.Printf("❌ Error cloning repository %s: %s", repo.Name, err)
			} else {
				fmt.Printf("✅ Repository %s cloned successfully in %s\n", repo.Name, repo.Path)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(githubCmd)
	githubCmd.Flags().StringVar(&githubOwner, "owner", "", "GitHub organization or username")
	githubCmd.Flags().StringVar(&rootDirFlag, "root-dir", "", "Root directory for cloning repositories")
}
