package tui

import (
	"fmt"
	"strings"

	"github.com/alexDouze/gitm/pkg/git"
)

func StatusRender(status *git.RepositoryStatus) {
	// Render the repository status
	fmt.Printf("=== %s/%s/%s ===\n", status.Repository.Host, status.Repository.Organization, status.Repository.Name)

	hasIssues := false

	// Check for branches behind remote
	if status.HasBranchesBehindRemote {
		branchesBehind := getBranchesWithIssue(status.Branches, func(b git.BranchInfo) bool {
			return b.Behind > 0
		})
		fmt.Printf("❌ Branches behind remote: %s\n", branchesBehind)
		hasIssues = true
	}

	// Check for branches with remote gone
	if status.HasBranchesWithRemoteGone {
		branchesGone := getBranchesWithIssue(status.Branches, func(b git.BranchInfo) bool {
			return b.RemoteGone
		})
		fmt.Printf("❌ Branches with remote gone: %s\n", branchesGone)
		hasIssues = true
	}

	// Check for branches without remote
	if status.HasBranchesWithoutRemote {
		branchesNoRemote := getBranchesWithIssue(status.Branches, func(b git.BranchInfo) bool {
			return b.NoRemoteTracking
		})
		fmt.Printf("❌ Branches without remote: %s\n", branchesNoRemote)
		hasIssues = true
	}

	// Check for uncommitted changes
	if status.HasUncommittedChanges {
		fmt.Println("❌ Uncommitted changes")
		hasIssues = true
	}

	if !hasIssues {
		fmt.Println("✅ Repository is clean")
	}
	fmt.Println()
}

func AllStatusCleanRender() {
	fmt.Println(" ✅ All repositories are clean")
}

// getBranchesWithIssue returns a comma-separated list of branch names that match the given condition
func getBranchesWithIssue(branches []git.BranchInfo, condition func(git.BranchInfo) bool) string {
	var problematicBranches []string
	
	for _, branch := range branches {
		if condition(branch) {
			// Add a * marker for the current branch
			branchName := branch.Name
			if branch.Current {
				branchName = "*" + branchName
			}
			
			// For branches behind remote, include how many commits behind
			if branch.Behind > 0 {
				branchName = fmt.Sprintf("%s (%d behind)", branchName, branch.Behind)
			}
			
			problematicBranches = append(problematicBranches, branchName)
		}
	}
	
	return strings.Join(problematicBranches, ", ")
}
