package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Interact with OneDev projects",
}

var projectCurrentCmd = &cobra.Command{
	Use:   "current",
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

var projectGetCmd = &cobra.Command{
	Use:   "get <project-path>",
	Short: "Print info of the specified project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-project", url.Values{"project": {args[0]}})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

func initProjectCommands() {
	projectCurrentCmd.Flags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")

	projectCmd.AddCommand(
		projectCurrentCmd,
		projectGetCmd,
	)
}
