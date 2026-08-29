package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type pushedSnapshot struct {
	repositoryDir string
	commit        string
}

func TestIsRefLockFailure(t *testing.T) {
	lockError := &gitCommandError{
		description: "git push",
		err:         errors.New("exit status 1"),
		stderr:      "remote: error: cannot lock ref 'refs/onedev/tod': is at abc but expected def",
	}
	if !isRefLockFailure(fmt.Errorf("failed to push: %w", lockError)) {
		t.Fatal("expected wrapped ref lock error to be retryable")
	}

	permissionError := &gitCommandError{
		description: "git push",
		err:         errors.New("exit status 1"),
		stderr:      "remote: permission denied",
	}
	if isRefLockFailure(permissionError) {
		t.Fatal("expected permission error not to be retryable")
	}
	if isRefLockFailure(errors.New("cannot lock ref")) {
		t.Fatal("expected an unrelated error string not to be retryable")
	}
}

func TestPrepareLocalSnapshotIncludesNestedSubmoduleChanges(t *testing.T) {
	setupTestGitEnvironment(t)
	testRoot := t.TempDir()
	leafSource := createTestRepository(t, filepath.Join(testRoot, "leaf-source"), map[string]string{
		"tracked.txt": "leaf base\n",
	})
	middleSource := createTestRepository(t, filepath.Join(testRoot, "middle-source"), map[string]string{
		"middle.txt": "middle base\n",
	})
	addTestSubmodule(t, middleSource, leafSource, "deps/leaf")
	commitTestRepository(t, middleSource, "add leaf")

	root := createTestRepository(t, filepath.Join(testRoot, "root"), map[string]string{
		".onedev-buildspec.yml": "version: 1\n",
		"root.txt":              "root base\n",
	})
	addTestSubmodule(t, root, middleSource, "deps/middle")
	commitTestRepository(t, root, "add middle")
	runTestGit(t, root, "-c", "protocol.file.allow=always", "submodule", "update", "--init", "--recursive")

	middleDir := filepath.Join(root, "deps", "middle")
	leafDir := filepath.Join(middleDir, "deps", "leaf")
	writeTestFile(t, filepath.Join(leafDir, "tracked.txt"), "leaf local\n")
	writeTestFile(t, filepath.Join(root, ".onedev-buildspec.yml"), "version: 2\n")
	writeTestFile(t, filepath.Join(root, "root.txt"), "root staged\n")
	runTestGit(t, root, "add", "root.txt")

	rootHead := runTestGit(t, root, "rev-parse", "HEAD")
	middleHead := runTestGit(t, middleDir, "rev-parse", "HEAD")
	leafHead := runTestGit(t, leafDir, "rev-parse", "HEAD")
	rootIndexBefore := runTestGit(t, root, "diff", "--cached", "--binary")
	middleIndexBefore := runTestGit(t, middleDir, "diff", "--cached", "--binary")
	leafIndexBefore := runTestGit(t, leafDir, "diff", "--cached", "--binary")

	var pushed []pushedSnapshot
	snapshot, err := prepareLocalSnapshot(root, filepath.Join(root, ".onedev-buildspec.yml"),
		collectTestPushes(&pushed), testLogger())
	if err != nil {
		t.Fatal(err)
	}

	if len(pushed) != 2 {
		t.Fatalf("expected leaf and middle snapshots to be pushed, got %#v", pushed)
	}
	if pushed[0].repositoryDir != leafDir || pushed[1].repositoryDir != middleDir {
		t.Fatalf("expected deepest-first pushes, got %#v", pushed)
	}
	leafSnapshot := pushed[0].commit
	middleSnapshot := pushed[1].commit
	if content := runTestGit(t, leafDir, "show", leafSnapshot+":tracked.txt"); content != "leaf local" {
		t.Fatalf("unexpected leaf snapshot content: %q", content)
	}
	if gitlink := runTestGit(t, middleDir, "rev-parse", middleSnapshot+":deps/leaf"); gitlink != leafSnapshot {
		t.Fatalf("middle snapshot points to %s instead of leaf snapshot %s", gitlink, leafSnapshot)
	}
	if gitlink := runTestGit(t, root, "rev-parse", snapshot+":deps/middle"); gitlink != middleSnapshot {
		t.Fatalf("root snapshot points to %s instead of middle snapshot %s", gitlink, middleSnapshot)
	}
	if content := runTestGit(t, root, "show", snapshot+":.onedev-buildspec.yml"); content != "version: 2" {
		t.Fatalf("unexpected build spec snapshot content: %q", content)
	}
	if content := runTestGit(t, root, "show", snapshot+":root.txt"); content != "root staged" {
		t.Fatalf("unexpected staged snapshot content: %q", content)
	}

	assertTestGitOutput(t, root, rootHead, "rev-parse", "HEAD")
	assertTestGitOutput(t, middleDir, middleHead, "rev-parse", "HEAD")
	assertTestGitOutput(t, leafDir, leafHead, "rev-parse", "HEAD")
	assertTestGitOutput(t, root, rootIndexBefore, "diff", "--cached", "--binary")
	assertTestGitOutput(t, middleDir, middleIndexBefore, "diff", "--cached", "--binary")
	assertTestGitOutput(t, leafDir, leafIndexBefore, "diff", "--cached", "--binary")
}

