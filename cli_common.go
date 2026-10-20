package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func markdownContentArgs(fileFlag string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.RangeArgs(1, 2)(cmd, args); err != nil {
			return err
		}
		if cmd.Flags().Changed(fileFlag) {
			if len(args) == 2 {
				return fmt.Errorf("<content> and --%s cannot be used together", fileFlag)
			}
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("<content> or --%s is required", fileFlag)
		}
		if strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("<content> must not be empty")
		}
		return nil
	}
}

func readMarkdownContent(cmd *cobra.Command, args []string, fileFlag string) (string, error) {
	var content string
	if len(args) == 2 {
		content = args[1]
	}
	content, err := readMarkdown(cmd, content, fileFlag)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("<content> must not be empty")
	}
	return content, nil
}

func readMarkdownFlag(cmd *cobra.Command, inlineFlag string) (string, error) {
	if cmd.Flags().Changed(inlineFlag) && cmd.Flags().Changed("file") {
		return "", fmt.Errorf("--%s and --file cannot be used together", inlineFlag)
	}
	content, _ := cmd.Flags().GetString(inlineFlag)
	return readMarkdown(cmd, content, "file")
}

// readMarkdown preserves the content exactly, including whitespace and newlines.
func readMarkdown(cmd *cobra.Command, content, fileFlag string) (string, error) {
	if !cmd.Flags().Changed(fileFlag) {
		return content, nil
	}
	path, _ := cmd.Flags().GetString(fileFlag)
	if path == "" {
		return "", fmt.Errorf("--%s must not be empty", fileFlag)
	}
	if path == "-" {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("failed to read Markdown from stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read Markdown file %q: %w", path, err)
	}
	return string(data), nil
}

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

// currentProjectFor returns an optional project context for server API calls.
// Missing repository context is allowed; configuration and server errors are not.
func currentProjectFor(cmd *cobra.Command) (string, error) {
	project, err := requireCurrentProjectFor(cmd)
	if errors.Is(err, errNoCurrentProject) {
		return "", nil
	}
	return project, err
}

func requireCurrentProjectFor(cmd *cobra.Command) (string, error) {
	_, project, err := inferProject(workingDirOf(cmd))
	if err != nil {
		return "", err
	}
	return project, nil
}

// currentReferenceError suppresses usage for runtime derivation failures while
// preserving usage output for argument validation errors handled before RunE.
func currentReferenceError(cmd *cobra.Command, err error) error {
	cmd.SilenceUsage = true
	return err
}

func suppressUsageForRunErrors(command *cobra.Command) {
	if command.RunE != nil {
		runE := command.RunE
		command.RunE = func(cmd *cobra.Command, args []string) error {
			err := runE(cmd, args)
			if err != nil {
				cmd.SilenceUsage = true
				cmd.SilenceErrors = true
			}
			return err
		}
	}

	for _, child := range command.Commands() {
		suppressUsageForRunErrors(child)
	}
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
