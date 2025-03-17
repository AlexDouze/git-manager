package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexDouze/gitm/pkg/git"
)

// RenderPruneResults renders the results of pruning branches in the terminal
// using the same style as StatusRender and UpdateRender
func RenderPruneResults(pruneResults map[string]git.PruneResult, isDryRun bool) error {
	// No results to display
	if len(pruneResults) == 0 {
		return nil
	}

	// Print dry run warning if applicable
	if isDryRun {
		fmt.Println("⚠️  DRY RUN - No branches were actually deleted")
		fmt.Println()
	}

	// Sort the results by repository path for consistent display
	var paths []string
	for path := range pruneResults {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Display results for each repository
	for _, path := range paths {
		result := pruneResults[path]
		repo := result.Repository

		// Print repository header
		fmt.Printf("=== %s/%s/%s ===\n", repo.Host, repo.Organization, repo.Name)

		// Print error if one occurred
		if result.Error != nil {
			fmt.Printf("❌ Error: %s\n", result.Error)
		} else if len(result.PrunedBranches) == 0 {
			// No branches to prune
			fmt.Println("✅ No branches to prune")
		} else {
			// Branches were pruned
			fmt.Printf("✅ Pruned %d branches: %s\n", 
				len(result.PrunedBranches), 
				strings.Join(result.PrunedBranches, ", "))
		}

		fmt.Println()
	}

	return nil
}