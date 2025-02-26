// cmd/config.go
package cmd

import (
	"fmt"

	"github.com/alexDouze/gitm/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and modify the configuration for the git repository manager.`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	Long:  `Create a new configuration file with default values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.InitConfig(); err != nil {
			return fmt.Errorf("failed to initialize configuration: %w", err)
		}
		
		fmt.Printf("Configuration initialized at %s\n", viper.ConfigFileUsed())
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get configuration value",
	Long:  `Get the value of a configuration key. If no key is provided, show all configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Show all configuration
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}
			
			fmt.Printf("rootDirectory: %s\n", cfg.RootDirectory)
			fmt.Printf("clone.defaultOptions: %s\n", cfg.Clone.DefaultOptions)
			return nil
		}
		
		// Show specific configuration key
		key := args[0]
		value := viper.Get(key)
		if value == nil {
			return fmt.Errorf("configuration key not found: %s", key)
		}
		
		fmt.Printf("%s: %v\n", key, value)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set configuration value",
	Long:  `Set the value of a configuration key.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]
		
		viper.Set(key, value)
		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("failed to write configuration: %w", err)
		}
		
		fmt.Printf("Set %s to %s\n", key, value)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}
