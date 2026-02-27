package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the nexctl configuration.
type Config struct {
	ServerURL    string `yaml:"server_url" json:"server_url"`
	AuthToken    string `yaml:"auth_token" json:"auth_token"`
	OutputFormat string `yaml:"output_format" json:"output_format"`
	Context      string `yaml:"context" json:"context"`
}

// DefaultPath returns the default config file path: ~/.nexus/config.yaml
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".nexus", "config.yaml")
	}
	return filepath.Join(home, ".nexus", "config.yaml")
}

// Load reads the configuration from the given YAML file path.
// If the file does not exist, it returns a default Config with no error.
func Load(path string) (*Config, error) {
	cfg := &Config{
		ServerURL:    "http://localhost:9100",
		OutputFormat: "table",
		Context:      "default",
	}

	// Check permissions before reading: warn if the config file is
	// world-readable, since it may contain an auth_token.
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		fmt.Fprintf(os.Stderr,
			"warning: config file %s has permissions %04o — expected 0600. "+
				"Auth tokens may be exposed to other users.\n",
			path, perm)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
