package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

// cliLogger returns a stderr-backed logger used by CLI subcommands so that
// stdout remains clean for the raw API payloads the commands print.
func cliLogger(prefix string) *log.Logger {
	return log.New(os.Stderr, prefix, log.LstdFlags)
}

// workingDirOf reads the persistent --working-dir flag, falling back to ".".
func workingDirOf(cmd *cobra.Command) string {
	if wd, err := cmd.Flags().GetString("working-dir"); err == nil && wd != "" {
		return wd
	}
	return "."
}

// currentProjectFor infers the OneDev project from the working directory.
func currentProjectFor(cmd *cobra.Command) (string, error) {
	_, project, err := inferProject(workingDirOf(cmd), cliLogger("[tod] "))
	if err != nil {
		return "", err
	}
	return project, nil
}

// emit writes a raw response body to stdout, appending a trailing newline only
// if the server response did not already include one.
func emit(body []byte) {
	if len(body) == 0 {
		return
	}
	os.Stdout.Write(body)
	if body[len(body)-1] != '\n' {
		fmt.Println()
	}
}

// die prints an error to stderr and exits with status 1.
func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
