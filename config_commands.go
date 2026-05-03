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

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create or update the tod config file with server URL and access token",
	Long: `Create or update the tod config file.

If --server-url or --access-token is omitted, the existing value (if any) is
kept. When still missing, you are prompted interactively unless
--non-interactive is set.

The default destination is the first existing file in the standard search
order ($XDG_CONFIG_HOME/tod/config, ~/.config/tod/config, ~/.todconfig). If
none exists, ~/.config/tod/config is created. Override with --path.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverUrl, _ := cmd.Flags().GetString("server-url")
		accessToken, _ := cmd.Flags().GetString("access-token")
		targetPath, _ := cmd.Flags().GetString("path")
		nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

		existing, _ := LoadConfig()
		if existing != nil {
			if serverUrl == "" {
				serverUrl = existing.ServerUrl
			}
			if accessToken == "" {
				accessToken = existing.AccessToken
			}
		}

		reader := bufio.NewReader(os.Stdin)

		if serverUrl == "" {
			if nonInteractive {
				return fmt.Errorf("--server-url is required in non-interactive mode")
			}
			value, err := promptValue(reader, "OneDev server URL (e.g. https://onedev.example.com): ")
			if err != nil {
				return err
			}
			serverUrl = value
		}
		serverUrl = strings.TrimRight(strings.TrimSpace(serverUrl), "/")
		if serverUrl == "" {
			return fmt.Errorf("server-url must not be empty")
		}
		if !(strings.HasPrefix(serverUrl, "http://") || strings.HasPrefix(serverUrl, "https://")) {
			return fmt.Errorf("invalid server url (must start with http:// or https://): %s", serverUrl)
		}

		if accessToken == "" {
			if nonInteractive {
				return fmt.Errorf("--access-token is required in non-interactive mode")
			}
			value, err := promptValue(reader, "OneDev personal access token (input is visible): ")
			if err != nil {
				return err
			}
			accessToken = value
		}
		accessToken = strings.TrimSpace(accessToken)
		if accessToken == "" {
			return fmt.Errorf("access-token must not be empty")
		}

		if targetPath == "" {
			path, err := findConfigFile()
			if err != nil {
				return err
			}
			targetPath = path
		}

		if err := writeConfigFile(targetPath, serverUrl, accessToken); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Wrote config to %s\n", targetPath)

		if err := checkVersion(serverUrl, accessToken); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to verify config against server: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "Verified configuration against the server.")
		}
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the active configuration (access token redacted by default)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		showToken, _ := cmd.Flags().GetBool("show-token")

		path, _ := findConfigFile()
		fmt.Fprintf(os.Stderr, "# config file: %s\n", path)

		c, err := LoadConfig()
		if err != nil {
			return err
		}
		fmt.Printf("%s=%s\n", serverUrlKey, c.ServerUrl)
		token := c.AccessToken
		if !showToken {
			token = redactToken(token)
		}
		fmt.Printf("%s=%s\n", accessTokenKey, token)
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
	configInitCmd.Flags().String("server-url", "", "OneDev server URL (e.g. https://onedev.example.com)")
	configInitCmd.Flags().String("access-token", "", "OneDev personal access token")
	configInitCmd.Flags().String("path", "", "Path of config file to write (defaults to existing file or ~/.config/tod/config)")
	configInitCmd.Flags().Bool("non-interactive", false, "Fail instead of prompting when a value is missing")

	configShowCmd.Flags().Bool("show-token", false, "Print the access token in plain text instead of redacting it")

	configCmd.AddCommand(configInitCmd, configShowCmd, configPathCmd)
}
