// cmd/status.go
package cmd

import (
	"fmt"
	"sync"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/tui"
	"github.com/spf13/cobra"
)

var (
	hostFilter         string
	organizationFilter string
	repositoryFilter   string
	pathFilter         string
	allRepositories    bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of repositories",
	Long: `Check the status of git repositories, showing uncommitted changes, 
branch status, and other important information.`,
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

		// Create a channel to collect status results
		type repoResult struct {
			status *git.RepositoryStatus
			err    error
		}
		resultChan := make(chan repoResult, len(repositories))

		// Create a wait group to wait for all goroutines to complete
		var wg sync.WaitGroup
		wg.Add(len(repositories))

		// Process repositories in parallel
		for _, repo := range repositories {
			go func(r *git.Repository) {
				defer wg.Done()

				// Update repository (fetch remote branches)
				updateErr := r.Update(true, false)

				// Get repository status
				status, statusErr := r.Status()

				// Send result to channel
				resultChan <- repoResult{
					status: status,
					err:    statusErr,
				}

				// Log update error separately
				if updateErr != nil {
					fmt.Printf("Warning: failed to fetch remote branches for %s: %v\n", r.Path, updateErr)
				}
			}(repo)
		}

		// Wait for all goroutines to complete
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// Collect results
		results := make(map[string]repoResult)
		for result := range resultChan {
			results[result.status.Repository.Path] = result
		}

		// Display results in the same order as the original repositories list
		for _, repo := range repositories {
			result, exists := results[repo.Path]
			if !exists {
				fmt.Printf("Warning: no result found for %s\n", repo.Path)
				continue
			}

			if result.err != nil {
				fmt.Printf("Warning: failed to get status for %s: %v\n", result.status.Repository.Path, result.err)
				continue
			}
			if result.status.HasIssues() {
				tui.StatusRender(result.status)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Add filter flags
	statusCmd.Flags().StringVar(&hostFilter, "host", "", "Filter repositories by host")
	statusCmd.Flags().StringVar(&organizationFilter, "org", "", "Filter repositories by organization/username")
	statusCmd.Flags().StringVar(&repositoryFilter, "repo", "", "Filter repositories by name")
	statusCmd.Flags().StringVar(&pathFilter, "path", "", "Filter repositories by path")
	statusCmd.Flags().BoolVar(&allRepositories, "all", false, "Check all repositories")
}
