package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAPICommandsWithoutCheckout(t *testing.T) {
	setupTestGitEnvironment(t)
	tests := []struct {
		name     string
		command  *cobra.Command
		args     []string
		flags    map[string]string
		method   string
		endpoint string
		query    map[string]string
	}{
		{"get issue", issueGetCmd, []string{"acme/project#42"}, nil, "GET", "get-issue", map[string]string{"reference": "acme/project#42"}},
		{"get PR", prGetCmd, []string{"ACME-42"}, nil, "GET", "get-pull-request", map[string]string{"reference": "ACME-42"}},
		{"get build", buildGetCmd, []string{"acme/project#42"}, nil, "GET", "get-build", map[string]string{"reference": "acme/project#42"}},
		{"list issues globally", issueListCmd, nil, nil, "GET", "query-issues", nil},
		{"list PRs globally", prListCmd, nil, nil, "GET", "query-pull-requests", nil},
		{"list builds globally", buildListCmd, nil, nil, "GET", "query-builds", nil},
		{"list project issues", issueListCmd, nil, map[string]string{"project": "acme/project"}, "GET", "query-issues", map[string]string{"project": "acme/project"}},
		{"create issue", issueCreateCmd, []string{"Fix login"}, map[string]string{"project": "acme/project"}, "POST", "create-issue", map[string]string{"project": "acme/project"}},
		{"create issue branch", issueCreateBranchCmd, []string{"acme/project#42"}, nil, "POST", "ensure-issue-branch", map[string]string{"reference": "acme/project#42"}},
		{"comment on issue", issueAddCommentCmd, []string{"ACME-42", "Investigating"}, nil, "POST", "add-issue-comment", map[string]string{"reference": "ACME-42"}},
		{"approve PR", prApproveCmd, []string{"ACME-42"}, nil, "POST", "approve-pull-request", map[string]string{"reference": "ACME-42"}},
		{"create PR", prCreateCmd, []string{"Fix login"}, map[string]string{"source-project": "acme/project", "source-branch": "issue-42"}, "POST", "create-pull-request", map[string]string{"sourceProject": "acme/project"}},
		{"PR title requirements", prGetTitleAndDescriptionRequirementCmd, nil, map[string]string{"source-project": "acme/project", "source-branch": "issue-42"}, "GET", "get-pull-request-title-and-description-requirement", map[string]string{"sourceProject": "acme/project", "sourceBranch": "issue-42"}},
		{"PR commit requirements", prGetCommitMessageRequirementCmd, nil, map[string]string{"source-project": "acme/project", "source-branch": "issue-42"}, "GET", "get-pull-request-commit-message-requirement", map[string]string{"sourceProject": "acme/project", "sourceBranch": "issue-42"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Method != test.method || r.URL.Path != "/~api/tod/"+test.endpoint {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				query := r.URL.Query()
				for _, name := range []string{"currentProject", "project", "sourceProject", "targetProject", "targetBranch"} {
					if test.query[name] == "" && query.Has(name) {
						t.Errorf("optional %s should be absent, got %q", name, query.Get(name))
					}
				}
				for name, want := range test.query {
					if got := query.Get(name); got != want {
						t.Errorf("%s: got %q, want %q", name, got, want)
					}
				}
				fmt.Fprint(w, `{}`)
			}))
			defer server.Close()
			oldConfig := config
			config = &Config{ServerUrl: server.URL, AccessToken: "test-token"}
			defer func() { config = oldConfig }()

			cmd := &cobra.Command{}
			cmd.Flags().String("working-dir", t.TempDir(), "")
			for name, value := range test.flags {
				cmd.Flags().String(name, value, "")
			}
			if err := test.command.RunE(cmd, test.args); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("command did not call the server")
			}
		})
	}
}

func TestCurrentProjectContext(t *testing.T) {
	setupTestGitEnvironment(t)
	repository := createTestRepository(t, filepath.Join(t.TempDir(), "repo"), map[string]string{"file": "content"})
	runTestGit(t, repository, "remote", "add", "origin", "https://example.test/acme/project.git")

	for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"http":"https://example.test"}`)
			}))
			defer server.Close()
			oldConfig := config
			config = &Config{ServerUrl: server.URL, AccessToken: "test-token"}
			defer func() { config = oldConfig }()
			cmd := &cobra.Command{}
			cmd.Flags().String("working-dir", repository, "")

			project, err := currentProjectFor(cmd)
			if status == http.StatusOK {
				if err != nil || project != "acme/project" {
					t.Fatalf("expected inferred project, got %q, %v", project, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
				t.Fatalf("server error must not become absent project context: %q, %v", project, err)
			}
		})
	}
}

func TestCurrentProjectContextWithNoRemotes(t *testing.T) {
	setupTestGitEnvironment(t)
	repository := createTestRepository(t, filepath.Join(t.TempDir(), "repo"), map[string]string{"file": "content"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"http":"https://example.test"}`)
	}))
	defer server.Close()
	oldConfig := config
	config = &Config{ServerUrl: server.URL, AccessToken: "test-token"}
	defer func() { config = oldConfig }()
	cmd := &cobra.Command{}
	cmd.Flags().String("working-dir", repository, "")
	if project, err := currentProjectFor(cmd); err != nil || project != "" {
		t.Fatalf("expected absent project context, got %q, %v", project, err)
	}
	if _, err := requireCurrentProjectFor(cmd); err == nil {
		t.Fatal("a local command must still reject missing project context")
	}
}

