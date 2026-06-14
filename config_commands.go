package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the tod config file",
	// Override the root PersistentPreRunE so config subcommands can run even
	// when the file is missing or contains invalid values.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [server-url|access-token|trust-certs-file] [value]",
	Short: "Create or update the tod config file",
	Long: `Create or update the tod config file.

When invoked without arguments, you are prompted for each property in turn.
The current server URL (if any) is shown in square brackets as a default —
press Enter to keep it, or type a new value to replace it. The access token
prompt is always blank for security; press Enter on an empty line to keep
the existing token, or type a new one to replace it. The trust certs file is
optional; leave it blank to skip adding custom trusted certificates.

To update a single property without prompts (for scripts), pass the
property name and value positionally:

  tod config set server-url https://onedev.example.com
  tod config set access-token your-personal-access-token
  tod config set trust-certs-file /path/to/trust-certs.pem

The config file is written to the first existing file in the standard
search order ($XDG_CONFIG_HOME/tod/config, ~/.config/tod/config). If
neither exists, ~/.config/tod/config (or $XDG_CONFIG_HOME/tod/config when
set) is created.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("accepts either no arguments or exactly 2 arguments: property name and value")
		}
		return validateConfigPropertyName(args[0])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 2 {
			return setConfigProperty(args[0], args[1])
		}
		return setFullConfig()
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [server-url|access-token|trust-certs-file]",
	Short: "Print the active configuration (access token is always redacted)",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("accepts at most 1 argument: property name")
		}
		if len(args) == 1 {
			return validateConfigPropertyName(args[0])
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := LoadConfig()
		if err != nil {
			return err
		}

		if len(args) == 1 {
			fmt.Println(configPropertyValue(c, args[0]))
			return nil
		}

		path, _ := findConfigFile()
		fmt.Fprintf(os.Stderr, "# config file: %s\n", path)
		fmt.Printf("%s=%s\n", serverUrlKey, c.ServerUrl)
		fmt.Printf("%s=%s\n", accessTokenKey, configPropertyValue(c, accessTokenKey))
		fmt.Printf("%s=%s\n", trustCertsFileKey, c.TrustCertsFile)
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path of the tod config file",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := findConfigFile()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	},
}

func promptValue(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func setFullConfig() error {
	targetPath, err := findConfigFile()
	if err != nil {
		return err
	}

	existing, err := loadConfigFile(targetPath)
	if err != nil {
		return err
	}

	serverUrl := existing.ServerUrl
	accessToken := existing.AccessToken
	trustCertsFile := existing.TrustCertsFile

	reader := bufio.NewReader(os.Stdin)

	var prompt string
	if serverUrl != "" {
		prompt = fmt.Sprintf("OneDev server URL [%s]: ", serverUrl)
	} else {
		prompt = "OneDev server URL (e.g. https://onedev.example.com): "
	}
	value, err := promptValue(reader, prompt)
	if err != nil {
		return err
	}
	if value != "" {
		serverUrl = value
	}
	serverUrl, err = normalizeConfigProperty(serverUrlKey, serverUrl)
	if err != nil {
		return err
	}

	if accessToken != "" {
		prompt = "OneDev personal access token (press Enter to keep existing): "
	} else {
		prompt = "OneDev personal access token (input is visible): "
	}
	value, err = promptValue(reader, prompt)
	if err != nil {
		return err
	}
	if value != "" {
		accessToken = value
	}
	accessToken, err = normalizeConfigProperty(accessTokenKey, accessToken)
	if err != nil {
		return err
	}

	if trustCertsFile != "" {
		prompt = fmt.Sprintf("Trust certs file [%s]: ", trustCertsFile)
	} else {
		prompt = "Trust certs file (optional, press Enter to skip): "
	}
	value, err = promptValue(reader, prompt)
	if err != nil {
		return err
	}
	if value != "" {
		trustCertsFile = value
	}
	trustCertsFile, err = normalizeConfigProperty(trustCertsFileKey, trustCertsFile)
	if err != nil {
		return err
	}

	for _, stale := range allConfigFilePaths() {
		if stale == targetPath {
			continue
		}
		if _, err := os.Stat(stale); err == nil {
			if err := os.Remove(stale); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", stale, err)
			} else {
				fmt.Fprintf(os.Stderr, "Removed existing config at %s\n", stale)
			}
		}
	}

	if err := writeConfigFile(targetPath, serverUrl, accessToken, trustCertsFile); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Wrote config to %s\n", targetPath)
	return nil
}

func setConfigProperty(propertyName, propertyValue string) error {
	targetPath, err := findConfigFile()
	if err != nil {
		return err
	}

	config, err := loadConfigFile(targetPath)
	if err != nil {
		return err
	}
	normalizedValue, err := normalizeConfigProperty(propertyName, propertyValue)
	if err != nil {
		return err
	}

	switch propertyName {
	case serverUrlKey:
		config.ServerUrl = normalizedValue
	case accessTokenKey:
		config.AccessToken = normalizedValue
	case trustCertsFileKey:
		config.TrustCertsFile = normalizedValue
	default:
		return validateConfigPropertyName(propertyName)
	}

	if err := writeConfigFile(targetPath, config.ServerUrl, config.AccessToken, config.TrustCertsFile); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Wrote %s to %s\n", propertyName, targetPath)
	return nil
}

func writeConfigFile(path, serverUrl, accessToken, trustCertsFile string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to create config directory %s: %w", dir, err)
		}
	}

	contents := fmt.Sprintf("%s=%s\n%s=%s\n", serverUrlKey, serverUrl, accessTokenKey, accessToken)
	if trustCertsFile != "" {
		contents += fmt.Sprintf("%s=%s\n", trustCertsFileKey, trustCertsFile)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}
	return nil
}

func validateConfigPropertyName(propertyName string) error {
	switch propertyName {
	case serverUrlKey, accessTokenKey, trustCertsFileKey:
		return nil
	default:
		return fmt.Errorf("unknown config property %q (expected %q, %q, or %q)", propertyName, serverUrlKey, accessTokenKey, trustCertsFileKey)
	}
}

func normalizeConfigProperty(propertyName, propertyValue string) (string, error) {
	value := strings.TrimSpace(propertyValue)
	if value == "" && propertyName != trustCertsFileKey {
		return "", fmt.Errorf("%s must not be empty", propertyName)
	}

	switch propertyName {
	case serverUrlKey:
		value = strings.TrimRight(value, "/")
		if !(strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")) {
			return "", fmt.Errorf("invalid server url (must start with http:// or https://): %s", value)
		}
	case accessTokenKey:
	case trustCertsFileKey:
		if value != "" {
			info, err := os.Stat(value)
			if err != nil {
				return "", fmt.Errorf("invalid trust-certs-file %q: %w", value, err)
			}
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("invalid trust-certs-file %q: not a regular file", value)
			}
			value = filepath.Clean(value)
		}
	default:
		return "", validateConfigPropertyName(propertyName)
	}
	return value, nil
}

func configPropertyValue(config *Config, propertyName string) string {
	switch propertyName {
	case serverUrlKey:
		return config.ServerUrl
	case accessTokenKey:
		return redactToken(config.AccessToken)
	case trustCertsFileKey:
		return config.TrustCertsFile
	default:
		return ""
	}
}

func redactToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + strings.Repeat("*", len(token)-8) + token[len(token)-4:]
}

func initConfigCommands() {
	configCmd.AddCommand(configSetCmd, configGetCmd, configPathCmd)
}
