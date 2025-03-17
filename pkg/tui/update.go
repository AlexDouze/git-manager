package tui

import (
	"fmt"

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
		for key, branch := range status.BranchUpdateResults {
			if branch.Err != nil {
				fmt.Printf("❌ Error on branch %s: %s\n", key, branch.Err)
			} else {
				fmt.Printf("✅ Branch %s is up to date\n", key)
			}
		}
	}
	fmt.Println()
}
