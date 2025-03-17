// cmd/update.go
package cmd

import (
	"fmt"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	fetchOnly bool
	prune     bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update repositories",
	Long: `Update git repositories by fetching and optionally pulling the latest changes.
Can also prune remote-tracking branches that no longer exist on the remote.`,
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

		// Process repositories sequentially
		for _, repo := range repositories {
			// Update repository (fetch and optionally pull)
			updateResult, updateErr := repo.Update(fetchOnly, prune)
			if updateErr != nil {
				fmt.Printf("Warning: failed to update %s: %v\n", repo.Path, updateErr)
			} else if updateResult != nil {
				if fetchOnly {
					fmt.Printf("=== %s/%s/%s ===\n", repo.Host, repo.Organization, repo.Name)
					fmt.Println("✅ Changes fetched successfully")
					fmt.Println()
				} else {
					tui.UpdateRender(updateResult)
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	// Add filter flags (reusing the same flags from status command)
	updateCmd.Flags().StringVar(&hostFilter, "host", "", "Filter repositories by host")
	updateCmd.Flags().StringVar(&organizationFilter, "org", "", "Filter repositories by organization/username")
	updateCmd.Flags().StringVar(&repositoryFilter, "repo", "", "Filter repositories by name")
	updateCmd.Flags().StringVar(&pathFilter, "path", "", "Filter repositories by path")
	updateCmd.Flags().BoolVar(&allRepositories, "all", false, "Update all repositories")

	// Add update-specific flags
	updateCmd.Flags().BoolVar(&fetchOnly, "fetch-only", false, "Only fetch changes without pulling")
	updateCmd.Flags().BoolVar(&prune, "prune", false, "Prune remote-tracking branches")
}
