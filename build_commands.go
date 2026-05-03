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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Query builds and run CI/CD jobs",
}

var buildListCmd = &cobra.Command{
	Use:   "list",
	Short: "Query builds in the current (or a specified) project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		query, _ := cmd.Flags().GetString("query")
		offset, _ := cmd.Flags().GetInt("offset")
		count, _ := cmd.Flags().GetInt("count")

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := queryEntities("query-builds", project, currentProject, query, offset, count)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var buildShowCmd = &cobra.Command{
	Use:   "show <build-reference>",
	Short: "Get information about a single build",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := getEntityData("get-build", args[0], currentProject)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var buildLogCmd = &cobra.Command{
	Use:   "log <build-reference>",
	Short: "Get the log for a build",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		info, err := getBuildDetail(args[0], currentProject)
		if err != nil {
			return err
		}
		projectId := int64(info["projectId"].(float64))
		buildNumber := int64(info["number"].(float64))
		logURL := fmt.Sprintf("%s/~downloads/projects/%d/builds/%d/log", config.ServerUrl, projectId, buildNumber)
		body, err := apiGetAbsolute(logURL)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var buildFileContentCmd = &cobra.Command{
	Use:   "file-content <build-reference>",
	Short: "Get the content of a file at the commit used by a build",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		if path == "" {
			return fmt.Errorf("--path is required")
		}
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		build, err := getBuildDetail(args[0], currentProject)
		if err != nil {
			return err
		}
		project, _ := build["project"].(string)
		commitHash, _ := build["commitHash"].(string)
		rawURL := fmt.Sprintf("%s/%s/~raw/%s/%s", config.ServerUrl, project, commitHash, path)
		body, err := apiGetAbsolute(rawURL)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var buildChangesSinceSuccessCmd = &cobra.Command{
	Use:   "changes-since-success <build-reference>",
	Short: "Show file changes since the previous successful similar build",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		build, err := getBuildDetail(args[0], currentProject)
		if err != nil {
			return err
		}
		previousBody, err := apiGetBytes("get-previous-successful-similar-build", url.Values{
			"currentProject": {currentProject},
			"reference":      {args[0]},
		})
		if err != nil {
			return err
		}
		var previous map[string]interface{}
		if err := json.Unmarshal(previousBody, &previous); err != nil {
			return fmt.Errorf("failed to parse previous build response: %v", err)
		}
		projectId := int(build["projectId"].(float64))
		patchURL := fmt.Sprintf("%s/~downloads/projects/%d/patch?%s", config.ServerUrl, projectId, url.Values{
			"old-commit": {previous["commitHash"].(string)},
			"new-commit": {build["commitHash"].(string)},
		}.Encode())
		body, err := apiGetAbsolute(patchURL)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var buildRunCmd = &cobra.Command{
	Use:   "run <job-name>",
	Short: "Run a CI/CD job against a specific branch or tag",
	Long: `Run a CI/CD job against a specific branch or tag in the repository.
Either --branch or --tag must be specified, but not both.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := cliLogger("[run] ")
		return runBuildJobCommand(cmd, args, logger)
	},
}

var buildRunLocalCmd = &cobra.Command{
	Use:   "run-local <job-name>",
	Short: "Run a CI/CD job against local changes",
	Long: `Run a CI/CD job against your local changes without committing/pushing.
This command stashes your local changes, pushes them to a temporal ref,
and streams the job execution logs back to your terminal.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logger := cliLogger("[run-local] ")
		return runLocalBuildJobCommand(cmd, args, logger)
	},
}

var buildSpecCmd = &cobra.Command{
	Use:   "spec",
	Short: "Download the OneDev build spec YAML definition",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := http.NewRequest("GET", config.ServerUrl+"/~api/build-spec-schema.yml", nil)
		if err != nil {
			return err
		}
		body, err := makeAPICall(req)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var buildCheckSpecCmd = &cobra.Command{
	Use:   "check-spec",
	Short: "Validate and upgrade .onedev-buildspec.yml in the working directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBuildspecCheckCommand(cmd, cliLogger("[build] "))
	},
}

type ParamMap map[string][]string

func (p ParamMap) String() string {
	str := "{"
	for key, values := range p {
		str += fmt.Sprintf("%s: %v, ", key, values)
	}
	str += "}"
	return str
}

func (p ParamMap) Set(value string) error {
	parts := strings.Split(value, "=")
	if len(parts) != 2 {
		return fmt.Errorf("invalid parameter format (expected key=value): %s", value)
	}

	key := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])

	if len(key) == 0 {
		return fmt.Errorf("parameter key cannot be empty: %s", value)
	}

	if len(val) == 0 {
		return fmt.Errorf("parameter value cannot be empty: %s", value)
	}

	if existingValues, exists := p[key]; exists {
		p[key] = append(existingValues, val)
	} else {
		p[key] = []string{val}
	}

	return nil
}

func runBuildJobCommand(cmd *cobra.Command, args []string, logger *log.Logger) error {
	_, currentProject, err := inferProject(workingDirOf(cmd), logger)
	if err != nil {
		return err
	}

	jobName := args[0]
	branch, _ := cmd.Flags().GetString("branch")
	tag, _ := cmd.Flags().GetString("tag")

	if branch == "" && tag == "" {
		return fmt.Errorf("either --branch or --tag must be specified")
	}
	if branch != "" && tag != "" {
		return fmt.Errorf("option --branch and --tag cannot be specified at the same time")
	}

	params, err := paramsFromFlags(cmd)
	if err != nil {
		return err
	}

	jobMap := map[string]interface{}{
		"jobName": jobName,
		"reason":  "Submitted via tod",
	}
	if branch != "" {
		jobMap["branch"] = branch
	} else {
		jobMap["tag"] = tag
	}
	if len(params) > 0 {
		paramStrings := make([]string, 0)
		for key, values := range params {
			for _, value := range values {
				paramStrings = append(paramStrings, fmt.Sprintf("%s=%s", key, value))
			}
		}
		jobMap["params"] = paramStrings
	}

	fmt.Printf("Running job '%s' against ", jobName)
	if branch != "" {
		fmt.Printf("branch '%s'...\n", branch)
	} else {
		fmt.Printf("tag '%s'...\n", tag)
	}

	build, err := runJob(currentProject, jobMap)
	if err != nil {
		return err
	}

	return streamBuild(build)
}

func runLocalBuildJobCommand(cmd *cobra.Command, args []string, logger *log.Logger) error {
	jobName := args[0]
	params, err := paramsFromFlags(cmd)
	if err != nil {
		return err
	}

	fmt.Println("Collecting local changes...")

	build, err := runLocalJob(jobName, workingDirOf(cmd), params, "Submitted via tod", logger)
	if err != nil {
		return err
	}

	fmt.Println("Sending local changes to server...")

	return streamBuild(build)
}

func paramsFromFlags(cmd *cobra.Command) (ParamMap, error) {
	paramArray, err := cmd.Flags().GetStringArray("param")
	if err != nil {
		return nil, fmt.Errorf("failed to get parameters: %v", err)
	}

	params := make(ParamMap)
	for _, paramStr := range paramArray {
		if err := params.Set(paramStr); err != nil {
			return nil, fmt.Errorf("failed to parse parameter: %v", err)
		}
	}
	return params, nil
}

func streamBuild(build map[string]interface{}) error {
	buildId := int(build["id"].(float64))
	buildNumber := int(build["number"].(float64))

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	return streamBuildLog(buildId, buildNumber, signalChannel)
}

func runBuildspecCheckCommand(cmd *cobra.Command, logger *log.Logger) error {
	fmt.Println("Checking build spec...")

	if err := checkBuildSpec(workingDirOf(cmd), logger); err != nil {
		return fmt.Errorf("failed to check build spec: %v", err)
	}

	fmt.Println("Build spec checked successfully.")
	return nil
}

func runJob(currentProject string, jobMap map[string]interface{}) (map[string]interface{}, error) {
	jobBytes, err := json.Marshal(jobMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal map to JSON: %v", err)
	}
	jobData := string(jobBytes)

	apiURL := config.ServerUrl + "/~api/tod/run-job?currentProject=" + url.QueryEscape(currentProject)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(jobData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make API call: %v", err)
	}

	var build map[string]interface{}
	if err := json.Unmarshal(body, &build); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return build, nil
}

func runLocalJob(jobName string, workingDir string, params map[string][]string,
	reason string, logger *log.Logger) (map[string]interface{}, error) {

	_, project, err := inferProject(workingDir, logger)
	if err != nil {
		return nil, err
	}

	gitRoot, err := findGitRoot(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find git root directory: %v", err)
	}

	buildSpecFile := filepath.Join(gitRoot, ".onedev-buildspec.yml")
	if _, err := os.Stat(buildSpecFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("build spec not found: %v", err)
	}

	cmd := exec.Command("git", "add", buildSpecFile)
	cmd.Dir = workingDir
	logger.Printf("Running command: git add %s\n", buildSpecFile)
	out, err := cmd.CombinedOutput()
	logger.Printf("Command output:\n%s", string(out))
	if err != nil {
		return nil, fmt.Errorf("failed to add build spec file to git index: %v", err)
	}

	projectUrl := config.ServerUrl + "/" + project

	cmd = exec.Command("git", "stash", "create")
	cmd.Dir = workingDir

	logger.Printf("Running command: git stash create\n")
	out, err = cmd.CombinedOutput()
	logger.Printf("Command output:\n%s", string(out))
	if err != nil {
		return nil, fmt.Errorf("failed to execute git stash create: %v", err)
	}

	runCommit := strings.TrimSpace(string(out))

	if runCommit == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = workingDir

		logger.Printf("Running command: git rev-parse HEAD\n")
		out, err := cmd.CombinedOutput()
		logger.Printf("Command output:\n%s", string(out))
		if err != nil {
			return nil, fmt.Errorf("failed to execute git rev-parse HEAD: %v", err)
		}

		runCommit = strings.TrimSpace(string(out))
	}

	cmd = exec.Command("git", "-c", "http.extraHeader=Authorization: Bearer "+config.AccessToken, "push", "-f", projectUrl, runCommit+":refs/onedev/tod")
	cmd.Dir = workingDir

	logger.Printf("Running command: git push -f %s %s:refs/onedev/tod\n", projectUrl, runCommit)
	stdoutStderr, err := cmd.CombinedOutput()
	logger.Printf("Command output:\n%s", string(stdoutStderr))
	if err != nil {
		return nil, fmt.Errorf("failed to push local changes: %v", err)
	}

	jobMap := map[string]interface{}{
		"commitHash": runCommit,
		"refName":    "refs/onedev/tod",
		"jobName":    jobName,
		"params":     params,
		"reason":     reason,
	}

	return runJob(project, jobMap)
}

func findGitRoot(workingDir string) (string, error) {
	_, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git executable not found in system path")
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = workingDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git working directory: %s", workingDir)
	}

	gitRoot := strings.TrimSpace(string(output))
	return gitRoot, nil
}

