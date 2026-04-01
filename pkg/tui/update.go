package tui

import (
	"fmt"
	"sort"

	"github.com/alexDouze/gitm/pkg/git"
)

func UpdateRender(status *git.UpdateResult) {
	// Return early if status is nil (fetch-only mode or error)
	if status == nil {
		return
	}

	// Render the repository status
	fmt.Printf("=== %s/%s/%s ===\n", status.Repository.Host, status.Repository.Organization, status.Repository.Name)

	if len(status.BranchUpdateResults) == 0 {
		fmt.Println("✅ Fetched changes only (no pull)")
	} else {
		keys := make([]string, 0, len(status.BranchUpdateResults))
		for k := range status.BranchUpdateResults {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			branch := status.BranchUpdateResults[key]
			if branch.Err != nil {
				fmt.Printf("❌ Error on branch %s: %s\n", key, branch.Err)
			} else {
				fmt.Printf("✅ Branch %s is up to date\n", key)
			}
		}
	}
	fmt.Println()
}

// UpdateErrorRender renders an update error through the TUI
func UpdateErrorRender(repo *git.Repository, err error) {
	fmt.Printf("=== %s/%s/%s ===\n", repo.Host, repo.Organization, repo.Name)
	fmt.Printf("❌ Error: %v\n", err)
	fmt.Println()
}
