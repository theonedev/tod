package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const todRef = "refs/onedev/tod"

const localSnapshotPushAttempts = 3

type localSnapshotPusher func(repositoryDir, commit string) error

// prepareLocalSnapshot creates a commit representing the repository's local
// state. Initialized submodules are processed recursively and pushed unless the
// server is already known to have their snapshot.
func prepareLocalSnapshot(repositoryDir, buildSpecFile string, push localSnapshotPusher,
	logger *log.Logger) (string, error) {

	submodules, err := initializedSubmodules(repositoryDir, logger)
	if err != nil {
		return "", err
	}

	childCommits := make(map[string]string, len(submodules))
	for _, submodulePath := range submodules {
		submoduleDir := filepath.Join(repositoryDir, filepath.FromSlash(submodulePath))
		childCommit, err := prepareLocalSnapshot(submoduleDir, "", push, logger)
		if err != nil {
			return "", fmt.Errorf("failed to collect local changes in submodule %q: %w", submodulePath, err)
		}
		childCommits[submodulePath] = childCommit

		onRemote, err := reachableFromRemotes(submoduleDir, childCommit, logger)
		if err != nil {
			return "", fmt.Errorf("failed to check local changes in submodule %q: %w", submodulePath, err)
		}
		if !onRemote {
			if err := push(submoduleDir, childCommit); err != nil {
				return "", fmt.Errorf("failed to send local changes in submodule %q: %w", submodulePath, err)
			}
		}
	}

	baseCommit, indexFile, cleanup, err := createBaseSnapshot(repositoryDir, buildSpecFile, logger)
	defer cleanup()
	if err != nil {
		return "", err
	}

	return rewriteSnapshotGitlinks(repositoryDir, baseCommit, indexFile, childCommits, logger)
}

// reachableFromRemotes tells whether commit is contained in a remote tracking
// ref, in which case the server is expected to have it already. Snapshots
// holding local changes are dangling commits and are never reachable, while an
// untouched submodule is skipped so that pushing to it requires no permission.
func reachableFromRemotes(repositoryDir, commit string, logger *log.Logger) (bool, error) {
	unreachable, err := gitOutput(repositoryDir, logger, "rev-list", "--max-count=1", commit, "--not", "--remotes")
	if err != nil {
		return false, err
	}
	return unreachable == "", nil
}

func initializedSubmodules(repositoryDir string, logger *log.Logger) ([]string, error) {
	output, err := gitRawOutput(repositoryDir, logger, "submodule", "foreach", "--quiet", `printf '%s\0' "$sm_path"`)
	if err != nil {
		return nil, fmt.Errorf("failed to list initialized submodules: %w", err)
	}

	var paths []string
	for _, field := range bytes.Split(output, []byte{0}) {
		if len(field) != 0 {
			paths = append(paths, string(field))
		}
	}
	return paths, nil
}

// createBaseSnapshot commits the local state of repositoryDir via a temporary
// index, leaving the index of the user untouched. The returned cleanup function
// is always safe to call, including when an error is returned.
func createBaseSnapshot(repositoryDir, stageFile string, logger *log.Logger) (
	commit string, indexFile string, cleanup func(), err error) {

	indexFile, cleanup, err = copyGitIndex(repositoryDir, logger)
	if err != nil {
		return "", "", cleanup, err
	}

	if stageFile != "" {
		relativePath, relativeErr := filepath.Rel(repositoryDir, stageFile)
		if relativeErr != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
			return "", "", cleanup, fmt.Errorf("build spec is outside of git working tree: %s", stageFile)
		}
		if _, err := gitOutputWithIndex(repositoryDir, indexFile, logger, "add", "--", filepath.ToSlash(relativePath)); err != nil {
			return "", "", cleanup, fmt.Errorf("failed to add build spec to temporary git index: %w", err)
		}
	}

	commit, err = gitOutputWithIndex(repositoryDir, indexFile, logger, "stash", "create")
	if err != nil {
		return "", "", cleanup, err
	}
	if commit == "" {
		commit, err = gitOutput(repositoryDir, logger, "rev-parse", "HEAD")
		if err != nil {
			return "", "", cleanup, err
		}
	}
	return commit, indexFile, cleanup, nil
}

