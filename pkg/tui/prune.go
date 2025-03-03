package tui

import (
	"fmt"
	"sort"

	"github.com/alexDouze/gitm/pkg/git"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RenderPruneResults renders the results of pruning branches in the terminal
func RenderPruneResults(pruneResults map[string]git.PruneResult, isDryRun bool) error {
	// No results to display
	if len(pruneResults) == 0 {
		return nil
	}

	// Create the application
	app := tview.NewApplication()

	// Create the table
	table := tview.NewTable().
		SetBorders(false)

	// Set the table headers
	headers := []string{"Repository", "Pruned Branches", "Error"}
	for i, header := range headers {
		table.SetCell(0, i,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignLeft).
				SetSelectable(false))
	}

	// Sort the results by repository path for consistent display
	var paths []string
	for path := range pruneResults {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	// Add the results to the table
	row := 1
	for _, path := range paths {
		result := pruneResults[path]
		repo := result.Repository

		// Repository name column
		repoName := fmt.Sprintf("%s/%s", repo.Organization, repo.Name)
		textColor := tcell.ColorWhite
		if result.Error != nil {
			textColor = tcell.ColorRed
		} else if len(result.PrunedBranches) == 0 {
			textColor = tcell.ColorGray
		}

		table.SetCell(row, 0,
			tview.NewTableCell(repoName).
				SetTextColor(textColor).
				SetReference(path))

		// Pruned branches column
		var branchesText string
		if result.Error == nil {
			if len(result.PrunedBranches) == 0 {
				branchesText = "No branches to prune"
			} else {
				for i, branch := range result.PrunedBranches {
					if i > 0 {
						branchesText += ", "
					}
					branchesText += branch
				}
			}
		} else {
			branchesText = "-"
		}
		table.SetCell(row, 1,
			tview.NewTableCell(branchesText).
				SetTextColor(textColor))

		// Error column
		var errorText string
		if result.Error != nil {
			errorText = result.Error.Error()
		} else {
			errorText = "-"
		}
		table.SetCell(row, 2,
			tview.NewTableCell(errorText).
				SetTextColor(textColor))

		row++
	}

	// Create a frame for the table
	title := "Branch Pruning Results"
	if isDryRun {
		title += " (DRY RUN - No branches were actually deleted)"
	}
	frame := tview.NewFrame(table).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText(title, true, tview.AlignCenter, tcell.ColorYellow).
		AddText("Press Esc or q to exit", false, tview.AlignCenter, tcell.ColorGray)

	// Set up keyboard input
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			app.Stop()
			return nil
		}
		return event
	})

	// Run the application
	if err := app.SetRoot(frame, true).Run(); err != nil {
		return err
	}

	return nil
}