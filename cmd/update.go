// cmd/update.go
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
	fetchOnly bool
	prune     bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update repositories",
	Long: `Update git repositories by fetching and optionally pulling the latest changes.
Can also prune remote-tracking branches that no longer exist on the remote.`,
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
			ui.NewUpdateModel(repositories, fetchOnly, prune),
			tea.WithAltScreen(),
		)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running UI: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	// Add filter flags (reusing the same flags from status command)
	updateCmd.Flags().StringVar(&hostFilter, "host", "", "Filter repositories by host")
	updateCmd.Flags().StringVar(&organizationFilter, "org", "", "Filter repositories by organization/username")
	updateCmd.Flags().StringVar(&repositoryFilter, "repo", "", "Filter repositories by name")
	updateCmd.Flags().StringVar(&pathFilter, "path", "", "Filter repositories by path")
	updateCmd.Flags().BoolVar(&allRepositories, "all", false, "Update all repositories")

	// Add update-specific flags
	updateCmd.Flags().BoolVar(&fetchOnly, "fetch-only", false, "Only fetch changes without pulling")
	updateCmd.Flags().BoolVar(&prune, "prune", false, "Prune remote-tracking branches")
}
