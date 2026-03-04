package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

type LogsCommand struct {
}

func (command LogsCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Exactly one build number is required")
		os.Exit(1)
	}

	buildNumber, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Build number must be an integer:", args[0])
		os.Exit(1)
	}

	// Search recent builds to resolve build number → build ID
	// OneDev API uses internal IDs, not display numbers
	searchURL := config.ServerUrl + "/~api/builds?offset=0&count=100"
	body, err := makeAPICallSimple("GET", searchURL, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to query builds:", err)
		os.Exit(1)
	}

	var builds []map[string]interface{}
	if err := json.Unmarshal(body, &builds); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to parse builds:", err)
		os.Exit(1)
	}

	var buildId int
	found := false
	for _, build := range builds {
		num := int(build["number"].(float64))
		if num == buildNumber {
			buildId = int(build["id"].(float64))
			found = true
			break
		}
	}

	if !found {
		// Try direct ID access as fallback
		directURL := config.ServerUrl + fmt.Sprintf("/~api/builds/%d", buildNumber)
		body, err = makeAPICallSimple("GET", directURL, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Build #%d not found\n", buildNumber)
			os.Exit(1)
		}
		var build map[string]interface{}
		if err := json.Unmarshal(body, &build); err != nil {
			fmt.Fprintf(os.Stderr, "Build #%d not found\n", buildNumber)
			os.Exit(1)
		}
		buildId = int(build["id"].(float64))
		buildNumber = int(build["number"].(float64))
	}

	fmt.Printf("Streaming log for build #%d...\n", buildNumber)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	err = streamBuildLog(buildId, buildNumber, signalChannel)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
