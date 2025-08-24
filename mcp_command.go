package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type MCPCommand struct{}

// Configuration for MCP server
type MCPConfig struct {
	Server         string
	Token          string
	LogFile        string
	WorkingDir     string
	CurrentProject string
}

// Global config instance
var globalConfig MCPConfig

// Global logger instance
var logger *log.Logger

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

// JSON-RPC 2.0 standard error codes
const (
	ErrorCodeParseError      = -32700 // Parse error – Invalid JSON was received
	ErrorCodeInvalidRequest  = -32600 // Invalid Request – The JSON sent is not a valid Request object
	ErrorCodeMethodNotFound  = -32601 // Method not found – The method does not exist or is not available
	ErrorCodeInvalidParams   = -32602 // Invalid params – Invalid method parameter(s)
	ErrorCodeInternalError   = -32603 // Internal error – Internal JSON-RPC error
	MaxQueryCount            = 100    // Maximum number of entities to query
	DefaultQueryCount        = 25     // Default number of entities to query
	IssueReferenceDesc       = "issue reference is of form &#35;&lt;number&gt;, &lt;project&gt;&#35;&lt;number&gt;, or &lt;project key&gt;-&lt;number&gt;"
	PullRequestReferenceDesc = "pull request reference is of form &#35;&lt;number&gt;, &lt;project&gt;&#35;&lt;number&gt;, or &lt;project key&gt;-&lt;number&gt;"
)

// MCP Protocol Messages
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Initialize response
type InitializeResult struct {
	ProtocolVersion string      `json:"protocolVersion"`
	Capabilities    interface{} `json:"capabilities"`
	ServerInfo      ServerInfo  `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Tools
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ToolContent `json:"content"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Prompts
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments"`
}

type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type GetPromptParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type GetPromptResult struct {
	Messages []PromptMessage `json:"messages"`
}

type PromptMessage struct {
	Role    string               `json:"role"`
	Content PromptMessageContent `json:"content"`
}

type PromptMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func getCurrentProject() (string, error) {
	if globalConfig.CurrentProject == "" {
		var cmd *exec.Cmd
		cmd = exec.Command("git", "rev-parse", "--is-inside-work-tree")
		cmd.Dir = globalConfig.WorkingDir
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("working dir not inside a git repository: %v", globalConfig.WorkingDir)
		}

		cmd = exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = globalConfig.WorkingDir
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get origin remote URL: %v", err)
		}

		originURL := strings.TrimSpace(string(output))
		if originURL == "" {
			return "", fmt.Errorf("origin remote URL is empty")
		}

		// Find the substring after "//"
		idx := strings.Index(originURL, "//")
		if idx == -1 || idx+2 >= len(originURL) {
			return "", fmt.Errorf("invalid git URL format: %q", originURL)
		}
		// Find the first "/" after the "//"
		slashIdx := strings.Index(originURL[idx+2:], "/")
		if slashIdx == -1 || idx+2+slashIdx+1 > len(originURL) {
			return "", fmt.Errorf("could not find project path in git URL: %q", originURL)
		}

		projectPath := strings.TrimSuffix(originURL[idx+2+slashIdx+1:], ".git")
		projectPath = strings.TrimSuffix(projectPath, "/")
		globalConfig.CurrentProject = projectPath
	}
	return globalConfig.CurrentProject, nil
}

// isGitExecutableAvailable checks if git executable is available in PATH
func isGitExecutableAvailable() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git executable not found in PATH: %v", err)
	}
	return nil
}

// initializeLogging sets up the global logger based on the logfile parameter
func initializeLogging(logFile string) {
	if logFile != "" {
		// Open log file for writing (create if not exists, append if exists)
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// If we can't open the log file, fall back to stderr for this error
			fmt.Fprintf(os.Stderr, "Failed to open log file %q: %v\n", logFile, err)
			// Initialize with discard writer so logging calls don't panic
			logger = log.New(io.Discard, "", 0)
			return
		}
		// Create logger that writes to the file
		logger = log.New(file, "[MCP] ", log.LstdFlags|log.Lmicroseconds)
	} else {
		// If no log file specified, create a logger that discards output
		logger = log.New(io.Discard, "", 0)
	}
}

// logf logs a formatted message if logging is enabled
func logf(format string, v ...interface{}) {
	if logger != nil {
		logger.Printf(format, v...)
	}
}

