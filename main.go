package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "3.0.0"

const minRequiredServerVersion = "15.1.0"

var config *Config

var rootCmd = &cobra.Command{
	Use:   "tod",
	Short: "TOD (TheOneDev) is a command line tool for OneDev 15.1+",
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
// Prefer the grouped equivalents (`tod build run`, `tod pr checkout`).

var runJobCmd = &cobra.Command{
	Use:    "run <job-name>",
	Short:  "[deprecated] alias for `tod build run`",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildJobCommand(cmd, args)
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
	runJobCmd.Flags().String("branch", "", "Specify branch to run job against (one of --branch, --tag, or --local is required)")
	runJobCmd.Flags().String("tag", "", "Specify tag to run job against (one of --branch, --tag, or --local is required)")
	runJobCmd.Flags().Bool("local", false, "Run the job against local changes without committing/pushing (one of --branch, --tag, or --local is required)")
	runJobCmd.Flags().StringArrayP("param", "p", nil, "Specify job parameters in form of key=value (can be used multiple times)")
	runJobCmd.Flags().String("working-dir", "", "Specify working directory to run job against (defaults to current directory)")

	checkoutPullRequestCmd.Flags().String("working-dir", "", "Specify working directory to checkout pull request against (defaults to current directory)")

	initIssueCommands()
	initPullRequestCommands()
	initCodeCommentCommands()
	initBuildCommands()
	initProjectCommands()
	initMiscCommands()
	initConfigCommands()

	rootCmd.AddCommand(
		issueCmd,
		prCmd,
		codeCommentCmd,
		buildCmd,
		configCmd,
		getLoginNameCmd,
		getUnixTimestampCmd,
		projectCmd,
		remoteCmd,
		getValidLabelsCmd,
		getCommitMessageRequirementCmd,
	)

	rootCmd.AddCommand(
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
