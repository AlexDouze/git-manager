// cmd/prune.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/alexDouze/gitm/pkg/git"
	"github.com/alexDouze/gitm/pkg/ui"
	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
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
		repositories, err := findRepositories(cfg.RootDirectory, hostFilter, organizationFilter, repositoryFilter, pathFilter, allRepositories)
		if err != nil {
			return fmt.Errorf("failed to find repositories: %w", err)
		}

		if len(repositories) == 0 {
			fmt.Println("No repositories found matching the specified filters.")
			return nil
		}

		// Create and start the BubbleTea program
		p := bubbletea.NewProgram(
			ui.NewPruneModel(repositories, dryRun, goneOnly, mergedOnly),
			bubbletea.WithAltScreen(),
		)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running UI: %w", err)
		}

		return nil
	},
}

// findRepositories finds repositories based on filters
func findRepositories(rootDir, host, org, repo, path string, all bool) ([]*git.Repository, error) {
	var repositories []*git.Repository

	// If path is specified, only check that path
	if path != "" {
		// Check if path is a git repository
		if isGitRepo(path) {
			repository, err := createRepositoryFromPath(path)
			if err != nil {
				return nil, err
			}
			repositories = append(repositories, repository)
		} else {
			// Check if path contains git repositories
			err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info.IsDir() && isGitRepo(p) {
					repository, err := createRepositoryFromPath(p)
					if err != nil {
						return err
					}
					repositories = append(repositories, repository)
					return filepath.SkipDir
				}

				return nil
			})

			if err != nil {
				return nil, err
			}
		}

		return filterRepositories(repositories, host, org, repo), nil
	}

	// If all is true or any filter is specified, scan the root directory
	if all || host != "" || org != "" || repo != "" {
		err := filepath.Walk(rootDir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() && isGitRepo(p) {
				repository, err := createRepositoryFromPath(p)
				if err != nil {
					return err
				}
				repositories = append(repositories, repository)
				return filepath.SkipDir
			}

			return nil
		})

		if err != nil {
			return nil, err
		}

		return filterRepositories(repositories, host, org, repo), nil
	}

	// If no filters are specified, use the current directory
	if isGitRepo(".") {
		repository, err := createRepositoryFromPath(".")
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, repository)
	}

	return repositories, nil
}

// isGitRepo checks if a directory is a git repository
func isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

// createRepositoryFromPath creates a Repository object from a path
func createRepositoryFromPath(path string) (*git.Repository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Extract host, organization, and name from path
	// This is a simplified approach and may need to be adjusted
	parts := strings.Split(absPath, string(os.PathSeparator))

	// We need at least 3 parts for host/org/repo
	if len(parts) < 3 {
		return &git.Repository{
			Path: absPath,
			Name: filepath.Base(absPath),
		}, nil
	}

	// Try to extract host, org, repo from path
	name := parts[len(parts)-1]
	org := parts[len(parts)-2]
	host := parts[len(parts)-3]

	// Validate that host looks like a domain
	if !strings.Contains(host, ".") {
		return &git.Repository{
			Path: absPath,
			Name: name,
		}, nil
	}

	return &git.Repository{
		Host:         host,
		Organization: org,
		Name:         name,
		Path:         absPath,
	}, nil
}

// filterRepositories filters repositories based on host, org, and repo
func filterRepositories(repositories []*git.Repository, host, org, repo string) []*git.Repository {
	if host == "" && org == "" && repo == "" {
		return repositories
	}

	var filtered []*git.Repository

	for _, r := range repositories {
		if host != "" && r.Host != host {
			continue
		}

		if org != "" && r.Organization != org {
			continue
		}

		if repo != "" && r.Name != repo {
			continue
		}

		filtered = append(filtered, r)
	}

	return filtered
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
