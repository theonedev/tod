package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestMarkdownCommandInput(t *testing.T) {
	setupTestGitEnvironment(t)
	markdown := "  # Review 报告\r\n\r\n```sh\r\necho \"$HOME\" 'quoted' `pwd` $(whoami)\r\n```\r\n\n"
	directory := t.TempDir()
	contentPath := filepath.Join(directory, "comment with spaces.md")
	emptyPath := filepath.Join(directory, "empty.md")
	for path, content := range map[string]string{contentPath: markdown, emptyPath: " \t\r\n"} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	commands := []struct {
		name       string
		command    *cobra.Command
		args       []string
		inlineFlag string
		fileFlag   string
		endpoint   string
	}{
		{"issue create", issueCreateCmd, []string{"Title", "--project", "onedev/tod"}, "description", "file", "create-issue"},
		{"issue edit", issueEditCmd, []string{"TOD-17"}, "description", "file", "edit-issue"},
		{"issue comment", issueAddCommentCmd, []string{"TOD-17"}, "", "file", "add-issue-comment"},
		{"issue state", issueChangeStateCmd, []string{"TOD-17", "Closed"}, "comment", "file", "change-issue-state"},
		{"issue work", issueLogWorkCmd, []string{"TOD-17", "2"}, "comment", "file", "log-work"},
		{"PR create", prCreateCmd, []string{"Title", "--source-project", "onedev/tod", "--source-branch", "issue-17"}, "description", "file", "create-pull-request"},
		{"PR edit", prEditCmd, []string{"TOD-17"}, "description", "file", "edit-pull-request"},
		{"PR comment", prAddCommentCmd, []string{"TOD-17"}, "", "file", "add-pull-request-comment"},
		{"PR code comment", prAddCodeCommentCmd, []string{"TOD-17", "--on-file", "src/main.go", "--from-line", "3"}, "", "file", "add-pull-request-code-comment"},
		{"PR approval", prApproveCmd, []string{"TOD-17"}, "summary", "file", "approve-pull-request"},
		{"PR changes", prRequestChangesCmd, []string{"TOD-17"}, "summary", "file", "request-changes-on-pull-request"},
		{"PR discard", prDiscardCmd, []string{"TOD-17"}, "comment", "file", "discard-pull-request"},
		{"code reply", codeCommentAddReplyCmd, []string{"17"}, "", "file", "add-code-comment-reply"},
		{"code resolve", codeCommentResolveCmd, []string{"17"}, "note", "file", "resolve-code-comment"},
		{"code unresolve", codeCommentUnresolveCmd, []string{"17"}, "note", "file", "unresolve-code-comment"},
	}
	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			sources := []struct {
				name    string
				inline  bool
				file    bool
				path    string
				content string
				wantErr string
			}{
				{name: "inline", inline: true, content: markdown},
				{name: "file", file: true, path: contentPath, content: markdown},
				{name: "stdin", file: true, path: "-", content: markdown},
				{name: "literal dash", inline: true, content: "-"},
				{name: "conflicting sources", inline: true, file: true, path: contentPath, wantErr: "cannot be used together"},
				{name: "missing file", file: true, path: filepath.Join(directory, "missing.md"), wantErr: "failed to read Markdown file"},
				{name: "empty path", file: true, wantErr: "must not be empty"},
				{name: "empty stdin", file: true, path: "-"},
				{name: "blank stdin", file: true, path: "-", content: " \t\r\n"},
				{name: "blank file", file: true, path: emptyPath, content: " \t\r\n"},
				{name: "empty inline", inline: true},
				{name: "omitted"},
			}
			for _, source := range sources {
				t.Run(source.name, func(t *testing.T) {
					wantErr := source.wantErr
					if wantErr == "" && command.inlineFlag == "" && strings.TrimSpace(source.content) == "" {
						wantErr = "must not be empty"
						if !source.inline && !source.file {
							wantErr = "is required"
						}
					}

					called := false
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						called = true
						if r.Method != "POST" || r.URL.Path != "/~api/tod/"+command.endpoint {
							t.Errorf("unexpected request: %s %s", r.Method, r.URL)
						}
						body, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
						}
						content := string(body)
						if command.command == issueChangeStateCmd || command.inlineFlag == "description" {
							var payload map[string]interface{}
							if err := json.Unmarshal(body, &payload); err != nil {
								t.Error(err)
								return
							}
							content, _ = payload[command.inlineFlag].(string)
							if source.content == "" {
								if _, present := payload[command.inlineFlag]; present {
									t.Errorf("empty optional %s should be omitted", command.inlineFlag)
								}
							}
							if command.command == issueChangeStateCmd && payload["state"] != "Closed" {
								t.Errorf("unexpected state: %v", payload["state"])
							}
						}
						if content != source.content {
							t.Errorf("Markdown changed: got %q, want %q", content, source.content)
						}
						if command.command == prAddCodeCommentCmd {
							query := r.URL.Query()
							if query.Get("filePath") != "src/main.go" || query.Get("fromLineNumber") != "3" || query.Get("toLineNumber") != "3" {
								t.Errorf("unexpected code comment position: %v", query)
							}
						}
					}))
					defer server.Close()
					oldConfig := config
					config = &Config{ServerUrl: server.URL, AccessToken: "test-token"}
					defer func() { config = oldConfig }()

					cmd := markdownTestCommand(t, command.command)
					cmd.Flags().String("working-dir", directory, "")
					cmd.SetIn(strings.NewReader(source.content))
					args := append([]string(nil), command.args...)
					if source.inline {
						if command.inlineFlag != "" {
							args = append(args, "--"+command.inlineFlag, source.content)
						} else {
							args = append(args, source.content)
						}
					}
					if source.file {
						args = append(args, "--"+command.fileFlag, source.path)
					}
					cmd.SetArgs(args)
					err := cmd.Execute()
					if wantErr != "" {
						if err == nil || !strings.Contains(err.Error(), wantErr) {
							t.Fatalf("expected error containing %q, got %v", wantErr, err)
						}
						if called {
							t.Fatal("invalid input must not submit a request")
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						if !called {
							t.Fatal("command did not submit a request")
						}
					}
				})
			}
		})
	}
}

