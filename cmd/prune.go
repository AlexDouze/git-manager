// cmd/prune.go
package cmd

import (
	"fmt"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	dryRun     bool
	goneOnly   bool
	mergedOnly bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Prune branches",
	Long: `Prune git branches that match certain criteria, such as
branches with remote gone or branches that have been fully merged.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Find repositories based on filters
		repositories, err := git.FindRepositories(cfg.RootDirectory, hostFilter, organizationFilter, repositoryFilter, pathFilter, allRepositories)
		if err != nil {
			return fmt.Errorf("failed to find repositories: %w", err)
		}

		if len(repositories) == 0 {
			fmt.Println("No repositories found matching the specified filters.")
			return nil
		}

		// Create and start the BubbleTea program
		p := tea.NewProgram(
			ui.NewPruneModel(repositories, dryRun, goneOnly, mergedOnly),
			tea.WithAltScreen(),
		)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running UI: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(pruneCmd)

	// Add filter flags (reusing the same flags from status command)
	pruneCmd.Flags().StringVar(&hostFilter, "host", "", "Filter repositories by host")
	pruneCmd.Flags().StringVar(&organizationFilter, "org", "", "Filter repositories by organization/username")
	pruneCmd.Flags().StringVar(&repositoryFilter, "repo", "", "Filter repositories by name")
	pruneCmd.Flags().StringVar(&pathFilter, "path", "", "Filter repositories by path")
	pruneCmd.Flags().BoolVar(&allRepositories, "all", false, "Prune branches in all repositories")

	// Add prune-specific flags
	pruneCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be pruned without actually deleting")
	pruneCmd.Flags().BoolVar(&goneOnly, "gone-only", false, "Only prune branches whose remote is gone")
	pruneCmd.Flags().BoolVar(&mergedOnly, "merged-only", false, "Only prune branches that are fully merged")
}