func (mcpCommand MCPCommand) Execute(args []string) {
	// Initialize variables
	var server, token, logFile string

	// Create a new flag set for MCP command
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	fs.StringVar(&server, "server", "", "Specify OneDev server url")
	if server != "" {
		server = strings.TrimRight(server, "/")
	}
	fs.StringVar(&token, "token", "", "Specify access token to authentication against OneDev server")
	fs.StringVar(&logFile, "logfile", "", "Specify log file path for debug logging (optional)")

	// Parse the flags
	fs.Parse(args)

	// Initialize logging based on logfile parameter
	initializeLogging(logFile)

	globalConfig = MCPConfig{
		Server:  server,
		Token:   token,
		LogFile: logFile,
	}

	logf("MCP server starting with server=%s, logfile=%s", server, logFile)

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var request MCPRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			sendError(nil, ErrorCodeParseError, fmt.Sprintf("Failed to parse JSON request: %v, line: %s", err, line))
			continue
		}

		logf("Received request: method=%s, id=%v", request.Method, request.ID)

		// Validate required fields
		if request.JSONRPC != "2.0" {
			sendError(request.ID, ErrorCodeInvalidRequest, "Invalid Request: jsonrpc must be '2.0'")
			continue
		}

		if request.Method == "" {
			sendError(request.ID, ErrorCodeInvalidRequest, "Invalid Request: method is required")
			continue
		}

		// Check if this is a notification (no ID field or ID is null)
		isNotification := request.ID == nil

		switch request.Method {
		case "initialize":
			handleInitialize(request)
		case "initialized", "notifications/initialized":
			// Client sends this after successful initialization - just acknowledge
			// For notifications (no ID), don't send a response
			if !isNotification {
				sendResponse(request.ID, map[string]interface{}{})
			}
		case "tools/list":
			handleToolsList(request)
		case "tools/call":
			handleToolsCall(request)
		case "prompts/list":
			handlePromptsList(request)
		case "prompts/get":
			handlePromptsGet(request)
		case "ping":
			// Standard ping/pong for keepalive
			sendResponse(request.ID, map[string]interface{}{})
		case "notifications/cancelled":
			// Handle notification cancellations gracefully
			// No response needed for notifications
		default:
			// Log the unknown method for debugging
			logf("Unknown method requested: %s (ID: %v, isNotification: %v)", request.Method, request.ID, isNotification)
			// Only send error response for requests (not notifications)
			if !isNotification {
				sendError(request.ID, ErrorCodeMethodNotFound, fmt.Sprintf("Unknown method requested: %s", request.Method))
			}
		}
	}
}

func handleInitialize(request MCPRequest) {
	logf("Handling initialize request")

	// Check if git executable is available
	if err := isGitExecutableAvailable(); err != nil {
		message := fmt.Sprintf("MCP server initialization failed: %v. Git is expected to be installed and available in PATH.", err)
		logf(message)
		sendError(request.ID, ErrorCodeInvalidRequest, message)
		return
	}

	// Validate required configuration before initializing
	var missingArgs []string
	if globalConfig.Server == "" {
		missingArgs = append(missingArgs, "server")
	}
	if globalConfig.Token == "" {
		missingArgs = append(missingArgs, "token")
	}

	if len(missingArgs) > 0 {
		message := fmt.Sprintf("MCP server initialization failed: missing required arguments: %s. Please restart with -server and -token flags.", strings.Join(missingArgs, ", "))
		logf(message)
		sendError(request.ID, ErrorCodeInvalidRequest, message)
		return
	}

	workspaceEnv := os.Getenv("WORKSPACE_FOLDER_PATHS")
	var wd string
	if workspaceEnv != "" {
		if idx := strings.Index(workspaceEnv, ","); idx != -1 {
			wd = workspaceEnv[:idx]
		} else {
			wd = workspaceEnv
		}
	} else {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			message := fmt.Sprintf("Failed to get current working directory: %v", err)
			logf(message)
			sendError(request.ID, ErrorCodeInternalError, message)
			return
		}
	}
	globalConfig.WorkingDir = wd

	serverName := "tod"
	if globalConfig.Server != "" {
		serverName = fmt.Sprintf("tod (%s)", globalConfig.Server)
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": true,
			},
			"prompts": map[string]interface{}{
				"listChanged": true,
			},
		},
		ServerInfo: ServerInfo{
			Name:    serverName,
			Version: "1.0.0",
		},
	}

	logf("Initialize successful, sending response")
	sendResponse(request.ID, result)
}

// createErrorString creates a well-formatted string for logging purposes
func createErrorString(err error) string {
	// If this is an APIError, include additional structured details
	if apiErr, ok := err.(*APIError); ok {
		return fmt.Sprintf("%s, endpoint: %s", apiErr.Error(), apiErr.Endpoint)
	} else {
		return err.Error()
	}
}

