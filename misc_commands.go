package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var getLoginNameCmd = &cobra.Command{
	Use:   "get-login-name",
	Short: "Get the OneDev login name of the current user (or of --user)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		userName, _ := cmd.Flags().GetString("user")
		body, err := apiGetBytes("get-login-name", url.Values{"userName": {userName}})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var getUnixTimestampCmd = &cobra.Command{
	Use:   "get-unix-timestamp <datetime-description>",
	Short: "Convert a natural-language datetime description to a Unix timestamp (milliseconds)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-unix-timestamp", url.Values{"dateTimeDescription": {args[0]}})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Print the git remote that points at the inferred OneDev project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, _, err := inferProject(workingDirOf(cmd))
		if err != nil {
			return err
		}
		fmt.Println(remote)
		return nil
	},
}

var getValidLabelsCmd = &cobra.Command{
	Use:   "get-valid-labels",
	Short: "Print valid label names for this OneDev server",
	Long: `Print valid label names for this OneDev server. Use this to
discover which label names are accepted by --label when running
'tod pr create'.

The list is fetched from the OneDev server endpoint
get-valid-labels.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-valid-labels", nil)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var getCommitMessageRequirementCmd = &cobra.Command{
	Use:   "get-commit-message-requirement",
	Short: "Print commit message requirement for non-merge commits",
	Long: `Print commit message requirement for non-merge commits in a project.
The project is inferred from the current git repository's OneDev project.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := apiGetBytes("get-commit-message-requirement", url.Values{"project": {project}})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

func initMiscCommands() {
	getLoginNameCmd.Flags().String("user", "", "User name (defaults to the current user)")

	remoteCmd.Flags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")
	getCommitMessageRequirementCmd.Flags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")
}