func checkBuildSpec(workingDir string, logger *log.Logger) error {
	_, project, err := inferProject(workingDir, logger)
	if err != nil {
		return err
	}

	gitRoot, err := findGitRoot(workingDir)
	if err != nil {
		return fmt.Errorf("failed to find git root directory: %v", err)
	}

	buildSpecFile := filepath.Join(gitRoot, ".onedev-buildspec.yml")
	if _, err := os.Stat(buildSpecFile); os.IsNotExist(err) {
		return fmt.Errorf("build spec not found: %v", err)
	}

	buildSpecContent, err := os.ReadFile(buildSpecFile)
	if err != nil {
		return fmt.Errorf("failed to read build spec: %v", err)
	}

	apiURL := config.ServerUrl + "/~api/tod/check-build-spec?project=" + url.QueryEscape(project)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(buildSpecContent)))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "text/plain")

	body, err := makeAPICall(req)
	if err != nil {
		return fmt.Errorf("failed to make API call: %v", err)
	}

	// Only write the file if the content has changed
	if string(body) != string(buildSpecContent) {
		logger.Printf("Writing updated build spec to file: %s", buildSpecFile)
		err = os.WriteFile(buildSpecFile, body, 0644)
		if err != nil {
			return fmt.Errorf("failed to write updated build spec: %v", err)
		}
	}

	return nil
}

