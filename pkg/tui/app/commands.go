package app

import (
	"context"
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
