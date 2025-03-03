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
	// Filter flags
	pruneHostFilter string
	pruneOrgFilter  string
	pruneRepoFilter string
	prunePathFilter string
	pruneAllRepos   bool

	// Prune-specific flags
	goneOnly   bool
	mergedOnly bool
	dryRun     bool
)

// pruneCmd represents the prune command
var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune branches in git repositories",
	Long: `Prune branches in git repositories that match specified criteria.

You can specify to prune only branches with gone remotes, only merged branches, or both.
By default, this command operates in dry-run mode, which shows what would be deleted.
Use the --no-dry-run flag to actually delete the branches.

Examples:
  # Show branches that would be pruned in all repositories
  gitm prune --all

  # Prune branches with gone remotes only (dry run)
  gitm prune --all --gone-only

  # Prune merged branches only (dry run)
  gitm prune --all --merged-only

  # Actually prune branches (no dry run)
  gitm prune --all --no-dry-run

  # Limit pruning to specific host/organization/repository
  gitm prune --host github.com --org username --repo repository`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// At least one filter must be specified
		if !pruneAllRepos && pruneHostFilter == "" && pruneOrgFilter == "" && pruneRepoFilter == "" && prunePathFilter == "" {
			return fmt.Errorf("at least one filter must be specified: --all, --host, --org, --repo, or --path")
		}

		// Must specify at least one pruning criteria
		if !goneOnly && !mergedOnly {
			return fmt.Errorf("at least one pruning criteria must be specified: --gone-only or --merged-only")
		}

		// Find repositories based on filters
		repositories, err := git.FindRepositories(cfg.RootDirectory, pruneHostFilter, pruneOrgFilter, pruneRepoFilter, prunePathFilter, pruneAllRepos)
		if err != nil {
			return fmt.Errorf("failed to find repositories: %w", err)
		}

		if len(repositories) == 0 {
			fmt.Println("No repositories found matching the criteria.")
			return nil
		}

		return pruneRepositories(repositories)
	},
}

func pruneRepositories(repositories []*git.Repository) error {
	pruneResults := make(map[string]git.PruneResult)
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Process repositories concurrently
	for _, repo := range repositories {
		wg.Add(1)
		go func(repo *git.Repository) {
			defer wg.Done()

			// Create a prune result object for this repository
			result := git.PruneResult{
				Repository: repo,
			}

			// Get status first to check for uncommitted changes
			_, err := repo.Status()
			if err != nil {
				result.Error = fmt.Errorf("failed to get status: %w", err)
				mutex.Lock()
				pruneResults[repo.Path] = result
				mutex.Unlock()
				return
			}

			// Prune branches
			prunedBranches, err := repo.PruneBranches(goneOnly, mergedOnly, dryRun)
			if err != nil {
				result.Error = fmt.Errorf("failed to prune branches: %w", err)
			} else {
				result.PrunedBranches = prunedBranches
			}

			mutex.Lock()
			pruneResults[repo.Path] = result
			mutex.Unlock()
		}(repo)
	}

	wg.Wait()

	// Display results using the terminal UI
	return tui.RenderPruneResults(pruneResults, dryRun)
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	// Add filter flags
	pruneCmd.Flags().StringVar(&pruneHostFilter, "host", "", "Filter repositories by host (e.g., github.com)")
	pruneCmd.Flags().StringVar(&pruneOrgFilter, "org", "", "Filter repositories by organization/username")
	pruneCmd.Flags().StringVar(&pruneRepoFilter, "repo", "", "Filter repositories by name")
	pruneCmd.Flags().StringVar(&prunePathFilter, "path", "", "Filter repositories by path")
	pruneCmd.Flags().BoolVar(&pruneAllRepos, "all", false, "Prune all repositories")

	// Add prune-specific flags
	pruneCmd.Flags().BoolVar(&goneOnly, "gone-only", false, "Prune only branches with gone remotes")
	pruneCmd.Flags().BoolVar(&mergedOnly, "merged-only", false, "Prune only branches that have been merged")
	pruneCmd.Flags().BoolVar(&dryRun, "dry-run", true, "Show branches that would be pruned without actually pruning them")
	// Note: no-dry-run will set dry-run to false when used
	pruneCmd.Flags().BoolVar(&dryRun, "no-dry-run", false, "Actually prune branches (overrides --dry-run)")
}