func handleToolsList(request MCPRequest) {
	logf("Handling tools/list request")

	apiURL := globalConfig.Server + "/~api/mcp-helper/get-tool-input-schemas"
	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	schemas, err := getJSONMapFromAPI(apiURL + "?currentProject=" + url.QueryEscape(currentProject))
	if err != nil {
		logf("Failed to get tool input schemas: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to get tool input schemas: "+err.Error())
		return
	}

	tools := []Tool{}

	queryIssuesSchema := getInputSchemaForTool("queryIssues", schemas)
	if queryIssuesSchema.Type == "" {
		logf("Failed to get input schema for queryIssues tool")
		sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for queryIssues tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "queryIssues",
		Description: "Query issues in current project",
		InputSchema: queryIssuesSchema,
	})
	tools = append(tools, Tool{
		Name:        "getIssue",
		Description: "Get issue information",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"issueReference": map[string]interface{}{
					"type":        "string",
					"description": IssueReferenceDesc,
				},
			},
			Required: []string{"issueReference"},
		},
	})
	tools = append(tools, Tool{
		Name:        "getIssueComments",
		Description: "Get issue comments",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"issueReference": map[string]interface{}{
					"type":        "string",
					"description": IssueReferenceDesc,
				},
			},
			Required: []string{"issueReference"},
		},
	})

	createIssueSchema := getInputSchemaForTool("createIssue", schemas)
	if createIssueSchema.Type == "" {
		logf("Failed to get input schema for createIssue tool")
		sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for createIssue tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "createIssue",
		Description: "Create a new issue in current project",
		InputSchema: createIssueSchema,
	})

	editIssueSchema := getInputSchemaForTool("editIssue", schemas)
	if editIssueSchema.Type == "" {
		logf("Failed to get input schema for editIssue tool")
		sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for editIssue tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "editIssue",
		Description: "Edit an existing issue",
		InputSchema: editIssueSchema,
	})

	transitIssueSchema := getInputSchemaForTool("transitIssue", schemas)
	if transitIssueSchema.Type != "" {
		tools = append(tools, Tool{
			Name:        "transitIssue",
			Description: "Transit specified issue to specified state",
			InputSchema: transitIssueSchema,
		})
	}

	linkIssuesSchema := getInputSchemaForTool("linkIssues", schemas)
	if linkIssuesSchema.Type != "" {
		tools = append(tools, Tool{
			Name:        "linkIssues",
			Description: "Set up links between two issues. Semantic of params of this tool is: add <targetIssueReference> as a <linkName> of <sourceIssueReference>",
			InputSchema: linkIssuesSchema,
		})
	}

	tools = append(tools, Tool{
		Name:        "addIssueComment",
		Description: "Add a comment to an issue. For issue state change (work on issue, mark issue as done etc.), use the transitIssue tool instead.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"issueReference": map[string]interface{}{
					"type":        "string",
					"description": IssueReferenceDesc,
				},
				"commentContent": map[string]interface{}{
					"type":        "string",
					"description": "content of the comment to add",
				},
			},
			Required: []string{"issueReference", "commentContent"},
		},
	})

	tools = append(tools, Tool{
		Name:        "logWork",
		Description: "Log spent time on an issue",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"issueReference": map[string]interface{}{
					"type":        "string",
					"description": IssueReferenceDesc,
				},
				"spentHours": map[string]interface{}{
					"type":        "integer",
					"description": "spent time in hours",
				},
				"comment": map[string]interface{}{
					"type":        "string",
					"description": "comment to add to the work log",
				},
			},
			Required: []string{"issueReference", "spentHours"},
		},
	})

	queryPullRequestsSchema := getInputSchemaForTool("queryPullRequests", schemas)
	if queryPullRequestsSchema.Type == "" {
		logf("Failed to get input schema for queryPullRequests tool")
		sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for queryPullRequests tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "queryPullRequests",
		Description: "Query pull requests in current project",
		InputSchema: queryPullRequestsSchema,
	})

	tools = append(tools, Tool{
		Name:        "getPullRequest",
		Description: "Get pull request information",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
			},
			Required: []string{"pullRequestReference"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getPullRequestComments",
		Description: "Get pull request comments",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
			},
			Required: []string{"pullRequestReference"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getPullRequestCodeComments",
		Description: "Get pull request code comments",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
			},
			Required: []string{"pullRequestReference"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getPullRequestFileChanges",
		Description: "Get pull request file changes in patch format, optionally since last review",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
				"sinceLastReview": map[string]interface{}{
					"type":        "boolean",
					"description": "if true, only changes since last review will be returned",
				},
			},
			Required: []string{"pullRequestReference", "sinceLastReview"},
		},
	})

	createPullRequestSchema := getInputSchemaForTool("createPullRequest", schemas)
	if createPullRequestSchema.Type == "" {
		logf("Failed to get input schema for createPullRequest tool")
		sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for createPullRequest tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "createPullRequest",
		Description: "Create a new pull request in target project, against changes in current project",
		InputSchema: createPullRequestSchema,
	})

	editPullRequestSchema := getInputSchemaForTool("editPullRequest", schemas)
	if editPullRequestSchema.Type == "" {
		logf("Failed to get input schema for editPullRequest tool")
		sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for editPullRequest tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "editPullRequest",
		Description: "Edit an existing pull request",
		InputSchema: editPullRequestSchema,
	})

	tools = append(tools, Tool{
		Name:        "reviewPullRequest",
		Description: "Approve a pull request or request for changes",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
				"approved": map[string]interface{}{
					"type":        "boolean",
					"description": "if true, the pull request will be approved, otherwise it will be requested for changes",
				},
				"comment": map[string]interface{}{
					"type":        "string",
					"description": "comment to add",
				},
			},
			Required: []string{"pullRequestReference", "approved"},
		},
	})

	tools = append(tools, Tool{
		Name:        "addPullRequestComment",
		Description: "Add a comment to a pull request. For pull request review (approve/request changes), use the reviewPullRequest tool instead.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
				"commentContent": map[string]interface{}{
					"type":        "string",
					"description": "content of the comment to add",
				},
			},
			Required: []string{"pullRequestReference", "commentContent"},
		},
	})

	tools = append(tools, Tool{
		Name:        "checkoutPullRequest",
		Description: "AI assistant should follow returned instruction of the tool call to set up local git working directory to work on specified pull request",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
			},
			Required: []string{"pullRequestReference"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getWorkingDir",
		Description: "Get absolute path in the file system used used as working directory for tod",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
	})
	tools = append(tools, Tool{
		Name:        "getCurrentProject",
		Description: "Get current OneDev project for tod operations",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
	})
	tools = append(tools, Tool{
		Name:        "getLoginName",
		Description: "Returns login name of specified user or current user",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"userName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the user. If not provided, login name of the current user will be returned.",
				},
			},
			Required: []string{},
		},
	})
	tools = append(tools, Tool{
		Name:        "getUnixTimestamp",
		Description: "Returns unix timestamp in milliseconds since epoch",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"dateTimeDescription": map[string]interface{}{
					"type":        "string",
					"description": "Description of the date and time to be converted to unix timestamp, for instance: today, next month, two hours ago, 2025-01-01, 2025-01-01 12:00:00, etc.",
				},
			},
			Required: []string{"dateTimeDescription"},
		},
	})

	result := map[string]interface{}{
		"tools": tools,
	}

	logf("Sending tools list with %d tools", len(tools))
	sendResponse(request.ID, result)
}

