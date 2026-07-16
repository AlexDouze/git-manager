package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alexDouze/gitm/pkg/git"
)

// RenderPruneResults renders the results of pruning branches in the terminal.
func RenderPruneResults(pruneResults map[string]git.PruneResult, isDryRun bool) {
	if len(pruneResults) == 0 {
		return
	}

	if isDryRun {
		WarnStyle.Println("⚠️  DRY RUN — no branches were actually deleted")
		fmt.Println()
	}

	// Sort by repository path for deterministic output
	paths := make([]string, 0, len(pruneResults))
	for path := range pruneResults {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		result := pruneResults[path]
		repo := result.Repository

		HeaderStyle.Printf("=== %s/%s/%s ===\n", repo.Host, repo.Organization, repo.Name)

		if result.Error != nil {
			ErrorStyle.Printf("❌ Error: %s\n", result.Error)
		} else if len(result.PrunedBranches) == 0 && len(result.SkippedBranches) == 0 {
			SuccessStyle.Println("✅ No branches to prune")
		} else if len(result.PrunedBranches) > 0 {
			SuccessStyle.Printf("✅ Pruned %d branch(es): %s\n",
				len(result.PrunedBranches),
				strings.Join(result.PrunedBranches, ", "))
		}

		if len(result.SkippedBranches) > 0 {
			skips := make([]string, 0, len(result.SkippedBranches))
			for _, s := range result.SkippedBranches {
				skips = append(skips, fmt.Sprintf("%s (%s)", s.Name, s.Reason))
			}
			WarnStyle.Printf("⚠️  Skipped %d branch(es): %s\n",
				len(result.SkippedBranches), strings.Join(skips, ", "))
		}

		fmt.Println()
	}
}
