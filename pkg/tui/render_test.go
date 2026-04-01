package tui

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexDouze/gitm/pkg/git"
	"github.com/fatih/color"
)

// captureStdout redirects os.Stdout and color.Output for the duration of f and
// returns what was written. Colors are disabled for predictable text matching.
func captureStdout(f func()) string {
	origStdout := os.Stdout
	origColorOutput := color.Output
	origNoColor := color.NoColor

	r, w, _ := os.Pipe()
	os.Stdout = w
	color.Output = w
	color.NoColor = true

	f()

	w.Close()
	os.Stdout = origStdout
	color.Output = origColorOutput
	color.NoColor = origNoColor

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestGetStaleBranchesDisplay(t *testing.T) {
	now := time.Now()
	old := now.Add(-60 * 24 * time.Hour)
	recent := now.Add(-5 * 24 * time.Hour)
	threshold := 30 * 24 * time.Hour

	tests := []struct {
		name     string
		branches []git.BranchInfo
		contains []string
		excludes []string
	}{
		{
			name: "stale branch with remote gone",
			branches: []git.BranchInfo{
				{Name: "old-feature", LastCommitDate: old, RemoteGone: true, CommitsBehindDefault: 5},
			},
			contains: []string{"old-feature", "remote gone", "5 behind"},
		},
		{
			name: "stale branch with no remote",
			branches: []git.BranchInfo{
				{Name: "local", LastCommitDate: old, NoRemoteTracking: true, CommitsBehindDefault: 2},
			},
			contains: []string{"local", "no remote", "2 behind"},
		},
		{
			name: "stale branch with remote tracking",
			branches: []git.BranchInfo{
				{Name: "tracked", LastCommitDate: old, RemoteTracking: "origin/tracked", CommitsBehindDefault: 1},
			},
			contains: []string{"tracked", "has remote", "1 behind"},
		},
		{
			name: "recent branch not included",
			branches: []git.BranchInfo{
				{Name: "fresh", LastCommitDate: recent},
			},
			contains: []string{},
			excludes: []string{"fresh"},
		},
		{
			name: "branch with zero date not included",
			branches: []git.BranchInfo{
				{Name: "no-date"},
			},
			contains: []string{},
			excludes: []string{"no-date"},
		},
		{
			name: "current branch marked with asterisk",
			branches: []git.BranchInfo{
				{Name: "stale-current", LastCommitDate: old, Current: true, CommitsBehindDefault: 3},
			},
			contains: []string{"*stale-current"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &git.RepositoryStatus{
				StaleBranchThreshold: threshold,
				Branches:             tt.branches,
			}
			result := getStaleBranchesDisplay(status)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("getStaleBranchesDisplay() = %q, want it to contain %q", result, want)
				}
			}
			for _, absent := range tt.excludes {
				if strings.Contains(result, absent) {
					t.Errorf("getStaleBranchesDisplay() = %q, should not contain %q", result, absent)
				}
			}
		})
	}
}

