// cmd/status.go
package cmd

import (
	"fmt"
	"sync"

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
	displayAll         bool
	olderThan          string
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

		// Parse stale threshold before processing repos; fail fast on invalid value
		threshold, err := git.ParseHumanDuration(olderThan)
		if err != nil {
			return fmt.Errorf("invalid --older-than value %q: %w", olderThan, err)
		}

		// Create a wait group to wait for all goroutines to complete
		var wg sync.WaitGroup
		wg.Add(len(repositories))

		// Process repositories in parallel
		for _, repo := range repositories {
			go func(r *git.Repository) {
				defer wg.Done()

				// Update repository (fetch remote branches)
				r.Update(true, false)

				// Get repository status
				status, statusErr := r.Status()
				if statusErr != nil {
					fmt.Printf("Warning: failed to get status for %s: %v\n", r.Path, statusErr)
					return
				}

				// Mark stale branches
				if markErr := r.MarkStaleBranches(status, threshold); markErr != nil {
					fmt.Printf("Warning: failed to check stale branches for %s: %v\n", r.Path, markErr)
				}

				if status.HasIssues() || displayAll {
					tui.StatusRender(status)
				}

			}(repo)
		}

		wg.Wait()
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
	statusCmd.Flags().BoolVar(&displayAll, "display-all", false, "Display all repositories")
	statusCmd.Flags().StringVar(&olderThan, "older-than", "1m", "Threshold for stale branch detection (e.g., 30d, 4w, 1m)")

}
