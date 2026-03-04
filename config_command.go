package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/ini.v1"
)

// ConfigShowCommand shows all config values
func ConfigShowCommand(cobraCmd *cobra.Command, args []string) {
	configFilePath, err := findConfigFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to find config file:", err)
		os.Exit(1)
	}

	fmt.Printf("Config file: %s\n\n", configFilePath)

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		fmt.Println("(file does not exist)")
		return
	}

	cfg, err := ini.Load(configFilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read config:", err)
		os.Exit(1)
	}

	for _, key := range cfg.Section("").Keys() {
		value := key.String()
		if key.Name() == accessTokenKey && len(value) > 8 {
			value = value[:4] + "..." + value[len(value)-4:]
		}
		fmt.Printf("%-15s = %s\n", key.Name(), value)
	}
}

// ConfigGetCommand gets a specific config value
func ConfigGetCommand(cobraCmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Exactly one key name is required")
		os.Exit(1)
	}

	configFilePath, err := findConfigFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to find config file:", err)
		os.Exit(1)
	}

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Config file not found: %s\n", configFilePath)
		os.Exit(1)
	}

	cfg, err := ini.Load(configFilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read config:", err)
		os.Exit(1)
	}

	value := cfg.Section("").Key(args[0]).String()
	if value == "" {
		fmt.Fprintf(os.Stderr, "Key '%s' not found in config\n", args[0])
		os.Exit(1)
	}

	fmt.Println(value)
}

// ConfigSetCommand sets a config value
func ConfigSetCommand(cobraCmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Exactly one key=value pair is required")
		os.Exit(1)
	}

	parts := strings.SplitN(args[0], "=", 2)
	if len(parts) != 2 {
		fmt.Fprintln(os.Stderr, "Format must be key=value")
		os.Exit(1)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Determine config file path - prefer existing, otherwise use XDG default
	configFilePath, err := findConfigFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to find config file:", err)
		os.Exit(1)
	}

	// If file doesn't exist, create at XDG location
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		homeDir, _ := os.UserHomeDir()
		configFilePath = filepath.Join(homeDir, ".config", "tod", "config")
		if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to create config directory:", err)
			os.Exit(1)
		}
	}

	var cfg *ini.File
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		cfg = ini.Empty()
	} else {
		cfg, err = ini.Load(configFilePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to read config:", err)
			os.Exit(1)
		}
	}

	cfg.Section("").Key(key).SetValue(value)

	if err := cfg.SaveTo(configFilePath); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to save config:", err)
		os.Exit(1)
	}

	fmt.Printf("Set '%s' in %s\n", key, configFilePath)
}

// ConfigPathCommand shows the config file path
func ConfigPathCommand(cobraCmd *cobra.Command, args []string) {
	configFilePath, err := findConfigFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to find config file:", err)
		os.Exit(1)
	}
	fmt.Println(configFilePath)
}
