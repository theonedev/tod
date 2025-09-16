package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type CheckBuildSpecCommand struct {
}

func (command CheckBuildSpecCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	// Get working directory flag
	workingDir, _ := cobraCmd.Flags().GetString("working-dir")
	if workingDir == "" {
		workingDir = "."
	}

	fmt.Println("Checking build spec...")

	// Check the build spec
	err := checkBuildSpec(workingDir, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to check build spec:", err)
		os.Exit(1)
	}

	fmt.Println("Build spec checked successfully.")
}