func handlePromptsList(request MCPRequest) {
	logf("Handling prompts/list request")

	apiURL := globalConfig.Server + "/~api/mcp-helper/get-prompt-arguments"
	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	arguments, err := getJSONMapFromAPI(apiURL + "?currentProject=" + url.QueryEscape(currentProject))
	if err != nil {
		logf("Failed to get prompt arguments: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to get prompt arguments: "+err.Error())
		return
	}

	prompts := []Prompt{
		{
			Name:        "createIssue",
			Description: "Create a new issue in current project",
			Arguments:   getArgumentsForPrompt("createIssue", arguments),
		},
	}

	result := map[string]interface{}{
		"prompts": prompts,
	}

	logf("Sending prompts list with %d prompts", len(prompts))
	sendResponse(request.ID, result)
}

func handlePromptsGet(request MCPRequest) {
	logf("Handling prompts/get request")

	paramsBytes, _ := json.Marshal(request.Params)
	var params GetPromptParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		logf("Failed to parse prompt get params: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Invalid params: "+err.Error())
		return
	}

	logf("Getting prompt: %s", params.Name)
	switch params.Name {
	case "createIssue":
		handleCreateIssuePrompt(request, params)
	default:
		logf("Unknown prompt requested: %s", params.Name)
		sendError(request.ID, ErrorCodeInvalidParams, "Unknown prompt: "+params.Name)
	}
}

func getParamsPrompt(params GetPromptParams) string {
	if len(params.Arguments) == 0 {
		return ""
	}

	var lines []string
	for paramName, paramValue := range params.Arguments {
		lines = append(lines, fmt.Sprintf("%s: %v", paramName, paramValue))
	}

	return strings.Join(lines, "\n")
}

func handleCreateIssuePrompt(request MCPRequest, params GetPromptParams) {
	logf("Handling createIssue prompt")

	promptText := fmt.Sprintf(
		"Please create an issue using the createIssue tool with below params:\n%s",
		getParamsPrompt(params))

	result := GetPromptResult{
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: PromptMessageContent{
					Type: "text",
					Text: promptText,
				},
			},
		},
	}

	logf("createIssue prompt successful")
	sendResponse(request.ID, result)
}

func handleGetWorkingDirTool(request MCPRequest) {
	logf("Handling getWorkingDir tool call")

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: globalConfig.WorkingDir,
			},
		},
	}

	logf("getWorkingDir tool call successful")
	sendResponse(request.ID, result)
}

func handleGetCurrentProjectTool(request MCPRequest) {
	logf("Handling getCurrentProject tool call")

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: currentProject,
			},
		},
	}

	logf("getCurrentProject tool call successful")
	sendResponse(request.ID, result)
}

// handleGetLoginNameTool handles the getLoginName tool call
func handleGetLoginNameTool(request MCPRequest, params CallToolParams) {
	logf("Handling getLoginName tool call")

	// Get userName parameter if provided
	var userName string
	if userNameVal, exists := params.Arguments["userName"]; exists {
		if userNameStr, ok := userNameVal.(string); ok {
			userName = userNameStr
		}
	}

	urlQuery := url.Values{
		"userName": {userName},
	}
	// Build the API URL
	apiURL := globalConfig.Server + "/~api/mcp-helper/get-login-name?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	loginName := string(body)

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: loginName,
			},
		},
	}

	logf("getLoginName tool call successful")
	sendResponse(request.ID, result)
}

// handleGetUnixTimestampTool handles the getUnixTimestamp tool call
func handleGetUnixTimestampTool(request MCPRequest, params CallToolParams) {
	logf("Handling getUnixTimestamp tool call")

	// Get dateTimeDescription parameter, report error if not provided
	dateTimeDescriptionArg, exists := params.Arguments["dateTimeDescription"]
	if !exists {
		logf("Missing required parameter: dateTimeDescription")
		sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: dateTimeDescription")
		return
	}
	dateTimeDescription, ok := dateTimeDescriptionArg.(string)
	if !ok {
		logf("Invalid type for dateTimeDescription parameter: expected string")
		sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for dateTimeDescription parameter: expected string")
		return
	}

	urlQuery := url.Values{
		"dateTimeDescription": {dateTimeDescription},
	}
	// Build the API URL
	apiURL := globalConfig.Server + "/~api/mcp-helper/get-unix-timestamp?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	unixMillis := string(body)

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: unixMillis,
			},
		},
	}

	logf("getUnixTimestamp tool call successful")
	sendResponse(request.ID, result)
}

func handleQueryEntitiesTool(request MCPRequest, params CallToolParams, toolName string,
	endpointSuffix string) {

	logf("Handling %s tool call", toolName)

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	// Extract optional query parameter (default to empty string for all issues)
	var query string
	if queryVal, exists := params.Arguments["query"]; exists {
		if queryStr, ok := queryVal.(string); ok {
			query = queryStr
		}
	}

	// Extract optional offset parameter (default to 0)
	offset := 0
	if offsetVal, exists := params.Arguments["offset"]; exists {
		offsetFloat, ok := offsetVal.(float64)
		if !ok {
			sendError(request.ID, ErrorCodeInvalidParams, "Invalid 'offset' parameter - must be an integer")
			return
		}
		offset = int(offsetFloat)
	}

	// Extract optional count parameter (default to 25)
	count := DefaultQueryCount
	if countVal, exists := params.Arguments["count"]; exists {
		countFloat, ok := countVal.(float64)
		if !ok {
			sendError(request.ID, ErrorCodeInvalidParams, "Invalid 'count' parameter - must be an integer")
			return
		}
		count = int(countFloat)
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"query":          {query},
		"offset":         {fmt.Sprintf("%d", offset)},
		"count":          {fmt.Sprintf("%d", count)},
	}

	apiURL := globalConfig.Server + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	issuesResult := string(body)

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: issuesResult,
			},
		},
	}

	logf("%s tool call successful", toolName)
	sendResponse(request.ID, result)
}

