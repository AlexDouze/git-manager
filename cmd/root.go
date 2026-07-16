// cmd/root.go
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version information (set by goreleaser)
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var cfgFile string
var noColor bool

var rootCmd = &cobra.Command{
	Use:   "gitm",
	Short: "A multi-git repository manager",
	Long: `gitm is a CLI tool for managing multiple git repositories
from different hosts (GitHub, GitLab, etc.) with a structured folder hierarchy.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if noColor {
			color.NoColor = true
		}
	},
}

func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gitm.yaml)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(".")
		viper.AddConfigPath(home)
		viper.SetConfigName(".gitm")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	// A missing config file is fine (defaults apply). Any other read error means
	// the file exists but is broken (bad YAML, unreadable) — running with silent
	// defaults against the wrong root directory is worse than failing loudly.
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			fmt.Fprintf(os.Stderr, "failed to read config file %s: %v\n", viper.ConfigFileUsed(), err)
			os.Exit(1)
		}
	}
}
