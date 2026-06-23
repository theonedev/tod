package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

const (
	serverUrlKey                 = "server-url"
	accessTokenKey               = "access-token"
	trustCertsFileKey            = "trust-certs-file"
	serverUrlEnvironmentKey      = "ONEDEV_SERVER_URL"
	accessTokenEnvironmentKey    = "ONEDEV_ACCESS_TOKEN"
	trustCertsFileEnvironmentKey = "ONEDEV_TRUST_CERTS_FILE"
)

// Config holds configuration values shared across all commands
type Config struct {
	ServerUrl      string
	AccessToken    string
	TrustCertsFile string
}

// allConfigFilePaths returns every location where a config file could exist,
// in search order. Used by `config set` to clean up stale files.
func allConfigFilePaths() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var paths []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "tod", "config"))
	}
	paths = append(paths, filepath.Join(homeDir, ".config", "tod", "config"))
	return paths
}

// findConfigFile returns the path of the config file. It first searches for
// an existing file, in order:
//  1. $XDG_CONFIG_HOME/tod/config (if XDG_CONFIG_HOME is set)
//  2. ~/.config/tod/config
//
// If none exists, it falls back to the preferred XDG location:
// $XDG_CONFIG_HOME/tod/config when set, otherwise ~/.config/tod/config. This
// is also the path `tod config set` writes to by default.
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

	config, err := loadConfigFile(configFilePath)
	if err != nil {
		return nil, err
	}
	applyConfigEnvironmentOverrides(config)
	return config, nil
}

func loadConfigFile(configFilePath string) (*Config, error) {
	config := &Config{}

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return config, nil
	}

	cfg, err := ini.Load(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFilePath, err)
	}

	// Read from top-level (default section)
	defaultSec := cfg.Section("")
	config.ServerUrl = defaultSec.Key(serverUrlKey).String()
	config.AccessToken = defaultSec.Key(accessTokenKey).String()
	config.TrustCertsFile = defaultSec.Key(trustCertsFileKey).String()
	return config, nil
}

func applyConfigEnvironmentOverrides(config *Config) {
	if serverUrl, ok := os.LookupEnv(serverUrlEnvironmentKey); ok {
		config.ServerUrl = serverUrl
	}
	if accessToken, ok := os.LookupEnv(accessTokenEnvironmentKey); ok {
		config.AccessToken = accessToken
	}
	if trustCertsFile, ok := os.LookupEnv(trustCertsFileEnvironmentKey); ok {
		config.TrustCertsFile = trustCertsFile
	}
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

	normalizedServerUrl, err := normalizeServerURL(config.ServerUrl)
	if err != nil {
		return err
	}
	config.ServerUrl = normalizedServerUrl

	if config.TrustCertsFile != "" {
		info, err := os.Stat(config.TrustCertsFile)
		if err != nil {
			return fmt.Errorf("invalid trust-certs-file %q: %w", config.TrustCertsFile, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("invalid trust-certs-file %q: not a regular file", config.TrustCertsFile)
		}
	}

	return nil
}

func normalizeServerURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !(strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")) {
		return "", fmt.Errorf("invalid server url (must start with http:// or https://): %s", value)
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid server url: %w", err)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid server url (must include a host): %s", value)
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return "", fmt.Errorf("invalid server url (must include a port after colon): %s", value)
	}
	if path := strings.TrimRight(parsed.Path, "/"); path != "" {
		return "", fmt.Errorf("invalid server url (must not contain a path): %s", value)
	}

	return parsed.Scheme + "://" + parsed.Host, nil
}
