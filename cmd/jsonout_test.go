package cmd

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/alexDouze/gitm/pkg/git"
)

func TestBranchToJSON(t *testing.T) {
	commitDate := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input git.BranchInfo
		check func(t *testing.T, got branchJSON)
	}{
		{
			name: "full branch with commit date",
			input: git.BranchInfo{
				Name:                 "feature/x",
				Current:              true,
				RemoteTracking:       "origin/feature/x",
				NoRemoteTracking:     false,
				RemoteGone:           false,
				Ahead:                2,
				Behind:               1,
				LastCommitDate:       commitDate,
				CommitsBehindDefault: 3,
				Stale:                false,
				WorktreePath:         "/tmp/wt",
			},
			check: func(t *testing.T, got branchJSON) {
				if got.Name != "feature/x" || !got.Current {
					t.Errorf("unexpected name/current: %+v", got)
				}
				if got.LastCommitDate == nil || !got.LastCommitDate.Equal(commitDate) {
					t.Errorf("LastCommitDate = %v, want %v", got.LastCommitDate, commitDate)
				}
				if got.WorktreePath != "/tmp/wt" {
					t.Errorf("WorktreePath = %q, want /tmp/wt", got.WorktreePath)
				}
			},
		},
		{
			name: "zero commit date maps to nil pointer",
			input: git.BranchInfo{
				Name:           "main",
				LastCommitDate: time.Time{},
			},
			check: func(t *testing.T, got branchJSON) {
				if got.LastCommitDate != nil {
					t.Errorf("LastCommitDate = %v, want nil for zero time", got.LastCommitDate)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := branchToJSON(tt.input)
			tt.check(t, got)

			// Round-trip: must marshal without error and preserve the name.
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}
			var back branchJSON
			if err := json.Unmarshal(data, &back); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			if back.Name != tt.input.Name {
				t.Errorf("round-trip name = %q, want %q", back.Name, tt.input.Name)
			}
		})
	}
}

func TestStatusToJSON(t *testing.T) {
	repo := &git.Repository{
		Host:         "github.com",
		Organization: "octocat",
		Name:         "hello-world",
		Path:         "/repos/github.com/octocat/hello-world",
	}
	status := &git.RepositoryStatus{
		Repository:                repo,
		HasUncommittedChanges:     true,
		UncommittedChanges:        []string{" M file.go"},
		CurrentBranch:             "main",
		HasBranchesWithoutRemote:  true,
		HasBranchesWithRemoteGone: false,
		HasBranchesBehindRemote:   true,
		HasStaleBranches:          true,
		StashCount:                2,
		StaleBranchThreshold:      30 * 24 * time.Hour,
		Branches: []git.BranchInfo{
			{Name: "main", Current: true},
			{Name: "feature/y", Stale: true},
		},
	}

	got := statusToJSON(status)

	if got.Host != "github.com" || got.Organization != "octocat" || got.Name != "hello-world" {
		t.Errorf("identity fields wrong: %+v", got)
	}
	if got.Path != repo.Path {
		t.Errorf("Path = %q, want %q", got.Path, repo.Path)
	}
	if !got.HasIssues {
		t.Errorf("HasIssues = false, want true (status has issues)")
	}
	if !got.HasUncommittedChanges || len(got.UncommittedChanges) != 1 {
		t.Errorf("uncommitted changes not propagated: %+v", got)
	}
	if got.StashCount != 2 {
		t.Errorf("StashCount = %d, want 2", got.StashCount)
	}
	if got.StaleBranchThresholdDays != 30 {
		t.Errorf("StaleBranchThresholdDays = %v, want 30", got.StaleBranchThresholdDays)
	}
	if len(got.Branches) != 2 {
		t.Fatalf("Branches len = %d, want 2", len(got.Branches))
	}
	if got.Branches[0].Name != "main" || got.Branches[1].Name != "feature/y" {
		t.Errorf("branch order/name wrong: %+v", got.Branches)
	}

	// Round-trip through JSON.
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var back statusJSON
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if back.Name != got.Name || back.HasIssues != got.HasIssues || len(back.Branches) != len(got.Branches) {
		t.Errorf("round-trip mismatch: got %+v, back %+v", got, back)
	}
}

func TestPruneToJSON(t *testing.T) {
	t.Run("full result", func(t *testing.T) {
		repo := &git.Repository{
			Host:         "gitlab.com",
			Organization: "group/sub",
			Name:         "repo",
			Path:         "/repos/gitlab.com/group/sub/repo",
		}
		result := git.PruneResult{
			Repository:     repo,
			PrunedBranches: []string{"gone-1", "gone-2"},
			SkippedBranches: []git.SkippedBranch{
				{Name: "unmerged", Reason: "not fully merged (use --force)"},
			},
			Error: nil,
		}

		got := pruneToJSON(result)

		if got.Host != "gitlab.com" || got.Organization != "group/sub" || got.Name != "repo" {
			t.Errorf("identity fields wrong: %+v", got)
		}
		if got.Path != repo.Path {
			t.Errorf("Path = %q, want %q", got.Path, repo.Path)
		}
		if len(got.PrunedBranches) != 2 {
			t.Errorf("PrunedBranches = %v, want 2 entries", got.PrunedBranches)
		}
		if len(got.SkippedBranches) != 1 || got.SkippedBranches[0].Name != "unmerged" {
			t.Errorf("SkippedBranches wrong: %+v", got.SkippedBranches)
		}
		if got.Error != "" {
			t.Errorf("Error = %q, want empty", got.Error)
		}

		// Round-trip through JSON.
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		var back pruneJSON
		if err := json.Unmarshal(data, &back); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		if back.Name != got.Name || len(back.PrunedBranches) != 2 || len(back.SkippedBranches) != 1 {
			t.Errorf("round-trip mismatch: got %+v, back %+v", got, back)
		}
	})

	t.Run("error becomes string field", func(t *testing.T) {
		result := git.PruneResult{
			Repository: &git.Repository{Name: "repo"},
			Error:      errors.New("failed to prune branches: boom"),
		}
		got := pruneToJSON(result)
		if got.Error != "failed to prune branches: boom" {
			t.Errorf("Error = %q, want the error string", got.Error)
		}
	})

	t.Run("nil repository is safe", func(t *testing.T) {
		result := git.PruneResult{Repository: nil, PrunedBranches: []string{"x"}}
		got := pruneToJSON(result)
		if got.Host != "" || got.Name != "" || got.Path != "" {
			t.Errorf("expected empty identity for nil repository, got %+v", got)
		}
		if len(got.PrunedBranches) != 1 {
			t.Errorf("PrunedBranches = %v, want 1 entry", got.PrunedBranches)
		}
	})
}
