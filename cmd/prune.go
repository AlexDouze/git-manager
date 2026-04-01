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
	pruneFilters    FilterFlags
	pruneAllRepos   bool
	goneOnly        bool
	mergedOnly      bool
	execute         bool
	dryRun          bool // deprecated, kept for backward compatibility
	keepCurrent     bool
	noPruneCurrent  bool // deprecated, kept for backward compatibility
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune branches in git repositories",
	Long: `Prune branches in git repositories that match specified criteria.

You can specify to prune only branches with gone remotes, only merged branches, or both.
By default, this command operates in dry-run mode and only shows what would be deleted.
Use --execute to actually delete the branches.

By default, the current branch will be pruned if eligible (it will checkout the default branch first).
Use --keep-current to prevent pruning the current branch.

Examples:
  # Preview branches that would be pruned (dry run, default)
  gitm prune --all --gone-only

  # Preview merged branches that would be pruned
  gitm prune --all --merged-only

  # Actually prune branches with gone remotes
  gitm prune --all --gone-only --execute

  # Prune but keep the current branch
  gitm prune --all --gone-only --execute --keep-current

  # Limit pruning to a specific host/organization
  gitm prune --host github.com --org username --gone-only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if !pruneAllRepos && pruneFilters.Host == "" && pruneFilters.Org == "" && pruneFilters.Repo == "" && pruneFilters.Path == "" {
			return fmt.Errorf("at least one filter must be specified: --all, --host, --org, --repo, or --path")
		}

		if !goneOnly && !mergedOnly {
			return fmt.Errorf("at least one pruning criteria must be specified: --gone-only or --merged-only")
		}

		repositories, err := git.FindRepositories(cfg.RootDirectory, pruneFilters.Host, pruneFilters.Org, pruneFilters.Repo, pruneFilters.Path)
		if err != nil {
			return fmt.Errorf("failed to find repositories: %w", err)
		}

		if len(repositories) == 0 {
			fmt.Println("No repositories found matching the criteria.")
			return nil
		}

		// --dry-run=false is the legacy way to execute; honour it
		isDryRun := !execute && dryRun
		// --no-prune-current is the legacy way to keep current; honour it
		effectiveKeepCurrent := keepCurrent || noPruneCurrent

		return pruneRepositories(repositories, isDryRun, effectiveKeepCurrent)
	},
}

func pruneRepositories(repositories []*git.Repository, isDryRun bool, keepCurrentBranch bool) error {
	pruneResults := make(map[string]git.PruneResult)
	var mutex sync.Mutex
	var wg sync.WaitGroup

	prog := tui.NewProgress("Pruning branches", len(repositories))

	for _, repo := range repositories {
		wg.Add(1)
		go func(repo *git.Repository) {
			defer wg.Done()
			defer prog.Increment()

			result := git.PruneResult{Repository: repo}

			_, err := repo.Status()
			if err != nil {
				result.Error = fmt.Errorf("failed to get status: %w", err)
				mutex.Lock()
				pruneResults[repo.Path] = result
				mutex.Unlock()
				return
			}

			prunedBranches, err := repo.PruneBranches(goneOnly, mergedOnly, isDryRun, keepCurrentBranch)
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

	tui.RenderPruneResults(pruneResults, isDryRun)
	return nil
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneFilters.Register(pruneCmd)
	pruneCmd.Flags().BoolVar(&pruneAllRepos, "all", false, "Prune all repositories")

	pruneCmd.Flags().BoolVar(&goneOnly, "gone-only", false, "Prune only branches with gone remotes")
	pruneCmd.Flags().BoolVar(&mergedOnly, "merged-only", false, "Prune only branches that have been merged")

	pruneCmd.Flags().BoolVar(&execute, "execute", false, "Actually delete branches (default is dry-run)")

	// Deprecated flag kept for backward compatibility
	pruneCmd.Flags().BoolVar(&dryRun, "dry-run", true, "Show branches that would be pruned without actually pruning them")
	if err := pruneCmd.Flags().MarkHidden("dry-run"); err != nil {
		panic(err)
	}

	pruneCmd.Flags().BoolVar(&keepCurrent, "keep-current", false, "Do not prune the current branch even if eligible")

	// Deprecated flag kept for backward compatibility
	pruneCmd.Flags().BoolVar(&noPruneCurrent, "no-prune-current", false, "Disable pruning the current branch")
	if err := pruneCmd.Flags().MarkDeprecated("no-prune-current", "use --keep-current instead"); err != nil {
		panic(err)
	}
}
