package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type BuildsCommand struct {
}

func (command BuildsCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	count, _ := cobraCmd.Flags().GetInt("count")
	query, _ := cobraCmd.Flags().GetString("query")

	apiURL := config.ServerUrl + "/~api/builds?offset=0&count=" + fmt.Sprintf("%d", count)
	if query != "" {
		apiURL += "&query=" + url.QueryEscape(query)
	}

	body, err := makeAPICallSimple("GET", apiURL, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to query builds:", err)
		os.Exit(1)
	}

	var builds []map[string]interface{}
	if err := json.Unmarshal(body, &builds); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse builds:", err)
		os.Exit(1)
	}

	if len(builds) == 0 {
		fmt.Println("No builds found.")
		return
	}

	for _, build := range builds {
		number := int(build["number"].(float64))
		status, _ := build["status"].(string)
		jobName, _ := build["jobName"].(string)
		commitHash, _ := build["commitHash"].(string)
		projectId := int(build["projectId"].(float64))

		statusColored := colorizeStatus(status)
		hash := commitHash
		if len(hash) > 8 {
			hash = hash[:8]
		}

		fmt.Printf("#%-4d %-12s %-30s %s  (project %d)\n", number, statusColored, jobName, hash, projectId)
	}
}

func colorizeStatus(status string) string {
	switch strings.ToUpper(status) {
	case "SUCCESSFUL":
		return wrapWithGreen(status)
	case "FAILED", "CANCELLED", "TIMED_OUT":
		return wrapWithRed(status)
	case "RUNNING":
		return wrapWithColor(status, "33") // yellow
	default:
		return status
	}
}
