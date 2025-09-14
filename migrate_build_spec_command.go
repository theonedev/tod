package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type MigrateBuildSpecCommand struct {
}

// Execute executes the migrate build spec command
func (migrateBuildSpecCommand MigrateBuildSpecCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	// Get working directory flag
	workingDir, _ := cobraCmd.Flags().GetString("working-dir")
	if workingDir == "" {
		workingDir = "."
	}

	fmt.Println("Migrating build spec...")

	// Migrate the build spec
	err := migrateBuildSpec(workingDir, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Println("Build spec migration completed successfully.")
}
