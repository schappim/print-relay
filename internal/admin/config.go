package admin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the admin TUI configuration
type Config struct {
	ServerURL    string `yaml:"server" json:"server"`
	APIKey       string `yaml:"apiKey" json:"apiKey"`
	AdminKey     string `yaml:"adminKey" json:"adminKey"`
	MonitorToken string `yaml:"monitorToken" json:"monitorToken"`
}

// Validate checks that required fields are set
func (c *Config) Validate() error {
	if c.ServerURL == "" {
		return fmt.Errorf("server URL is required")
	}
	if c.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	return nil
}

// LoadConfig loads configuration from a file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// FindConfigFile looks for a config file in standard locations
func FindConfigFile() string {
	// Check common locations
	locations := []string{
		"./printrelay-admin.yaml",
		"./printrelay-admin.yml",
	}

	// Add home directory locations
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations,
			filepath.Join(home, ".config", "printrelay", "admin.yaml"),
			filepath.Join(home, ".config", "printrelay", "admin.yml"),
			filepath.Join(home, ".printrelay-admin.yaml"),
		)
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}
