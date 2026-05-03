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
	Use:   "set [server-url|access-token] [value]",
	Short: "Create or update the tod config file",
	Long: `Create or update the tod config file.

When invoked without arguments, you are prompted for each property in turn.
The current server URL (if any) is shown in square brackets as a default —
press Enter to keep it, or type a new value to replace it. The access token
prompt is always blank for security; press Enter on an empty line to keep
the existing token, or type a new one to replace it.

If --server-url or --access-token is passed, the value from the flag is used
without prompting. With --non-interactive, no prompts are shown and any
missing value causes an error.

To update a single property, pass the property name and value:

  tod config set server-url https://onedev.example.com
  tod config set access-token your-personal-access-token

The default destination is the first existing file in the standard search
order ($XDG_CONFIG_HOME/tod/config, ~/.config/tod/config, ~/.todconfig). If
none exists, ~/.config/tod/config is created. Override with --path.`,
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
			return setConfigProperty(cmd, args[0], args[1])
		}
		return setFullConfig(cmd)
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [server-url|access-token]",
	Short: "Print the active configuration (access token redacted by default)",
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
		showToken, _ := cmd.Flags().GetBool("show-token")

		c, err := LoadConfig()
		if err != nil {
			return err
		}

		if len(args) == 1 {
			fmt.Println(configPropertyValue(c, args[0], showToken))
			return nil
		}

		path, _ := findConfigFile()
		fmt.Fprintf(os.Stderr, "# config file: %s\n", path)
		fmt.Printf("%s=%s\n", serverUrlKey, c.ServerUrl)
		fmt.Printf("%s=%s\n", accessTokenKey, configPropertyValue(c, accessTokenKey, showToken))
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

func setFullConfig(cmd *cobra.Command) error {
	serverUrlFlag, _ := cmd.Flags().GetString("server-url")
	accessTokenFlag, _ := cmd.Flags().GetString("access-token")
	targetPath, _ := cmd.Flags().GetString("path")
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

	if targetPath == "" {
		path, err := findConfigFile()
		if err != nil {
			return err
		}
		targetPath = path
	}

	existing, err := loadConfigFile(targetPath)
	if err != nil {
		return err
	}

	serverUrl := serverUrlFlag
	if serverUrl == "" {
		serverUrl = existing.ServerUrl
	}
	accessToken := accessTokenFlag
	if accessToken == "" {
		accessToken = existing.AccessToken
	}

	reader := bufio.NewReader(os.Stdin)

	// Prompt for server URL unless --server-url was passed explicitly. In
	// interactive mode, always prompt — pre-filling the existing value as a
	// default in square brackets so the user can keep it by pressing Enter.
	if serverUrlFlag == "" {
		if nonInteractive {
			if serverUrl == "" {
				return fmt.Errorf("--server-url is required in non-interactive mode")
			}
		} else {
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
		}
	}
	serverUrl, err = normalizeConfigProperty(serverUrlKey, serverUrl)
	if err != nil {
		return err
	}

	// Prompt for access token unless --access-token was passed explicitly. The
	// prompt is always blank — never echo the existing token. Pressing Enter
	// on an empty line keeps the existing value when one is present.
	if accessTokenFlag == "" {
		if nonInteractive {
			if accessToken == "" {
				return fmt.Errorf("--access-token is required in non-interactive mode")
			}
		} else {
			var prompt string
			if accessToken != "" {
				prompt = "OneDev personal access token (press Enter to keep existing): "
			} else {
				prompt = "OneDev personal access token (input is visible): "
			}
			value, err := promptValue(reader, prompt)
			if err != nil {
				return err
			}
			if value != "" {
				accessToken = value
			}
		}
	}
	accessToken, err = normalizeConfigProperty(accessTokenKey, accessToken)
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

	if err := writeConfigFile(targetPath, serverUrl, accessToken); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Wrote config to %s\n", targetPath)
	return nil
}

func setConfigProperty(cmd *cobra.Command, propertyName, propertyValue string) error {
	targetPath, _ := cmd.Flags().GetString("path")
	if targetPath == "" {
		path, err := findConfigFile()
		if err != nil {
			return err
		}
		targetPath = path
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
	default:
		return validateConfigPropertyName(propertyName)
	}

	if err := writeConfigFile(targetPath, config.ServerUrl, config.AccessToken); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Wrote %s to %s\n", propertyName, targetPath)
	return nil
}

func writeConfigFile(path, serverUrl, accessToken string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to create config directory %s: %w", dir, err)
		}
	}

	contents := fmt.Sprintf("%s=%s\n%s=%s\n", serverUrlKey, serverUrl, accessTokenKey, accessToken)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}
	return nil
}

func validateConfigPropertyName(propertyName string) error {
	switch propertyName {
	case serverUrlKey, accessTokenKey:
		return nil
	default:
		return fmt.Errorf("unknown config property %q (expected %q or %q)", propertyName, serverUrlKey, accessTokenKey)
	}
}

func normalizeConfigProperty(propertyName, propertyValue string) (string, error) {
	value := strings.TrimSpace(propertyValue)
	if value == "" {
		return "", fmt.Errorf("%s must not be empty", propertyName)
	}

	switch propertyName {
	case serverUrlKey:
		value = strings.TrimRight(value, "/")
		if !(strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")) {
			return "", fmt.Errorf("invalid server url (must start with http:// or https://): %s", value)
		}
	case accessTokenKey:
	default:
		return "", validateConfigPropertyName(propertyName)
	}
	return value, nil
}

func configPropertyValue(config *Config, propertyName string, showToken bool) string {
	switch propertyName {
	case serverUrlKey:
		return config.ServerUrl
	case accessTokenKey:
		if showToken {
			return config.AccessToken
		}
		return redactToken(config.AccessToken)
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
	configSetCmd.Flags().String("server-url", "", "OneDev server URL (e.g. https://onedev.example.com)")
	configSetCmd.Flags().String("access-token", "", "OneDev personal access token")
	configSetCmd.Flags().String("path", "", "Path of config file to write (defaults to existing file or ~/.config/tod/config)")
	configSetCmd.Flags().Bool("non-interactive", false, "Fail instead of prompting when a value is missing")

	configGetCmd.Flags().Bool("show-token", false, "Print the access token in plain text instead of redacting it")

	configCmd.AddCommand(configSetCmd, configGetCmd, configPathCmd)
}
