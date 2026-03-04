package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type WebhooksCommand struct {
}

func (command WebhooksCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
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

	// Webhooks are part of project settings
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

	webhooks, _ := settings["webHooks"].([]interface{})
	if len(webhooks) == 0 {
		fmt.Printf("No webhooks in project '%s'.\n", project)
		return
	}

	fmt.Printf("Webhooks for project '%s':\n\n", project)
	for _, w := range webhooks {
		webhook, _ := w.(map[string]interface{})
		postUrl, _ := webhook["postUrl"].(string)
		secret, _ := webhook["secret"].(string)

		secretDisplay := "(none)"
		if secret != "" {
			secretDisplay = secret[:4] + "..."
		}

		events := ""
		if eventTypes, ok := webhook["eventTypes"].([]interface{}); ok {
			for i, e := range eventTypes {
				if i > 0 {
					events += ", "
				}
				events += fmt.Sprintf("%v", e)
			}
		}

		fmt.Printf("  URL:    %s\n", postUrl)
		fmt.Printf("  Secret: %s\n", secretDisplay)
		fmt.Printf("  Events: %s\n\n", events)
	}
}
