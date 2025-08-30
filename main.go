package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

const version = "1.0.0"

type CompatibleVersions struct {
	MinVersion string `json:"minVersion"`
	MaxVersion string `json:"maxVersion"`
}

var config *Config

var rootCmd = &cobra.Command{
	Use:   "tod",
	Short: "TOD (TheOneDev) is a command line tool for OneDev 12.1+",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration from file
		var err error
		config, err = LoadConfig()
		if err != nil {
			return err
		}

		// Override with flags if provided
		if serverUrl, _ := cmd.Flags().GetString("server-url"); serverUrl != "" {
			config.ServerUrl = serverUrl
		}
		if accessToken, _ := cmd.Flags().GetString("access-token"); accessToken != "" {
			config.AccessToken = accessToken
		}

		if err := config.Validate(); err != nil {
			return err
		}

		return nil
	},
}

var runJobCmd = &cobra.Command{
	Use:   "run [job-name]",
	Short: "Run a CI/CD job against local changes",
	Long: `Run a CI/CD job against your local changes without committing/pushing.
This command stashes your local changes, pushes them to a temporal ref,
and streams the job execution logs back to your terminal.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Run job command
		runJobCommand := RunJobCommand{}
		runJobCommand.Execute(cmd, args)
		return nil
	},
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server",
	Long:  `Start the Model Context Protocol server for tool integration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// MCP command
		mcpCommand := MCPCommand{}
		mcpCommand.Execute(cmd, args)
		return nil
	},
}

var checkoutCmd = &cobra.Command{
	Use:   "checkout [pull-request-reference]",
	Short: "Checkout a pull request",
	Long:  `Checkout a pull request by its reference.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Checkout command
		checkoutCommand := CheckoutCommand{}
		// Create a logger that prints to stdout
		logger := log.New(os.Stdout, "[CHECKOUT] ", log.LstdFlags)
		checkoutCommand.Execute(cmd, args, logger)
		return nil
	},
}

func init() {
	// Global persistent flags
	rootCmd.PersistentFlags().String("server-url", "", "Specify OneDev server url, for instance: https://onedev.example.com")
	rootCmd.PersistentFlags().String("access-token", "", "Specify access token for OneDev server")

	// Run command specific flags
	runJobCmd.Flags().String("working-dir", "", "Specify working directory to run job against (defaults to current directory)")
	runJobCmd.Flags().StringArrayP("param", "p", nil, "Specify job parameters in form of key=value (can be used multiple times)")

	// Checkout command specific flags
	checkoutCmd.Flags().String("working-dir", "", "Specify working directory to checkout pull request against (defaults to current directory)")

	// MCP command specific flags
	mcpCmd.Flags().String("log-file", "", "Specify log file path for debug logging")

	// Add commands to root
	rootCmd.AddCommand(runJobCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(checkoutCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
