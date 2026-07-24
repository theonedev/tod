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

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "Query accessible OneDev projects",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")
		offset, _ := cmd.Flags().GetInt("offset")
		count, _ := cmd.Flags().GetInt("count")

		body, err := apiGetBytes("query-projects", url.Values{
			"query":  {query},
			"offset": {fmt.Sprintf("%d", offset)},
			"count":  {fmt.Sprintf("%d", count)},
		})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
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

var projectGetQueryDescriptionCmd = &cobra.Command{
	Use:   "get-query-description",
	Short: "Get the OneDev project query DSL description (DSL for `--query` of `project list`)",
	Long: `Get the OneDev project query DSL description so you know what syntax
'tod project list --query' accepts (operators, ordering, the set of supported
field/criteria keys, etc.).

The description is fetched from the OneDev server endpoint
/~api/tod/get-project-query-description, which returns the canonical project
query syntax reference for this server.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-project-query-description", nil)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

func initProjectCommands() {
	projectCurrentCmd.Flags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")

	projectListCmd.Flags().String("query", "", "OneDev project query string (run 'tod project get-query-description' for the supported query DSL)")
	projectListCmd.Flags().Int("offset", 0, "start position for the query (optional, defaults to 0)")
	projectListCmd.Flags().Int("count", DefaultQueryCount, fmt.Sprintf("number of projects to return (optional, defaults to %d, max %d)", DefaultQueryCount, MaxQueryCount))

	projectCmd.AddCommand(
		projectListCmd,
		projectCurrentCmd,
		projectGetCmd,
		projectGetQueryDescriptionCmd,
	)
}
