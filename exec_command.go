package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
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

	"gopkg.in/ini.v1"
)

const (
	refName     = "refs/onedev/tod"
	execSection = "exec"
	projectKey  = "project"
	workdirKey  = "workdir"
	tokenKey    = "token"
	paramPrefix = "param."
)

type ExecCommand struct {
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
	if len(parts) == 2 {
		key := strings.TrimSpace(parts[0])
		valuesString := strings.TrimSpace(parts[1])
		if len(valuesString) == 0 {
			delete(p, key)
		} else {
			values := strings.Split(valuesString, ",")
			for i := range values {
				values[i] = strings.TrimSpace(values[i])
			}
			p[key] = values
		}
		return nil
	} else {
		return fmt.Errorf("invalid parameter format: %s", value)
	}
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

func (execCommand ExecCommand) Execute(args []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting user home:", err)
		os.Exit(1)
	}

	// Define path to the INI configuration file
	configFilePath := filepath.Join(homeDir, ".tod/config")

	project := ""
	workdir := ""
	token := ""
	params := make(ParamMap)

	if _, err := os.Stat(configFilePath); !os.IsNotExist(err) {
		cfg, err := ini.Load(configFilePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error reading config file:", err)
			os.Exit(1)
		}

		section := cfg.Section(execSection)

		project = section.Key(projectKey).String()
		workdir = section.Key(workdirKey).String()
		token = section.Key(tokenKey).String()

		for _, key := range section.Keys() {
			if strings.HasPrefix(key.Name(), paramPrefix) {
				paramName := strings.TrimPrefix(key.Name(), paramPrefix)
				paramValues := strings.Split(key.Value(), ",")
				params[paramName] = paramValues
			}
		}
	}

	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	fs.StringVar(&project, "project", project, "Specify project url, for instance: https://onedev.example.com/your/project")
	fs.StringVar(&workdir, "workdir", workdir, "Specify working directory to run job against")
	fs.StringVar(&token, "token", token, "Specify access token with permission to run specified job")
	fs.Var(&params, "param", "Specify job parameters in form of key=value")

	fs.Parse(args)

	if project == "" {
		fmt.Fprintln(os.Stderr, "Missing project url. Check https://code.onedev.io/onedev/tod for details")
		os.Exit(1)
	}

	if token == "" {
		fmt.Fprintln(os.Stderr, "Missing access token. Check https://code.onedev.io/onedev/tod for details")
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Missing job name. Check https://code.onedev.io/onedev/tod for details")
		os.Exit(1)
	}

	jobName := fs.Arg(0)

	if workdir == "" {
		workdir = "."
	}

	absoluteWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting absolute path:", err)
		os.Exit(1)
	}

	gitDir := filepath.Join(absoluteWorkdir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Invalid workdir: not a git working directory:", absoluteWorkdir)
		os.Exit(1)
	}

	buildSpecFile := filepath.Join(absoluteWorkdir, ".onedev-buildspec.yml")
	if _, err := os.Stat(buildSpecFile); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Invalid workdir: OneDev build spec not found:", absoluteWorkdir)
		os.Exit(1)
	}

	hostIndex := strings.Index(project, "://") + 3
	if hostIndex == -1 {
		fmt.Fprintln(os.Stderr, "Invalid project url:", project)
		os.Exit(1)
	}

	pathIndex := strings.Index(project[hostIndex:], "/") + 1
	if pathIndex == 0 {
		fmt.Fprintln(os.Stderr, "Invalid project url:", project)
		os.Exit(1)
	}

	serverUrl := project[:hostIndex+pathIndex-1]
	projectPath := project[hostIndex+pathIndex:]

	if projectPath == "" {
		fmt.Fprintln(os.Stderr, "Invalid project url:", project)
		os.Exit(1)
	}

	err = checkVersion(serverUrl, token)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error checking version:", err)
		os.Exit(1)
	}

	client := http.Client{}

	query := url.Values{}
	query.Set("query", fmt.Sprintf("\"Path\" is \"%s\"", projectPath))
	query.Set("offset", "0")
	query.Set("count", "1")

	targetUrl, err := url.Parse(serverUrl)
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

	req.Header.Set("Authorization", "Bearer "+token)

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
		fmt.Fprintln(os.Stderr, "Project not found on server:", projectPath)
		os.Exit(1)
	}

	projectId := projects[0]["id"]

	fmt.Println("Collecting local changes...")

	cmd := exec.Command("git", "stash", "create")
	cmd.Dir = absoluteWorkdir
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error executing command:", err)
		os.Exit(1)
	}

	runCommit := strings.TrimSpace(string(out))

	if runCommit == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = absoluteWorkdir
		cmd.Stderr = os.Stderr

		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error executing command:", err)
			os.Exit(1)
		}

		runCommit = strings.TrimSpace(string(out))
	}

	fmt.Println("Sending local changes to server...")

	cmd = exec.Command("git", "-c", "http.extraHeader=Authorization: Bearer "+token, "push", "-f", project, runCommit+":"+refName)
	cmd.Dir = absoluteWorkdir

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
	req.Header.Set("Authorization", "Bearer "+token)

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

			targetUrl := fmt.Sprintf("%s/~api/job-runs/%v", serverUrl, buildId)

			req, err := http.NewRequest("DELETE", targetUrl, nil)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error cancelling build:", err)
				return
			}
			req.Header.Set("Authorization", "Bearer "+token)

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
	req.Header.Set("Authorization", "Bearer "+token)

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
