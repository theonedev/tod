package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptConfigPropertyRetriesInvalidServerURL(t *testing.T) {
	stderr := captureStderr(t, func() {
		got, err := promptConfigProperty(
			bufio.NewReader(strings.NewReader("onedev.example.com\nhttps://onedev.example.com/\n")),
			"OneDev server URL: ",
			serverUrlKey,
			"",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "https://onedev.example.com" {
			t.Fatalf("got %q, want %q", got, "https://onedev.example.com")
		}
	})

	if !strings.Contains(stderr, "Error: invalid server url") {
		t.Fatalf("stderr did not include validation error: %q", stderr)
	}
}

func TestPromptConfigPropertyRetriesInvalidTrustCertsFile(t *testing.T) {
	dir := t.TempDir()
	certsFile := filepath.Join(dir, "certs.pem")
	if err := os.WriteFile(certsFile, []byte("-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stderr := captureStderr(t, func() {
		got, err := promptConfigProperty(
			bufio.NewReader(strings.NewReader(filepath.Join(dir, "missing.pem")+"\n"+certsFile+"\n")),
			"Trust certs file: ",
			trustCertsFileKey,
			"",
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != certsFile {
			t.Fatalf("got %q, want %q", got, certsFile)
		}
	})

	if !strings.Contains(stderr, "Error: invalid trust-certs-file") {
		t.Fatalf("stderr did not include validation error: %q", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = original
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
