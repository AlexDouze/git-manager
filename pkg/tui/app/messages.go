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

// opKind names the kind of action an opDoneMsg reports, so Update can decide
// what to refresh afterwards and the footer can describe what happened.
type opKind int

const (
	opCheckout opKind = iota
	opUpdate
	opDeleteBranch
)

// opDoneMsg reports the completion of a mutating git action (checkout, update,
// delete). path is the repo the action ran against; branch is the branch acted
// on when relevant. summary is a short human-readable result for the footer.
//
// notFullyMerged is set when a safe branch delete was refused because the branch
// was not fully merged; the app turns that into a force-delete confirm prompt
// rather than a plain error.
type opDoneMsg struct {
	kind           opKind
	path           string
	branch         string
	summary        string
	err            error
	notFullyMerged bool
}

// bulkResult pairs a repository (by path) with the outcome of a bulk action
// (update-all/prune-all) run against it.
type bulkResult struct {
	path string
	err  error
}

// bulkOpDoneMsg reports the completion of an all-repos action (update-all,
// prune-all). summary is precomputed for the footer (e.g. "updated 10/12 (2
// failed)") since the aggregate wording depends on the op kind.
type bulkOpDoneMsg struct {
	kind    opKind
	results []bulkResult
	summary string
}

// ghReposLoadedMsg carries the result of loadGitHubReposCmd: the repositories
// listed for an owner via `gh`, or the error that stopped the listing.
type ghReposLoadedMsg struct {
	repos []git.Repository
	err   error
}

// cloneResult pairs a repository name with the outcome of cloning it.
type cloneResult struct {
	name string
	path string
	err  error
}

// ghCloneDoneMsg carries the results of a clone batch, one entry per selected
// repository.
type ghCloneDoneMsg struct {
	results []cloneResult
}

// ghExitMsg asks the app to leave the GitHub browser. When the browser is
// embedded in the main app it returns to the repo list (and reloads, since new
// clones may have appeared); when run standalone it quits the program.
type ghExitMsg struct{}