func getNonEmptyStringParam(params CallToolParams, paramName string) (string, error) {
	// Extract required parameter
	paramVal, exists := params.Arguments[paramName]
	if !exists {
		return "", fmt.Errorf("missing required parameter: %s", paramName)
	}

	param, ok := paramVal.(string)
	if !ok {
		return "", fmt.Errorf("invalid type for %s parameter: expected string", paramName)
	}

	if param == "" {
		return "", fmt.Errorf("%s parameter cannot be empty", paramName)
	}

	return param, nil
}

func handleGetEntityDataTool(request MCPRequest, params CallToolParams, toolName string,
	endpointSuffix string, referenceParamName string) {

	logf("Handling %s tool call", toolName)

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	reference, err := getNonEmptyStringParam(params, referenceParamName)
	if err != nil {
		logf("Failed to extract %s: %v", referenceParamName, createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract "+referenceParamName+": "+err.Error())
		return
	}

	// Build the API URL
	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	apiURL := globalConfig.Server + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	changes := string(body)

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: changes,
			},
		},
	}

	logf("%s tool call successful", toolName)
	sendResponse(request.ID, result)
}

func handleCheckoutPullRequestTool(request MCPRequest, params CallToolParams) {

	logf("Handling checkoutPullRequest tool call")

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	reference, err := getNonEmptyStringParam(params, "pullRequestReference")
	if err != nil {
		logf("Failed to extract pullRequestReference: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract pullRequestReference: "+err.Error())
		return
	}

	// Build the API URL
	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	apiURL := globalConfig.Server + "/~api/mcp-helper/get-pull-request-checkout-instruction?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(body),
			},
		},
	}

	logf("checkoutPullRequest tool call successful")
	sendResponse(request.ID, result)
}

func handleGetPullRequestFileChangesTool(request MCPRequest, params CallToolParams) {
	logf("Handling getPullRequestFileChanges tool call")

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	reference, err := getNonEmptyStringParam(params, "pullRequestReference")
	if err != nil {
		logf("Failed to extract pullRequestReference: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract pullRequestReference: "+err.Error())
		return
	}

	sinceLastReviewVal, exists := params.Arguments["sinceLastReview"]

	if !exists {
		logf("Missing required parameter: sinceLastReview")
		sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: sinceLastReview")
		return
	}

	sinceLastReview, ok := sinceLastReviewVal.(bool)
	if !ok {
		logf("Invalid type for sinceLastReview parameter: expected boolean")
		sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for sinceLastReview parameter: expected boolean")
		return
	}

	// Build the API URL
	urlQuery := url.Values{
		"currentProject":  {currentProject},
		"reference":       {reference},
		"sinceLastReview": {fmt.Sprintf("%t", sinceLastReview)},
	}

	apiURL := globalConfig.Server + "/~api/mcp-helper/get-pull-request-patch-info?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	var patchInfo map[string]interface{}
	if err := json.Unmarshal(body, &patchInfo); err != nil {
		logf("Failed to parse JSON response: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to parse JSON response: "+err.Error())
		return
	}

	projectId, _ := patchInfo["projectId"].(string)
	oldCommitHash, _ := patchInfo["oldCommitHash"].(string)
	newCommitHash, _ := patchInfo["newCommitHash"].(string)

	urlQuery = url.Values{
		"old-commit": {oldCommitHash},
		"new-commit": {newCommitHash},
	}

	apiURL = globalConfig.Server + "/~downloads/projects/" + projectId + "/patch?" + urlQuery.Encode()

	req, err = http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	body, err = makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(body),
			},
		},
	}

	logf("getPullRequestFileChanges tool call successful")
	sendResponse(request.ID, result)
}

func handleAddEntityCommentTool(request MCPRequest, params CallToolParams, toolName string,
	endpointSuffix string, referenceParamName string) {

	logf("Handling %s tool call", toolName)

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	reference, err := getNonEmptyStringParam(params, referenceParamName)
	if err != nil {
		logf("Failed to extract %s: %v", referenceParamName, createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract "+referenceParamName+": "+err.Error())
		return
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	// Build the API URL
	apiURL := globalConfig.Server + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	// Extract required commentContent parameter
	commentContentVal, exists := params.Arguments["commentContent"]
	if !exists {
		logf("Missing required parameter: commentContent")
		sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: commentContent")
		return
	}

	commentContent, ok := commentContentVal.(string)
	if !ok {
		logf("Invalid type for commentContent parameter: expected string")
		sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for commentContent parameter: expected string")
		return
	}

	if commentContent == "" {
		logf("Empty commentContent parameter")
		sendError(request.ID, ErrorCodeInvalidParams, "commentContent parameter cannot be empty")
		return
	}

	// Create HTTP POST request with comment content as body
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(commentContent))
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "text/plain")

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	result := string(body)

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: result,
			},
		},
	}

	logf("%s tool call successful", toolName)
	sendResponse(request.ID, response)
}

