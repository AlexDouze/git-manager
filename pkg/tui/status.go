package tui

import (
	"fmt"

	"github.com/alexDouze/gitm/pkg/git"
)

func StatusRender(status *git.RepositoryStatus) {
	// Render the repository status
	fmt.Printf("=== %s/%s/%s ===\n", status.Repository.Host, status.Repository.Organization, status.Repository.Name)

	hasIssues := false

	if status.HasBranchesBehindRemote {
		fmt.Println("❌ Branches behind remote")
		hasIssues = true
	}
	if status.HasBranchesWithRemoteGone {
		fmt.Println("❌ Branches with remote gone")
		hasIssues = true
	}
	if status.HasBranchesWithoutRemote {
		fmt.Println("❌ Branches without remote")
		hasIssues = true
	}
	if status.HasUncommittedChanges {
		fmt.Println("❌ Uncommitted changes")
		hasIssues = true
	}
	if !hasIssues {
		fmt.Println("✅ Repository is clean")
	}
}
