package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Print the OneDev login name of the current user (or of --user)",
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

var unixTimeCmd = &cobra.Command{
	Use:   "unix-time <datetime-description>",
	Short: "Convert a natural-language datetime description to a unix timestamp (milliseconds)",
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

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Print the OneDev project inferred from the working directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		fmt.Println(project)
		return nil
	},
}

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Print the git remote that points at the inferred OneDev project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, _, err := inferProject(workingDirOf(cmd), cliLogger("[tod] "))
		if err != nil {
			return err
		}
		fmt.Println(remote)
		return nil
	},
}

func initMiscCommands() {
	whoamiCmd.Flags().String("user", "", "User name (defaults to the current user)")

	projectCmd.Flags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")
	remoteCmd.Flags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")
}
