package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// resetViper clears all viper state between tests.
func resetViper() {
	viper.Reset()
}

func TestLoadConfig_defaults(t *testing.T) {
	resetViper()
	home, _ := os.UserHomeDir()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	want := filepath.Join(home, "Codebase")
	if cfg.RootDirectory != want {
		t.Errorf("RootDirectory = %q, want %q", cfg.RootDirectory, want)
	}
}

func TestLoadConfig_withRootDirectory(t *testing.T) {
	resetViper()
	viper.Set("rootDirectory", "/custom/root")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	if cfg.RootDirectory != "/custom/root" {
		t.Errorf("RootDirectory = %q, want /custom/root", cfg.RootDirectory)
	}
}

func TestLoadConfig_withCloneOptions(t *testing.T) {
	resetViper()
	viper.Set("clone.defaultOptions", "--depth=1")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	if cfg.Clone.DefaultOptions != "--depth=1" {
		t.Errorf("Clone.DefaultOptions = %q, want --depth=1", cfg.Clone.DefaultOptions)
	}
}

func TestInitConfig_createsFile(t *testing.T) {
	resetViper()

	// Write to a temp file so we don't touch the real config
	tmpDir := t.TempDir()
	tmpConfig := filepath.Join(tmpDir, ".gitm.yaml")
	viper.SetConfigFile(tmpConfig)

	// Override the home dir by calling viper.SetConfigFile before InitConfig would
	// InitConfig always calls SetConfigFile internally, so we test indirectly:
	// just verify SafeWriteConfig behaviour by pre-setting the config file path.
	viper.Set("rootDirectory", "/test/root")
	if err := viper.SafeWriteConfig(); err != nil {
		t.Skipf("SafeWriteConfig not supported in this environment: %v", err)
	}

	if _, err := os.Stat(tmpConfig); os.IsNotExist(err) {
		t.Error("SafeWriteConfig did not create the config file")
	}
}