// Exercise the registered argument validation, flags, and handler without the
// root command's configuration loading and server version check.
func markdownTestCommand(t *testing.T, original *cobra.Command) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: original.Use, Args: original.Args, RunE: original.RunE}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	original.Flags().VisitAll(func(flag *pflag.Flag) {
		switch flag.Value.Type() {
		case "string":
			cmd.Flags().String(flag.Name, flag.DefValue, flag.Usage)
		case "int":
			value, err := strconv.Atoi(flag.DefValue)
			if err != nil {
				t.Fatal(err)
			}
			cmd.Flags().Int(flag.Name, value, flag.Usage)
		case "stringArray":
			cmd.Flags().StringArray(flag.Name, nil, flag.Usage)
		case "bool":
			value, err := strconv.ParseBool(flag.DefValue)
			if err != nil {
				t.Fatal(err)
			}
			cmd.Flags().Bool(flag.Name, value, flag.Usage)
		case "float64":
			value, err := strconv.ParseFloat(flag.DefValue, 64)
			if err != nil {
				t.Fatal(err)
			}
			cmd.Flags().Float64(flag.Name, value, flag.Usage)
		default:
			t.Fatalf("unsupported flag type: %s", flag.Value.Type())
		}
	})
	return cmd
}

type failingMarkdownReader struct{}

func (failingMarkdownReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("input interrupted")
}

func TestMarkdownStdinReadError(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("file", "", "")
	if err := cmd.Flags().Set("file", "-"); err != nil {
		t.Fatal(err)
	}
	cmd.SetIn(failingMarkdownReader{})
	if _, err := readMarkdown(cmd, "", "file"); err == nil || !strings.Contains(err.Error(), "failed to read Markdown from stdin: input interrupted") {
		t.Fatalf("expected stdin error, got %v", err)
	}
}
