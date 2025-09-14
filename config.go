package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

const (
	serverUrlKey   = "server-url"
	accessTokenKey = "access-token"
)

// Config holds configuration values shared across all commands
type Config struct {
	ServerUrl   string
	AccessToken string
}

// LoadConfig loads common configuration from the config file
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error getting user home: %w", err)
	}

	configFilePath := filepath.Join(homeDir, ".todconfig")
	config := &Config{}

	if _, err := os.Stat(configFilePath); !os.IsNotExist(err) {
		cfg, err := ini.Load(configFilePath)
		if err != nil {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}

		// Read from top-level (default section)
		defaultSec := cfg.Section("")
		config.ServerUrl = defaultSec.Key(serverUrlKey).String()
		config.AccessToken = defaultSec.Key(accessTokenKey).String()
	}

	return config, nil
}

// Validate validates the common configuration
func (config *Config) Validate() error {
	if config.ServerUrl == "" {
		return fmt.Errorf("missing server url. Check https://code.onedev.io/onedev/tod for details")
	}
	if config.AccessToken == "" {
		return fmt.Errorf("missing access token. Check https://code.onedev.io/onedev/tod for details")
	}

	// Validate server URL format: must start with http:// or https://, and trim trailing slash
	if !(strings.HasPrefix(config.ServerUrl, "http://") || strings.HasPrefix(config.ServerUrl, "https://")) {
		return fmt.Errorf("invalid server url (must start with http:// or https://): %s", config.ServerUrl)
	}
	// Trim trailing slash if present
	config.ServerUrl = strings.TrimRight(config.ServerUrl, "/")

	return nil
}