// copyGitIndex returns the path of a throw away copy of the index of the
// repository. The returned cleanup function is always safe to call, including
// when an error is returned.
func copyGitIndex(repositoryDir string, logger *log.Logger) (string, func(), error) {
	noCleanup := func() {}

	indexPath, err := gitOutput(repositoryDir, logger, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", noCleanup, err
	}
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(repositoryDir, indexPath)
	}

	temporary, err := os.CreateTemp("", "tod-git-index-*")
	if err != nil {
		return "", noCleanup, fmt.Errorf("failed to create temporary git index: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() { _ = os.Remove(temporaryPath) }
	if err := temporary.Close(); err != nil {
		return "", cleanup, fmt.Errorf("failed to close temporary git index: %w", err)
	}

	contents, err := os.ReadFile(indexPath)
	switch {
	case err == nil:
		if err := os.WriteFile(temporaryPath, contents, 0o600); err != nil {
			return "", cleanup, fmt.Errorf("failed to copy git index: %w", err)
		}
	case os.IsNotExist(err):
		// A repository without index yet needs git to populate the copy.
		if err := os.Remove(temporaryPath); err != nil {
			return "", cleanup, fmt.Errorf("failed to initialize temporary git index: %w", err)
		}
		if _, err := gitOutputWithIndex(repositoryDir, temporaryPath, logger, "read-tree", "HEAD"); err != nil {
			return "", cleanup, fmt.Errorf("failed to initialize temporary git index: %w", err)
		}
	default:
		return "", cleanup, fmt.Errorf("failed to read git index: %w", err)
	}
	return temporaryPath, cleanup, nil
}

// rewriteSnapshotGitlinks points the gitlinks of baseCommit at the snapshots
// taken of the submodules, as git stash does not record submodule state.
func rewriteSnapshotGitlinks(repositoryDir, baseCommit, indexFile string,
	childCommits map[string]string, logger *log.Logger) (string, error) {

	if len(childCommits) == 0 {
		return baseCommit, nil
	}

	if _, err := gitOutputWithIndex(repositoryDir, indexFile, logger, "read-tree", baseCommit); err != nil {
		return "", fmt.Errorf("failed to read local snapshot tree: %w", err)
	}
	for submodulePath, childCommit := range childCommits {
		_, err := gitOutputWithIndex(repositoryDir, indexFile, logger, "update-index", "--add", "--cacheinfo",
			"160000,"+childCommit+","+submodulePath)
		if err != nil {
			return "", fmt.Errorf("failed to update temporary gitlink for submodule %q: %w", submodulePath, err)
		}
	}

	tree, err := gitOutputWithIndex(repositoryDir, indexFile, logger, "write-tree")
	if err != nil {
		return "", err
	}
	baseTree, err := gitOutput(repositoryDir, logger, "rev-parse", baseCommit+"^{tree}")
	if err != nil {
		return "", err
	}
	if tree == baseTree {
		return baseCommit, nil
	}

	head, err := gitOutput(repositoryDir, logger, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commitObject, err := gitRawOutput(repositoryDir, logger, "cat-file", "commit", baseCommit)
	if err != nil {
		return "", err
	}
	author, committer := commitIdentityHeaders(commitObject)
	if author == "" || committer == "" {
		return "", fmt.Errorf("failed to read commit identity from local snapshot %s", baseCommit)
	}

	contents := fmt.Sprintf("tree %s\nparent %s\n%s\n%s\n\nSubmitted via tod\n", tree, head, author, committer)
	cmd := exec.Command("git", "hash-object", "-t", "commit", "-w", "--stdin")
	cmd.Dir = repositoryDir
	cmd.Stdin = strings.NewReader(contents)
	output, err := runGit(cmd, "git hash-object -t commit -w --stdin", logger)
	if err != nil {
		return "", fmt.Errorf("failed to create local snapshot commit: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func commitIdentityHeaders(commit []byte) (string, string) {
	var author, committer string
	for _, line := range strings.Split(string(commit), "\n") {
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "author ") {
			author = line
		} else if strings.HasPrefix(line, "committer ") {
			committer = line
		}
	}
	return author, committer
}

func pushLocalSnapshot(repositoryDir, commit string, logger *log.Logger) error {
	_, project, err := inferProjectWithRemoteValidation(repositoryDir, true)
	if err != nil {
		return err
	}
	return pushLocalSnapshotToProject(repositoryDir, project, commit, logger)
}

func pushLocalSnapshotToProject(repositoryDir, project, commit string, logger *log.Logger) error {
	projectURL := config.ServerUrl + "/" + project
	description := fmt.Sprintf("git push -f %s %s:%s", projectURL, commit, todRef)
	for attempt := 1; attempt <= localSnapshotPushAttempts; attempt++ {
		cmd, cleanup, err := newTrustedGitCommand(repositoryDir, "-c",
			"http.extraHeader=Authorization: Bearer "+config.AccessToken,
			"push", "-f", projectURL, commit+":"+todRef)
		if err != nil {
			return fmt.Errorf("failed to prepare git push: %w", err)
		}

		_, err = runGit(cmd, description, logger)
		cleanup()
		if err == nil {
			return nil
		}
		if !isRefLockFailure(err) || attempt == localSnapshotPushAttempts {
			return fmt.Errorf("failed to push local changes: %w", err)
		}

		delay := time.Duration(1<<(attempt-1))*100*time.Millisecond +
			time.Duration(rand.Intn(100))*time.Millisecond
		logger.Printf("Snapshot ref is locked; retrying push in %s (attempt %d/%d)\n",
			delay, attempt+1, localSnapshotPushAttempts)
		time.Sleep(delay)
	}
	panic("unreachable")
}

func isRefLockFailure(err error) bool {
	var commandErr *gitCommandError
	return errors.As(err, &commandErr) &&
		strings.Contains(strings.ToLower(commandErr.stderr), "cannot lock ref")
}

func gitCommandWithIndex(repositoryDir, indexFile string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = repositoryDir
	cmd.Env = environmentWith("GIT_INDEX_FILE", indexFile)
	return cmd
}

func environmentWith(key, value string) []string {
	prefix := key + "="
	current := os.Environ()
	environment := make([]string, 0, len(current)+1)
	for _, entry := range current {
		if !strings.HasPrefix(entry, prefix) {
			environment = append(environment, entry)
		}
	}
	return append(environment, prefix+value)
}

func gitOutput(repositoryDir string, logger *log.Logger, args ...string) (string, error) {
	output, err := gitRawOutput(repositoryDir, logger, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func gitRawOutput(repositoryDir string, logger *log.Logger, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repositoryDir
	return runGit(cmd, "git "+strings.Join(args, " "), logger)
}

func gitOutputWithIndex(repositoryDir, indexFile string, logger *log.Logger, args ...string) (string, error) {
	cmd := gitCommandWithIndex(repositoryDir, indexFile, args...)
	output, err := runGit(cmd, "git "+strings.Join(args, " ")+" (with temporary index)", logger)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// runGit captures stdout separately from stderr, as warnings printed by git
// would otherwise end up in output parsed as commit id.
func runGit(cmd *exec.Cmd, description string, logger *log.Logger) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logger.Printf("Running command: %s\n", description)
	err := cmd.Run()
	logger.Printf("Command output:\n%s%s", stdout.String(), stderr.String())
	if err != nil {
		return nil, &gitCommandError{
			description: description,
			err:         err,
			stderr:      strings.TrimSpace(stderr.String()),
		}
	}
	return stdout.Bytes(), nil
}

type gitCommandError struct {
	description string
	err         error
	stderr      string
}

func (e *gitCommandError) Error() string {
	return fmt.Sprintf("%s failed: %v: %s", e.description, e.err, e.stderr)
}

func (e *gitCommandError) Unwrap() error {
	return e.err
}
