package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type ProjectsCommand struct {
}

func (command ProjectsCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	count, _ := cobraCmd.Flags().GetInt("count")

	apiURL := config.ServerUrl + fmt.Sprintf("/~api/projects?offset=0&count=%d", count)

	body, err := makeAPICallSimple("GET", apiURL, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to query projects:", err)
		os.Exit(1)
	}

	var projects []map[string]interface{}
	if err := json.Unmarshal(body, &projects); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse projects:", err)
		os.Exit(1)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return
	}

	for _, project := range projects {
		id := int(project["id"].(float64))
		name, _ := project["name"].(string)
		path, _ := project["path"].(string)
		description, _ := project["description"].(string)

		if description != "" {
			fmt.Printf("%-4d %-20s %-30s %s\n", id, name, path, description)
		} else {
			fmt.Printf("%-4d %-20s %s\n", id, name, path)
		}
	}
}
