package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestFindMainGitRootFromNestedSubmodule(t *testing.T) {
	setupTestGitEnvironment(t)
	testRoot := t.TempDir()
	leafSource := createTestRepository(t, filepath.Join(testRoot, "leaf-source"), map[string]string{
		"leaf.txt": "leaf\n",
	})
	middleSource := createTestRepository(t, filepath.Join(testRoot, "middle-source"), map[string]string{
		"middle.txt": "middle\n",
	})
	addTestSubmodule(t, middleSource, leafSource, "deps/leaf")
	commitTestRepository(t, middleSource, "add leaf")

	root := createTestRepository(t, filepath.Join(testRoot, "root"), map[string]string{
		"root.txt": "root\n",
	})
	addTestSubmodule(t, root, middleSource, "deps/middle")
	commitTestRepository(t, root, "add middle")
	runTestGit(t, root, "-c", "protocol.file.allow=always", "submodule", "update", "--init", "--recursive")

	nestedDir := filepath.Join(root, "deps", "middle", "deps", "leaf", "nested")
	if err := os.Mkdir(nestedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	actual, err := findMainGitRoot(nestedDir)
	if err != nil {
		t.Fatal(err)
	}
	expectedInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	actualInfo, err := os.Stat(actual)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(expectedInfo, actualInfo) {
		t.Fatalf("expected main repository %q, got %q", root, actual)
	}
}

func TestLocalBuildWorkingDirHonorsExplicitOverride(t *testing.T) {
	cmd := &cobra.Command{Use: "run"}
	cmd.Flags().String("working-dir", "", "")
	explicitDir := filepath.Join(t.TempDir(), "submodule")
	if err := cmd.Flags().Set("working-dir", explicitDir); err != nil {
		t.Fatal(err)
	}

	actual, err := localBuildWorkingDir(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if actual != explicitDir {
		t.Fatalf("expected explicit working directory %q, got %q", explicitDir, actual)
	}
}

func TestGetBuildUnitTestReport(t *testing.T) {
	tests := []struct {
		name           string
		artifact       string
		expectArtifact string
		expectResponse string
	}{
		{
			name:           "report data",
			expectResponse: `{"testSuites":[]}`,
		},
		{
			name:           "artifact",
			artifact:       "artifacts/junit/failure.txt",
			expectArtifact: "artifacts/junit/failure.txt",
			expectResponse: "failure details",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/~api/tod/get-build-unit-test-report" {
					t.Fatalf("unexpected request path: %s", r.URL.Path)
				}
				query := r.URL.Query()
				if query.Get("currentProject") != "acme/project" {
					t.Fatalf("unexpected currentProject: %q", query.Get("currentProject"))
				}
				if query.Get("reference") != "#42" {
					t.Fatalf("unexpected reference: %q", query.Get("reference"))
				}
				if query.Get("reportName") != "junit" {
					t.Fatalf("unexpected reportName: %q", query.Get("reportName"))
				}
				if query.Get("artifactPath") != test.expectArtifact {
					t.Fatalf("unexpected artifactPath: %q", query.Get("artifactPath"))
				}
				fmt.Fprint(w, test.expectResponse)
			}))
			defer server.Close()

			oldConfig := config
			config = &Config{ServerUrl: server.URL, AccessToken: "test-token"}
			defer func() { config = oldConfig }()

			body, err := getBuildUnitTestReport("#42", "acme/project", "junit", test.artifact)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != test.expectResponse {
				t.Fatalf("unexpected response: %q", string(body))
			}
		})
	}
}
