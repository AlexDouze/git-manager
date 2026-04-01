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

		results := make([]result, 0, len(repositories))
		prog := tui.NewProgress("Updating repositories", len(repositories))

		for _, repo := range repositories {
			ur, err := repo.Update(fetchOnly, prune)
			prog.Increment()
			results = append(results, result{repo: repo, updateResult: ur, err: err})
		}

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
