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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %s: %v", req.URL.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d error for endpoint %s: %s", resp.StatusCode, req.URL.String(), string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %v", req.URL.String(), err)
	}

	return body, nil
}

func getJSONMapFromAPI(apiURL string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	body, err := makeAPICall(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make API call: %v", err)
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return jsonData, nil
}

func inferProject(workingDir string, logger *log.Logger) (string, string, error) {
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
		logger.Printf("Using remote '%s' to infer project", remote)

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
			logger.Printf("Using remote '%s' to infer project", remote)

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

func hasUncommittedChanges(workingDir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %v", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

func checkoutPullRequest(workingDir string, pullRequestReference string, logger *log.Logger) error {
	remote, project, err := inferProject(workingDir, logger)
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
	number, _ := pullRequest["number"].(float64)
	headCommitHash, _ := pullRequest["headCommitHash"].(string)

	projectUrl := config.ServerUrl + "/" + targetProject

	fetchCmd := exec.Command("git", "-c", "http.extraHeader=Authorization: Bearer "+config.AccessToken, "fetch", projectUrl, headCommitHash)
	fetchCmd.Dir = workingDir
	stdoutStderr, err := fetchCmd.CombinedOutput()
	logger.Printf("Running command: git fetch %s %s\n", projectUrl, headCommitHash)
	logger.Printf("Command output:\n%s", string(stdoutStderr))
	if err != nil {
		return fmt.Errorf("git fetch failed: %v", err)
	}

	var localBranch string
	needsUpstream := false
	if open && project == sourceProject {
		localBranch = sourceBranch
		needsUpstream = true
	} else {
		localBranch = fmt.Sprintf("pr-%d", int(number))
	}

	hasChanges, err := hasUncommittedChanges(workingDir)
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %v", err)
	}

	if hasChanges {
		return fmt.Errorf("you have uncommitted changes in your working directory. Please commit or stash your changes first")
	}

	checkBranchCmd := exec.Command("git", "rev-parse", "--verify", localBranch)
	checkBranchCmd.Dir = workingDir
	stdoutStderr, err = checkBranchCmd.CombinedOutput()
	logger.Printf("Running command: git rev-parse --verify %s\n", localBranch)
	logger.Printf("Command output:\n%s", string(stdoutStderr))

	if err == nil {
		checkoutCmd := exec.Command("git", "checkout", localBranch)
		checkoutCmd.Dir = workingDir
		stdoutStderr, err := checkoutCmd.CombinedOutput()
		logger.Printf("Running command: git checkout %s\n", localBranch)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git checkout failed: %v", err)
		}
		mergeCmd := exec.Command("git", "merge", "--ff-only", headCommitHash)
		mergeCmd.Dir = workingDir
		stdoutStderr, err = mergeCmd.CombinedOutput()
		logger.Printf("Running command: git merge --ff-only %s\n", headCommitHash)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git merge --ff-only failed: %v", err)
		}
	} else {
		checkoutCmd := exec.Command("git", "checkout", "-b", localBranch, headCommitHash)
		checkoutCmd.Dir = workingDir
		stdoutStderr, err := checkoutCmd.CombinedOutput()
		logger.Printf("Running command: git checkout -b %s %s\n", localBranch, headCommitHash)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git checkout -b failed: %v", err)
		}
	}

	if needsUpstream {
		updateRemoteTrackRefCmd := exec.Command("git", "update-ref", fmt.Sprintf("refs/remotes/%s/%s", remote, sourceBranch), headCommitHash)
		updateRemoteTrackRefCmd.Dir = workingDir
		stdoutStderr, err = updateRemoteTrackRefCmd.CombinedOutput()
		logger.Printf("Running command: git update-ref %s %s\n", fmt.Sprintf("refs/remotes/%s/%s", remote, sourceBranch), headCommitHash)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err == nil {
			upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", localBranch+"@{upstream}")
			upstreamCmd.Dir = workingDir
			output, err := upstreamCmd.Output()
			expectedUpstream := fmt.Sprintf("%s/%s", remote, sourceBranch)

			if err != nil || strings.TrimSpace(string(output)) != expectedUpstream {
				setUpstreamCmd := exec.Command("git", "branch", "--set-upstream-to", expectedUpstream, localBranch)
				setUpstreamCmd.Dir = workingDir
				stdoutStderr, err := setUpstreamCmd.CombinedOutput()
				logger.Printf("Running command: git branch --set-upstream-to %s %s\n", expectedUpstream, localBranch)
				logger.Printf("Command output:\n%s", string(stdoutStderr))
				if err != nil {
					return fmt.Errorf("git branch --set-upstream-to failed: %v", err)
				} else {
					return nil
				}
			}
		} else {
			return fmt.Errorf("git update-ref failed: %v", err)
		}
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

func checkVersion(serverUrl string, accessToken string) error {
	client := &http.Client{}

	req, err := http.NewRequest("GET", serverUrl+"/~api/version/compatible-tod-versions", nil)
	if err != nil {
		return fmt.Errorf("failed to request compatible versions: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request compatible versions: %v", err)
	}

	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to request compatible versions: %v", err)
	}

	var compatibleVersions CompatibleVersions

	err = json.NewDecoder(resp.Body).Decode(&compatibleVersions)
	if err != nil {
		return fmt.Errorf("failed to decode compatible versions response: %v", err)
	}

	semVer, err := semver.NewVersion(version)
	if err != nil {
		return fmt.Errorf("failed to parse semver: %v", err)
	}

	var minVersionSatisfied = true

	if compatibleVersions.MinVersion != "" {
		minSemVer, err := semver.NewVersion(compatibleVersions.MinVersion)
		if err != nil {
			return fmt.Errorf("failed to parse semver: %v", err)
		}
		if semVer.LessThan(minSemVer) {
			minVersionSatisfied = false
		}
	}

	var maxVersionSatisfied = true

	if compatibleVersions.MaxVersion != "" {
		maxSemVer, err := semver.NewVersion(compatibleVersions.MaxVersion)
		if err != nil {
			return fmt.Errorf("failed to parse semver: %v", err)
		}
		if semVer.GreaterThan(maxSemVer) {
			maxVersionSatisfied = false
		}
	}

	if minVersionSatisfied && maxVersionSatisfied {
		return nil
	} else if compatibleVersions.MinVersion != "" && compatibleVersions.MaxVersion != "" {
		return fmt.Errorf("this server requires version >= %s and version <= %s, please download from https://code.onedev.io/onedev/tod/~builds?query=%%22Job%%22+is+%%22Release%%22+and+successful", compatibleVersions.MinVersion, compatibleVersions.MaxVersion)
	} else if compatibleVersions.MinVersion != "" {
		return fmt.Errorf("this server requires version >= %s, please download from https://code.onedev.io/onedev/tod/~builds?query=%%22Job%%22+is+%%22Release%%22+and+successful", compatibleVersions.MinVersion)
	} else {
		return fmt.Errorf("this server requires version <= %s, please download from https://code.onedev.io/onedev/tod/~builds?query=%%22Job%%22+is+%%22Release%%22+and+successful", compatibleVersions.MaxVersion)
	}
}
