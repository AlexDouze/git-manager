package app

import "github.com/alexDouze/gitm/pkg/git"

// reposLoadedMsg carries the result of loadReposCmd: the discovered repositories
// (identity + path only, no status yet) or the error that stopped the walk.
type reposLoadedMsg struct {
	repos []*git.Repository
	err   error
}

// repoStatusResult pairs a repository (by its unique local path) with the
// outcome of loading its status. err is non-nil when Status failed for that one
// repo; other repos in the batch are unaffected.
type repoStatusResult struct {
	path   string
	status *git.RepositoryStatus
	err    error
}

// statusesLoadedMsg carries the result of loadStatusesCmd — one entry per
// repository, in the same order as the input slice.
type statusesLoadedMsg struct {
	results []repoStatusResult
}

// branchesLoadedMsg carries the result of loadBranchesCmd: the branch list for
// the repository we drilled into, or the error that stopped it. path identifies
// the repo so a stale message (from a repo the user already navigated away from)
// can be ignored.
type branchesLoadedMsg struct {
	path     string
	branches []git.BranchInfo
	err      error
}
