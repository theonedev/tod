package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type SettingsCommand struct {
}

func (command SettingsCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	project, _ := cobraCmd.Flags().GetString("project")
	if project == "" {
		_, inferredProject, err := inferProject(".", logger)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not infer project. Use --project flag or run from a git repo with OneDev remote.")
			os.Exit(1)
		}
		project = inferredProject
	}

	projectId, err := getProjectId(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get project:", err)
		os.Exit(1)
	}

	apiURL := config.ServerUrl + fmt.Sprintf("/~api/projects/%d/setting", projectId)
	body, err := makeAPICallSimple("GET", apiURL, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get project settings:", err)
		os.Exit(1)
	}

	section, _ := cobraCmd.Flags().GetString("section")

	if section != "" {
		// Show specific section
		var settings map[string]interface{}
		if err := json.Unmarshal(body, &settings); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse settings:", err)
			os.Exit(1)
		}

		sectionData, ok := settings[section]
		if !ok {
			fmt.Fprintf(os.Stderr, "Section '%s' not found. Available sections:\n", section)
			for key := range settings {
				fmt.Fprintf(os.Stderr, "  %s\n", key)
			}
			os.Exit(1)
		}

		prettyJSON, err := json.MarshalIndent(sectionData, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to format settings:", err)
			os.Exit(1)
		}
		fmt.Println(string(prettyJSON))
	} else {
		// List available sections
		var settings map[string]interface{}
		if err := json.Unmarshal(body, &settings); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse settings:", err)
			os.Exit(1)
		}

		fmt.Printf("Settings sections for project '%s' (id: %d):\n\n", project, projectId)
		for key := range settings {
			fmt.Printf("  %s\n", key)
		}
		fmt.Println("\nUse --section <name> to view a specific section.")
	}
}