func TestStatusRender(t *testing.T) {
	repo := &git.Repository{Host: "github.com", Organization: "acme", Name: "api"}

	t.Run("clean repository", func(t *testing.T) {
		status := &git.RepositoryStatus{Repository: repo}
		out := captureStdout(func() { StatusRender(status) })
		if !strings.Contains(out, "✅ Repository is clean") {
			t.Errorf("StatusRender() output = %q, want clean message", out)
		}
		if !strings.Contains(out, "github.com/acme/api") {
			t.Errorf("StatusRender() output = %q, want repo header", out)
		}
	})

	t.Run("uncommitted changes", func(t *testing.T) {
		status := &git.RepositoryStatus{
			Repository:            repo,
			HasUncommittedChanges: true,
			UncommittedChanges:    []string{"M README.md"},
		}
		out := captureStdout(func() { StatusRender(status) })
		if !strings.Contains(out, "❌ Uncommitted changes") {
			t.Errorf("StatusRender() output = %q, want uncommitted message", out)
		}
		if strings.Contains(out, "✅") {
			t.Errorf("StatusRender() output = %q, should not show clean when has issues", out)
		}
	})

	t.Run("branches behind remote", func(t *testing.T) {
		status := &git.RepositoryStatus{
			Repository:              repo,
			HasBranchesBehindRemote: true,
			Branches: []git.BranchInfo{
				{Name: "main", Behind: 3, Current: true},
			},
		}
		out := captureStdout(func() { StatusRender(status) })
		if !strings.Contains(out, "❌ Branches behind remote") {
			t.Errorf("StatusRender() output = %q, want behind message", out)
		}
		if !strings.Contains(out, "3 behind") {
			t.Errorf("StatusRender() output = %q, want behind count", out)
		}
	})

	t.Run("branches with remote gone", func(t *testing.T) {
		status := &git.RepositoryStatus{
			Repository:                repo,
			HasBranchesWithRemoteGone: true,
			Branches: []git.BranchInfo{
				{Name: "old-feat", RemoteGone: true},
			},
		}
		out := captureStdout(func() { StatusRender(status) })
		if !strings.Contains(out, "❌ Branches with remote gone") {
			t.Errorf("StatusRender() output = %q, want gone message", out)
		}
		if !strings.Contains(out, "old-feat") {
			t.Errorf("StatusRender() output = %q, want branch name", out)
		}
	})

	t.Run("branches without remote", func(t *testing.T) {
		status := &git.RepositoryStatus{
			Repository:               repo,
			HasBranchesWithoutRemote: true,
			Branches: []git.BranchInfo{
				{Name: "local", NoRemoteTracking: true},
			},
		}
		out := captureStdout(func() { StatusRender(status) })
		if !strings.Contains(out, "Branches without remote") {
			t.Errorf("StatusRender() output = %q, want no-remote message", out)
		}
	})

	t.Run("stash count displayed", func(t *testing.T) {
		status := &git.RepositoryStatus{
			Repository: repo,
			StashCount: 3,
		}
		out := captureStdout(func() { StatusRender(status) })
		if !strings.Contains(out, "3 stash") {
			t.Errorf("StatusRender() output = %q, want stash count", out)
		}
	})
}

func TestRenderPruneResults(t *testing.T) {
	repo := &git.Repository{Host: "github.com", Organization: "acme", Name: "api"}

	t.Run("pruned branches", func(t *testing.T) {
		results := map[string]git.PruneResult{
			"/path/to/repo": {
				Repository:     repo,
				PrunedBranches: []string{"old-feature", "stale"},
			},
		}
		out := captureStdout(func() { RenderPruneResults(results, false) })
		if !strings.Contains(out, "Pruned 2 branch") {
			t.Errorf("RenderPruneResults() output = %q, want pruned message", out)
		}
		if !strings.Contains(out, "old-feature") {
			t.Errorf("RenderPruneResults() output = %q, want branch name", out)
		}
	})

	t.Run("no branches to prune", func(t *testing.T) {
		results := map[string]git.PruneResult{
			"/path": {Repository: repo},
		}
		out := captureStdout(func() { RenderPruneResults(results, false) })
		if !strings.Contains(out, "✅ No branches to prune") {
			t.Errorf("RenderPruneResults() output = %q, want no-prune message", out)
		}
	})

	t.Run("error result", func(t *testing.T) {
		results := map[string]git.PruneResult{
			"/path": {Repository: repo, Error: errors.New("permission denied")},
		}
		out := captureStdout(func() { RenderPruneResults(results, false) })
		if !strings.Contains(out, "❌ Error") {
			t.Errorf("RenderPruneResults() output = %q, want error message", out)
		}
		if !strings.Contains(out, "permission denied") {
			t.Errorf("RenderPruneResults() output = %q, want error detail", out)
		}
	})

	t.Run("dry run shows warning", func(t *testing.T) {
		results := map[string]git.PruneResult{
			"/path": {Repository: repo, PrunedBranches: []string{"old"}},
		}
		out := captureStdout(func() { RenderPruneResults(results, true) })
		if !strings.Contains(out, "DRY RUN") {
			t.Errorf("RenderPruneResults() output = %q, want DRY RUN warning", out)
		}
	})
}

