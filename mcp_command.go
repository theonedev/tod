package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type MCPCommand struct {
	workingDir          string
	currentProjectCache string
	logger              *log.Logger
}

const (
	ErrorCodeParseError      = -32700 // Parse error – Invalid JSON was received
	ErrorCodeInvalidRequest  = -32600 // Invalid Request – The JSON sent is not a valid Request object
	ErrorCodeMethodNotFound  = -32601 // Method not found – The method does not exist or is not available
	ErrorCodeInvalidParams   = -32602 // Invalid params – Invalid method parameter(s)
	ErrorCodeInternalError   = -32603 // Internal error – Internal JSON-RPC error
	MaxQueryCount            = 100    // Maximum number of entities to query
	DefaultQueryCount        = 25     // Default number of entities to query
	IssueReferenceDesc       = "Issue reference is of form &#35;&lt;number&gt;, &lt;project&gt;&#35;&lt;number&gt;, or &lt;project key&gt;-&lt;number&gt;"
	PullRequestReferenceDesc = "Pull request reference is of form &#35;&lt;number&gt;, &lt;project&gt;&#35;&lt;number&gt;, or &lt;project key&gt;-&lt;number&gt;"
	BuildReferenceDesc       = "Build reference is of form &#35;&lt;number&gt;, &lt;project&gt;&#35;&lt;number&gt;, or &lt;project key&gt;-&lt;number&gt;"
)

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

type MCPParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

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

