package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/alexDouze/gitm/internal/workerpool"
	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	pruneFilters   FilterFlags
	pruneAllRepos  bool
	goneOnly       bool
	mergedOnly     bool
	execute        bool
	dryRun         bool // deprecated, kept for backward compatibility
	keepCurrent    bool
	noPruneCurrent bool // deprecated, kept for backward compatibility
	forceDelete    bool
	pruneJSONOut   bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune branches in git repositories",
	Long: `Prune branches in git repositories that match specified criteria.

You can specify to prune only branches with gone remotes, only merged branches, or both.
By default, this command operates in dry-run mode and only shows what would be deleted.
Use --execute to actually delete the branches.

Branches are deleted with the safe "git branch -d", which refuses branches that are
not fully merged; those are reported as skipped. Use --force to delete them anyway
(equivalent to "git branch -D"). Branches checked out in a linked worktree are always skipped.

By default, the current branch will be pruned if eligible (it will checkout the default branch first).
Use --keep-current to prevent pruning the current branch.

Examples:
  # Preview branches that would be pruned (dry run, default)
  gitm prune --all --gone-only

  # Preview merged branches that would be pruned
  gitm prune --all --merged-only

  # Actually prune branches with gone remotes
  gitm prune --all --gone-only --execute

  # Force-delete even branches that are not fully merged
  gitm prune --all --gone-only --execute --force

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
			// Keep stdout clean for JSON consumers; the notice goes to stderr.
			fmt.Fprintln(cmd.ErrOrStderr(), "No repositories found matching the criteria.")
			return nil
		}

		// --dry-run=false is the legacy way to execute; honour it
		isDryRun := !execute && dryRun
		// --no-prune-current is the legacy way to keep current; honour it
		effectiveKeepCurrent := keepCurrent || noPruneCurrent

		opts := git.PruneOptions{
			GoneOnly:    goneOnly,
			MergedOnly:  mergedOnly,
			DryRun:      isDryRun,
			KeepCurrent: effectiveKeepCurrent,
			Force:       forceDelete,
		}

		results := pruneRepositories(cmd.Context(), repositories, opts)

		if pruneJSONOut {
			// --json keeps stdout clean: per-repo failures become an "error"
			// field rather than TUI warnings.
			out := make([]pruneJSON, 0, len(results))
			for _, r := range results {
				out = append(out, pruneToJSON(r))
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}

		pruneResults := make(map[string]git.PruneResult, len(results))
		for _, r := range results {
			if r.Repository != nil {
				pruneResults[r.Repository.Path] = r
			}
		}
		tui.RenderPruneResults(pruneResults, opts.DryRun)
		return nil
	},
}

// pruneRepositories prunes each repository in parallel and returns the results
// in the same order as the input slice.
func pruneRepositories(ctx context.Context, repositories []*git.Repository, opts git.PruneOptions) []git.PruneResult {
	prog := tui.NewProgress("Pruning branches", len(repositories))

	return workerpool.Map(ctx, repositories, workerpool.Default(), func(ctx context.Context, repo *git.Repository) git.PruneResult {
		defer prog.Increment()

		// PruneBranches calls Status() itself, so there's no need for a
		// separate status probe here.
		result, err := repo.PruneBranches(ctx, opts)
		if result == nil {
			result = &git.PruneResult{Repository: repo}
		}
		if err != nil {
			result.Error = fmt.Errorf("failed to prune branches: %w", err)
		}
		return *result
	})
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	pruneFilters.Register(pruneCmd)
	pruneCmd.Flags().BoolVar(&pruneAllRepos, "all", false, "Prune all repositories")

	pruneCmd.Flags().BoolVar(&goneOnly, "gone-only", false, "Prune only branches with gone remotes")
	pruneCmd.Flags().BoolVar(&mergedOnly, "merged-only", false, "Prune only branches that have been merged")

	pruneCmd.Flags().BoolVar(&execute, "execute", false, "Actually delete branches (default is dry-run)")

	pruneCmd.Flags().BoolVar(&forceDelete, "force", false, "Force-delete branches that are not fully merged (git branch -D)")

	pruneCmd.Flags().BoolVar(&pruneJSONOut, "json", false, "Output results as JSON")

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
