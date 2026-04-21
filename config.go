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

// configSearchPaths returns all candidate config file paths, in priority order:
//  1. $XDG_CONFIG_HOME/tod/config (if XDG_CONFIG_HOME is set)
//  2. ~/.config/tod/config
//  3. ~/.todconfig (legacy)
func configSearchPaths() []string {
	var paths []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "tod", "config"))
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(homeDir, ".config", "tod", "config"))
		paths = append(paths, filepath.Join(homeDir, ".todconfig"))
	}
	return paths
}

// findConfigFile returns the first existing config file, or the legacy path as fallback
func findConfigFile() (string, error) {
	paths := configSearchPaths()
	if len(paths) == 0 {
		return "", fmt.Errorf("failed to determine config file location (is $HOME set?)")
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return paths[len(paths)-1], nil
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

	if config.ServerUrl == "" || config.AccessToken == "" {
		if _, err := os.Stat(configFilePath); err != nil {
			return fmt.Errorf("no config file found (searched: %s)", strings.Join(configSearchPaths(), ", "))
		}
		missing := serverUrlKey
		if config.ServerUrl != "" {
			missing = accessTokenKey
		}
		var altPaths []string
		for _, p := range configSearchPaths() {
			if p != configFilePath {
				altPaths = append(altPaths, p)
			}
		}
		hint := ""
		if len(altPaths) > 0 {
			hint = " (also searched: " + strings.Join(altPaths, ", ") + ")"
		}
		return fmt.Errorf("missing setting '%s' in %s%s", missing, configFilePath, hint)
	}

	// Validate server URL format: must start with http:// or https://, and trim trailing slash
	if !(strings.HasPrefix(config.ServerUrl, "http://") || strings.HasPrefix(config.ServerUrl, "https://")) {
		return fmt.Errorf("invalid server url (must start with http:// or https://): %s", config.ServerUrl)
	}
	// Trim trailing slash if present
	config.ServerUrl = strings.TrimRight(config.ServerUrl, "/")

	return nil
}
