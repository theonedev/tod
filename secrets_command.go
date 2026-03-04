package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type SecretsCommand struct {
}

func (command SecretsCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	project, _ := cobraCmd.Flags().GetString("project")
	if project == "" {
		// Try to infer from current directory
		_, inferredProject, err := inferProject(".", logger)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not infer project. Use --project flag or run from a git repo with OneDev remote.")
			os.Exit(1)
		}
		project = inferredProject
	}

	// Get project ID
	projectId, err := getProjectId(project)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get project:", err)
		os.Exit(1)
	}

	// Get project settings
	apiURL := config.ServerUrl + fmt.Sprintf("/~api/projects/%d/setting", projectId)
	body, err := makeAPICallSimple("GET", apiURL, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get project settings:", err)
		os.Exit(1)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(body, &settings); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse settings:", err)
		os.Exit(1)
	}

	buildSetting, _ := settings["buildSetting"].(map[string]interface{})
	if buildSetting == nil {
		fmt.Println("No build settings found.")
		return
	}

	secrets, _ := buildSetting["jobSecrets"].([]interface{})
	if len(secrets) == 0 {
		fmt.Printf("No job secrets in project '%s'.\n", project)
		return
	}

	fmt.Printf("Job secrets for project '%s':\n\n", project)
	for _, s := range secrets {
		secret, _ := s.(map[string]interface{})
		name, _ := secret["name"].(string)
		auth, _ := secret["authorization"].(string)
		fmt.Printf("  %-30s  authorization: %s\n", name, auth)
	}
}
