// cmd/status.go
package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/alexDouze/gitm/internal/workerpool"
	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	statusFilters FilterFlags
	displayAll    bool
	noFetch       bool
	olderThan     string
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
		repositories, err := git.FindRepositories(cfg.RootDirectory, statusFilters.Host, statusFilters.Org, statusFilters.Repo, statusFilters.Path)
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

		// Process repositories in parallel, collecting results
		type repoStatus struct {
			status    *git.RepositoryStatus
			warn      string
			fetchWarn string
			sortKey   string
		}

		prog := tui.NewProgress("Scanning repositories", len(repositories))

		// Limit concurrent SSH signing operations to avoid overwhelming the SSH agent
		// (especially hardware keys like YubiKey that handle signing sequentially).
		// The worker pool bounds total parallelism; this semaphore bounds the
		// number of concurrent fetches within it.
		fetchSem := make(chan struct{}, 4)

		ctx := cmd.Context()

		results := workerpool.Map(ctx, repositories, workerpool.Default(), func(ctx context.Context, r *git.Repository) repoStatus {
			defer prog.Increment()

			sortKey := fmt.Sprintf("%s/%s/%s", r.Host, r.Organization, r.Name)

			var fetchWarn string
			if !noFetch {
				fetchSem <- struct{}{}
				_, fetchErr := r.Update(ctx, true, false)
				<-fetchSem
				if fetchErr != nil {
					fetchWarn = fmt.Sprintf("Warning: failed to fetch %s: %v", r.Path, fetchErr)
				}
			}

			// Get repository status
			status, statusErr := r.Status(ctx)
			if statusErr != nil {
				return repoStatus{
					warn:      fmt.Sprintf("Warning: failed to get status for %s: %v", r.Path, statusErr),
					fetchWarn: fetchWarn,
					sortKey:   sortKey,
				}
			}

			// Mark stale branches
			if markErr := r.MarkStaleBranches(ctx, status, threshold); markErr != nil {
				fetchWarn += fmt.Sprintf("\nWarning: failed to check stale branches for %s: %v", r.Path, markErr)
			}

			return repoStatus{status: status, fetchWarn: fetchWarn, sortKey: sortKey}
		})

		// Sort results deterministically by host/org/name
		sort.Slice(results, func(i, j int) bool {
			return results[i].sortKey < results[j].sortKey
		})

		// Print fetch/scan warnings after progress clears, before the status output
		for _, r := range results {
			if r.fetchWarn != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), r.fetchWarn)
			}
		}

		// Render status results
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

	// Add filter flags (shared)
	statusFilters.Register(statusCmd)

	// Display flags
	statusCmd.Flags().BoolVar(&displayAll, "all", false, "Display all repositories including clean ones")
	statusCmd.Flags().BoolVar(&displayAll, "display-all", false, "Display all repositories")
	if err := statusCmd.Flags().MarkHidden("display-all"); err != nil {
		panic(err)
	}

	// Status-specific flags
	statusCmd.Flags().BoolVar(&noFetch, "no-fetch", false, "Skip fetching from remotes before checking status")
	statusCmd.Flags().StringVar(&olderThan, "older-than", "30d", "Threshold for stale branch detection (e.g., 30d, 4w, 3m where m=months)")
}
