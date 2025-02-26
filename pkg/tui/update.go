package tui

import (
	"fmt"

	"github.com/alexDouze/gitm/pkg/git"
)

func UpdateRender(status *git.UpdateResult) {
	// Render the repository status
	fmt.Printf("=== %s/%s/%s ===\n", status.Repository.Host, status.Repository.Organization, status.Repository.Name)
	for key, branch := range status.BranchUpdateResults {
		if branch.Err != nil {
			fmt.Printf("❌ Error on branch %s: %s\n", key, branch.Err)
		} else {
			fmt.Printf("✅ Branch %s is up to date\n", key)
		}
	}
	fmt.Println()
}
