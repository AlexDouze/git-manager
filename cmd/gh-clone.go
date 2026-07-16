// cmd/gh-clone.go
package cmd

import (
	"fmt"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/tui/app"
	"github.com/spf13/cobra"
)

var (
	ghCloneOwner   string
	ghCloneRootDir string
	ghCloneLimit   int
)

var ghCloneCmd = &cobra.Command{
	Use:   "gh-clone [owner]",
	Short: "List and clone GitHub repositories",
	Long: `List repositories from a GitHub organization or username,
select repositories from a filterable list, and clone the selected repositories.

Examples:
  # List repositories from a GitHub organization
  gitm gh-clone alexdouze

  # List repositories from a GitHub organization with a limit
  gitm gh-clone alexdouze --limit 10

  # List repositories from a GitHub organization and clone to a specific directory
  gitm gh-clone alexdouze --root-dir ~/projects`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Set owner from args if provided
		if len(args) > 0 {
			ghCloneOwner = args[0]
		}

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Launch the interactive GitHub clone browser. It lists the owner's
		// repositories, lets the user multi-select, and clones the selection
		// into rootDir/host/org/name (skipping any already on disk).
		return app.RunBrowse(cmd.Context(), cfg, ghCloneOwner, ghCloneRootDir, ghCloneLimit, noColor)
	},
}

func init() {
	rootCmd.AddCommand(ghCloneCmd)
	ghCloneCmd.Flags().StringVar(&ghCloneOwner, "owner", "", "GitHub organization or username")
	ghCloneCmd.Flags().StringVar(&ghCloneRootDir, "root-dir", "", "Root directory for cloning repositories")
	ghCloneCmd.Flags().IntVar(&ghCloneLimit, "limit", 1000, "Maximum number of repositories to list")
}
