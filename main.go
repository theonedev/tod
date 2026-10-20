package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "4.4.0"

const minRequiredServerVersion = "16.9.5"

var config *Config

var checkedVersionInfo VersionInfo

var rootCmd = &cobra.Command{
	Use:   "tod",
	Short: "TOD (TheOneDev) is a command line tool for OneDev " + minRequiredServerVersion + "+",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "version" {
			return nil
		}

		var err error
		config, err = LoadConfig()
		if err != nil {
			return err
		}

		if err := config.Validate(); err != nil {
			return err
		}

		if checkedVersionInfo, err = checkVersion(config); err != nil {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		return nil
	},
}

func init() {
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
		downloadCmd,
		versionCmd,
		checkVersionCmd,
	)

	suppressUsageForRunErrors(rootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
