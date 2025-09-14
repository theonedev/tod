package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Masterminds/semver"
)

// Custom error type for API errors with additional context
type APIError struct {
	StatusCode int
	Endpoint   string
	Response   string
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Response)
	} else {
		return e.Response
	}
}

func makeAPICall(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+config.AccessToken)

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &APIError{
			StatusCode: -1,
			Endpoint:   req.URL.String(),
			Response:   fmt.Sprintf("failed to call API: %v", err),
		}
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Endpoint:   req.URL.String(),
			Response:   string(body),
		}
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &APIError{
			StatusCode: -1,
			Endpoint:   req.URL.String(),
			Response:   fmt.Sprintf("failed to read response: %v", err),
		}
	}

	return body, nil
}

func getJSONMapFromAPI(apiURL string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, &APIError{
			StatusCode: -1,
			Endpoint:   apiURL,
			Response:   fmt.Sprintf("failed to create request: %v", err),
		}
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return nil, &APIError{
			StatusCode: -1,
			Endpoint:   apiURL,
			Response:   fmt.Sprintf("failed to parse JSON response: %v", err),
		}
	}

	return jsonData, nil
}

// inferProject extracts the project path from a git working directory
func inferProject(workingDir string) (string, error) {
	// 1. Check if git executable is in system path
	_, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git executable not found in system path")
	}

	// 2. Check if workingDir is a git working directory
	gitDir := filepath.Join(workingDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", fmt.Errorf("not a git working directory: %s", workingDir)
	}

	// 3. Check if the git working directory has a remote named origin
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("remote 'origin' not found in git repository")
	}

	remoteUrl := strings.TrimSpace(string(output))
	if remoteUrl == "" {
		return "", fmt.Errorf("remote 'origin' not found in git repository")
	}

	// 4. Check if the remote URL is of the expected format and extract project path
	var project string

	// Check for HTTP(S) format: http[s]://host[:port]/project
	if strings.HasPrefix(remoteUrl, "http://") || strings.HasPrefix(remoteUrl, "https://") {
		// Find the protocol separator
		protocolIndex := strings.Index(remoteUrl, "://")
		if protocolIndex == -1 {
			return "", fmt.Errorf("invalid remote URL format: %s", remoteUrl)
		}

		// Find the path separator after the host[:port]
		hostPart := remoteUrl[protocolIndex+3:]
		pathIndex := strings.Index(hostPart, "/")
		if pathIndex == -1 {
			return "", fmt.Errorf("invalid remote URL format: %s", remoteUrl)
		}

		project = hostPart[pathIndex+1:]
	} else if strings.HasPrefix(remoteUrl, "ssh://") {
		// Check for SSH format: ssh://host[:port]/project
		protocolIndex := strings.Index(remoteUrl, "://")
		if protocolIndex == -1 {
			return "", fmt.Errorf("invalid remote URL format: %s", remoteUrl)
		}

		hostPart := remoteUrl[protocolIndex+3:]
		pathIndex := strings.Index(hostPart, "/")
		if pathIndex == -1 {
			return "", fmt.Errorf("invalid remote URL format: %s", remoteUrl)
		}

		project = hostPart[pathIndex+1:]
	} else {
		return "", fmt.Errorf("unsupported remote URL format: %s (expected http[s]:// or ssh://)", remoteUrl)
	}

	// Remove .git suffix if present
	project = strings.TrimSuffix(project, ".git")

	// Ensure project path is not empty after processing
	if project == "" {
		return "", fmt.Errorf("invalid remote URL format: %s (empty project path)", remoteUrl)
	}

	return project, nil
}

// hasUncommittedChanges checks if there are uncommitted changes in the working directory
func hasUncommittedChanges(workingDir string) (bool, error) {
	// Check for staged and unstaged changes
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	// If output is empty, no changes; if not empty, there are changes
	return len(strings.TrimSpace(string(output))) > 0, nil
}

