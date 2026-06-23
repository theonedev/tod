package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver"
)

func makeAPICall(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+config.AccessToken)

	client, err := newHTTPClient(config)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %s: %v", req.URL.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d error for endpoint %s: %s", resp.StatusCode, req.URL.String(), string(body))
	}

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %v", req.URL.String(), err)
	}

	return body, nil
}

func inferProject(workingDir string) (string, string, error) {
	_, err := exec.LookPath("git")
	if err != nil {
		return "", "", fmt.Errorf("git executable not found in system path")
	}

	var prefix = "failed to infer OneDev project from working directory '" + workingDir + "': "
	var suffix = ". Working directory is expected to be inside a git repository, with one of the remote pointing to OneDev project"

	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = workingDir
	_, err = cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf(prefix + "working directory is not inside a git repository" + suffix)
	}

	apiURL := config.ServerUrl + "/~api/tod/get-clone-roots"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", "", fmt.Errorf(prefix+"failed to create request: %v"+suffix, err)
	}

	body, err := makeAPICall(req)
	if err != nil {
		return "", "", fmt.Errorf(prefix+"failed to make API call: %v"+suffix, err)
	}

	var cloneRoots map[string]interface{}
	if err := json.Unmarshal(body, &cloneRoots); err != nil {
		return "", "", fmt.Errorf(prefix+"failed to parse JSON response: %v"+suffix, err)
	}

	httpCloneRoot, _ := cloneRoots["http"].(string)
	sshCloneRoot, _ := cloneRoots["ssh"].(string)

	cmd = exec.Command("git", "remote")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf(prefix+"failed to list git remotes: %v"+suffix, err)
	}

	remotes := strings.Fields(strings.TrimSpace(string(output)))
	if len(remotes) == 0 {
		return "", "", fmt.Errorf(prefix + "no git remotes found in repository" + suffix)
	}

	if len(remotes) == 1 {
		remote := remotes[0]

		cmd = exec.Command("git", "remote", "get-url", remote)
		cmd.Dir = workingDir
		output, err := cmd.Output()
		if err != nil {
			return "", "", fmt.Errorf(prefix+"failed to get URL for remote '%s': %v"+suffix, remote, err)
		}

		remoteUrl := strings.TrimSpace(string(output))
		if remoteUrl == "" {
			return "", "", fmt.Errorf(prefix+"remote '%s' has no URL"+suffix, remote)
		}

		project, err := extractProjectFromUrl(remoteUrl)
		if err != nil {
			return "", "", fmt.Errorf(prefix+"failed to extract project from remote '%s': %v"+suffix, remote, err)
		}

		return remote, project, nil
	}

	for _, remote := range remotes {
		cmd = exec.Command("git", "remote", "get-url", remote)
		cmd.Dir = workingDir
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		remoteUrl := strings.TrimSpace(string(output))
		if remoteUrl == "" {
			continue
		}

		if matchesCloneRoot(remoteUrl, httpCloneRoot, sshCloneRoot) {
			project, err := extractProjectFromUrl(remoteUrl)
			if err != nil {
				return "", "", fmt.Errorf(prefix+"failed to extract project from remote '%s': %v"+suffix, remote, err)
			}
			return remote, project, nil
		}
	}

	return "", "", fmt.Errorf(prefix + "no remote found corresponding to a OneDev project" + suffix)
}

func matchesCloneRoot(remoteUrl, httpCloneRoot, sshCloneRoot string) bool {
	remoteProtocol, remoteHostAndPort, err := parseUrlComponents(remoteUrl)
	if err != nil {
		return false
	}

	httpProtocol, httpHostAndPort, err := parseUrlComponents(httpCloneRoot)
	if err == nil && remoteProtocol == httpProtocol && remoteHostAndPort == httpHostAndPort {
		return true
	}

	if sshCloneRoot != "" {
		sshProtocol, sshHostAndPort, err := parseUrlComponents(sshCloneRoot)
		if err == nil && remoteProtocol == sshProtocol && remoteHostAndPort == sshHostAndPort {
			return true
		}
	}

	return false
}

