// cmd/clone.go
package cmd

import (
	"fmt"
	"strings"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/spf13/cobra"
)

var rootDir string

var cloneCmd = &cobra.Command{
	Use:   "clone <repository-url>",
	Short: "Clone a git repository",
	Long: `Clone a git repository into a structured directory hierarchy.
The repository will be cloned into <root-directory>/<host>/<organization>/<repository>.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("failed to load configuration: %s", err)
			return
		}

		// Use specified root directory or default from config
		targetDir := cfg.RootDirectory

		// Parse the repository URL
		repo, err := git.ParseURL(url)
		if err != nil {
			fmt.Printf("failed to parse repository URL: %s", err)
			return
		}

		// Parse clone options
		var cloneOptions []string
		if cfg.Clone.DefaultOptions != "" {
			cloneOptions = strings.Fields(cfg.Clone.DefaultOptions)
		}

		// Clone the repository
		fmt.Printf("Cloning %s to %s/%s/%s/%s\n",
			url, targetDir, repo.Host, repo.Organization, repo.Name)

		err = repo.Clone(targetDir, url, cloneOptions)
		if err != nil {
			fmt.Printf("failed to clone repository: %s", err)
			return
		}

		fmt.Printf("Successfully cloned repository to %s/%s/%s/%s\n",
			targetDir, repo.Host, repo.Organization, repo.Name)

	},
}

func init() {
	rootCmd.AddCommand(cloneCmd)
	cloneCmd.Flags().StringVar(&rootDir, "root-dir", "", "Root directory for cloning repositories")
}
