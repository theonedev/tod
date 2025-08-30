package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
)

const (
	refName     = "refs/onedev/tod"
	paramPrefix = "param."
)

type RunJobCommand struct {
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

	// Append to existing values instead of replacing
	if existingValues, exists := p[key]; exists {
		p[key] = append(existingValues, val)
	} else {
		p[key] = []string{val}
	}

	return nil
}

type FilteredWriter struct {
}

func (fw *FilteredWriter) Write(p []byte) (n int, err error) {
	scanner := bufio.NewScanner(strings.NewReader(string(p)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "To ") && !strings.Contains(line, "->") && !strings.Contains(line, "Everything up-to-date") {
			fmt.Fprintln(os.Stderr, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return len(p), nil
}

var buildFinished bool
var mutex sync.Mutex

// Execute executes the run job command
func (runJobCommand RunJobCommand) Execute(cobraCmd *cobra.Command, args []string) {
	// Extract job name from arguments
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Error: exactly one job name is required")
		os.Exit(1)
	}
	jobName := args[0]

	// Get working directory from command flag, default to current directory
	workingDir, _ := cobraCmd.Flags().GetString("working-dir")
	if workingDir == "" {
		workingDir = "."
	}

	// Get command line parameters
	paramArray, err := cobraCmd.Flags().GetStringArray("param")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting parameters:", err)
		os.Exit(1)
	}

	params := make(ParamMap)

	// Parse parameter array into ParamMap
	for _, paramStr := range paramArray {
		if err := params.Set(paramStr); err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing parameter:", err)
			os.Exit(1)
		}
	}

	// Derive project path from working directory
	project, err := inferProject(workingDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting project from working directory:", err)
		os.Exit(1)
	}

	// Construct project URL from server URL and project path
	projectUrl := config.ServerUrl + "/" + project

	buildSpecFile := filepath.Join(workingDir, ".onedev-buildspec.yml")
	if _, err := os.Stat(buildSpecFile); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Invalid working dir: OneDev build spec not found:", workingDir)
		os.Exit(1)
	}

	err = checkVersion(config.ServerUrl, config.AccessToken)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error checking version:", err)
		os.Exit(1)
	}

	client := http.Client{}

	query := url.Values{}
	query.Set("query", fmt.Sprintf("\"Path\" is \"%s\"", project))
	query.Set("offset", "0")
	query.Set("count", "1")

	targetUrl, err := url.Parse(config.ServerUrl)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing server url", err)
		os.Exit(1)
	}
	targetUrl.Path = "~api/projects"
	targetUrl.RawQuery = query.Encode()

	req, err := http.NewRequest("GET", targetUrl.String(), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error querying project", err)
		os.Exit(1)
	}

	req.Header.Set("Authorization", "Bearer "+config.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error querying project", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error querying project:", err)
		os.Exit(1)
	}

	var projects []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&projects)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error decoding project query response:", err)
		os.Exit(1)
	}

	if len(projects) == 0 {
		fmt.Fprintln(os.Stderr, "Project not found on server:", project)
		os.Exit(1)
	}

	projectId := projects[0]["id"]

	fmt.Println("Collecting local changes...")

	cmd := exec.Command("git", "stash", "create")
	cmd.Dir = workingDir
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error executing command:", err)
		os.Exit(1)
	}

	runCommit := strings.TrimSpace(string(out))

	if runCommit == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = workingDir
		cmd.Stderr = os.Stderr

		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error executing command:", err)
			os.Exit(1)
		}

		runCommit = strings.TrimSpace(string(out))
	}

	fmt.Println("Sending local changes to server...")

	cmd = exec.Command("git", "-c", "http.extraHeader=Authorization: Bearer "+config.AccessToken, "push", "-f", projectUrl, runCommit+":"+refName)
	cmd.Dir = workingDir

	cmd.Stderr = &FilteredWriter{}

	_, err = cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error executing command:", err)
		os.Exit(1)
	}

	targetUrl.Path = "~api/job-runs"
	targetUrl.RawQuery = ""

	jobRunData := map[string]interface{}{
		"@type":      "JobRunOnCommit",
		"projectId":  projectId,
		"commitHash": runCommit,
		"refName":    refName,
		"jobName":    jobName,
		"params":     params,
		"reason":     "Submitted via tod",
	}

	jsonData, _ := json.Marshal(jobRunData)

	req, err = http.NewRequest("POST", targetUrl.String(), bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error requesting build:", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.AccessToken)

	resp, err = client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error requesting build:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error requesting build:", err)
		os.Exit(1)
	}

	var buildId interface{}
	err = json.NewDecoder(resp.Body).Decode(&buildId)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error decoding build request response:", err)
		os.Exit(1)
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChannel
		mutex.Lock()
		defer mutex.Unlock()
		if !buildFinished {
			fmt.Println("Cancelling build...")
			client := &http.Client{}

			targetUrl := fmt.Sprintf("%s/~api/job-runs/%v", config.ServerUrl, buildId)

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

	targetUrl.Path = fmt.Sprintf("~api/streaming/build-logs/%v", buildId)

	req, err = http.NewRequest("GET", targetUrl.String(), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error streaming build log:", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+config.AccessToken)

	resp, err = client.Do(req)
	if err != nil {
		fmt.Println("Error streaming build log:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	err = checkResponse(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error streaming build log:", err)
		os.Exit(1)
	}

	for {
		var len int32
		err := binary.Read(resp.Body, binary.BigEndian, &len)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error streaming build log:", err)
			os.Exit(1)
		}

		if len > 0 {
			logEntryBytes := make([]byte, len)
			_, err := io.ReadFull(resp.Body, logEntryBytes)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error streaming build log:", resp.Status)
				os.Exit(1)
			}

			var logEntry map[string]interface{}
			err = json.Unmarshal(logEntryBytes, &logEntry)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error decoding log entry json:", err)
				os.Exit(1)
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
				fmt.Fprintln(os.Stderr, "Error reading build status:", err)
				os.Exit(1)
			}

			buildStatus := string(buildStatusBytes)

			var message = "Build is " + strings.ToLower(buildStatus)
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
}