func parseUrlComponents(url string) (protocol, hostAndPort string, err error) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "ssh://") {
		protocolIndex := strings.Index(url, "://")
		protocol = url[:protocolIndex+3]
		remainingPart := url[protocolIndex+3:]

		if atIndex := strings.Index(remainingPart, "@"); atIndex != -1 {
			remainingPart = remainingPart[atIndex+1:]
		}

		pathIndex := strings.Index(remainingPart, "/")
		if pathIndex == -1 {
			hostAndPort = remainingPart
		} else {
			hostAndPort = remainingPart[:pathIndex]
		}
	}

	return protocol, hostAndPort, nil
}

func extractProjectFromUrl(remoteUrl string) (string, error) {
	var project string

	if strings.HasPrefix(remoteUrl, "http://") || strings.HasPrefix(remoteUrl, "https://") || strings.HasPrefix(remoteUrl, "ssh://") {
		protocolIndex := strings.Index(remoteUrl, "://")
		if protocolIndex == -1 {
			return "", fmt.Errorf("invalid URL format")
		}

		hostPart := remoteUrl[protocolIndex+3:]
		pathIndex := strings.Index(hostPart, "/")
		if pathIndex == -1 {
			return "", fmt.Errorf("invalid URL format")
		}

		project = hostPart[pathIndex+1:]
	} else {
		return "", fmt.Errorf("unsupported URL format")
	}

	project = strings.TrimSuffix(project, ".git")

	if project == "" {
		return "", fmt.Errorf("project path is empty in URL")
	}

	return project, nil
}

// currentBranch returns the name of the currently checked-out branch in
// workingDir, or an empty string if the repo is in detached HEAD state.
func currentBranch(workingDir string) (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = workingDir
	out, err := cmd.Output()
	if err != nil {
		// detached HEAD – not an error in the OS sense, just no branch
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func hasUncommittedChanges(workingDir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %v", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

func checkoutFetchedBranch(workingDir string, remote string, branch string, commitHash string, logger *log.Logger) error {
	localBranch := branch

	checkBranchCmd := exec.Command("git", "rev-parse", "--verify", "--quiet", localBranch)
	checkBranchCmd.Dir = workingDir
	_, err := checkBranchCmd.CombinedOutput()
	logger.Printf("Running command: git rev-parse --verify --quiet %s\n", localBranch)

	if err == nil {
		checkoutCmd := exec.Command("git", "checkout", localBranch)
		checkoutCmd.Dir = workingDir
		stdoutStderr, err := checkoutCmd.CombinedOutput()
		logger.Printf("Running command: git checkout %s\n", localBranch)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git checkout failed: %v", err)
		}
		mergeCmd := exec.Command("git", "merge", "--ff-only", commitHash)
		mergeCmd.Dir = workingDir
		stdoutStderr, err = mergeCmd.CombinedOutput()
		logger.Printf("Running command: git merge --ff-only %s\n", commitHash)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git merge --ff-only failed: %v", err)
		}
	} else {
		checkoutCmd := exec.Command("git", "checkout", "-b", localBranch, commitHash)
		checkoutCmd.Dir = workingDir
		stdoutStderr, err := checkoutCmd.CombinedOutput()
		logger.Printf("Running command: git checkout -b %s %s\n", localBranch, commitHash)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git checkout -b failed: %v", err)
		}
	}

	updateRemoteTrackRefCmd := exec.Command("git", "update-ref", fmt.Sprintf("refs/remotes/%s/%s", remote, branch), commitHash)
	updateRemoteTrackRefCmd.Dir = workingDir
	stdoutStderr, err := updateRemoteTrackRefCmd.CombinedOutput()
	logger.Printf("Running command: git update-ref %s %s\n", fmt.Sprintf("refs/remotes/%s/%s", remote, branch), commitHash)
	logger.Printf("Command output:\n%s", string(stdoutStderr))
	if err != nil {
		return fmt.Errorf("git update-ref failed: %v", err)
	}

	upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", localBranch+"@{upstream}")
	upstreamCmd.Dir = workingDir
	output, err := upstreamCmd.Output()
	expectedUpstream := fmt.Sprintf("%s/%s", remote, branch)

	if err != nil || strings.TrimSpace(string(output)) != expectedUpstream {
		setUpstreamCmd := exec.Command("git", "branch", "--set-upstream-to", expectedUpstream, localBranch)
		setUpstreamCmd.Dir = workingDir
		stdoutStderr, err := setUpstreamCmd.CombinedOutput()
		logger.Printf("Running command: git branch --set-upstream-to %s %s\n", expectedUpstream, localBranch)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git branch --set-upstream-to failed: %v", err)
		}
	}

	return nil
}