func handleReviewPullRequestTool(request MCPRequest, params CallToolParams) {
	logf("Handling reviewPullRequest tool call")

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	reference, err := getNonEmptyStringParam(params, "pullRequestReference")
	if err != nil {
		logf("Failed to extract pullRequestReference: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract pullRequestReference: "+err.Error())
		return
	}

	approvedVal, exists := params.Arguments["approved"]
	if !exists {
		logf("Missing required parameter: approved")
		sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: approved")
		return
	}

	approved, ok := approvedVal.(bool)
	if !ok {
		logf("Invalid type for approved parameter: expected boolean")
		sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for approved parameter: expected boolean")
		return
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
		"approved":       {fmt.Sprintf("%t", approved)},
	}

	// Build the API URL
	apiURL := globalConfig.Server + "/~api/mcp-helper/review-pull-request?" + urlQuery.Encode()

	comment := ""
	commentVal, exists := params.Arguments["comment"]
	if exists {
		comment, ok = commentVal.(string)
		if !ok {
			logf("Invalid type for comment parameter: expected string")
			sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for comment parameter: expected string")
			return
		}
	}

	// Create HTTP POST request with comment content as body
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(comment))
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "text/plain")

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	result := string(body)

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: result,
			},
		},
	}

	logf("reviewPullRequest tool call successful")
	sendResponse(request.ID, response)
}

func handleLogWorkTool(request MCPRequest, params CallToolParams) {
	logf("Handling logWork tool call")

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	issueReference, err := getNonEmptyStringParam(params, "issueReference")
	if err != nil {
		logf("Failed to extract issue reference: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract issue reference: "+err.Error())
		return
	}

	spentHoursVal, exists := params.Arguments["spentHours"]
	if !exists {
		logf("Missing required parameter: spentHours")
		sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: spentHours")
		return
	}

	spentHours, ok := spentHoursVal.(float64)
	if !ok {
		logf("Invalid type for spentHours parameter: expected float64")
		sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for spentHours parameter: expected float64")
		return
	}

	var comment string
	// Extract required commentContent parameter
	commentVal, exists := params.Arguments["comment"]
	if exists {
		comment, ok = commentVal.(string)
		if !ok {
			logf("Invalid type for comment parameter: expected string")
			sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for comment parameter: expected string")
			return
		}
	}

	var urlQuery = url.Values{
		"currentProject": {currentProject},
		"reference":      {issueReference},
		"spentHours":     {fmt.Sprintf("%d", int64(spentHours))},
	}
	// Build the API URL
	apiURL := globalConfig.Server + "/~api/mcp-helper/log-work?" + urlQuery.Encode()

	// Create HTTP POST request with comment content as body
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(comment))
	if err != nil {
		logf("Failed to create request: %v", err)
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "text/plain")

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	result := string(body)

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: result,
			},
		},
	}

	logf("logWork tool call successful")
	sendResponse(request.ID, response)
}

func handleCreateEntityTool(request MCPRequest, params CallToolParams, toolName string,
	endpointSuffix string) {

	logf("Handling %s tool call", toolName)

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", err)
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	entityMap := make(map[string]interface{})

	// Extract all parameters from arguments
	for paramName, paramValue := range params.Arguments {
		entityMap[paramName] = paramValue
	}

	entityBytes, err := json.Marshal(entityMap)
	if err != nil {
		logf("Failed to marshal map to JSON: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to marshal map to JSON: "+err.Error())
		return
	}
	entityData := string(entityBytes)

	// Build the API URL
	apiURL := globalConfig.Server + "/~api/mcp-helper/" + endpointSuffix + "?currentProject=" + url.QueryEscape(currentProject)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(entityData))
	if err != nil {
		logf("Failed to create request: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(body),
			},
		},
	}

	logf("%s tool call successful", toolName)
	sendResponse(request.ID, response)
}

func handleEditEntityTool(request MCPRequest, params CallToolParams, toolName string,
	endpointSuffix string, referenceParamName string) {

	logf("Handling %s tool call", toolName)

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", err)
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	entityMap := make(map[string]interface{})

	// Extract all parameters from arguments
	for paramName, paramValue := range params.Arguments {
		if paramName != referenceParamName {
			entityMap[paramName] = paramValue
		}
	}

	entityBytes, err := json.Marshal(entityMap)
	if err != nil {
		logf("Failed to marshal map to JSON: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to marshal map to JSON: "+err.Error())
		return
	}
	entityData := string(entityBytes)

	reference, err := getNonEmptyStringParam(params, referenceParamName)
	if err != nil {
		logf("Failed to extract %s: %v", referenceParamName, createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract "+referenceParamName+": "+err.Error())
		return
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	// Build the API URL
	apiURL := globalConfig.Server + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(entityData))
	if err != nil {
		logf("Failed to create request: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(body),
			},
		},
	}

	logf("%s tool call successful", toolName)
	sendResponse(request.ID, response)
}

