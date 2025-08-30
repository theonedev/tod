package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

type CheckoutCommand struct {
}

func (checkoutCommand CheckoutCommand) Execute(cobraCmd *cobra.Command, args []string, logger *log.Logger) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Error: exactly one pull request reference is required")
		os.Exit(1)
	}
	pullRequestReference := args[0]

	// Get working directory from command flag, default to current directory
	workingDir, _ := cobraCmd.Flags().GetString("working-dir")
	if workingDir == "" {
		workingDir = "."
	}

	err := checkoutPullRequest(workingDir, pullRequestReference, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error checking out pull request:", err)
		os.Exit(1)
	}

	logger.Printf("Checked out pull request %s", pullRequestReference)
}