func TestUpdateRender(t *testing.T) {
	repo := &git.Repository{Host: "github.com", Organization: "acme", Name: "api"}

	t.Run("nil status returns without output", func(t *testing.T) {
		out := captureStdout(func() { UpdateRender(nil) })
		if out != "" {
			t.Errorf("UpdateRender(nil) output = %q, want empty", out)
		}
	})

	t.Run("no branch results shows up to date", func(t *testing.T) {
		status := &git.UpdateResult{
			Repository:          repo,
			BranchUpdateResults: map[string]git.BranchUpdateResult{},
		}
		out := captureStdout(func() { UpdateRender(status) })
		if !strings.Contains(out, "All branches are up to date") {
			t.Errorf("UpdateRender() output = %q, want up-to-date message", out)
		}
	})

	t.Run("branch updated successfully", func(t *testing.T) {
		branch := &git.BranchInfo{Name: "main"}
		status := &git.UpdateResult{
			Repository: repo,
			BranchUpdateResults: map[string]git.BranchUpdateResult{
				"main": {Branch: branch},
			},
		}
		out := captureStdout(func() { UpdateRender(status) })
		if !strings.Contains(out, "✅ Branch main is up to date") {
			t.Errorf("UpdateRender() output = %q, want up-to-date message", out)
		}
	})

	t.Run("branch update error", func(t *testing.T) {
		branch := &git.BranchInfo{Name: "feature"}
		status := &git.UpdateResult{
			Repository: repo,
			BranchUpdateResults: map[string]git.BranchUpdateResult{
				"feature": {Branch: branch, Err: errors.New("pull failed")},
			},
		}
		out := captureStdout(func() { UpdateRender(status) })
		if !strings.Contains(out, "❌ Error on branch feature") {
			t.Errorf("UpdateRender() output = %q, want error message", out)
		}
		if !strings.Contains(out, "pull failed") {
			t.Errorf("UpdateRender() output = %q, want error detail", out)
		}
	})

	t.Run("multiple branches rendered in sorted order", func(t *testing.T) {
		status := &git.UpdateResult{
			Repository: repo,
			BranchUpdateResults: map[string]git.BranchUpdateResult{
				"zebra": {Branch: &git.BranchInfo{Name: "zebra"}},
				"alpha": {Branch: &git.BranchInfo{Name: "alpha"}},
				"main":  {Branch: &git.BranchInfo{Name: "main"}},
			},
		}
		out := captureStdout(func() { UpdateRender(status) })
		alphaIdx := strings.Index(out, "alpha")
		mainIdx := strings.Index(out, "main")
		zebraIdx := strings.Index(out, "zebra")
		if !(alphaIdx < mainIdx && mainIdx < zebraIdx) {
			t.Errorf("UpdateRender() branches not in sorted order: alpha=%d main=%d zebra=%d\noutput=%q",
				alphaIdx, mainIdx, zebraIdx, out)
		}
	})
}

func TestUpdateErrorRender(t *testing.T) {
	repo := &git.Repository{Host: "github.com", Organization: "acme", Name: "api"}

	out := captureStdout(func() {
		UpdateErrorRender(repo, errors.New("connection refused"))
	})

	if !strings.Contains(out, "github.com/acme/api") {
		t.Errorf("UpdateErrorRender() output = %q, want repo header", out)
	}
	if !strings.Contains(out, "❌ Error") {
		t.Errorf("UpdateErrorRender() output = %q, want error marker", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("UpdateErrorRender() output = %q, want error message", out)
	}
}