func handleTransitIssueTool(request MCPRequest, params CallToolParams) {
	logf("Handling transitIssue tool call")

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", err)
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	issueMap := make(map[string]interface{})

	// Extract all parameters from arguments
	for paramName, paramValue := range params.Arguments {
		if paramName != "issueReference" {
			issueMap[paramName] = paramValue
		}
	}

	issueBytes, err := json.Marshal(issueMap)
	if err != nil {
		logf("Failed to marshal map to JSON: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to marshal map to JSON: "+err.Error())
		return
	}
	issueData := string(issueBytes)

	issueReference, err := getNonEmptyStringParam(params, "issueReference")
	if err != nil {
		logf("Failed to extract issue reference: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract issue reference: "+err.Error())
		return
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {issueReference},
	}
	// Build the API URL
	apiURL := globalConfig.Server + "/~api/mcp-helper/transit-issue?" + urlQuery.Encode()

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(issueData))
	if err != nil {
		logf("Failed to create request: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to make API call: "+err.Error())
		return
	}

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(body),
			},
		},
	}

	logf("transitIssue tool call successful")
	sendResponse(request.ID, response)
}

func handleLinkIssuesTool(request MCPRequest, params CallToolParams) {
	logf("Handling linkIssues tool call")

	currentProject, err := getCurrentProject()
	if err != nil {
		logf("Failed to get current project: %v", err)
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to get current project: "+err.Error())
		return
	}

	sourceReference, err := getNonEmptyStringParam(params, "sourceIssueReference")
	if err != nil {
		logf("Failed to extract source issue reference: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract source issuereference: "+err.Error())
		return
	}
	targetReference, err := getNonEmptyStringParam(params, "targetIssueReference")
	if err != nil {
		logf("Failed to extract target issue reference: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract target issue reference: "+err.Error())
		return
	}
	linkNameArg, exists := params.Arguments["linkName"]
	if !exists {
		logf("Missing required parameter: linkName")
		sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: linkName")
		return
	}
	linkName, ok := linkNameArg.(string)
	if !ok {
		logf("Invalid type for linkName parameter: expected string")
		sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for linkName parameter: expected string")
		return
	}

	urlQuery := url.Values{
		"currentProject":  {currentProject},
		"sourceReference": {sourceReference},
		"targetReference": {targetReference},
		"linkName":        {linkName},
	}
	apiURL := globalConfig.Server + "/~api/mcp-helper/link-issues?" + urlQuery.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logf("Failed to create request: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		logf("Failed to make API call: %v", createErrorString(err))
		sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(body),
			},
		},
	}

	logf("createIssue tool call successful")
	sendResponse(request.ID, response)
}

func handleToolsCall(request MCPRequest) {
	logf("Handling tools/call request")

	paramsBytes, _ := json.Marshal(request.Params)
	var params CallToolParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		logf("Failed to parse tool call params: %v", err)
		sendError(request.ID, ErrorCodeInvalidParams, "Invalid params: "+err.Error())
		return
	}

	logf("Calling tool: %s", params.Name)
	switch params.Name {
	case "queryIssues":
		handleQueryEntitiesTool(request, params, "queryIssues", "query-issues")
	case "getIssue":
		handleGetEntityDataTool(request, params, "getIssue", "get-issue", "issueReference")
	case "getIssueComments":
		handleGetEntityDataTool(request, params, "getIssueComments", "get-issue-comments", "issueReference")
	case "createIssue":
		handleCreateEntityTool(request, params, "createIssue", "create-issue")
	case "editIssue":
		handleEditEntityTool(request, params, "editIssue", "edit-issue", "issueReference")
	case "transitIssue":
		handleTransitIssueTool(request, params)
	case "linkIssues":
		handleLinkIssuesTool(request, params)
	case "addIssueComment":
		handleAddEntityCommentTool(request, params, "addIssueComment", "add-issue-comment", "issueReference")
	case "logWork":
		handleLogWorkTool(request, params)
	case "queryPullRequests":
		handleQueryEntitiesTool(request, params, "queryPullRequests", "query-pull-requests")
	case "getPullRequest":
		handleGetEntityDataTool(request, params, "getPullRequest", "get-pull-request", "pullRequestReference")
	case "getPullRequestComments":
		handleGetEntityDataTool(request, params, "getPullRequestComments", "get-pull-request-comments", "pullRequestReference")
	case "getPullRequestCodeComments":
		handleGetEntityDataTool(request, params, "getPullRequestCodeComments", "get-pull-request-code-comments", "pullRequestReference")
	case "getPullRequestFileChanges":
		handleGetPullRequestFileChangesTool(request, params)
	case "createPullRequest":
		handleCreateEntityTool(request, params, "createPullRequest", "create-pull-request")
	case "editPullRequest":
		handleEditEntityTool(request, params, "editPullRequest", "edit-pull-request", "pullRequestReference")
	case "reviewPullRequest":
		handleReviewPullRequestTool(request, params)
	case "addPullRequestComment":
		handleAddEntityCommentTool(request, params, "addPullRequestComment", "add-pull-request-comment", "pullRequestReference")
	case "checkoutPullRequest":
		handleCheckoutPullRequestTool(request, params)
	case "getWorkingDir":
		handleGetWorkingDirTool(request)
	case "getCurrentProject":
		handleGetCurrentProjectTool(request)
	case "getLoginName":
		handleGetLoginNameTool(request, params)
	case "getUnixTimestamp":
		handleGetUnixTimestampTool(request, params)
	default:
		logf("Unknown tool requested: %s", params.Name)
		sendError(request.ID, ErrorCodeInvalidParams, "Unknown tool: "+params.Name)
	}
}

func makeAPICall(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+globalConfig.Token)

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

