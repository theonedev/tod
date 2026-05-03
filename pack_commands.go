package main

import (
	"github.com/spf13/cobra"
)

var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Query OneDev packages",
}

var packListCmd = &cobra.Command{
	Use:   "list",
	Short: "Query packages in the current (or a specified) project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		query, _ := cmd.Flags().GetString("query")
		offset, _ := cmd.Flags().GetInt("offset")
		count, _ := cmd.Flags().GetInt("count")

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := queryEntities("query-packs", project, currentProject, query, offset, count)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

func initPackCommands() {
	packCmd.PersistentFlags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")

	packListCmd.Flags().String("project", "", "Project to query (defaults to the current project)")
	packListCmd.Flags().String("query", "", "OneDev package query string (run 'tod schema show query-packs' for valid keys)")
	packListCmd.Flags().Int("offset", 0, "Starting offset")
	packListCmd.Flags().Int("count", DefaultQueryCount, "Number of packages to return")

	packCmd.AddCommand(packListCmd)
}
