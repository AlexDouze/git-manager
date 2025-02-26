// cmd/status.go
package cmd

import (
	"fmt"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	hostFilter         string
	organizationFilter string
	repositoryFilter   string
	pathFilter         string
	allRepositories    bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of repositories",
	Long: `Check the status of git repositories, showing uncommitted changes, 
branch status, and other important information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Find repositories based on filters
		repositories, err := git.FindRepositories(cfg.RootDirectory, hostFilter, organizationFilter, repositoryFilter, pathFilter, allRepositories)
		if err != nil {
			return fmt.Errorf("failed to find repositories: %w", err)
		}

		if len(repositories) == 0 {
			fmt.Println("No repositories found matching the specified filters.")
			return nil
		}

		// Fetch all remote branches for each repository
		for _, repo := range repositories {
			if err := repo.Update(true, false); err != nil {
				fmt.Printf("Warning: failed to fetch remote branches for %s: %v\n", repo.Path, err)
			}
			status, err := repo.Status()
			if err != nil {
				fmt.Printf("Warning: failed to get status for %s: %v\n", repo.Path, err)
			}
			tui.StatusRender(status)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Add filter flags
	statusCmd.Flags().StringVar(&hostFilter, "host", "", "Filter repositories by host")
	statusCmd.Flags().StringVar(&organizationFilter, "org", "", "Filter repositories by organization/username")
	statusCmd.Flags().StringVar(&repositoryFilter, "repo", "", "Filter repositories by name")
	statusCmd.Flags().StringVar(&pathFilter, "path", "", "Filter repositories by path")
	statusCmd.Flags().BoolVar(&allRepositories, "all", false, "Check all repositories")
}