func checkoutPullRequest(workingDir string, pullRequestReference string, logger *log.Logger) error {
	project, err := inferProject(workingDir)
	if err != nil {
		return fmt.Errorf("error inferring project: %w", err)
	}

	// Build the API URL
	urlQuery := url.Values{
		"currentProject": {project},
		"reference":      {pullRequestReference},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/get-pull-request?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		return fmt.Errorf("error making API call: %w", err)
	}

	var pullRequest map[string]interface{}
	if err := json.Unmarshal(body, &pullRequest); err != nil {
		return fmt.Errorf("error parsing JSON response: %w", err)
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

	// Run 'git fetch' against projectUrl to get headCommitHash
	cmd := exec.Command("git", "-c", "http.extraHeader=Authorization: Bearer "+config.AccessToken, "fetch", projectUrl, headCommitHash)
	cmd.Dir = workingDir

	stdoutStderr, err := cmd.CombinedOutput()
	logger.Printf("Running command: git fetch %s %s\n", projectUrl, headCommitHash)
	logger.Printf("Command output:\n%s", string(stdoutStderr))
	if err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
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
		return fmt.Errorf("error checking for uncommitted changes: %w", err)
	}

	if hasChanges {
		return fmt.Errorf("you have uncommitted changes in your working directory. Please commit or stash your changes first")
	}

	// Check if the local branch exists
	checkBranchCmd := exec.Command("git", "rev-parse", "--verify", localBranch)
	checkBranchCmd.Dir = workingDir
	if err := checkBranchCmd.Run(); err == nil {
		// Branch exists, switch to it
		checkoutCmd := exec.Command("git", "checkout", localBranch)
		checkoutCmd.Dir = workingDir
		stdoutStderr, err := checkoutCmd.CombinedOutput()
		logger.Printf("Running command: git checkout %s\n", localBranch)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git checkout failed: %w", err)
		}
		// Fast-forward the branch to headCommitHash
		mergeCmd := exec.Command("git", "merge", "--ff-only", headCommitHash)
		mergeCmd.Dir = workingDir
		stdoutStderr, err = mergeCmd.CombinedOutput()
		logger.Printf("Running command: git merge --ff-only %s\n", headCommitHash)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git merge --ff-only failed: %w", err)
		}
	} else {
		// Branch does not exist, create and switch to it
		checkoutCmd := exec.Command("git", "checkout", "-b", localBranch, headCommitHash)
		checkoutCmd.Dir = workingDir
		stdoutStderr, err := checkoutCmd.CombinedOutput()
		logger.Printf("Running command: git checkout -b %s %s\n", localBranch, headCommitHash)
		logger.Printf("Command output:\n%s", string(stdoutStderr))
		if err != nil {
			return fmt.Errorf("git checkout -b failed: %w", err)
		}
	}

	if needsUpstream {
		// Always ensure remote-tracking branch points to headCommitHash
		updateRemoteTrackRefCmd := exec.Command("git", "update-ref", fmt.Sprintf("refs/remotes/origin/%s", sourceBranch), headCommitHash)
		updateRemoteTrackRefCmd.Dir = workingDir
		if err := updateRemoteTrackRefCmd.Run(); err == nil {
			// Set up upstream of localBranch to track sourceBranch if the upstream is not set correctly
			// Check if upstream is already set to the correct branch
			upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", localBranch+"@{upstream}")
			upstreamCmd.Dir = workingDir
			output, err := upstreamCmd.Output()
			expectedUpstream := fmt.Sprintf("origin/%s", sourceBranch)

			// Set upstream only if it needs to be set/updated
			if err != nil || strings.TrimSpace(string(output)) != expectedUpstream {
				// Now set upstream
				setUpstreamCmd := exec.Command("git", "branch", "--set-upstream-to", expectedUpstream, localBranch)
				setUpstreamCmd.Dir = workingDir
				stdoutStderr, err := setUpstreamCmd.CombinedOutput()
				logger.Printf("Running command: git branch --set-upstream-to %s %s\n", expectedUpstream, localBranch)
				logger.Printf("Command output:\n%s", string(stdoutStderr))
				if err != nil {
					return fmt.Errorf("git branch --set-upstream-to failed: %w", err)
				} else {
					return nil
				}
			}
		} else {
			return fmt.Errorf("git update-ref failed: %w", err)
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

func runJob(project string, currentProject string, jobMap map[string]interface{},
	logger *log.Logger) (map[string]interface{}, error) {

	jobBytes, err := json.Marshal(jobMap)
	if err != nil {
		logger.Printf("Failed to marshal map to JSON: %v", createErrorString(err))
		return nil, fmt.Errorf("failed to marshal map to JSON: %w", err)
	}
	jobData := string(jobBytes)

	var urlQuery = url.Values{
		"project":        {project},
		"currentProject": {currentProject},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/run-job?" + urlQuery.Encode()

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(jobData))
	if err != nil {
		logger.Printf("Failed to create request: %v", createErrorString(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		logger.Printf("Failed to make API call: %v", createErrorString(err))
		return nil, fmt.Errorf("failed to make API call: %w", err)
	}

	var build map[string]interface{}
	if err := json.Unmarshal(body, &build); err != nil {
		logger.Printf("Failed to parse JSON response: %v", err)
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return build, nil
}

func runLocalJob(jobName string, workingDir string, params map[string][]string,
	reason string, logger *log.Logger) (map[string]interface{}, error) {

	if workingDir == "" {
		workingDir = "."
	}

	// Derive project path from working directory
	project, err := inferProject(workingDir)
	if err != nil {
		return nil, fmt.Errorf("error getting project from working directory: %w", err)
	}

	// Construct project URL from server URL and project path
	projectUrl := config.ServerUrl + "/" + project

	buildSpecFile := filepath.Join(workingDir, ".onedev-buildspec.yml")
	if _, err := os.Stat(buildSpecFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("invalid working dir: OneDev build spec not found: %s", workingDir)
	}

	// Collect local changes
	cmd := exec.Command("git", "stash", "create")
	cmd.Dir = workingDir

	logger.Printf("Running command: git stash create\n")
	out, err := cmd.CombinedOutput()
	logger.Printf("Command output:\n%s", string(out))
	if err != nil {
		return nil, fmt.Errorf("error executing git stash create: %w", err)
	}

	runCommit := strings.TrimSpace(string(out))

	if runCommit == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = workingDir

		logger.Printf("Running command: git rev-parse HEAD\n")
		out, err := cmd.CombinedOutput()
		logger.Printf("Command output:\n%s", string(out))
		if err != nil {
			return nil, fmt.Errorf("error executing git rev-parse HEAD: %w", err)
		}

		runCommit = strings.TrimSpace(string(out))
	}

	// Push local changes to server
	cmd = exec.Command("git", "-c", "http.extraHeader=Authorization: Bearer "+config.AccessToken, "push", "-f", projectUrl, runCommit+":refs/onedev/tod")
	cmd.Dir = workingDir

	logger.Printf("Running command: git push -f %s %s:refs/onedev/tod\n", projectUrl, runCommit)
	stdoutStderr, err := cmd.CombinedOutput()
	logger.Printf("Command output:\n%s", string(stdoutStderr))
	if err != nil {
		return nil, fmt.Errorf("error pushing local changes: %w", err)
	}

	jobMap := map[string]interface{}{
		"commitHash": runCommit,
		"refName":    "refs/onedev/tod",
		"jobName":    jobName,
		"params":     params,
		"reason":     reason,
	}

	return runJob(project, project, jobMap, logger)
}

func migrateBuildSpec(workingDir string, logger *log.Logger) error {

	if workingDir == "" {
		workingDir = "."
	}

	buildSpecFile := filepath.Join(workingDir, ".onedev-buildspec.yml")
	if _, err := os.Stat(buildSpecFile); os.IsNotExist(err) {
		return fmt.Errorf("invalid working dir: OneDev build spec not found: %s", workingDir)
	}

	// Read the build spec file content
	buildSpecContent, err := os.ReadFile(buildSpecFile)
	if err != nil {
		logger.Printf("Failed to read build spec file: %v", err)
		return fmt.Errorf("failed to read build spec file: %w", err)
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/migrate-build-spec"

	// Create POST request with build spec content
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(buildSpecContent)))
	if err != nil {
		logger.Printf("Failed to create request: %v", err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logger.Printf("Failed to make API call: %v", err)
		return fmt.Errorf("failed to make API call: %w", err)
	}

	// Write the migrated content back to the build spec file
	err = os.WriteFile(buildSpecFile, body, 0644)
	if err != nil {
		logger.Printf("Failed to write migrated build spec: %v", err)
		return fmt.Errorf("failed to write migrated build spec: %w", err)
	}

	logger.Printf("Build spec migrated successfully")

	return nil
}

func checkVersion(serverUrl string, accessToken string) error {
	// Make a GET request to the API endpoint
	client := &http.Client{}

	// Create a new GET request
	req, err := http.NewRequest("GET", serverUrl+"/~api/version/compatible-tod-versions", nil)
	if err != nil {
		return fmt.Errorf("error requesting compatible versions: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Send the request using the HTTP client
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error requesting compatible versions: %w", err)
	}

	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		return fmt.Errorf("error requesting compatible versions: %w", err)
	}

	// Decode the JSON response into a VersionInfo struct
	var compatibleVersions CompatibleVersions

	err = json.NewDecoder(resp.Body).Decode(&compatibleVersions)
	if err != nil {
		return fmt.Errorf("error decoding compatible versions response: %w", err)
	}

	semVer, err := semver.NewVersion(version)
	if err != nil {
		return fmt.Errorf("error parsing semver: %w", err)
	}

	var minVersionSatisfied = true

	if compatibleVersions.MinVersion != "" {
		minSemVer, err := semver.NewVersion(compatibleVersions.MinVersion)
		if err != nil {
			return fmt.Errorf("error parsing semver: %w", err)
		}
		if semVer.LessThan(minSemVer) {
			minVersionSatisfied = false
		}
	}

	var maxVersionSatisfied = true

	if compatibleVersions.MaxVersion != "" {
		maxSemVer, err := semver.NewVersion(compatibleVersions.MaxVersion)
		if err != nil {
			return fmt.Errorf("error parsing semver: %w", err)
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

func streamBuildLog(buildId int, buildNumber int, signalChannel <-chan os.Signal, buildFinished *bool, mutex *sync.Mutex) error {
	targetUrl, err := url.Parse(config.ServerUrl)
	if err != nil {
		return fmt.Errorf("error parsing server url: %w", err)
	}
	targetUrl.Path = fmt.Sprintf("~api/streaming/build-logs/%d", buildId)

	req, err := http.NewRequest("GET", targetUrl.String(), nil)
	if err != nil {
		return fmt.Errorf("error streaming build log: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.AccessToken)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error streaming build log: %w", err)
	}
	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		return fmt.Errorf("error streaming build log: %w", err)
	}

	// Handle cancellation in background goroutine
	go func() {
		<-signalChannel
		mutex.Lock()
		defer mutex.Unlock()
		if !*buildFinished {
			fmt.Println("Cancelling build...")
			client := &http.Client{}

			targetUrl := fmt.Sprintf("%s/~api/job-runs/%d", config.ServerUrl, buildId)

			req, err := http.NewRequest("DELETE", targetUrl, nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error cancelling build:", err)
				return
			}
			req.Header.Set("Authorization", "Bearer "+config.AccessToken)

			resp, err := client.Do(req)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error cancelling build:", err)
				return
			}
			defer resp.Body.Close()

			err = checkResponse(resp)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error cancelling build:", err)
				os.Exit(1)
			}
		}
	}()

	for {
		var len int32
		err := binary.Read(resp.Body, binary.BigEndian, &len)
		if err != nil {
			return fmt.Errorf("error streaming build log: %w", err)
		}

		if len > 0 {
			logEntryBytes := make([]byte, len)
			_, err := io.ReadFull(resp.Body, logEntryBytes)
			if err != nil {
				return fmt.Errorf("error streaming build log: %s", resp.Status)
			}

			var logEntry map[string]interface{}
			err = json.Unmarshal(logEntryBytes, &logEntry)
			if err != nil {
				return fmt.Errorf("error decoding log entry json: %w", err)
			}

			line := ""
			messages := logEntry["messages"].([]interface{})
			for _, messageNode := range messages {
				message := messageNode.(map[string]interface{})["text"].(string)
				styleNode := messageNode.(map[string]interface{})["style"].(map[string]interface{})

				fgColor := styleNode["color"].(string)
				if fgColor != "fg-default" {
					message = wrapWithColor(message, fgColor)
				}

				bgColor := styleNode["backgroundColor"].(string)
				if bgColor != "bg-default" {
					message = wrapWithColor(message, bgColor)
				}

				bold := styleNode["bold"].(bool)
				if bold {
					message = wrapWithBold(message)
				}

				line += message
			}
			fmt.Println(line)
		} else if len < 0 {
			buildStatusBytes := make([]byte, -len)
			_, err := io.ReadFull(resp.Body, buildStatusBytes)
			if err != nil {
				return fmt.Errorf("error reading build status: %w", err)
			}

			buildStatus := string(buildStatusBytes)

			var message = "Build #" + strconv.Itoa(buildNumber) + " is " + strings.ToLower(buildStatus)
			if buildStatus == "SUCCESSFUL" {
				mutex.Lock()
				*buildFinished = true
				mutex.Unlock()
				fmt.Println(wrapWithBold(wrapWithGreen(message)))
				break
			} else if buildStatus == "FAILED" || buildStatus == "CANCELLED" || buildStatus == "TIMED_OUT" {
				mutex.Lock()
				*buildFinished = true
				mutex.Unlock()
				fmt.Println(wrapWithBold(wrapWithRed(message)))
				break
			} else {
				fmt.Println(wrapWithBold(message))
			}
		}
	}
	return nil
}
