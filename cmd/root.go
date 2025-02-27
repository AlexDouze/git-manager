// cmd/root.go
package cmd

import (
	"fmt"
	"os"

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
var rootCmd = &cobra.Command{
	Use:   "gitm",
	Short: "A multi-git repository manager",
	Long: `gitm is a CLI tool for managing multiple git repositories
from different hosts (GitHub, GitLab, etc.) with a structured folder hierarchy.`,
	Version: Version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gitm.yaml)")
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
		viper.SetConfigName("gitm")
		viper.SetConfigName(".gitm")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	viper.ReadInConfig()
}