type CallToolResult struct {
	Content []ToolContent `json:"content"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

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

// initializeLogging sets up the instance logger based on the command.logfile parameter
func (command *MCPCommand) initializeLogging(logFile string) {
	if logFile != "" {
		// Open log file for writing (create if not exists, append if exists)
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// If we can't open the log file, fall back to stderr for this error
			fmt.Fprintf(os.Stderr, "Failed to open log file %q: %v\n", logFile, err)
			// Initialize with discard writer so logging calls don't panic
			command.logger = log.New(io.Discard, "", 0)
			return
		}
		// Create logger that writes to the file
		command.logger = log.New(file, "[MCP] ", log.LstdFlags|log.Lmicroseconds)
	} else {
		// If no log file specified, create a logger that discards output
		command.logger = log.New(io.Discard, "", 0)
	}
}

// command.logf logs a formatted message if logging is enabled
func (command *MCPCommand) logf(format string, v ...interface{}) {
	if command.logger != nil {
		command.logger.Printf(format, v...)
	}
}

func (command *MCPCommand) Execute(cobraCmd *cobra.Command, args []string) {
	var logFile string

	logFile, _ = cobraCmd.Flags().GetString("log-file")

	command.initializeLogging(logFile)

	// Load configuration after logger is initialized
	var err error
	config, err = LoadConfig()
	if err != nil {
		command.logf("Failed to load config: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Configuration is loaded from file only

	if err := config.Validate(); err != nil {
		command.logf("Config validation failed: %v", err)
		fmt.Fprintf(os.Stderr, "Config validation failed: %v\n", err)
		os.Exit(1)
	}

	if err := checkVersion(config.ServerUrl, config.AccessToken); err != nil {
		command.logf("Version check failed: %v", err)
		fmt.Fprintf(os.Stderr, "Version check failed: %v\n", err)
		os.Exit(1)
	}

	command.logf("MCP server starting with serverUrl=%s, logFile=%s", config.ServerUrl, logFile)

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var request MCPRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			command.sendError(nil, ErrorCodeParseError, fmt.Sprintf("Failed to parse JSON request: %v, line: %s", err, line))
			continue
		}

		command.logf("Received request: method=%s, id=%v", request.Method, request.ID)

		if request.JSONRPC != "2.0" {
			command.sendError(request.ID, ErrorCodeInvalidRequest, "Invalid Request: jsonrpc must be '2.0'")
			continue
		}

		if request.Method == "" {
			command.sendError(request.ID, ErrorCodeInvalidRequest, "Invalid Request: method is required")
			continue
		}

		isNotification := request.ID == nil

		switch request.Method {
		case "initialize":
			command.handleInitialize(request)
		case "initialized", "notifications/initialized":
			if !isNotification {
				command.sendResponse(request.ID, map[string]interface{}{})
			}
		case "tools/list":
			command.handleToolsList(request)
		case "tools/call":
			command.handleToolsCall(request)
		case "prompts/list":
			command.handlePromptsList(request)
		case "prompts/get":
			command.handleGetPrompt(request)
		case "ping":
			command.sendResponse(request.ID, map[string]interface{}{})
		case "notifications/cancelled":
		default:
			command.logf("Unknown method requested: %s (ID: %v, isNotification: %v)", request.Method, request.ID, isNotification)
			if !isNotification {
				command.sendError(request.ID, ErrorCodeMethodNotFound, fmt.Sprintf("Unknown method requested: %s", request.Method))
			}
		}
	}
}

func (command *MCPCommand) handleInitialize(request MCPRequest) {
	command.logf("Handling initialize request")

	// Validate required configuration before initializing
	if config.ServerUrl == "" || config.AccessToken == "" {
		homeDir, _ := os.UserHomeDir()
		configFilePath := filepath.Join(homeDir, ".todconfig")
		message := fmt.Sprintf("MCP server initialization failed: missing configuration in %s. Please ensure both 'server-url' and 'access-token' are configured.", configFilePath)
		command.logf(message)
		command.sendError(request.ID, ErrorCodeInvalidRequest, message)
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
			message := fmt.Sprintf("Failed to get working directory: %v", err)
			command.logf(message)
			command.sendError(request.ID, ErrorCodeInternalError, message)
			return
		}
	}
	command.workingDir = wd

	serverName := "tod"
	if config.ServerUrl != "" {
		serverName = fmt.Sprintf("tod (%s)", config.ServerUrl)
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

	command.logf("Initialize successful, sending response")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleToolsList(request MCPRequest) {
	command.logf("Handling tools/list request")

	schemas, err := getJSONMapFromAPI(config.ServerUrl + "/~api/mcp-helper/get-tool-input-schemas")
	if err != nil {
		command.logf("Failed to get tool input schemas: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get tool input schemas: "+err.Error())
		return
	}

	tools := []Tool{}

	queryIssuesSchema := command.getInputSchemaForTool("queryIssues", schemas)
	if queryIssuesSchema.Type == "" {
		command.logf("Failed to get input schema for queryIssues tool")
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for queryIssues tool")
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

	createIssueSchema := command.getInputSchemaForTool("createIssue", schemas)
	if createIssueSchema.Type == "" {
		command.logf("Failed to get input schema for createIssue tool")
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for createIssue tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "createIssue",
		Description: "Create a new issue",
		InputSchema: createIssueSchema,
	})

	editIssueSchema := command.getInputSchemaForTool("editIssue", schemas)
	if editIssueSchema.Type == "" {
		command.logf("Failed to get input schema for editIssue tool")
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for editIssue tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "editIssue",
		Description: "Edit an existing issue",
		InputSchema: editIssueSchema,
	})

	transitIssueSchema := command.getInputSchemaForTool("transitIssue", schemas)
	if transitIssueSchema.Type != "" {
		tools = append(tools, Tool{
			Name:        "transitIssue",
			Description: "Transit specified issue to specified state",
			InputSchema: transitIssueSchema,
		})
	}

	linkIssuesSchema := command.getInputSchemaForTool("linkIssues", schemas)
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
					"description": "Content of the comment to add",
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
					"description": "Spent time in hours",
				},
				"comment": map[string]interface{}{
					"type":        "string",
					"description": "Comment to add to the work log",
				},
			},
			Required: []string{"issueReference", "spentHours"},
		},
	})

	queryPullRequestsSchema := command.getInputSchemaForTool("queryPullRequests", schemas)
	if queryPullRequestsSchema.Type == "" {
		command.logf("Failed to get input schema for queryPullRequests tool")
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for queryPullRequests tool")
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
					"description": "If true, only changes since last review will be returned",
				},
			},
			Required: []string{"pullRequestReference", "sinceLastReview"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getPullRequestFileContent",
		Description: "Get content of specified file in pull request. When review a pull request, AI assistant may use this tool to examine content of some files if needed",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
				"filePath": map[string]interface{}{
					"type":        "string",
					"description": "Path of the file relative to repository root",
				},
				"revision": map[string]interface{}{
					"type":        "string",
					"description": "Must be one of: initial, latest, lastReviewed. Initial revision means the revision before pull request change; latest revision means the revision after pull request change; lastReviewed revision means the revision last reviewed",
				},
			},
			Required: []string{"pullRequestReference", "filePath", "revision"},
		},
	})

	createPullRequestSchema := command.getInputSchemaForTool("createPullRequest", schemas)
	if createPullRequestSchema.Type == "" {
		command.logf("Failed to get input schema for createPullRequest tool")
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for createPullRequest tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "createPullRequest",
		Description: "Create a new pull request",
		InputSchema: createPullRequestSchema,
	})

	editPullRequestSchema := command.getInputSchemaForTool("editPullRequest", schemas)
	if editPullRequestSchema.Type == "" {
		command.logf("Failed to get input schema for editPullRequest tool")
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for editPullRequest tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "editPullRequest",
		Description: "Edit an existing pull request",
		InputSchema: editPullRequestSchema,
	})

	tools = append(tools, Tool{
		Name:        "processPullRequest",
		Description: "Process a pull request",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"pullRequestReference": map[string]interface{}{
					"type":        "string",
					"description": PullRequestReferenceDesc,
				},
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform on the pull request. Expects one of: approve, requestChanges, merge, discard, reopen, deleteSourceBranch, restoreSourceBranch",
				},
				"comment": map[string]interface{}{
					"type":        "string",
					"description": "Comment for the operation",
				},
			},
			Required: []string{"pullRequestReference", "operation"},
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
					"description": "Content of the comment to add",
				},
			},
			Required: []string{"pullRequestReference", "commentContent"},
		},
	})

	tools = append(tools, Tool{
		Name:        "checkoutPullRequest",
		Description: "Checkout specified pull request in current working directory",
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

	queryBuildsSchema := command.getInputSchemaForTool("queryBuilds", schemas)
	if queryBuildsSchema.Type == "" {
		command.logf("Failed to get input schema for queryBuilds tool")
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get input schema for queryBuilds tool")
		return
	}
	tools = append(tools, Tool{
		Name:        "queryBuilds",
		Description: "Query builds in current project",
		InputSchema: queryBuildsSchema,
	})

	tools = append(tools, Tool{
		Name:        "getBuild",
		Description: "Get build information",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"buildReference": map[string]interface{}{
					"type":        "string",
					"description": BuildReferenceDesc,
				},
			},
			Required: []string{"buildReference"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getBuildLog",
		Description: "Get build log",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"buildReference": map[string]interface{}{
					"type":        "string",
					"description": BuildReferenceDesc,
				},
			},
			Required: []string{"buildReference"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getBuildFileContent",
		Description: "Get content of specified file in a build. When investigating a build failure, AI assistant may use this tool to examine content of some files. Specifically, file \".onedev-buildspec.yml\" defines the job used in the build",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"buildReference": map[string]interface{}{
					"type":        "string",
					"description": BuildReferenceDesc,
				},
				"filePath": map[string]interface{}{
					"type":        "string",
					"description": "Path of the file relative to repository root",
				},
			},
			Required: []string{"buildReference", "filePath"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getFileChangesSincePreviousSuccessfulSimilarBuild",
		Description: "Get file changes since previous successful build similar to specified build. When investigating a build failure, AI assistant may use this tool to check what has been changed recently",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"buildReference": map[string]interface{}{
					"type":        "string",
					"description": BuildReferenceDesc,
				},
			},
			Required: []string{"buildReference"},
		},
	})

	tools = append(tools, Tool{
		Name:        "runJob",
		Description: "Run specified job against specified branch or tag in specified project",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project": map[string]interface{}{
					"type":        "string",
					"description": "Project to run the job against",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Branch to run the job against. Either branch or tag should be provided, but not both",
				},
				"tag": map[string]interface{}{
					"type":        "string",
					"description": "Tag to run the job against. Either branch or tag should be provided, but not both",
				},
				"jobName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the job to run",
				},
				"params": map[string]interface{}{
					"type":        "array",
					"description": "Parameters to pass to the job in form of key=value",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			Required: []string{"jobName"},
		},
	})

	tools = append(tools, Tool{
		Name:        "runLocalJob",
		Description: "Run specified job against local changes in current working directory",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"jobName": map[string]interface{}{
					"type":        "string",
					"description": "Name of the job to run",
				},
				"params": map[string]interface{}{
					"type":        "array",
					"description": "Parameters to pass to the job in form of key=value",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
			Required: []string{"jobName"},
		},
	})

	tools = append(tools, Tool{
		Name:        "getBuildSpecSchema",
		Description: "Get build spec schema. AI assistant should call this tool to know how to edit build spec (.onedev-buildspec.yml)",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
	})

	tools = append(tools, Tool{
		Name:        "checkBuildSpec",
		Description: "Check build spec for its validity, as well as updating it to latest version if needed. AI assistant should call this tool before and after editing build spec (.onedev-buildspec.yml) to make sure it is valid and update to date",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
	})

	tools = append(tools, Tool{
		Name:        "getCurrentProject",
		Description: "Get default OneDev project for tod operations",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
	})
	tools = append(tools, Tool{
		Name:        "getWorkingDir",
		Description: "Get working directory of tod",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
	})
	tools = append(tools, Tool{
		Name:        "setWorkingDir",
		Description: "Set working directory of tod",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"workingDir": map[string]interface{}{
					"type":        "string",
					"description": "Absolute path in the file system",
				},
			},
			Required: []string{"workingDir"},
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

	command.logf("Sending tools list with %d tools", len(tools))
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleGetWorkingDirTool(request MCPRequest) {
	command.logf("Handling getWorkingDir tool call")

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: command.workingDir,
			},
		},
	}

	command.logf("getWorkingDir tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) getCurrentProject() (string, error) {
	if command.currentProjectCache == "" {
		var err error
		command.currentProjectCache, err = inferProject(command.workingDir)
		if err != nil {
			return "", err
		}
	}
	return command.currentProjectCache, nil
}

func (command *MCPCommand) handleGetCurrentProjectTool(request MCPRequest) {
	command.logf("Handling getCurrentProject tool call")

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
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

	command.logf("getCurrentProject tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleSetWorkingDirTool(request MCPRequest, params MCPParams) {
	command.logf("Handling setWorkingDir tool call")

	workingDirArg, exists := params.Arguments["workingDir"]
	if !exists {
		command.logf("Missing required parameter: workingDir")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: workingDir")
		return
	}

	var ok bool
	command.workingDir, ok = workingDirArg.(string)
	if !ok {
		command.logf("Invalid type for workingDir parameter: expected string")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for workingDir parameter: expected string")
		return
	}

	if command.workingDir == "" {
		command.logf("workingDir parameter cannot be empty")
		command.sendError(request.ID, ErrorCodeInvalidParams, "workingDir parameter cannot be empty")
		return
	}

	if !filepath.IsAbs(command.workingDir) {
		command.logf("workingDir parameter must be an absolute path")
		command.sendError(request.ID, ErrorCodeInvalidParams, "workingDir parameter must be an absolute path")
		return
	}

	command.currentProjectCache = ""

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: "Working directory set to " + command.workingDir,
			},
		},
	}

	command.logf("setWorkingDir tool call successful")
	command.sendResponse(request.ID, result)
}

// handleGetLoginNameTool handles the getLoginName tool call
func (command *MCPCommand) handleGetLoginNameTool(request MCPRequest, params MCPParams) {
	command.logf("Handling getLoginName tool call")

	// Get userName parameter if provided
	var userName string
	if userNameVal, exists := params.Arguments["userName"]; exists {
		if userNameStr, ok := userNameVal.(string); ok {
			userName = userNameStr
		}
	}

	// Build the API URL
	apiURL := config.ServerUrl + "/~api/mcp-helper/get-login-name?userName=" + url.QueryEscape(userName)

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("getLoginName tool call successful")
	command.sendResponse(request.ID, result)
}

// handleGetUnixTimestampTool handles the getUnixTimestamp tool call
func (command *MCPCommand) handleGetUnixTimestampTool(request MCPRequest, params MCPParams) {
	command.logf("Handling getUnixTimestamp tool call")

	// Get dateTimeDescription parameter, report error if not provided
	dateTimeDescriptionArg, exists := params.Arguments["dateTimeDescription"]
	if !exists {
		command.logf("Missing required parameter: dateTimeDescription")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: dateTimeDescription")
		return
	}
	dateTimeDescription, ok := dateTimeDescriptionArg.(string)
	if !ok {
		command.logf("Invalid type for dateTimeDescription parameter: expected string")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for dateTimeDescription parameter: expected string")
		return
	}

	urlQuery := url.Values{
		"dateTimeDescription": {dateTimeDescription},
	}
	// Build the API URL
	apiURL := config.ServerUrl + "/~api/mcp-helper/get-unix-timestamp?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("getUnixTimestamp tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) getBuild(buildReference string, currentProject string) (map[string]interface{}, error) {
	data, err := command.getEntityData(buildReference, currentProject, "get-build")
	if err != nil {
		return nil, err
	}

	var build map[string]interface{}
	if err := json.Unmarshal(data, &build); err != nil {
		command.logf("Failed to parse JSON response: %v", err)
		return nil, err
	}

	return build, nil
}

func (command *MCPCommand) getPullRequest(pullRequestReference string, currentProject string) (map[string]interface{}, error) {
	data, err := command.getEntityData(pullRequestReference, currentProject, "get-pull-request")
	if err != nil {
		return nil, err
	}

	var pullRequest map[string]interface{}
	if err := json.Unmarshal(data, &pullRequest); err != nil {
		command.logf("Failed to parse JSON response: %v", err)
		return nil, err
	}

	return pullRequest, nil
}

func (command *MCPCommand) handleGetPullRequestFileContentTool(request MCPRequest, params MCPParams) {
	command.logf("Handling getPullRequestFileContent tool call")

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	reference, err := command.getNonEmptyStringParam(params, "pullRequestReference")
	if err != nil {
		command.logf("Failed to extract pullRequestReference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract pullRequestReference: "+err.Error())
		return
	}

	filePath, err := command.getNonEmptyStringParam(params, "filePath")
	if err != nil {
		command.logf("Failed to extract filePath: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract filePath: "+err.Error())
		return
	}

	revision, err := command.getNonEmptyStringParam(params, "revision")
	if err != nil {
		command.logf("Failed to extract revision: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract revision: "+err.Error())
		return
	}

	pullRequest, err := command.getPullRequest(reference, currentProject)

	var commitHash string
	if revision == "latest" {
		commitHash = pullRequest["headCommitHash"].(string)
	} else {
		patchInfo, err := command.getPullRequestPatchInfo(currentProject, reference, revision != "initial")
		if err != nil {
			command.logf("Failed to get pull request patch info: %v", err)
			command.sendError(request.ID, ErrorCodeInternalError, "Failed to get pull request patch info: "+err.Error())
			return
		}
		commitHash = patchInfo["oldCommitHash"].(string)
	}

	if err != nil {
		command.logf("Failed to get pull request info: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get pull request info: "+err.Error())
		return
	}

	apiURL := fmt.Sprintf("%s/%s/~raw/%s/%s",
		config.ServerUrl,
		pullRequest["targetProject"].(string),
		commitHash,
		filePath,
	)

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("getPullRequestFileContent tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleGetBuildFileContentTool(request MCPRequest, params MCPParams) {
	command.logf("Handling getBuildFileContent tool call")

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	reference, err := command.getNonEmptyStringParam(params, "buildReference")
	if err != nil {
		command.logf("Failed to extract buildReference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract buildReference: "+err.Error())
		return
	}

	filePath, err := command.getNonEmptyStringParam(params, "filePath")
	if err != nil {
		command.logf("Failed to extract filePath: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract filePath: "+err.Error())
		return
	}

	build, err := command.getBuild(reference, currentProject)

	if err != nil {
		command.logf("Failed to get build info: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get build info: "+err.Error())
		return
	}

	apiURL := fmt.Sprintf("%s/%s/~raw/%s/%s",
		config.ServerUrl,
		build["project"].(string),
		build["commitHash"].(string),
		filePath,
	)

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("getBuildFileContent tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleGetFileChangesSincePreviousSuccessfulSimilarBuildTool(request MCPRequest, params MCPParams) {
	command.logf("Handling getFileChangesSincePreviousSuccessfulSimilarBuild tool call")

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	reference, err := command.getNonEmptyStringParam(params, "buildReference")
	if err != nil {
		command.logf("Failed to extract buildReference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract buildReference: "+err.Error())
		return
	}

	build, err := command.getBuild(reference, currentProject)
	if err != nil {
		command.logf("Failed to get build info: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get build info: "+err.Error())
		return
	}

	// Build the API URL
	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/get-previous-successful-similar-build?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	var previousSuccessfulBuild map[string]interface{}
	if err := json.Unmarshal(body, &previousSuccessfulBuild); err != nil {
		command.logf("Failed to parse JSON response: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to parse JSON response: "+err.Error())
		return
	}

	urlQuery = url.Values{
		"old-commit": {previousSuccessfulBuild["commitHash"].(string)},
		"new-commit": {build["commitHash"].(string)},
	}

	var projectId = int(build["projectId"].(float64))
	apiURL = fmt.Sprintf("%s/~downloads/projects/%d/patch?%s", config.ServerUrl, projectId, urlQuery.Encode())

	req, err = http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	body, err = makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("getFileChangesSincePreviousSuccessfulSimilarBuild tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleQueryEntitiesTool(request MCPRequest, params MCPParams, toolName string,
	endpointSuffix string) {

	command.logf("Handling %s tool call", toolName)

	var project string
	if projectArg, exists := params.Arguments["project"]; exists {
		var ok bool
		project, ok = projectArg.(string)
		if !ok {
			command.logf("Invalid type for project parameter: expected string")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for project parameter: expected string")
			return
		}
	}

	var query string
	if queryVal, exists := params.Arguments["query"]; exists {
		var ok bool
		query, ok = queryVal.(string)
		if !ok {
			command.logf("Invalid type for query parameter: expected string")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for query parameter: expected string")
			return
		}
	}

	var offset float64
	if offsetVal, exists := params.Arguments["offset"]; exists {
		var ok bool
		offset, ok = offsetVal.(float64)
		if !ok {
			command.logf("Invalid type for offset parameter: expected integer")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for offset parameter: expected integer")
			return
		}
	} else {
		offset = 0
	}

	var count float64
	if countVal, exists := params.Arguments["count"]; exists {
		var ok bool
		count, ok = countVal.(float64)
		if !ok {
			command.logf("Invalid type for count parameter: expected integer")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for count parameter: expected integer")
			return
		}
	} else {
		count = DefaultQueryCount
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	urlQuery := url.Values{
		"project":        {project},
		"currentProject": {currentProject},
		"query":          {query},
		"offset":         {fmt.Sprintf("%d", int(offset))},
		"count":          {fmt.Sprintf("%d", int(count))},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("%s tool call successful", toolName)
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) getNonEmptyStringParam(params MCPParams, paramName string) (string, error) {
	paramVal, exists := params.Arguments[paramName]
	if !exists {
		return "", fmt.Errorf("missing required parameter: %s", paramName)
	}

	param, ok := paramVal.(string)
	if !ok {
		return "", fmt.Errorf("expect string type for parameter: %s", paramName)
	}

	if param == "" {
		return "", fmt.Errorf("parameter cannot be empty: %s", paramName)
	}

	return param, nil
}

func (command *MCPCommand) handleGetEntityDataTool(request MCPRequest, params MCPParams, toolName string,
	endpointSuffix string, referenceParamName string) {

	command.logf("Handling %s tool call", toolName)

	reference, err := command.getNonEmptyStringParam(params, referenceParamName)
	if err != nil {
		command.logf("Failed to extract %s: %v", referenceParamName, err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract "+referenceParamName+": "+err.Error())
		return
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	data, err := command.getEntityData(reference, currentProject, endpointSuffix)
	if err != nil {
		command.logf("Failed to get entity data: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get entity data: "+err.Error())
		return
	}

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: string(data),
			},
		},
	}

	command.logf("%s tool call successful", toolName)
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) getEntityData(reference string, currentProject string, endpointSuffix string) ([]byte, error) {
	// Build the API URL
	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		return nil, err
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		return nil, err
	}

	return body, nil
}

func (command *MCPCommand) handleCheckoutPullRequestTool(request MCPRequest, params MCPParams) {
	command.logf("Handling checkoutPullRequest tool call")

	reference, err := command.getNonEmptyStringParam(params, "pullRequestReference")
	if err != nil {
		command.logf("Failed to extract pullRequestReference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract pullRequestReference: "+err.Error())
		return
	}

	err = checkoutPullRequest(command.workingDir, reference, command.logger)
	if err != nil {
		command.logf("Failed to checkout pull request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to checkout pull request: "+err.Error())
		return
	}

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: "Checked out pull request " + reference + ". AI assistant should not do anything else",
			},
		},
	}

	command.logf("checkoutPullRequest tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) getPullRequestPatchInfo(currentProject string, reference string, sinceLastReview bool) (map[string]interface{}, error) {
	// Build the API URL
	urlQuery := url.Values{
		"currentProject":  {currentProject},
		"reference":       {reference},
		"sinceLastReview": {fmt.Sprintf("%t", sinceLastReview)},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/get-pull-request-patch-info?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		return nil, err
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		return nil, err
	}

	var patchInfo map[string]interface{}
	if err := json.Unmarshal(body, &patchInfo); err != nil {
		command.logf("Failed to parse JSON response: %v", err)
		return nil, err
	}

	return patchInfo, nil
}

func (command *MCPCommand) handleGetPullRequestFileChangesTool(request MCPRequest, params MCPParams) {
	command.logf("Handling getPullRequestFileChanges tool call")

	reference, err := command.getNonEmptyStringParam(params, "pullRequestReference")
	if err != nil {
		command.logf("Failed to extract pullRequestReference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract pullRequestReference: "+err.Error())
		return
	}

	sinceLastReviewVal, exists := params.Arguments["sinceLastReview"]

	if !exists {
		command.logf("Missing required parameter: sinceLastReview")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: sinceLastReview")
		return
	}

	sinceLastReview, ok := sinceLastReviewVal.(bool)
	if !ok {
		command.logf("Invalid type for sinceLastReview parameter: expected boolean")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for sinceLastReview parameter: expected boolean")
		return
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	patchInfo, err := command.getPullRequestPatchInfo(currentProject, reference, sinceLastReview)
	if err != nil {
		command.logf("Failed to get pull request patch info: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get pull request patch info: "+err.Error())
		return
	}

	projectId, _ := patchInfo["projectId"].(string)
	oldCommitHash, _ := patchInfo["oldCommitHash"].(string)
	newCommitHash, _ := patchInfo["newCommitHash"].(string)

	urlQuery := url.Values{
		"old-commit": {oldCommitHash},
		"new-commit": {newCommitHash},
	}

	apiURL := config.ServerUrl + "/~downloads/projects/" + projectId + "/patch?" + urlQuery.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("getPullRequestFileChanges tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleGetBuildLogTool(request MCPRequest, params MCPParams) {
	command.logf("Handling getBuildLog tool call")

	reference, err := command.getNonEmptyStringParam(params, "buildReference")
	if err != nil {
		command.logf("Failed to extract buildReference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract buildReference: "+err.Error())
		return
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	// Build the API URL
	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/get-build?" + urlQuery.Encode()

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
		return
	}

	var buildInfo map[string]interface{}
	if err := json.Unmarshal(body, &buildInfo); err != nil {
		command.logf("Failed to parse JSON response: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to parse JSON response: "+err.Error())
		return
	}

	apiURL = fmt.Sprintf("%s/~downloads/projects/%d/builds/%d/log",
		config.ServerUrl,
		int64(buildInfo["projectId"].(float64)),
		int64(buildInfo["number"].(float64)),
	)

	req, err = http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	body, err = makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("getBuildLog tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleAddEntityCommentTool(request MCPRequest, params MCPParams, toolName string,
	endpointSuffix string, referenceParamName string) {

	command.logf("Handling %s tool call", toolName)

	reference, err := command.getNonEmptyStringParam(params, referenceParamName)
	if err != nil {
		command.logf("Failed to extract %s: %v", referenceParamName, err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract "+referenceParamName+": "+err.Error())
		return
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	commentContentVal, exists := params.Arguments["commentContent"]
	if !exists {
		command.logf("Missing required parameter: commentContent")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: commentContent")
		return
	}

	commentContent, ok := commentContentVal.(string)
	if !ok {
		command.logf("Invalid type for commentContent parameter: expected string")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for commentContent parameter: expected string")
		return
	}

	if commentContent == "" {
		command.logf("Empty commentContent parameter")
		command.sendError(request.ID, ErrorCodeInvalidParams, "commentContent parameter cannot be empty")
		return
	}

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(commentContent))
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "text/plain")

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("%s tool call successful", toolName)
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleProcessPullRequestTool(request MCPRequest, params MCPParams) {
	command.logf("Handling processPullRequest tool call")

	reference, err := command.getNonEmptyStringParam(params, "pullRequestReference")
	if err != nil {
		command.logf("Failed to extract pullRequestReference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract pullRequestReference: "+err.Error())
		return
	}

	operationVal, exists := params.Arguments["operation"]
	if !exists {
		command.logf("Missing required parameter: operation")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: operation")
		return
	}

	operation, ok := operationVal.(string)
	if !ok {
		command.logf("Invalid type for operation parameter: expected string")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for operation parameter: expected string")
		return
	}

	comment := ""
	commentVal, exists := params.Arguments["comment"]
	if exists {
		comment, ok = commentVal.(string)
		if !ok {
			command.logf("Invalid type for comment parameter: expected string")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for comment parameter: expected string")
			return
		}
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
		"operation":      {operation},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/process-pull-request?" + urlQuery.Encode()

	// Create HTTP POST request with comment content as body
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(comment))
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "text/plain")

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("processPullRequest tool call successful")
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleLogWorkTool(request MCPRequest, params MCPParams) {
	command.logf("Handling logWork tool call")

	issueReference, err := command.getNonEmptyStringParam(params, "issueReference")
	if err != nil {
		command.logf("Failed to extract issue reference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract issue reference: "+err.Error())
		return
	}

	spentHoursVal, exists := params.Arguments["spentHours"]
	if !exists {
		command.logf("Missing required parameter: spentHours")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: spentHours")
		return
	}

	spentHours, ok := spentHoursVal.(float64)
	if !ok {
		command.logf("Invalid type for spentHours parameter: expected float64")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for spentHours parameter: expected float64")
		return
	}

	var comment string
	// Extract required commentContent parameter
	commentVal, exists := params.Arguments["comment"]
	if exists {
		comment, ok = commentVal.(string)
		if !ok {
			command.logf("Invalid type for comment parameter: expected string")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for comment parameter: expected string")
			return
		}
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	var urlQuery = url.Values{
		"currentProject": {currentProject},
		"reference":      {issueReference},
		"spentHours":     {fmt.Sprintf("%d", int64(spentHours))},
	}
	// Build the API URL
	apiURL := config.ServerUrl + "/~api/mcp-helper/log-work?" + urlQuery.Encode()

	// Create HTTP POST request with comment content as body
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(comment))
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "text/plain")

	// Make the API call
	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("logWork tool call successful")
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleCreateIssueTool(request MCPRequest, params MCPParams, toolName string, endpointSuffix string) {

	command.logf("Handling %s tool call", toolName)

	entityMap := make(map[string]interface{})

	var project string
	if projectArg, exists := params.Arguments["project"]; exists {
		var ok bool
		project, ok = projectArg.(string)
		if !ok {
			command.logf("Invalid type for project parameter: expected string")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for project parameter: expected string")
			return
		}
	}

	for paramName, paramValue := range params.Arguments {
		if paramName != "project" {
			entityMap[paramName] = paramValue
		}
	}

	entityBytes, err := json.Marshal(entityMap)
	if err != nil {
		command.logf("Failed to marshal map to JSON: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to marshal map to JSON: "+err.Error())
		return
	}
	entityData := string(entityBytes)

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	var urlQuery = url.Values{
		"project":        {project},
		"currentProject": {currentProject},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(entityData))
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("%s tool call successful", toolName)
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleRunJobTool(request MCPRequest, params MCPParams) {
	command.logf("Handling runJob tool call")

	var project string
	if projectArg, exists := params.Arguments["project"]; exists {
		var ok bool
		project, ok = projectArg.(string)
		if !ok {
			command.logf("Invalid type for project parameter: expected string")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for project parameter: expected string")
			return
		}
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	jobMap := make(map[string]interface{})

	for paramName, paramValue := range params.Arguments {
		if paramName != "project" {
			jobMap[paramName] = paramValue
		}
	}

	jobMap["reason"] = "Submitted via MCP"

	build, err := runJob(project, currentProject, jobMap)
	if err != nil {
		command.logf("Failed to run job: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to run job: "+err.Error())
		return
	}

	project = build["project"].(string)
	buildNumber := int(build["number"].(float64))

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: fmt.Sprintf("%s/%s/~builds/%d", config.ServerUrl, project, buildNumber),
			},
		},
	}

	command.logf("runJob tool call successful")
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleCreatePullRequestTool(request MCPRequest, params MCPParams) {

	command.logf("Handling createPullRequest tool call")

	entityMap := make(map[string]interface{})

	var targetProject string
	if targetProjectArg, exists := params.Arguments["targetProject"]; exists {
		var ok bool
		targetProject, ok = targetProjectArg.(string)
		if !ok {
			command.logf("Invalid type for targetProject parameter: expected string")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for targetProject parameter: expected string")
			return
		}
	}

	var sourceProject string
	if sourceProjectArg, exists := params.Arguments["sourceProject"]; exists {
		var ok bool
		sourceProject, ok = sourceProjectArg.(string)
		if !ok {
			command.logf("Invalid type for sourceProject parameter: expected string")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for sourceProject parameter: expected string")
			return
		}
	}

	for paramName, paramValue := range params.Arguments {
		if paramName != "targetProject" && paramName != "sourceProject" {
			entityMap[paramName] = paramValue
		}
	}

	entityBytes, err := json.Marshal(entityMap)
	if err != nil {
		command.logf("Failed to marshal map to JSON: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to marshal map to JSON: "+err.Error())
		return
	}
	entityData := string(entityBytes)

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	var urlQuery = url.Values{
		"targetProject":  {targetProject},
		"sourceProject":  {sourceProject},
		"currentProject": {currentProject},
	}

	apiURL := config.ServerUrl + "/~api/mcp-helper/create-pull-request?" + urlQuery.Encode()

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(entityData))
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("createPullRequest tool call successful")
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleEditEntityTool(request MCPRequest, params MCPParams, toolName string,
	endpointSuffix string, referenceParamName string) {

	command.logf("Handling %s tool call", toolName)

	entityMap := make(map[string]interface{})

	// Extract all parameters from arguments
	for paramName, paramValue := range params.Arguments {
		if paramName != referenceParamName {
			entityMap[paramName] = paramValue
		}
	}

	entityBytes, err := json.Marshal(entityMap)
	if err != nil {
		command.logf("Failed to marshal map to JSON: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to marshal map to JSON: "+err.Error())
		return
	}
	entityData := string(entityBytes)

	reference, err := command.getNonEmptyStringParam(params, referenceParamName)
	if err != nil {
		command.logf("Failed to extract %s: %v", referenceParamName, err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract "+referenceParamName+": "+err.Error())
		return
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}

	// Build the API URL
	apiURL := config.ServerUrl + "/~api/mcp-helper/" + endpointSuffix + "?" + urlQuery.Encode()

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(entityData))
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("%s tool call successful", toolName)
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleTransitIssueTool(request MCPRequest, params MCPParams) {
	command.logf("Handling transitIssue tool call")

	issueMap := make(map[string]interface{})

	// Extract all parameters from arguments
	for paramName, paramValue := range params.Arguments {
		if paramName != "issueReference" {
			issueMap[paramName] = paramValue
		}
	}

	issueBytes, err := json.Marshal(issueMap)
	if err != nil {
		command.logf("Failed to marshal map to JSON: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to marshal map to JSON: "+err.Error())
		return
	}
	issueData := string(issueBytes)

	issueReference, err := command.getNonEmptyStringParam(params, "issueReference")
	if err != nil {
		command.logf("Failed to extract issue reference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract issue reference: "+err.Error())
		return
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	urlQuery := url.Values{
		"currentProject": {currentProject},
		"reference":      {issueReference},
	}
	// Build the API URL
	apiURL := config.ServerUrl + "/~api/mcp-helper/transit-issue?" + urlQuery.Encode()

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(issueData))
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to make API call: "+err.Error())
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

	command.logf("transitIssue tool call successful")
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleLinkIssuesTool(request MCPRequest, params MCPParams) {
	command.logf("Handling linkIssues tool call")

	sourceReference, err := command.getNonEmptyStringParam(params, "sourceIssueReference")
	if err != nil {
		command.logf("Failed to extract source issue reference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract source issuereference: "+err.Error())
		return
	}
	targetReference, err := command.getNonEmptyStringParam(params, "targetIssueReference")
	if err != nil {
		command.logf("Failed to extract target issue reference: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to extract target issue reference: "+err.Error())
		return
	}
	linkNameArg, exists := params.Arguments["linkName"]
	if !exists {
		command.logf("Missing required parameter: linkName")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: linkName")
		return
	}
	linkName, ok := linkNameArg.(string)
	if !ok {
		command.logf("Invalid type for linkName parameter: expected string")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for linkName parameter: expected string")
		return
	}

	currentProject, err := command.getCurrentProject()
	if err != nil {
		command.logf("Failed to get current project: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to get current project: "+err.Error())
		return
	}

	urlQuery := url.Values{
		"currentProject":  {currentProject},
		"sourceReference": {sourceReference},
		"targetReference": {targetReference},
		"linkName":        {linkName},
	}
	apiURL := config.ServerUrl + "/~api/mcp-helper/link-issues?" + urlQuery.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")

	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to make API call: "+err.Error())
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

	command.logf("createIssue tool call successful")
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleToolsCall(request MCPRequest) {
	command.logf("Handling tools/call request")

	paramsBytes, _ := json.Marshal(request.Params)
	var params MCPParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		command.logf("Failed to parse tool call params: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid params: "+err.Error())
		return
	}

	command.logf("Calling tool: %s", params.Name)
	switch params.Name {
	case "queryIssues":
		command.handleQueryEntitiesTool(request, params, "queryIssues", "query-issues")
	case "getIssue":
		command.handleGetEntityDataTool(request, params, "getIssue", "get-issue", "issueReference")
	case "getIssueComments":
		command.handleGetEntityDataTool(request, params, "getIssueComments", "get-issue-comments", "issueReference")
	case "createIssue":
		command.handleCreateIssueTool(request, params, "createIssue", "create-issue")
	case "editIssue":
		command.handleEditEntityTool(request, params, "editIssue", "edit-issue", "issueReference")
	case "transitIssue":
		command.handleTransitIssueTool(request, params)
	case "linkIssues":
		command.handleLinkIssuesTool(request, params)
	case "addIssueComment":
		command.handleAddEntityCommentTool(request, params, "addIssueComment", "add-issue-comment", "issueReference")
	case "logWork":
		command.handleLogWorkTool(request, params)
	case "queryPullRequests":
		command.handleQueryEntitiesTool(request, params, "queryPullRequests", "query-pull-requests")
	case "getPullRequest":
		command.handleGetEntityDataTool(request, params, "getPullRequest", "get-pull-request", "pullRequestReference")
	case "getPullRequestComments":
		command.handleGetEntityDataTool(request, params, "getPullRequestComments", "get-pull-request-comments", "pullRequestReference")
	case "getPullRequestCodeComments":
		command.handleGetEntityDataTool(request, params, "getPullRequestCodeComments", "get-pull-request-code-comments", "pullRequestReference")
	case "getPullRequestFileChanges":
		command.handleGetPullRequestFileChangesTool(request, params)
	case "getPullRequestFileContent":
		command.handleGetPullRequestFileContentTool(request, params)
	case "createPullRequest":
		command.handleCreatePullRequestTool(request, params)
	case "editPullRequest":
		command.handleEditEntityTool(request, params, "editPullRequest", "edit-pull-request", "pullRequestReference")
	case "processPullRequest":
		command.handleProcessPullRequestTool(request, params)
	case "addPullRequestComment":
		command.handleAddEntityCommentTool(request, params, "addPullRequestComment", "add-pull-request-comment", "pullRequestReference")
	case "checkoutPullRequest":
		command.handleCheckoutPullRequestTool(request, params)
	case "queryBuilds":
		command.handleQueryEntitiesTool(request, params, "queryBuilds", "query-builds")
	case "getBuild":
		command.handleGetEntityDataTool(request, params, "getBuild", "get-build", "buildReference")
	case "getBuildLog":
		command.handleGetBuildLogTool(request, params)
	case "getBuildFileContent":
		command.handleGetBuildFileContentTool(request, params)
	case "getFileChangesSincePreviousSuccessfulSimilarBuild":
		command.handleGetFileChangesSincePreviousSuccessfulSimilarBuildTool(request, params)
	case "runJob":
		command.handleRunJobTool(request, params)
	case "runLocalJob":
		command.handleRunLocalJobTool(request, params)
	case "getBuildSpecSchema":
		command.handleGetBuildSpecSchemaTool(request)
	case "checkBuildSpec":
		command.handleCheckBuildSpecTool(request)
	case "getCurrentProject":
		command.handleGetCurrentProjectTool(request)
	case "getWorkingDir":
		command.handleGetWorkingDirTool(request)
	case "setWorkingDir":
		command.handleSetWorkingDirTool(request, params)
	case "getLoginName":
		command.handleGetLoginNameTool(request, params)
	case "getUnixTimestamp":
		command.handleGetUnixTimestampTool(request, params)
	default:
		command.logf("Unknown tool requested: %s", params.Name)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Unknown tool: "+params.Name)
	}
}

func (command *MCPCommand) handleRunLocalJobTool(request MCPRequest, params MCPParams) {
	command.logf("Handling runLocalJob tool call")

	jobNameVal, exists := params.Arguments["jobName"]
	if !exists {
		command.logf("Missing required parameter: jobName")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: jobName")
		return
	}

	jobName, ok := jobNameVal.(string)
	if !ok {
		command.logf("Invalid type for jobName parameter: expected string")
		command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for jobName parameter: expected string")
		return
	}

	if jobName == "" {
		command.logf("jobName parameter cannot be empty")
		command.sendError(request.ID, ErrorCodeInvalidParams, "jobName parameter cannot be empty")
		return
	}

	// Parse parameters
	paramMap := make(map[string][]string)
	if paramsVal, exists := params.Arguments["params"]; exists {
		if paramsArray, ok := paramsVal.([]interface{}); ok {
			for _, paramInterface := range paramsArray {
				if paramStr, ok := paramInterface.(string); ok {
					parts := strings.Split(paramStr, "=")
					if len(parts) != 2 {
						command.logf("Invalid parameter format (expected key=value): %s", paramStr)
						command.sendError(request.ID, ErrorCodeInvalidParams, fmt.Sprintf("Invalid parameter format (expected key=value): %s", paramStr))
						return
					}

					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])

					if len(key) == 0 {
						command.logf("Parameter key cannot be empty: %s", paramStr)
						command.sendError(request.ID, ErrorCodeInvalidParams, fmt.Sprintf("Parameter key cannot be empty: %s", paramStr))
						return
					}

					if len(val) == 0 {
						command.logf("Parameter value cannot be empty: %s", paramStr)
						command.sendError(request.ID, ErrorCodeInvalidParams, fmt.Sprintf("Parameter value cannot be empty: %s", paramStr))
						return
					}

					// Append to existing values instead of replacing
					if existingValues, exists := paramMap[key]; exists {
						paramMap[key] = append(existingValues, val)
					} else {
						paramMap[key] = []string{val}
					}
				} else {
					command.logf("Invalid type for param element: expected string")
					command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for param element: expected string")
					return
				}
			}
		} else {
			command.logf("Invalid type for params parameter: expected array")
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid type for params parameter: expected array")
			return
		}
	}

	// Use the common prepareLocalJob function
	build, err := runLocalJob(jobName, command.workingDir, paramMap, "Submitted via MCP", command.logger)
	if err != nil {
		command.logf("Failed to prepare local job: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to prepare local job: "+err.Error())
		return
	}

	project := build["project"].(string)
	buildNumber := int(build["number"].(float64))

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: fmt.Sprintf("%s/%s/~builds/%d", config.ServerUrl, project, buildNumber),
			},
		},
	}

	command.logf("runLocalJob tool call successful")
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleGetBuildSpecSchemaTool(request MCPRequest) {
	command.logf("Handling getBuildSpecSchema tool call")

	// Build the API URL
	apiURL := config.ServerUrl + "/~api/build-spec-schema.yml"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		command.logf("Failed to create request: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to create request: "+err.Error())
		return
	}

	body, err := makeAPICall(req)
	if err != nil {
		command.logf("Failed to make API call: %v", err)
		command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to make API call: "+err.Error())
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

	command.logf("getBuildSpecSchema tool call successful")
	command.sendResponse(request.ID, response)
}

func (command *MCPCommand) handleCheckBuildSpecTool(request MCPRequest) {
	command.logf("Handling checkBuildSpec tool call")

	// Call the checkBuildSpec method using the global logger
	err := checkBuildSpec(command.workingDir, command.logger)
	if err != nil {
		command.logf("Failed to check build spec: %v", err)
		command.sendError(request.ID, ErrorCodeInternalError, "Failed to check build spec: "+err.Error())
		return
	}

	response := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: "Build spec checked successfully",
			},
		},
	}

	command.logf("checkBuildSpec tool call successful")
	command.sendResponse(request.ID, response)
}

// getInputSchemaForTool retrieves and converts a tool's input schema from the schemas map
// Returns an empty InputSchema if the tool is not found or conversion fails
func (command *MCPCommand) getInputSchemaForTool(toolName string, schemas map[string]interface{}) InputSchema {
	schemaData, exists := schemas[toolName]
	if !exists {
		command.logf("%s schema not found in API response", toolName)
		return InputSchema{}
	}

	// Check if schemaData is nil
	if schemaData == nil {
		command.logf("Schema data for %s is nil", toolName)
		return InputSchema{}
	}

	// Check if schemaData is the expected map type
	schemaMap, ok := schemaData.(map[string]interface{})
	if !ok {
		command.logf("Schema data for %s is not a map[string]interface{}, got %T", toolName, schemaData)
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
			command.logf("Type field for %s is not a string, got %T", toolName, typeVal)
			return InputSchema{}
		}
	}

	// Extract properties
	if propertiesVal, exists := schemaMap["Properties"]; exists {
		if properties, ok := propertiesVal.(map[string]interface{}); ok {
			schema.Properties = properties
		} else {
			command.logf("Properties field for %s is not a map[string]interface{}, got %T", toolName, propertiesVal)
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
					command.logf("Required field at index %d for %s is not a string, got %T", i, toolName, item)
					return InputSchema{}
				}
			}
			schema.Required = required
		} else {
			command.logf("Required field for %s is not a []interface{}, got %T", toolName, requiredVal)
			return InputSchema{}
		}
	}

	return schema
}

func (command *MCPCommand) sendResponse(id interface{}, result interface{}) {
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

func (command *MCPCommand) sendError(id interface{}, code int, message string) {
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

func (command *MCPCommand) handlePromptsList(request MCPRequest) {
	command.logf("Handling prompts/list request")

	prompts := []Prompt{
		{
			Name:        "edit-build-spec",
			Description: "Create or edit OneDev build spec (.onedev-buildspec.yml)",
			Arguments: []PromptArgument{
				{
					Name:        "instruction",
					Description: "Instruction to create or edit OneDev build spec",
					Required:    true,
				},
			},
		},
		{
			Name:        "investigate-build-failure",
			Description: "Investigate failure of a build",
			Arguments: []PromptArgument{
				{
					Name:        "buildReference",
					Description: "Reference of the build to investigate, for instance #123, project#123, or projectkey-123",
					Required:    true,
				},
			},
		},
		{
			Name:        "review-pull-request",
			Description: "Review a pull request",
			Arguments: []PromptArgument{
				{
					Name:        "pullRequestReference",
					Description: "Reference of the pull request to review, for instance #123, project#123, or projectkey-123",
					Required:    true,
				},
				{
					Name:        "sinceLastReview",
					Description: "Either true or false. If true, only changes since last review will be reviewed; otherwise all changes of the pull request will be reviewed",
					Required:    true,
				},
			},
		},
	}

	result := map[string]interface{}{
		"prompts": prompts,
	}

	command.logf("Sending prompts list with %d prompts", len(prompts))
	command.sendResponse(request.ID, result)
}

func (command *MCPCommand) handleGetPrompt(request MCPRequest) {
	command.logf("Handling prompts/get request")

	var params MCPParams
	if request.Params != nil {
		paramsBytes, err := json.Marshal(request.Params)
		if err != nil {
			command.sendError(request.ID, ErrorCodeInvalidParams, "Failed to parse parameters")
			return
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			command.sendError(request.ID, ErrorCodeInvalidParams, "Invalid parameters format")
			return
		}
	}

	if params.Name == "" {
		command.sendError(request.ID, ErrorCodeInvalidParams, "Missing required parameter: name")
		return
	}

	var messages []PromptMessage

	switch params.Name {
	case "edit-build-spec":
		instruction, err := command.getNonEmptyStringParam(params, "instruction")
		if err != nil {
			command.sendError(request.ID, ErrorCodeInvalidParams, "failed to extract instruction: "+err.Error())
			return
		}

		messages = []PromptMessage{
			{
				Role: "user",
				Content: PromptMessageContent{
					Type: "text",
					Text: instruction,
				},
			},
			{
				Role: "system",
				Content: PromptMessageContent{
					Type: "text",
					Text: `When create or edit OneDev build spec (.onedev-buildspec.yml), you should:
1. Call the getBuildSpecSchema tool first to know about syntax of OneDev build spec
2. Remember that different steps run in isolated environments, with shared job workspace. So it will not work installing dependencies in one step, and run commands relying on them in another step. You should put them in a single step unless requested by user explicitly
3. Remember that if cache step is used, it should be placed before the step building or testing the project
4. Remember that if cache step is used, and its key property contains checksum of lock files, the generate checksum step should be added before the cache step to generate the checksum
5. Remember that if build spec already exists, call checkBuildSpec tool to make sure it is valid and up to date before editing
6. Remember that to pass files between different jobs, one job should publish files via the publish artifact step, and another jobs can then download them into job workspace via job dependency
7. Inspect project structure and relevant files to figure out what docker image and commands to use to build or test the project if requested by user
8. After creating or editing the build spec, call the checkBuildSpec tool to make sure the new build spec is valid`,
				},
			},
		}

	case "investigate-build-failure":
		reference, err := command.getNonEmptyStringParam(params, "buildReference")
		if err != nil {
			command.sendError(request.ID, ErrorCodeInvalidParams, "failed to extract buildReference: "+err.Error())
			return
		}

		messages = []PromptMessage{
			{
				Role: "user",
				Content: PromptMessageContent{
					Type: "text",
					Text: `Please investigate build failure with below information:
1. Call the getBuild tool with parameter "buildReference" set to ` + reference + ` to get the build detail
2. Call the getBuildLog tool with parameter "buildReference" set to ` + reference + ` to get the build log
3. If you need to examine content of files mentioned in build log, call getBuildFileContent tool with parameter "buildReference" set to ` + reference + ` and "filePath" set to desired file path. Specifically specify file path as ".onedev-buildspec.yml" to get the build spec
4. You may also call getFileChangesSincePreviousSuccessfulSimilarBuild tool with parameter "buildReference" set to ` + reference + ` to get file changes since previous successful build similar to the current build`,
				},
			},
		}

	case "review-pull-request":
		reference, err := command.getNonEmptyStringParam(params, "pullRequestReference")
		if err != nil {
			command.sendError(request.ID, ErrorCodeInvalidParams, "failed to extract pullRequestReference: "+err.Error())
			return
		}

		sinceLastReview := false
		if sinceLastReviewVal, exists := params.Arguments["sinceLastReview"]; exists {
			if sinceLastReviewBool, ok := sinceLastReviewVal.(bool); ok {
				sinceLastReview = sinceLastReviewBool
			}
		}

		messages = []PromptMessage{
			{
				Role: "user",
				Content: PromptMessageContent{
					Type: "text",
					Text: `Please follow below steps to review pull request:
1. Call the getPullRequest tool with parameter "pullRequestReference" set to ` + reference + ` to get the pull request detail, including title and description
2. Call the getPullRequestFileChanges tool with below parameters:
	2.1 "pullRequestReference" set to ` + reference + `
	2.2 "sinceLastReview" set to ` + fmt.Sprintf("%t", sinceLastReview) + ` to get the file changes for review
3. If you need to examine full content of files mentioned in file changes, call getPullRequestFileContent tool with below parameters:
	3.1 "pullRequestReference" set to ` + reference + `
	3.2 "filePath" set to desired file path
	3.3 "revision" set to "initial" if sinceLastReview is false and you want to get file content before change, or "lastReviewed" if sinceLastReview is true and you want to get file content before change, or "latest" if you want to get file content after change
4. After reviewing the pull request, call the getLoginName tool without parameters to get your login name, and then check against reviews information in pull request detail to see if it is pending your review:
 	4.1 If the pull request is awaiting your review, request user's consent to call the processPullRequest tool with below parameters: 
		4.1.1 "pullRequestReference" set to ` + reference + `
		4.1.2 "operation" set to either "approve" or "requestChanges" based on your review result
		4.1.3 "comment" set to your review comment
	4.2 Otherwise request user's consent to leave a comment via addPullRequestComment tool with below parameters:
		4.2.1 "pullRequestReference" set to ` + reference + `
		4.2.2 "commentContent" set to your review comment`,
				},
			},
		}

	default:
		command.sendError(request.ID, ErrorCodeInvalidParams, fmt.Sprintf("Unknown prompt name: %s", params.Name))
		return
	}

	result := GetPromptResult{
		Messages: messages,
	}

	command.logf("Get prompt successful for: %s", params.Name)
	command.sendResponse(request.ID, result)
}