func TestCurrentProjectContextRejectsInvalidWorkingDirectory(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("working-dir", filepath.Join(t.TempDir(), "missing"), "")
	if _, err := currentProjectFor(cmd); err == nil {
		t.Fatal("invalid working directory must not become absent project context")
	}
}

func TestLocalCommandsStillRequireCheckout(t *testing.T) {
	setupTestGitEnvironment(t)
	cmd := &cobra.Command{}
	cmd.Flags().String("working-dir", t.TempDir(), "")
	for _, command := range []*cobra.Command{projectCurrentCmd, remoteCmd, issueCheckoutCmd, prCheckoutCmd, prCurrentReferenceCmd, buildCheckSpecCmd, getCommitMessageRequirementCmd} {
		t.Run(command.Use, func(t *testing.T) {
			if err := command.RunE(cmd, []string{"ACME-42"}); err == nil {
				t.Fatal("expected missing repository error")
			}
		})
	}
}

func TestRunJobProjectContext(t *testing.T) {
	for _, context := range []struct{ project, currentProject string }{
		{"acme/target", ""},
		{"acme/target", "acme/current"},
		{"", "acme/current"},
	} {
		t.Run("project="+context.project+"/current="+context.currentProject, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/~api/tod/run-job" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				query := r.URL.Query()
				if query.Get("project") != context.project || query.Get("currentProject") != context.currentProject {
					t.Errorf("unexpected project context: %v", query)
				}
				if context.currentProject == "" && query.Has("currentProject") {
					t.Error("absent current project should not be sent")
				}
				if context.project == "" && query.Has("project") {
					t.Error("absent explicit project should not be sent")
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["branch"] != "main" {
					t.Errorf("unexpected job body: %v, %v", body, err)
				}
				fmt.Fprint(w, `{"id":1,"number":42}`)
			}))
			defer server.Close()
			oldConfig := config
			config = &Config{ServerUrl: server.URL, AccessToken: "test-token"}
			defer func() { config = oldConfig }()
			if _, err := runJob(context.project, context.currentProject, map[string]interface{}{"jobName": "ci", "branch": "main"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBuildRunWithoutCheckout(t *testing.T) {
	setupTestGitEnvironment(t)
	for _, selector := range []string{"branch", "tag"} {
		t.Run(selector, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Method != "POST" || r.URL.Path != "/~api/tod/run-job" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if query := r.URL.Query(); query.Get("project") != "acme/project" || query.Has("currentProject") {
					t.Errorf("unexpected project context: %v", query)
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body[selector] != "revision" || body["jobName"] != "ci" {
					t.Errorf("unexpected job body: %v, %v", body, err)
				}
				// Stop after submission: this test does not exercise log streaming.
				http.Error(w, "test submission response", http.StatusConflict)
			}))
			defer server.Close()
			oldConfig := config
			config = &Config{ServerUrl: server.URL, AccessToken: "test-token"}
			defer func() { config = oldConfig }()
			cmd := &cobra.Command{}
			cmd.Flags().String("working-dir", t.TempDir(), "")
			cmd.Flags().String("project", "acme/project", "")
			cmd.Flags().String(selector, "revision", "")
			cmd.Flags().StringArray("param", nil, "")
			err := runBuildJobCommand(cmd, []string{"ci"})
			if !called || err == nil || !strings.Contains(err.Error(), "test submission response") {
				t.Fatalf("expected submission outside a checkout, got %v (called=%v)", err, called)
			}
		})
	}
}

func TestLocalBuildRejectsExplicitProject(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("local", true, "")
	cmd.Flags().String("project", "", "")
	if err := cmd.Flags().Set("project", "acme/other"); err != nil {
		t.Fatal(err)
	}
	err := runBuildJobCommand(cmd, []string{"ci"})
	if err == nil || !strings.Contains(err.Error(), "--project cannot be used with --local") {
		t.Fatalf("expected explicit project to be rejected for local build, got %v", err)
	}
}