func TestPrepareLocalSnapshotSkipsUnchangedAndUninitializedSubmodules(t *testing.T) {
	setupTestGitEnvironment(t)
	testRoot := t.TempDir()
	childSource := createTestRepository(t, filepath.Join(testRoot, "child-source"), map[string]string{
		"child.txt": "child\n",
	})
	root := createTestRepository(t, filepath.Join(testRoot, "root"), map[string]string{
		".onedev-buildspec.yml": "version: 1\n",
	})
	addTestSubmodule(t, root, childSource, "initialized")
	addTestSubmodule(t, root, childSource, "not-initialized")
	commitTestRepository(t, root, "add children")
	runTestGit(t, root, "submodule", "deinit", "-f", "not-initialized")

	var pushed []pushedSnapshot
	snapshot, err := prepareLocalSnapshot(root, filepath.Join(root, ".onedev-buildspec.yml"),
		collectTestPushes(&pushed), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if len(pushed) != 0 {
		t.Fatalf("expected no submodule pushes, got %#v", pushed)
	}
	if head := runTestGit(t, root, "rev-parse", "HEAD"); snapshot != head {
		t.Fatalf("expected unchanged root snapshot %s, got %s", head, snapshot)
	}
}

// A submodule commit missing on the remote has to be sent even when the parent
// already records it, as the snapshot pushed for the parent refers to it.
func TestPrepareLocalSnapshotSendsSubmoduleCommitMissingOnRemote(t *testing.T) {
	setupTestGitEnvironment(t)
	testRoot := t.TempDir()
	childSource := createTestRepository(t, filepath.Join(testRoot, "child-source"), map[string]string{
		"child.txt": "child base\n",
	})
	root := createTestRepository(t, filepath.Join(testRoot, "root"), map[string]string{
		".onedev-buildspec.yml": "version: 1\n",
	})
	addTestSubmodule(t, root, childSource, "deps/child")
	commitTestRepository(t, root, "add child")

	childDir := filepath.Join(root, "deps", "child")
	writeTestFile(t, filepath.Join(childDir, "child.txt"), "child local\n")
	runTestGit(t, childDir, "add", ".")
	runTestGit(t, childDir, "commit", "--quiet", "-m", "local only commit")
	childHead := runTestGit(t, childDir, "rev-parse", "HEAD")
	runTestGit(t, root, "add", "deps/child")
	commitTestRepository(t, root, "bump child gitlink")

	var pushed []pushedSnapshot
	snapshot, err := prepareLocalSnapshot(root, filepath.Join(root, ".onedev-buildspec.yml"),
		collectTestPushes(&pushed), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if len(pushed) != 1 || pushed[0].repositoryDir != childDir || pushed[0].commit != childHead {
		t.Fatalf("expected local submodule commit %s to be sent, got %#v", childHead, pushed)
	}
	if gitlink := runTestGit(t, root, "rev-parse", snapshot+":deps/child"); gitlink != childHead {
		t.Fatalf("root snapshot points to %s instead of local submodule commit %s", gitlink, childHead)
	}
}

func TestPrepareLocalSnapshotWithoutSubmodules(t *testing.T) {
	setupTestGitEnvironment(t)
	testRoot := t.TempDir()
	root := createTestRepository(t, filepath.Join(testRoot, "root"), map[string]string{
		".onedev-buildspec.yml": "version: 1\n",
		"root.txt":              "root base\n",
	})
	writeTestFile(t, filepath.Join(root, ".onedev-buildspec.yml"), "version: 2\n")
	writeTestFile(t, filepath.Join(root, "root.txt"), "root local\n")

	head := runTestGit(t, root, "rev-parse", "HEAD")
	indexBefore := runTestGit(t, root, "diff", "--cached", "--binary")
	statusBefore := runTestGit(t, root, "status", "--porcelain")

	var pushed []pushedSnapshot
	snapshot, err := prepareLocalSnapshot(root, filepath.Join(root, ".onedev-buildspec.yml"),
		collectTestPushes(&pushed), testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if len(pushed) != 0 {
		t.Fatalf("expected no pushes, got %#v", pushed)
	}
	if snapshot == head {
		t.Fatalf("expected snapshot to differ from HEAD %s", head)
	}
	if content := runTestGit(t, root, "show", snapshot+":.onedev-buildspec.yml"); content != "version: 2" {
		t.Fatalf("unexpected build spec snapshot content: %q", content)
	}
	if content := runTestGit(t, root, "show", snapshot+":root.txt"); content != "root local" {
		t.Fatalf("unexpected snapshot content: %q", content)
	}

	assertTestGitOutput(t, root, head, "rev-parse", "HEAD")
	assertTestGitOutput(t, root, indexBefore, "diff", "--cached", "--binary")
	assertTestGitOutput(t, root, statusBefore, "status", "--porcelain")
}

// setupTestGitEnvironment keeps the git configuration and repository of the
// developer running the tests from leaking into them.
func setupTestGitEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	// Submodule clones are not created by createTestRepository, so the identity
	// has to come from the environment to make commits work everywhere.
	t.Setenv("GIT_AUTHOR_NAME", "TOD Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "tod-test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "TOD Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "tod-test@example.com")
	for _, name := range []string{"GIT_INDEX_FILE", "GIT_DIR", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func collectTestPushes(pushed *[]pushedSnapshot) localSnapshotPusher {
	return func(repositoryDir, commit string) error {
		*pushed = append(*pushed, pushedSnapshot{repositoryDir: repositoryDir, commit: commit})
		return nil
	}
}

func testLogger() *log.Logger {
	return log.New(bytes.NewBuffer(nil), "", 0)
}

func createTestRepository(t *testing.T, path string, files map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, path, "init", "--quiet")
	for name, content := range files {
		writeTestFile(t, filepath.Join(path, name), content)
	}
	runTestGit(t, path, "add", ".")
	commitTestRepository(t, path, "initial")
	return path
}

func addTestSubmodule(t *testing.T, repositoryDir, source, path string) {
	t.Helper()
	runTestGit(t, repositoryDir, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", source, path)
}

func commitTestRepository(t *testing.T, repositoryDir, message string) {
	t.Helper()
	runTestGit(t, repositoryDir, "add", ".")
	runTestGit(t, repositoryDir, "commit", "--quiet", "-m", message)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runTestGit(t *testing.T, repositoryDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repositoryDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func assertTestGitOutput(t *testing.T, repositoryDir, expected string, args ...string) {
	t.Helper()
	if actual := runTestGit(t, repositoryDir, args...); actual != expected {
		t.Fatalf("git %s changed output\nexpected: %q\nactual:   %q", strings.Join(args, " "), expected, actual)
	}
}
