package cmd

import "github.com/spf13/cobra"

// FilterFlags holds the common repository filter flags shared across commands.
type FilterFlags struct {
	Host string
	Org  string
	Repo string
	Path string
}

// Register adds the standard filter flags to the given command.
func (f *FilterFlags) Register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.Host, "host", "", "Filter repositories by host (e.g., github.com)")
	cmd.Flags().StringVar(&f.Org, "org", "", "Filter repositories by organization/username")
	cmd.Flags().StringVar(&f.Repo, "repo", "", "Filter repositories by name")
	cmd.Flags().StringVar(&f.Path, "path", "", "Filter repositories by path")
}
