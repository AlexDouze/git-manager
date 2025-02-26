// pkg/config/config.go
package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	RootDirectory string `mapstructure:"rootDirectory"`
	Clone         struct {
		DefaultOptions string `mapstructure:"defaultOptions"`
	} `mapstructure:"clone"`
}

// LoadConfig loads the configuration from viper
func LoadConfig() (*Config, error) {
	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	// Set default root directory if not specified
	if config.RootDirectory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		config.RootDirectory = filepath.Join(home, "git-repos")
	}

	return &config, nil
}

// SaveConfig saves the configuration to the config file
func SaveConfig(config *Config) error {
	viper.Set("rootDirectory", config.RootDirectory)
	viper.Set("clone.defaultOptions", config.Clone.DefaultOptions)
	return viper.WriteConfig()
}

// InitConfig initializes a new configuration file
func InitConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	viper.SetConfigFile(filepath.Join(home, ".gitm.yaml"))
	
	// Set default values
	viper.Set("rootDirectory", filepath.Join(home, "git-repos"))
	viper.Set("clone.defaultOptions", "--recurse-submodules")

	return viper.WriteConfig()
}
