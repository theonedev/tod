package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSuppressUsageForRunErrors(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("HTTP 406 error for endpoint http://example.test: validation failed")
		},
	}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)

	suppressUsageForRunErrors(cmd)

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected command to fail")
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("expected no usage output, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "Error:") {
		t.Fatalf("expected no cobra error output, got %q", stderr.String())
	}
}

func TestSuppressUsageForRunErrorsKeepsArgUsage(t *testing.T) {
	cmd := &cobra.Command{
		Use:  "test <arg>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)

	suppressUsageForRunErrors(cmd)

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected command to fail")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", stderr.String())
	}
}

func TestNonEmptyCommentArgs(t *testing.T) {
	cmd := &cobra.Command{Use: "add-comment <reference> <content>"}

	if err := nonEmptyCommentArgs(cmd, []string{"#1", "A comment"}); err != nil {
		t.Fatalf("expected non-empty comment to be accepted: %v", err)
	}
	if err := nonEmptyCommentArgs(cmd, []string{"#1", " \t\n"}); err == nil || err.Error() != "<content> must not be empty" {
		t.Fatalf("expected whitespace-only comment to be rejected, got %v", err)
	}
	if err := nonEmptyCommentArgs(cmd, []string{"#1"}); err == nil {
		t.Fatal("expected missing comment argument to be rejected")
	}
}