// getArgumentsForPrompt converts JSON data from prompt arguments API to PromptArgument array
// Returns an empty PromptArgument array if the prompt is not found or conversion fails
func getArgumentsForPrompt(promptName string, arguments map[string]interface{}) []PromptArgument {
	// Extract the prompt-specific arguments array
	promptArgumentsVal, exists := arguments[promptName]
	if !exists {
		logf("Prompt '%s' not found in prompt arguments data", promptName)
		sendError(nil, ErrorCodeInternalError, fmt.Sprintf("Prompt '%s' not found in prompt arguments data", promptName))
		return []PromptArgument{}
	}

	promptArgumentsSlice, ok := promptArgumentsVal.([]interface{})
	if !ok {
		message := fmt.Sprintf("Arguments for prompt '%s' is not an array, got %T", promptName, promptArgumentsVal)
		logf(message)
		sendError(nil, ErrorCodeInternalError, message)
		return []PromptArgument{}
	}

	var promptArguments []PromptArgument
	for i, argVal := range promptArgumentsSlice {
		argMap, ok := argVal.(map[string]interface{})
		if !ok {
			message := fmt.Sprintf("Argument at index %d for prompt '%s' is not a map[string]interface{}, got %T", i, promptName, argVal)
			logf(message)
			sendError(nil, ErrorCodeInternalError, message)
			return []PromptArgument{}
		}

		// Extract name (required)
		nameVal, nameExists := argMap["name"]
		if !nameExists {
			message := fmt.Sprintf("Argument at index %d for prompt '%s' is missing 'name' field", i, promptName)
			logf(message)
			sendError(nil, ErrorCodeInternalError, message)
			return []PromptArgument{}
		}
		name, ok := nameVal.(string)
		if !ok {
			message := fmt.Sprintf("Argument at index %d for prompt '%s' has invalid 'name' field - must be a string, got %T", i, promptName, nameVal)
			logf(message)
			sendError(nil, ErrorCodeInternalError, message)
			return []PromptArgument{}
		}

		// Extract description (optional)
		var description string
		if descVal, exists := argMap["description"]; exists {
			if descStr, ok := descVal.(string); ok {
				description = descStr
			}
		}

		// Extract required (required)
		requiredVal, requiredExists := argMap["required"]
		if !requiredExists {
			message := fmt.Sprintf("Argument at index %d for prompt '%s' is missing 'required' field", i, promptName)
			logf(message)
			sendError(nil, ErrorCodeInternalError, message)
			return []PromptArgument{}
		}
		required, ok := requiredVal.(bool)
		if !ok {
			message := fmt.Sprintf("Argument at index %d for prompt '%s' has invalid 'required' field - must be a boolean, got %T", i, promptName, requiredVal)
			logf(message)
			sendError(nil, ErrorCodeInternalError, message)
			return []PromptArgument{}
		}

		promptArguments = append(promptArguments, PromptArgument{
			Name:        name,
			Description: description,
			Required:    required,
		})
	}

	return promptArguments
}

// getInputSchemaForTool retrieves and converts a tool's input schema from the schemas map
// Returns an empty InputSchema if the tool is not found or conversion fails
func getInputSchemaForTool(toolName string, schemas map[string]interface{}) InputSchema {
	schemaData, exists := schemas[toolName]
	if !exists {
		logf("%s schema not found in API response", toolName)
		return InputSchema{}
	}

	// Check if schemaData is nil
	if schemaData == nil {
		logf("Schema data for %s is nil", toolName)
		return InputSchema{}
	}

	// Check if schemaData is the expected map type
	schemaMap, ok := schemaData.(map[string]interface{})
	if !ok {
		logf("Schema data for %s is not a map[string]interface{}, got %T", toolName, schemaData)
		return InputSchema{}
	}

	schema := InputSchema{
		Type:       "object",
		Properties: make(map[string]interface{}),
		Required:   []string{},
	}

	// Extract type (should be "object")
	if typeVal, exists := schemaMap["Type"]; exists {
		if typeStr, ok := typeVal.(string); ok {
			schema.Type = typeStr
		} else {
			logf("Type field for %s is not a string, got %T", toolName, typeVal)
			return InputSchema{}
		}
	}

	// Extract properties
	if propertiesVal, exists := schemaMap["Properties"]; exists {
		if properties, ok := propertiesVal.(map[string]interface{}); ok {
			schema.Properties = properties
		} else {
			logf("Properties field for %s is not a map[string]interface{}, got %T", toolName, propertiesVal)
			return InputSchema{}
		}
	}

	// Extract required fields
	if requiredVal, exists := schemaMap["Required"]; exists {
		if requiredSlice, ok := requiredVal.([]interface{}); ok {
			required := make([]string, 0, len(requiredSlice))
			for i, item := range requiredSlice {
				if str, ok := item.(string); ok {
					required = append(required, str)
				} else {
					logf("Required field at index %d for %s is not a string, got %T", i, toolName, item)
					return InputSchema{}
				}
			}
			schema.Required = required
		} else {
			logf("Required field for %s is not a []interface{}, got %T", toolName, requiredVal)
			return InputSchema{}
		}
	}

	return schema
}

func sendResponse(id interface{}, result interface{}) {
	// Ensure ID is never nil for JSON-RPC compliance
	if id == nil {
		id = 0
	}

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, _ := json.Marshal(response)
	fmt.Println(string(data))
}

func sendError(id interface{}, code int, message string) {
	// Ensure ID is never nil for JSON-RPC compliance
	if id == nil {
		id = 0
	}

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}

	responseData, _ := json.Marshal(response)
	fmt.Println(string(responseData))
}
