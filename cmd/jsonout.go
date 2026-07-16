// cmd/jsonout.go
package cmd

import (
	"time"

	"github.com/alexDouze/gitm/pkg/git"
)

// The JSON DTOs below are deliberately decoupled from the pkg/git structs:
// RepositoryStatus embeds *Repository (which carries unexported fields and would
// serialize redundantly), and time.Duration marshals as an integer nanosecond
// count. Explicit DTOs keep the wire format stable and readable regardless of
// internal refactors, and turn per-repo failures into an "error" string field so
// scripts consuming the output can see them.

// branchJSON is the wire representation of a single branch.
type branchJSON struct {
	Name                 string     `json:"name"`
	Current              bool       `json:"current"`
	RemoteTracking       string     `json:"remoteTracking,omitempty"`
	NoRemoteTracking     bool       `json:"noRemoteTracking"`
	RemoteGone           bool       `json:"remoteGone"`
	Ahead                int        `json:"ahead"`
	Behind               int        `json:"behind"`
	LastCommitDate       *time.Time `json:"lastCommitDate,omitempty"`
	CommitsBehindDefault int        `json:"commitsBehindDefault"`
	Stale                bool       `json:"stale"`
	WorktreePath         string     `json:"worktreePath,omitempty"`
}

// statusJSON is the wire representation of a repository's status.
type statusJSON struct {
	Host                      string       `json:"host"`
	Organization              string       `json:"organization"`
	Name                      string       `json:"name"`
	Path                      string       `json:"path"`
	HasIssues                 bool         `json:"hasIssues"`
	HasUncommittedChanges     bool         `json:"hasUncommittedChanges"`
	UncommittedChanges        []string     `json:"uncommittedChanges,omitempty"`
	CurrentBranch             string       `json:"currentBranch,omitempty"`
	HasBranchesWithoutRemote  bool         `json:"hasBranchesWithoutRemote"`
	HasBranchesWithRemoteGone bool         `json:"hasBranchesWithRemoteGone"`
	HasBranchesBehindRemote   bool         `json:"hasBranchesBehindRemote"`
	HasStaleBranches          bool         `json:"hasStaleBranches"`
	StashCount                int          `json:"stashCount"`
	StaleBranchThresholdDays  float64      `json:"staleBranchThresholdDays"`
	Branches                  []branchJSON `json:"branches,omitempty"`
	Error                     string       `json:"error,omitempty"`
}

// skippedBranchJSON is the wire representation of a skipped prune candidate.
type skippedBranchJSON struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// pruneJSON is the wire representation of a repository's prune result.
type pruneJSON struct {
	Host            string              `json:"host"`
	Organization    string              `json:"organization"`
	Name            string              `json:"name"`
	Path            string              `json:"path"`
	PrunedBranches  []string            `json:"prunedBranches,omitempty"`
	SkippedBranches []skippedBranchJSON `json:"skippedBranches,omitempty"`
	Error           string              `json:"error,omitempty"`
}

// branchToJSON converts a git.BranchInfo to its wire representation.
func branchToJSON(b git.BranchInfo) branchJSON {
	bj := branchJSON{
		Name:                 b.Name,
		Current:              b.Current,
		RemoteTracking:       b.RemoteTracking,
		NoRemoteTracking:     b.NoRemoteTracking,
		RemoteGone:           b.RemoteGone,
		Ahead:                b.Ahead,
		Behind:               b.Behind,
		CommitsBehindDefault: b.CommitsBehindDefault,
		Stale:                b.Stale,
		WorktreePath:         b.WorktreePath,
	}
	if !b.LastCommitDate.IsZero() {
		d := b.LastCommitDate
		bj.LastCommitDate = &d
	}
	return bj
}

// statusToJSON converts a git.RepositoryStatus to its wire representation.
func statusToJSON(s *git.RepositoryStatus) statusJSON {
	sj := statusJSON{
		Host:                      s.Repository.Host,
		Organization:              s.Repository.Organization,
		Name:                      s.Repository.Name,
		Path:                      s.Repository.Path,
		HasIssues:                 s.HasIssues(),
		HasUncommittedChanges:     s.HasUncommittedChanges,
		UncommittedChanges:        s.UncommittedChanges,
		CurrentBranch:             s.CurrentBranch,
		HasBranchesWithoutRemote:  s.HasBranchesWithoutRemote,
		HasBranchesWithRemoteGone: s.HasBranchesWithRemoteGone,
		HasBranchesBehindRemote:   s.HasBranchesBehindRemote,
		HasStaleBranches:          s.HasStaleBranches,
		StashCount:                s.StashCount,
		StaleBranchThresholdDays:  s.StaleBranchThreshold.Hours() / 24,
	}
	for _, b := range s.Branches {
		sj.Branches = append(sj.Branches, branchToJSON(b))
	}
	return sj
}

// pruneToJSON converts a git.PruneResult to its wire representation.
func pruneToJSON(r git.PruneResult) pruneJSON {
	pj := pruneJSON{
		PrunedBranches: r.PrunedBranches,
	}
	if r.Repository != nil {
		pj.Host = r.Repository.Host
		pj.Organization = r.Repository.Organization
		pj.Name = r.Repository.Name
		pj.Path = r.Repository.Path
	}
	for _, s := range r.SkippedBranches {
		pj.SkippedBranches = append(pj.SkippedBranches, skippedBranchJSON{Name: s.Name, Reason: s.Reason})
	}
	if r.Error != nil {
		pj.Error = r.Error.Error()
	}
	return pj
}
