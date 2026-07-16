// cmd/update.go
package cmd

import (
	"context"
	"fmt"

	"github.com/alexDouze/gitm/internal/workerpool"
	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	updateFilters FilterFlags
	fetchOnly     bool
	prune         bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update repositories",
	Long: `Update git repositories by fetching and optionally pulling the latest changes.
Can also prune remote-tracking branches that no longer exist on the remote.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		repositories, err := git.FindRepositories(cfg.RootDirectory, updateFilters.Host, updateFilters.Org, updateFilters.Repo, updateFilters.Path)
		if err != nil {
			return fmt.Errorf("failed to find repositories: %w", err)
		}

		if len(repositories) == 0 {
			fmt.Println("No repositories found matching the specified filters.")
			return nil
		}

		type result struct {
			repo         *git.Repository
			updateResult *git.UpdateResult
			err          error
		}

		prog := tui.NewProgress("Updating repositories", len(repositories))

		ctx := cmd.Context()

		// Each Update performs a fetch (and checkouts when pulling), so keep the
		// worker count low to avoid overwhelming the SSH agent and remote server.
		workers := min(workerpool.Default(), 4)

		results := workerpool.Map(ctx, repositories, workers, func(ctx context.Context, repo *git.Repository) result {
			defer prog.Increment()
			ur, err := repo.Update(ctx, fetchOnly, prune)
			return result{repo: repo, updateResult: ur, err: err}
		})

		for _, r := range results {
			if r.err != nil {
				tui.UpdateErrorRender(r.repo, r.err)
			} else if r.updateResult != nil {
				if fetchOnly {
					tui.UpdateFetchOnlyRender(r.repo)
				} else {
					tui.UpdateRender(r.updateResult)
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateFilters.Register(updateCmd)

	updateCmd.Flags().BoolVar(&fetchOnly, "fetch-only", false, "Only fetch changes without pulling")
	updateCmd.Flags().BoolVar(&prune, "prune", false, "Prune remote-tracking branches")
}