func streamBuildLog(buildId int, buildNumber int, signalChannel <-chan os.Signal) error {
	buildFinished := false
	var mutex sync.Mutex
	targetUrl, err := url.Parse(config.ServerUrl)
	if err != nil {
		return fmt.Errorf("failed to parse server url: %v", err)
	}
	targetUrl.Path = fmt.Sprintf("~api/streaming/build-logs/%d", buildId)

	req, err := http.NewRequest("GET", targetUrl.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to stream build log: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.AccessToken)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to stream build log: %v", err)
	}
	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to stream build log: %v", err)
	}

	go func() {
		<-signalChannel
		mutex.Lock()
		defer mutex.Unlock()
		if !buildFinished {
			fmt.Println("Cancelling build...")
			client := &http.Client{}

			targetUrl := fmt.Sprintf("%s/~api/job-runs/%d", config.ServerUrl, buildId)

			req, err := http.NewRequest("DELETE", targetUrl, nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to cancel build:", err)
				return
			}
			req.Header.Set("Authorization", "Bearer "+config.AccessToken)

			resp, err := client.Do(req)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to cancel build:", err)
				return
			}
			defer resp.Body.Close()

			err = checkResponse(resp)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to cancel build:", err)
				os.Exit(1)
			}
		}
	}()

	for {
		var len int32
		err := binary.Read(resp.Body, binary.BigEndian, &len)
		if err != nil {
			return fmt.Errorf("failed to stream build log: %v", err)
		}

		if len > 0 {
			logEntryBytes := make([]byte, len)
			_, err := io.ReadFull(resp.Body, logEntryBytes)
			if err != nil {
				return fmt.Errorf("failed to stream build log: %v", err)
			}

			var logEntry map[string]interface{}
			err = json.Unmarshal(logEntryBytes, &logEntry)
			if err != nil {
				return fmt.Errorf("failed to decode log entry json: %v", err)
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
				return fmt.Errorf("failed to read build status: %v", err)
			}

			buildStatus := string(buildStatusBytes)

			var message = "Build #" + strconv.Itoa(buildNumber) + " is " + strings.ToLower(buildStatus)
			if buildStatus == "SUCCESSFUL" {
				mutex.Lock()
				buildFinished = true
				mutex.Unlock()
				fmt.Println(wrapWithBold(wrapWithGreen(message)))
				break
			} else if buildStatus == "FAILED" || buildStatus == "CANCELLED" || buildStatus == "TIMED_OUT" {
				mutex.Lock()
				buildFinished = true
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

func wrapWithRed(message string) string {
	return fmt.Sprintf("\033[31m%s\033[0m", message)
}

func wrapWithGreen(message string) string {
	return fmt.Sprintf("\033[32m%s\033[0m", message)
}

func wrapWithColor(message, color string) string {
	return fmt.Sprintf("\033[%sm%s\033[0m", color, message)
}

func wrapWithBold(message string) string {
	return fmt.Sprintf("\033[1m%s\033[0m", message)
}

func initBuildCommands() {
	buildCmd.PersistentFlags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")

	buildListCmd.Flags().String("project", "", "Project to query (defaults to the current project)")
	buildListCmd.Flags().String("query", "", "OneDev build query string (run 'tod schema show query-builds' for valid keys)")
	buildListCmd.Flags().Int("offset", 0, "Starting offset")
	buildListCmd.Flags().Int("count", DefaultQueryCount, "Number of builds to return")

	buildFileContentCmd.Flags().String("path", "", "Path of the file relative to repository root (required)")

	buildRunCmd.Flags().String("branch", "", "Branch to run the job against (either --branch or --tag is required)")
	buildRunCmd.Flags().String("tag", "", "Tag to run the job against (either --branch or --tag is required)")
	buildRunCmd.Flags().StringArrayP("param", "p", nil, "Job parameter in form key=value (repeatable)")

	buildRunLocalCmd.Flags().StringArrayP("param", "p", nil, "Job parameter in form key=value (repeatable)")

	buildCmd.AddCommand(
		buildListCmd,
		buildShowCmd,
		buildLogCmd,
		buildFileContentCmd,
		buildChangesSinceSuccessCmd,
		buildRunCmd,
		buildRunLocalCmd,
		buildSpecCmd,
		buildCheckSpecCmd,
	)
}
