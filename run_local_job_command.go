package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
)

type RunLocalJobCommand struct {
}

type ParamMap map[string][]string

func (p ParamMap) String() string {
	str := "{"
	for key, values := range p {
		str += fmt.Sprintf("%s: %v, ", key, values)
	}
	str += "}"
	return str
}

func (p ParamMap) Set(value string) error {
	parts := strings.Split(value, "=")
	if len(parts) != 2 {
		return fmt.Errorf("invalid parameter format (expected key=value): %s", value)
	}

	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])

	if len(key) == 0 {
		return fmt.Errorf("parameter key cannot be empty: %s", value)
	}

	if len(val) == 0 {
		return fmt.Errorf("parameter value cannot be empty: %s", value)
	}

	// Append to existing values instead of replacing
	if existingValues, exists := p[key]; exists {
		p[key] = append(existingValues, val)
	} else {
		p[key] = []string{val}
	}

	return nil
}

var buildFinished bool
var mutex sync.Mutex

// Execute executes the run job command
func (runLocalJobCommand RunLocalJobCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	// Extract job name from arguments
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Error: exactly one job name is required")
		os.Exit(1)
	}
	jobName := args[0]

	// Get working directory from command flag, default to current directory
	workingDir, _ := cobraCmd.Flags().GetString("working-dir")
	if workingDir == "" {
		workingDir = "."
	}

	// Get command line parameters
	paramArray, err := cobraCmd.Flags().GetStringArray("param")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting parameters:", err)
		os.Exit(1)
	}

	params := make(ParamMap)

	// Parse parameter array into ParamMap
	for _, paramStr := range paramArray {
		if err := params.Set(paramStr); err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing parameter:", err)
			os.Exit(1)
		}
	}

	fmt.Println("Collecting local changes...")

	build, err := runLocalJob(jobName, workingDir, params, "Submitted via tod", logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	buildId := int(build["id"].(float64))

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Sending local changes to server...")

	// Stream job logs using the utility function
	err = streamBuildLog(buildId, signalChannel, &buildFinished, &mutex)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
