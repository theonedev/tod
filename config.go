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

// findConfigFile returns the path of the config file. It first searches for
// an existing file, in order:
//  1. $XDG_CONFIG_HOME/tod/config (if XDG_CONFIG_HOME is set)
//  2. ~/.config/tod/config
//  3. ~/.todconfig (legacy)
//
// If none exists, it falls back to the preferred XDG location:
// $XDG_CONFIG_HOME/tod/config when set, otherwise ~/.config/tod/config. This
// is also the path `tod config init` writes to by default.
func findConfigFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	xdg := os.Getenv("XDG_CONFIG_HOME")

	if xdg != "" {
		candidate := filepath.Join(xdg, "tod", "config")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	xdgDefault := filepath.Join(homeDir, ".config", "tod", "config")
	if _, err := os.Stat(xdgDefault); err == nil {
		return xdgDefault, nil
	}

	legacy := filepath.Join(homeDir, ".todconfig")
	if _, err := os.Stat(legacy); err == nil {
		return legacy, nil
	}

	if xdg != "" {
		return filepath.Join(xdg, "tod", "config"), nil
	}
	return xdgDefault, nil
}

// LoadConfig loads common configuration from the config file
func LoadConfig() (*Config, error) {
	configFilePath, err := findConfigFile()
	if err != nil {
		return nil, err
	}

	config := &Config{}

	if _, err := os.Stat(configFilePath); !os.IsNotExist(err) {
		cfg, err := ini.Load(configFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", configFilePath, err)
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
	configFilePath, _ := findConfigFile()

	if config.ServerUrl == "" {
		return fmt.Errorf("missing setting 'server-url' in %s", configFilePath)
	}
	if config.AccessToken == "" {
		return fmt.Errorf("missing setting 'access-token' in %s", configFilePath)
	}

	// Validate server URL format: must start with http:// or https://, and trim trailing slash
	if !(strings.HasPrefix(config.ServerUrl, "http://") || strings.HasPrefix(config.ServerUrl, "https://")) {
		return fmt.Errorf("invalid server url (must start with http:// or https://): %s", config.ServerUrl)
	}
	// Trim trailing slash if present
	config.ServerUrl = strings.TrimRight(config.ServerUrl, "/")

	return nil
}
