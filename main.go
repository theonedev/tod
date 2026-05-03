package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "2.1.1"

type CompatibleVersions struct {
	MinVersion string `json:"minVersion"`
	MaxVersion string `json:"maxVersion"`
}

var config *Config

var rootCmd = &cobra.Command{
	Use:   "tod",
	Short: "TOD (TheOneDev) is a command line tool for OneDev 13+",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		config, err = LoadConfig()
		if err != nil {
			return err
		}

		if err := config.Validate(); err != nil {
			return err
		}

		if err := checkVersion(config.ServerUrl, config.AccessToken); err != nil {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		return nil
	},
}

// Hidden legacy top-level commands kept so existing scripts continue working.
// Prefer the grouped equivalents (`tod build run`, `tod build run-local`,
// `tod pr checkout`).

var runLocalJobCmd = &cobra.Command{
	Use:    "run-local <job-name>",
	Short:  "[deprecated] alias for `tod build run-local`",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLocalBuildJobCommand(cmd, args, cliLogger("[run-local] "))
	},
}

var runJobCmd = &cobra.Command{
	Use:    "run <job-name>",
	Short:  "[deprecated] alias for `tod build run`",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildJobCommand(cmd, args, cliLogger("[run] "))
	},
}

var checkoutPullRequestCmd = &cobra.Command{
	Use:    "checkout <pull-request-reference>",
	Short:  "[deprecated] alias for `tod pr checkout`",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := cliLogger("[checkout] ")
		if err := checkoutPullRequest(workingDirOf(cmd), args[0], logger); err != nil {
			return fmt.Errorf("failed to checkout pull request: %v", err)
		}
		logger.Printf("Checked out pull request %s successfully", args[0])
		return nil
	},
}

func init() {
	runLocalJobCmd.Flags().String("working-dir", "", "Specify working directory to run job against (defaults to current directory)")
	runLocalJobCmd.Flags().StringArrayP("param", "p", nil, "Specify job parameters in form of key=value (can be used multiple times)")

	runJobCmd.Flags().String("branch", "", "Specify branch to run job against (either --branch or --tag is required)")
	runJobCmd.Flags().String("tag", "", "Specify tag to run job against (either --branch or --tag is required)")
	runJobCmd.Flags().StringArrayP("param", "p", nil, "Specify job parameters in form of key=value (can be used multiple times)")

	checkoutPullRequestCmd.Flags().String("working-dir", "", "Specify working directory to checkout pull request against (defaults to current directory)")

	initIssueCommands()
	initPullRequestCommands()
	initBuildCommands()
	initPackCommands()
	initMiscCommands()
	initSchemaCommands()
	initConfigCommands()

	rootCmd.AddCommand(
		issueCmd,
		prCmd,
		buildCmd,
		packCmd,
		schemaCmd,
		configCmd,
		whoamiCmd,
		unixTimeCmd,
		projectCmd,
		remoteCmd,
	)

	rootCmd.AddCommand(
		runLocalJobCmd,
		runJobCmd,
		checkoutPullRequestCmd,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
