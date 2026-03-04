package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type AgentsCommand struct {
}

func (command AgentsCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	apiURL := config.ServerUrl + "/~api/settings/job-executors"

	body, err := makeAPICallSimple("GET", apiURL, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to query executors:", err)
		os.Exit(1)
	}

	var executors []map[string]interface{}
	if err := json.Unmarshal(body, &executors); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse executors:", err)
		os.Exit(1)
	}

	if len(executors) == 0 {
		fmt.Println("No job executors configured.")
		return
	}

	for i, executor := range executors {
		name, _ := executor["name"].(string)
		typeName, _ := executor["type"].(string)
		enabled := true
		if e, ok := executor["enabled"].(bool); ok {
			enabled = e
		}

		status := wrapWithGreen("enabled")
		if !enabled {
			status = wrapWithRed("disabled")
		}

		fmt.Printf("%-3d %-25s %-30s %s\n", i+1, name, typeName, status)
	}
}
