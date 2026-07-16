package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/alexDouze/gitm/internal/workerpool"
	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
)

// staleThreshold matches the `status` command's default (--older-than 30d) so
// the TUI's stale badges agree with the CLI.
const staleThreshold = 30 * 24 * time.Hour

// loadReposCmd finds repositories matching the filter. This is fast (a
// filesystem walk, no git subprocesses) so it runs as the initial command.
func loadReposCmd(cfg *config.Config, f Filter) tea.Cmd {
	return func() tea.Msg {
		repos, err := git.FindRepositories(cfg.RootDirectory, f.Host, f.Org, f.Repo, f.Path)
		return reposLoadedMsg{repos: repos, err: err}
	}
}

// loadStatusesCmd computes each repository's status in parallel via the worker
// pool. It never fetches from remotes (that is the explicit `u`/update action);
// refresh is a local-only re-read so it stays fast and offline-safe. Stale
// branches are marked with the same threshold the CLI uses.
func loadStatusesCmd(ctx context.Context, repos []*git.Repository) tea.Cmd {
	return func() tea.Msg {
		results := workerpool.Map(ctx, repos, workerpool.Default(), func(ctx context.Context, r *git.Repository) repoStatusResult {
			status, err := r.Status(ctx)
			if err != nil {
				return repoStatusResult{path: r.Path, err: err}
			}
			// Stale marking is best-effort; a failure here shouldn't discard the
			// otherwise-valid status.
			_ = r.MarkStaleBranches(ctx, status, staleThreshold)
			return repoStatusResult{path: r.Path, status: status}
		})
		return statusesLoadedMsg{results: results}
	}
}

// loadBranchesCmd lists a single repository's branches for the drill-in screen.
// It reuses Status (rather than the bare ListBranches) so the branch rows carry
// the same stale marking the repo-list badges use.
func loadBranchesCmd(ctx context.Context, r *git.Repository) tea.Cmd {
	return func() tea.Msg {
		status, err := r.Status(ctx)
		if err != nil {
			return branchesLoadedMsg{path: r.Path, err: err}
		}
		_ = r.MarkStaleBranches(ctx, status, staleThreshold)
		return branchesLoadedMsg{path: r.Path, branches: status.Branches}
	}
}

// checkoutCmd checks out a branch in the given repository.
func checkoutCmd(ctx context.Context, r *git.Repository, branch string) tea.Cmd {
	return func() tea.Msg {
		err := r.Checkout(ctx, branch)
		msg := opDoneMsg{kind: opCheckout, path: r.Path, branch: branch, err: err}
		if err == nil {
			msg.summary = "checked out " + branch
		} else {
			msg.summary = "checkout failed: " + err.Error()
		}
		return msg
	}
}

// updateCmd fetches and pulls (rebase) the given repository, mirroring the
// `gitm update` action for a single repo.
func updateCmd(ctx context.Context, r *git.Repository) tea.Cmd {
	return func() tea.Msg {
		_, err := r.Update(ctx, false, false)
		msg := opDoneMsg{kind: opUpdate, path: r.Path, err: err}
		if err == nil {
			msg.summary = "updated " + r.Name
		} else {
			msg.summary = "update failed: " + err.Error()
		}
		return msg
	}
}

// deleteBranchCmd deletes a branch. With force=false a "not fully merged"
// refusal is reported via opDoneMsg.notFullyMerged (not err) so the app can
// offer a force-delete confirm instead of surfacing a raw error.
func deleteBranchCmd(ctx context.Context, r *git.Repository, branch string, force bool) tea.Cmd {
	return func() tea.Msg {
		err := r.DeleteBranch(ctx, branch, force)
		msg := opDoneMsg{kind: opDeleteBranch, path: r.Path, branch: branch}
		switch {
		case err == nil:
			msg.summary = "deleted " + branch
		case errors.Is(err, git.ErrBranchNotFullyMerged):
			msg.notFullyMerged = true
			msg.summary = branch + " is not fully merged"
		default:
			msg.err = err
			msg.summary = "delete failed: " + err.Error()
		}
		return msg
	}
}

