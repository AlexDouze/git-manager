package tui

import (
	"testing"

	"github.com/alexDouze/gitm/pkg/git"
)

func TestGetBranchesWithIssue(t *testing.T) {
	tests := []struct {
		name      string
		branches  []git.BranchInfo
		condition func(git.BranchInfo) bool
		expected  string
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
			expected: "*main (2 behind), develop (5 behind)",
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
			expected: "feature, *old-feature",
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
			expected: "feature, test",
		},
		{
			name:      "no matching branches",
			branches:  []git.BranchInfo{{Name: "main"}, {Name: "feature"}},
			condition: func(b git.BranchInfo) bool { return false },
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBranchesWithIssue(tt.branches, tt.condition)
			if result != tt.expected {
				t.Errorf("getBranchesWithIssue() = %q, want %q", result, tt.expected)
			}
		})
	}
}