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

// findConfigFile returns the path to the config file, searching in order:
//  1. $XDG_CONFIG_HOME/tod/config (if XDG_CONFIG_HOME is set)
//  2. ~/.config/tod/config
//  3. ~/.todconfig (legacy)
func findConfigFile() (string, error) {
	// Try $XDG_CONFIG_HOME/tod/config if explicitly set
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidate := filepath.Join(xdg, "tod", "config")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Try ~/.config/tod/config (XDG default)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	candidate := filepath.Join(homeDir, ".config", "tod", "config")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}

	// Fall back to legacy ~/.todconfig
	return filepath.Join(homeDir, ".todconfig"), nil
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