func checkoutIssue(workingDir string, issueReference string, logger *log.Logger) error {
	remote, project, err := inferProject(workingDir)
	if err != nil {
		return err
	}

	issue, err := getIssueDetail(issueReference, project)
	if err != nil {
		return err
	}
	issueProject, _ := issue["Project"].(string)
	if issueProject == "" {
		return fmt.Errorf("issue detail response does not include Project")
	}
	if issueProject != project {
		return fmt.Errorf("issue %s belongs to project %s, but current project is %s", issueReference, issueProject, project)
	}

	body, err := apiPostBytes("ensure-issue-branch", url.Values{
		"currentProject": {project},
		"reference":      {issueReference},
	})
	if err != nil {
		return err
	}

	branch := strings.TrimSpace(string(body))
	if branch == "" {
		return fmt.Errorf("ensure-issue-branch returned empty branch name")
	}

	projectUrl := config.ServerUrl + "/" + project
	fetchCmd, cleanup, err := newTrustedGitCommand(workingDir, "-c", "http.extraHeader=Authorization: Bearer "+config.AccessToken, "fetch", projectUrl, branch)
	if err != nil {
		return fmt.Errorf("failed to prepare git fetch: %v", err)
	}
	defer cleanup()
	stdoutStderr, err := fetchCmd.CombinedOutput()
	logger.Printf("Running command: git fetch %s %s\n", projectUrl, branch)
	logger.Printf("Command output:\n%s", string(stdoutStderr))
	if err != nil {
		return fmt.Errorf("git fetch failed: %v", err)
	}

	revParseCmd := exec.Command("git", "rev-parse", "FETCH_HEAD")
	revParseCmd.Dir = workingDir
	output, err := revParseCmd.Output()
	logger.Printf("Running command: git rev-parse FETCH_HEAD\n")
	if err != nil {
		return fmt.Errorf("git rev-parse FETCH_HEAD failed: %v", err)
	}
	headCommitHash := strings.TrimSpace(string(output))
	if headCommitHash == "" {
		return fmt.Errorf("git rev-parse FETCH_HEAD returned empty commit hash")
	}

	hasChanges, err := hasUncommittedChanges(workingDir)
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %v", err)
	}

	if hasChanges {
		return fmt.Errorf("you have uncommitted changes in your working directory. Please commit or stash your changes first")
	}

	return checkoutFetchedBranch(workingDir, remote, branch, headCommitHash, logger)
}

