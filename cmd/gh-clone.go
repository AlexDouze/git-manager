// cmd/gh-clone.go
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
	ghCloneOwner   string
	ghCloneRootDir string
)

var ghCloneCmd = &cobra.Command{
	Use:   "gh-clone [owner]",
	Short: "List and clone GitHub repositories",
	Long: `List repositories from a GitHub organization or username,
select repositories from a filterable list, and clone the selected repositories.

Examples:
  # List repositories from a GitHub organization
  gitm gh-clone alexdouze

  # List repositories from a GitHub organization with a limit
  gitm gh-clone alexdouze --limit 10

  # List repositories from a GitHub organization and clone to a specific directory
  gitm gh-clone alexdouze --root-dir ~/projects`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set owner from args if provided
		if len(args) > 0 {
			ghCloneOwner = args[0]
		}

		// If no owner is provided, use the authenticated user
		if ghCloneOwner == "" {
			fmt.Println("No owner provided. Using authenticated user.")
		}

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Use specified root directory or default from config
		targetDir := cfg.RootDirectory
		if ghCloneRootDir != "" {
			targetDir = ghCloneRootDir
		}

		// List repositories using GitHub CLI
		repos, err := git.ListGitHubRepositories(ghCloneOwner)
		if err != nil {
			return fmt.Errorf("failed to list repositories: %w", err)
		}

		if len(repos) == 0 {
			fmt.Println("No repositories found.")
			return nil
		}
		selectedRepos, err := tui.SelectGithubReposRender(repos)
		if err != nil {
			return fmt.Errorf("failed to select repositories: %w", err)
		}

		var cloneOptions []string
		if cfg.Clone.DefaultOptions != "" {
			cloneOptions = strings.Fields(cfg.Clone.DefaultOptions)
		}

		for _, repo := range selectedRepos {
			url := fmt.Sprintf("git@%s:%s/%s.git", repo.Host, repo.Organization, repo.Name)
			if err = repo.Clone(targetDir, url, cloneOptions); err != nil {
				fmt.Printf("❌ Error cloning repository %s: %s\n", repo.Name, err)
			} else {
				fmt.Printf("✅ Repository %s cloned successfully in %s\n", repo.Name, repo.Path)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ghCloneCmd)
	ghCloneCmd.Flags().StringVar(&ghCloneOwner, "owner", "", "GitHub organization or username")
	ghCloneCmd.Flags().StringVar(&ghCloneRootDir, "root-dir", "", "Root directory for cloning repositories")
}
