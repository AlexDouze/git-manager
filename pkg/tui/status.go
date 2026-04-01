package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexDouze/gitm/pkg/git"
)

func StatusRender(status *git.RepositoryStatus) {
	// Render the repository status
	fmt.Printf("=== %s/%s/%s ===\n", status.Repository.Host, status.Repository.Organization, status.Repository.Name)

	// Check for branches behind remote
	if status.HasBranchesBehindRemote {
		branchesBehind := getBranchesWithIssue(status.Branches, func(b git.BranchInfo) bool {
			return b.Behind > 0
		}, true)
		fmt.Printf("❌ Branches behind remote: %s\n", branchesBehind)
	}

	// Check for branches with remote gone
	if status.HasBranchesWithRemoteGone {
		branchesGone := getBranchesWithIssue(status.Branches, func(b git.BranchInfo) bool {
			return b.RemoteGone
		}, false)
		fmt.Printf("❌ Branches with remote gone: %s\n", branchesGone)
	}

	// Check for branches without remote
	if status.HasBranchesWithoutRemote {
		branchesNoRemote := getBranchesWithIssue(status.Branches, func(b git.BranchInfo) bool {
			return b.NoRemoteTracking
		}, false)
		fmt.Printf("❌ Branches without remote: %s\n", branchesNoRemote)
	}

	// Check for stale branches
	if status.HasStaleBranches {
		staleBranches := getStaleBranchesDisplay(status)
		fmt.Printf("❌ Stale branches: %s\n", staleBranches)
	}

	// Check for uncommitted changes
	if status.HasUncommittedChanges {
		fmt.Println("❌ Uncommitted changes")
	}

	if !status.HasIssues() {
		fmt.Println("✅ Repository is clean")
	}
	fmt.Println()
}

// getBranchesWithIssue returns a comma-separated list of branch names that match the given condition.
// For branches that are behind their remote, the count is appended only when includeBehind is true.
func getBranchesWithIssue(branches []git.BranchInfo, condition func(git.BranchInfo) bool, includeBehind bool) string {
	var problematicBranches []string

	for _, branch := range branches {
		if condition(branch) {
			// Add a * marker for the current branch
			branchName := branch.Name
			if branch.Current {
				branchName = "*" + branchName
			}

			// For branches behind remote, include how many commits behind
			if includeBehind && branch.Behind > 0 {
				branchName = fmt.Sprintf("%s (%d behind)", branchName, branch.Behind)
			}

			problematicBranches = append(problematicBranches, branchName)
		}
	}

	return strings.Join(problematicBranches, ", ")
}

// formatBranchAge returns a human-readable string for how long ago the commit was made.
func formatBranchAge(commitDate time.Time) string {
	days := int(time.Since(commitDate).Hours() / 24)
	switch {
	case days == 0:
		return "today"
	case days < 7:
		return fmt.Sprintf("%d days", days)
	case days < 30:
		weeks := days / 7
		if weeks == 1 {
			return "1 week"
		}
		return fmt.Sprintf("%d weeks", weeks)
	case days < 365:
		months := days / 30
		if months == 1 {
			return "1 month"
		}
		return fmt.Sprintf("%d months", months)
	default:
		years := days / 365
		if years == 1 {
			return "1 year"
		}
		return fmt.Sprintf("%d years", years)
	}
}

// getStaleBranchesDisplay formats stale branches with name, age, remote status, and commits behind default.
func getStaleBranchesDisplay(status *git.RepositoryStatus) string {
	cutoff := time.Now().Add(-status.StaleBranchThreshold)
	var parts []string

	for _, branch := range status.Branches {
		if branch.LastCommitDate.IsZero() || !branch.LastCommitDate.Before(cutoff) {
			continue
		}

		name := branch.Name
		if branch.Current {
			name = "*" + name
		}

		age := formatBranchAge(branch.LastCommitDate)

		var remoteStatus string
		switch {
		case branch.RemoteGone:
			remoteStatus = "remote gone"
		case branch.NoRemoteTracking:
			remoteStatus = "no remote"
		default:
			remoteStatus = "has remote"
		}

		entry := fmt.Sprintf("%s (%s, %s, %d behind)", name, age, remoteStatus, branch.CommitsBehindDefault)
		parts = append(parts, entry)
	}

	return strings.Join(parts, ", ")
}