func checkoutPullRequest(workingDir string, pullRequestReference string, logger *log.Logger) error {
	remote, project, err := inferProject(workingDir)
	if err != nil {
		return err
	}

	urlQuery := url.Values{
		"currentProject": {project},
		"reference":      {pullRequestReference},
	}

	apiURL := config.ServerUrl + "/~api/tod/get-pull-request?" + urlQuery.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	body, err := makeAPICall(req)
	if err != nil {
		return fmt.Errorf("failed to make API call: %v", err)
	}

	var pullRequest map[string]interface{}
	if err := json.Unmarshal(body, &pullRequest); err != nil {
		return fmt.Errorf("failed to parse JSON response: %v", err)
	}

	state, _ := pullRequest["status"].(string)
	open := strings.HasPrefix(state, "OPEN")

	targetProject, _ := pullRequest["targetProject"].(string)

	sourceProject := ""
	if sp, ok := pullRequest["sourceProject"].(string); ok {
		sourceProject = sp
	}
	sourceBranch, _ := pullRequest["sourceBranch"].(string)
	headCommitHash, _ := pullRequest["headCommitHash"].(string)

	if project != sourceProject && project != targetProject {
		return fmt.Errorf("pull request %s has source project %s and target project %s, but current project is %s", pullRequestReference, sourceProject, targetProject, project)
	}

	projectUrl := config.ServerUrl + "/" + targetProject

	fetchCmd, cleanup, err := newTrustedGitCommand(workingDir, "-c", "http.extraHeader=Authorization: Bearer "+config.AccessToken, "fetch", projectUrl, headCommitHash)
	if err != nil {
		return fmt.Errorf("failed to prepare git fetch: %v", err)
	}
	defer cleanup()
	stdoutStderr, err := fetchCmd.CombinedOutput()
	logger.Printf("Running command: git fetch %s %s\n", projectUrl, headCommitHash)
	logger.Printf("Command output:\n%s", string(stdoutStderr))
	if err != nil {
		return fmt.Errorf("git fetch failed: %v", err)
	}

	hasChanges, err := hasUncommittedChanges(workingDir)
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %v", err)
	}

	if hasChanges {
		return fmt.Errorf("you have uncommitted changes in your working directory. Please commit or stash your changes first")
	}

	if open && project == sourceProject {
		return checkoutFetchedBranch(workingDir, remote, sourceBranch, headCommitHash, logger)
	}

	checkoutCmd := exec.Command("git", "checkout", headCommitHash)
	checkoutCmd.Dir = workingDir
	stdoutStderr, err = checkoutCmd.CombinedOutput()
	logger.Printf("Running command: git checkout %s\n", headCommitHash)
	logger.Printf("Command output:\n%s", string(stdoutStderr))
	if err != nil {
		return fmt.Errorf("git checkout failed: %v", err)
	}

	return nil
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			return fmt.Errorf("non-successful status code received (status: %s, message: %s)", resp.Status, string(body))
		} else {
			return fmt.Errorf("non-successful status code received: %s", resp.Status)
		}
	} else {
		return nil
	}
}

type VersionInfo struct {
	ServerVersion         string `json:"serverVersion"`
	MinRequiredTodVersion string `json:"minRequiredTodVersion"`
}

func checkVersion(config *Config) (VersionInfo, error) {
	client, err := newHTTPClient(config)
	if err != nil {
		return VersionInfo{}, err
	}

	req, err := http.NewRequest("GET", config.ServerUrl+"/~api/tod/check-version", nil)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to check version: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+config.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to check version: %v", err)
	}

	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to check version: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to read version info response from %s: %v", req.URL.String(), err)
	}

	var versionInfo VersionInfo
	if err := json.Unmarshal(body, &versionInfo); err != nil {
		responsePreview := strings.TrimSpace(string(body))
		if len(responsePreview) > 512 {
			responsePreview = responsePreview[:512] + "..."
		}
		return VersionInfo{}, fmt.Errorf(
			"failed to decode version info response from %s as JSON (content-type: %q): %v; response: %q",
			req.URL.String(), resp.Header.Get("Content-Type"), err, responsePreview,
		)
	}

	todSemVer, err := semver.NewVersion(version)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("failed to parse tod version %q: %v", version, err)
	}

	if versionInfo.MinRequiredTodVersion != "" {
		minTodSemVer, err := semver.NewVersion(versionInfo.MinRequiredTodVersion)
		if err != nil {
			return VersionInfo{}, fmt.Errorf("failed to parse minimum required tod version %q: %v", versionInfo.MinRequiredTodVersion, err)
		}
		if todSemVer.LessThan(minTodSemVer) {
			return VersionInfo{}, fmt.Errorf("this server requires tod version >= %s (current: %s), please download a newer tod from https://code.onedev.io/onedev/tod/~builds?query=%%22Job%%22+is+%%22Release%%22+and+successful", versionInfo.MinRequiredTodVersion, version)
		}
	}

	if versionInfo.ServerVersion != "" {
		serverSemVer, err := semver.NewVersion(versionInfo.ServerVersion)
		if err != nil {
			return VersionInfo{}, fmt.Errorf("failed to parse server version %q: %v", versionInfo.ServerVersion, err)
		}
		minServerSemVer, err := semver.NewVersion(minRequiredServerVersion)
		if err != nil {
			return VersionInfo{}, fmt.Errorf("failed to parse minimum required server version %q: %v", minRequiredServerVersion, err)
		}
		if serverSemVer.LessThan(minServerSemVer) {
			return VersionInfo{}, fmt.Errorf("this tod requires OneDev server version >= %s (current: %s), please upgrade your OneDev server", minRequiredServerVersion, versionInfo.ServerVersion)
		}
	}

	return versionInfo, nil
}
