package tui

import (
	"testing"
	"time"

	"github.com/alexDouze/gitm/pkg/git"
)

func TestGetBranchesWithIssue(t *testing.T) {
	tests := []struct {
		name          string
		branches      []git.BranchInfo
		condition     func(git.BranchInfo) bool
		includeBehind bool
		expected      string
	}{
		{
			name: "branches behind remote",
			branches: []git.BranchInfo{
				{Name: "main", Current: true, Behind: 2},
				{Name: "feature", Behind: 0},
				{Name: "develop", Behind: 5},
			},
			condition: func(b git.BranchInfo) bool {
				return b.Behind > 0
			},
			includeBehind: true,
			expected:      "*main (2 behind), develop (5 behind)",
		},
		{
			name: "branches with remote gone",
			branches: []git.BranchInfo{
				{Name: "main", RemoteGone: false},
				{Name: "feature", RemoteGone: true},
				{Name: "old-feature", RemoteGone: true, Current: true},
			},
			condition: func(b git.BranchInfo) bool {
				return b.RemoteGone
			},
			includeBehind: false,
			expected:      "feature, *old-feature",
		},
		{
			name: "branches without remote",
			branches: []git.BranchInfo{
				{Name: "main", NoRemoteTracking: false},
				{Name: "feature", NoRemoteTracking: true},
				{Name: "test", NoRemoteTracking: true},
			},
			condition: func(b git.BranchInfo) bool {
				return b.NoRemoteTracking
			},
			includeBehind: false,
			expected:      "feature, test",
		},
		{
			name:          "no matching branches",
			branches:      []git.BranchInfo{{Name: "main"}, {Name: "feature"}},
			condition:     func(b git.BranchInfo) bool { return false },
			includeBehind: false,
			expected:      "",
		},
		{
			name: "gone branch with behind count not shown when includeBehind false",
			branches: []git.BranchInfo{
				{Name: "feature", RemoteGone: true, Behind: 3},
			},
			condition: func(b git.BranchInfo) bool {
				return b.RemoteGone
			},
			includeBehind: false,
			expected:      "feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBranchesWithIssue(tt.branches, tt.condition, tt.includeBehind)
			if result != tt.expected {
				t.Errorf("getBranchesWithIssue() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatBranchAge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		date     time.Time
		expected string
	}{
		{
			name:     "today",
			date:     now.Add(-1 * time.Hour),
			expected: "today",
		},
		{
			name:     "2 days ago",
			date:     now.Add(-2 * 24 * time.Hour),
			expected: "2 days",
		},
		{
			name:     "1 week ago",
			date:     now.Add(-7 * 24 * time.Hour),
			expected: "1 week",
		},
		{
			name:     "2 weeks ago",
			date:     now.Add(-14 * 24 * time.Hour),
			expected: "2 weeks",
		},
		{
			name:     "1 month ago",
			date:     now.Add(-30 * 24 * time.Hour),
			expected: "1 month",
		},
		{
			name:     "2 months ago",
			date:     now.Add(-60 * 24 * time.Hour),
			expected: "2 months",
		},
		{
			name:     "1 year ago",
			date:     now.Add(-365 * 24 * time.Hour),
			expected: "1 year",
		},
		{
			name:     "2 years ago",
			date:     now.Add(-730 * 24 * time.Hour),
			expected: "2 years",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBranchAge(tt.date)
			if result != tt.expected {
				t.Errorf("formatBranchAge() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRenderPruneResults_empty(t *testing.T) {
	// Calling with empty map should not panic.
	RenderPruneResults(map[string]git.PruneResult{}, false)
}