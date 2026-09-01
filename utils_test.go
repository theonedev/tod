package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckVersionReportsNonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/~api/tod/check-version" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<html><body>Proxy error</body></html>")
	}))
	defer server.Close()

	_, err := checkVersion(&Config{
		ServerUrl:   server.URL,
		AccessToken: "test-token",
	})
	if err == nil {
		t.Fatal("expected checkVersion to reject an HTML response")
	}

	message := err.Error()
	for _, expected := range []string{
		server.URL + "/~api/tod/check-version",
		`content-type: "text/html; charset=utf-8"`,
		`response: "<html><body>Proxy error</body></html>"`,
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("expected error to contain %q, got %q", expected, message)
		}
	}
}

func TestRemoteURLForProjectPreservesHTTPRemoteShape(t *testing.T) {
	got, err := remoteURLForProject("https://example.com/old/project.git", "new/project")
	if err != nil {
		t.Fatal(err)
	}

	want := "https://example.com/new/project.git"
	if got != want {
		t.Fatalf("unexpected remote URL: got %q, want %q", got, want)
	}
}

func TestRemoteURLForProjectPreservesSSHRemoteShape(t *testing.T) {
	got, err := remoteURLForProject("ssh://git@example.com/old/project", "new/project")
	if err != nil {
		t.Fatal(err)
	}

	want := "ssh://git@example.com/new/project"
	if got != want {
		t.Fatalf("unexpected remote URL: got %q, want %q", got, want)
	}
}

func TestCheckoutFetchedBranchUpdatesRetrievedSubmodule(t *testing.T) {
	setupTestGitEnvironment(t)
	testRoot := t.TempDir()
	childSource := createTestRepository(t, filepath.Join(testRoot, "child-source"), map[string]string{
		"child.txt": "child base\n",
	})
	root := createTestRepository(t, filepath.Join(testRoot, "root"), map[string]string{
		"root.txt": "root base\n",
	})
	runTestGit(t, root, "remote", "add", "origin", childSource)
	addTestSubmodule(t, root, childSource, "deps/child")
	commitTestRepository(t, root, "add child")
	baseBranch := runTestGit(t, root, "branch", "--show-current")

	childDir := filepath.Join(root, "deps", "child")
	writeTestFile(t, filepath.Join(childDir, "child.txt"), "child on issue branch\n")
	runTestGit(t, childDir, "add", ".")
	runTestGit(t, childDir, "commit", "--quiet", "-m", "child on issue branch")
	childCommit := runTestGit(t, childDir, "rev-parse", "HEAD")

	runTestGit(t, root, "checkout", "--quiet", "-b", "issue-1")
	commitTestRepository(t, root, "bump child gitlink")
	issueCommit := runTestGit(t, root, "rev-parse", "HEAD")

	runTestGit(t, root, "checkout", "--quiet", baseBranch)
	runTestGit(t, root, "submodule", "update")
	runTestGit(t, root, "branch", "--delete", "--force", "issue-1")

	if err := checkoutFetchedBranch(root, "origin", "issue-1", issueCommit, testLogger()); err != nil {
		t.Fatal(err)
	}

	assertTestGitOutput(t, childDir, childCommit, "rev-parse", "HEAD")
	assertTestGitOutput(t, root, "", "status", "--porcelain")
}

func TestUpdateRetrievedSubmodulesSkipsUnretrievedSubmodule(t *testing.T) {
	setupTestGitEnvironment(t)
	testRoot := t.TempDir()
	childSource := createTestRepository(t, filepath.Join(testRoot, "child-source"), map[string]string{
		"child.txt": "child\n",
	})
	root := createTestRepository(t, filepath.Join(testRoot, "root"), map[string]string{
		"root.txt": "root\n",
	})
	addTestSubmodule(t, root, childSource, "not-retrieved")
	commitTestRepository(t, root, "add child")
	runTestGit(t, root, "submodule", "deinit", "--force", "not-retrieved")

	if err := updateRetrievedSubmodules(root, testLogger()); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(root, "not-retrieved"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected unretrieved submodule to stay empty, got %d entries", len(entries))
	}
}