// pruneGoneCmd prunes the given repository's branches whose upstream is gone. A
// gone upstream is treated as intent to delete, so this force-deletes (`-D`)
// even branches with unmerged local commits; only branches checked out in a
// worktree are still reported as skipped.
func pruneGoneCmd(ctx context.Context, r *git.Repository) tea.Cmd {
	return func() tea.Msg {
		result, err := r.PruneBranches(ctx, git.PruneOptions{GoneOnly: true, Force: true})
		msg := opDoneMsg{kind: opDeleteBranch, path: r.Path, err: err}
		switch {
		case err != nil:
			msg.summary = "prune failed: " + err.Error()
		case result == nil || (len(result.PrunedBranches) == 0 && len(result.SkippedBranches) == 0):
			msg.summary = "no gone branches to prune"
		default:
			msg.summary = fmt.Sprintf("pruned %d, skipped %d", len(result.PrunedBranches), len(result.SkippedBranches))
		}
		return msg
	}
}

// updateAllCmd fetches and pulls (rebase) every given repository in parallel,
// mirroring the `gitm update --all` action from the repo-list screen.
func updateAllCmd(ctx context.Context, repos []*git.Repository) tea.Cmd {
	return func() tea.Msg {
		results := workerpool.Map(ctx, repos, workerpool.Default(), func(ctx context.Context, r *git.Repository) bulkResult {
			_, err := r.Update(ctx, false, false)
			return bulkResult{path: r.Path, err: err}
		})
		return bulkOpDoneMsg{kind: opUpdate, results: results, summary: summarizeBulk("updated", results)}
	}
}

// pruneAllCmd prunes gone branches across every given repository in parallel,
// using the same force delete as pruneGoneCmd for each one.
func pruneAllCmd(ctx context.Context, repos []*git.Repository) tea.Cmd {
	return func() tea.Msg {
		results := workerpool.Map(ctx, repos, workerpool.Default(), func(ctx context.Context, r *git.Repository) bulkResult {
			_, err := r.PruneBranches(ctx, git.PruneOptions{GoneOnly: true, Force: true})
			return bulkResult{path: r.Path, err: err}
		})
		return bulkOpDoneMsg{kind: opDeleteBranch, results: results, summary: summarizeBulk("pruned", results)}
	}
}

// summarizeBulk turns a slice of per-repo results into a short footer summary,
// e.g. "updated 10/12 repositories (2 failed)".
func summarizeBulk(verb string, results []bulkResult) string {
	failed := 0
	for _, r := range results {
		if r.err != nil {
			failed++
		}
	}
	total := len(results)
	ok := total - failed
	summary := fmt.Sprintf("%s %d/%d repositories", verb, ok, total)
	if failed > 0 {
		summary += fmt.Sprintf(" (%d failed)", failed)
	}
	return summary
}

// loadGitHubReposCmd lists an owner's repositories via `gh` for the clone
// browser. An empty owner lists the authenticated user's repos.
func loadGitHubReposCmd(ctx context.Context, owner string, limit int) tea.Cmd {
	return func() tea.Msg {
		repos, err := git.ListGitHubRepositories(ctx, owner, limit)
		return ghReposLoadedMsg{repos: repos, err: err}
	}
}

// cloneReposCmd clones the selected repositories in parallel into
// rootDir/host/org/name, applying the configured default clone options. Each
// repo's outcome is reported independently so one failure doesn't hide the rest.
func cloneReposCmd(ctx context.Context, repos []git.Repository, rootDir string, options []string) tea.Cmd {
	return func() tea.Msg {
		results := workerpool.Map(ctx, repos, workerpool.Default(), func(ctx context.Context, repo git.Repository) cloneResult {
			url := fmt.Sprintf("git@%s:%s/%s.git", repo.Host, repo.Organization, repo.Name)
			err := repo.Clone(ctx, rootDir, url, options)
			return cloneResult{name: repo.Name, path: repo.Path, err: err}
		})
		return ghCloneDoneMsg{results: results}
	}
}
