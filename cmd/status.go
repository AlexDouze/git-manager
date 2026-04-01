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
		repositories, err := git.FindRepositories(cfg.RootDirectory, hostFilter, organizationFilter, repositoryFilter, pathFilter)
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

		// Process repositories in parallel, collecting results
		type repoStatus struct {
			status *git.RepositoryStatus
			warn   string
		}
		results := make([]repoStatus, 0, len(repositories))
		var mu sync.Mutex

		for _, repo := range repositories {
			go func(r *git.Repository) {
				defer wg.Done()

				// Fetch remote branches; warn but continue on failure
				if _, fetchErr := r.Update(true, false); fetchErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to fetch %s: %v\n", r.Path, fetchErr)
				}

				// Get repository status
				status, statusErr := r.Status()
				if statusErr != nil {
					mu.Lock()
					results = append(results, repoStatus{warn: fmt.Sprintf("Warning: failed to get status for %s: %v", r.Path, statusErr)})
					mu.Unlock()
					return
				}

				// Mark stale branches
				if markErr := r.MarkStaleBranches(status, threshold); markErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to check stale branches for %s: %v\n", r.Path, markErr)
				}

				mu.Lock()
				results = append(results, repoStatus{status: status})
				mu.Unlock()
			}(repo)
		}

		wg.Wait()

		// Render sequentially after all goroutines finish
		for _, r := range results {
			if r.warn != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), r.warn)
				continue
			}
			if r.status.HasIssues() || displayAll {
				tui.StatusRender(r.status)
			}
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
	statusCmd.Flags().BoolVar(&displayAll, "display-all", false, "Display all repositories")
	statusCmd.Flags().StringVar(&olderThan, "older-than", "1m", "Threshold for stale branch detection (e.g., 30d, 4w, 1m)")

}
