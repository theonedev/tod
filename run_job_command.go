package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

type RunJobCommand struct {
}

// Execute executes the run job command
func (command RunJobCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	// Get working directory from command flag, default to current directory
	workingDir, _ := cobraCmd.Flags().GetString("working-dir")
	if workingDir == "" {
		workingDir = "."
	}

	_, currentProject, err := inferProject(workingDir, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Extract job name from arguments
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Exactly one job name is required")
		os.Exit(1)
	}
	jobName := args[0]

	// Get branch and tag flags
	branch, _ := cobraCmd.Flags().GetString("branch")
	tag, _ := cobraCmd.Flags().GetString("tag")

	// Validate that either branch or tag is specified, but not both
	if branch == "" && tag == "" {
		fmt.Fprintln(os.Stderr, "Either --branch or --tag must be specified")
		os.Exit(1)
	}
	if branch != "" && tag != "" {
		fmt.Fprintln(os.Stderr, "Option --branch and --tag cannot be specified at the same time")
		os.Exit(1)
	}

	// Get command line parameters
	paramArray, err := cobraCmd.Flags().GetStringArray("param")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to get parameters:", err)
		os.Exit(1)
	}

	params := make(ParamMap)

	// Parse parameter array into ParamMap
	for _, paramStr := range paramArray {
		if err := params.Set(paramStr); err != nil {
			fmt.Fprintln(os.Stderr, "Failed to parse parameter:", err)
			os.Exit(1)
		}
	}

	// Build job map with parameters
	jobMap := make(map[string]interface{})
	jobMap["jobName"] = jobName

	// Add branch or tag to job map
	if branch != "" {
		jobMap["branch"] = branch
	} else {
		jobMap["tag"] = tag
	}

	// Add parameters
	if len(params) > 0 {
		paramStrings := make([]string, 0)
		for key, values := range params {
			for _, value := range values {
				paramStrings = append(paramStrings, fmt.Sprintf("%s=%s", key, value))
			}
		}
		jobMap["params"] = paramStrings
	}

	jobMap["reason"] = "Submitted via tod"

	fmt.Printf("Running job '%s' against ", jobName)
	if branch != "" {
		fmt.Printf("branch '%s'...\n", branch)
	} else {
		fmt.Printf("tag '%s'...\n", tag)
	}

	// Run the job
	build, err := runJob(currentProject, jobMap)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	buildId := int(build["id"].(float64))
	buildNumber := int(build["number"].(float64))

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	err = streamBuildLog(buildId, buildNumber, signalChannel)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
