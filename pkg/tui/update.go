package tui

import (
	"fmt"
	"sort"

	"github.com/alexDouze/gitm/pkg/git"
)

// UpdateRender renders the result of a full fetch+pull update.
func UpdateRender(status *git.UpdateResult) {
	if status == nil {
		return
	}

	HeaderStyle.Printf("=== %s/%s/%s ===\n", status.Repository.Host, status.Repository.Organization, status.Repository.Name)

	if len(status.BranchUpdateResults) == 0 {
		SuccessStyle.Println("✅ All branches are up to date")
	} else {
		keys := make([]string, 0, len(status.BranchUpdateResults))
		for k := range status.BranchUpdateResults {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			branch := status.BranchUpdateResults[key]
			if branch.Err != nil {
				ErrorStyle.Printf("❌ Error on branch %s: %s\n", key, branch.Err)
			} else {
				SuccessStyle.Printf("✅ Branch %s is up to date\n", key)
			}
		}
	}
	fmt.Println()
}

// UpdateFetchOnlyRender renders the result of a fetch-only update.
func UpdateFetchOnlyRender(repo *git.Repository) {
	HeaderStyle.Printf("=== %s/%s/%s ===\n", repo.Host, repo.Organization, repo.Name)
	SuccessStyle.Println("✅ Changes fetched successfully")
	fmt.Println()
}

// UpdateErrorRender renders an update error.
func UpdateErrorRender(repo *git.Repository, err error) {
	HeaderStyle.Printf("=== %s/%s/%s ===\n", repo.Host, repo.Organization, repo.Name)
	ErrorStyle.Printf("❌ Error: %v\n", err)
	fmt.Println()
}
