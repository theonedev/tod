# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TOD (**T**he**O**ne**D**ev) is a command line tool for OneDev 10.2+ that enables running CI/CD jobs against local changes without needing to commit/push to the repository. It works by stashing local changes, pushing them to a temporal ref, and streaming job execution logs back to the terminal.

## Architecture

### Core Components

- **main.go**: Entry point with command routing and version checking
- **command.go**: Command interface definition
- **exec_command.go**: Main execution command for running jobs against local changes
- **mcp_command.go**: MCP (Model Context Protocol) server implementation for tool integration
- **ansi_utils.go**: ANSI color formatting utilities for terminal output

### Command Structure

The application uses a command pattern with two main commands:
- `exec`: Runs CI/CD jobs against local changes
- `mcp`: Provides MCP server functionality for tool integration

### Key Architecture Patterns

1. **Git Integration**: Uses git stash/push operations to send local changes to OneDev server
2. **HTTP Streaming**: Real-time log streaming from OneDev server with binary protocol
3. **Configuration Management**: INI-based config file at `$HOME/.tod/config`
4. **Signal Handling**: Graceful cancellation of running jobs with Ctrl+C

## Development Commands

### Build
```bash
go build -o tod
```

### Test
```bash
go test
go test -v  # verbose output
```

### Run
```bash
# Basic exec command
./tod exec -project <project-url> -token <access-token> -workdir <git-directory> <job-name>

# MCP server mode
./tod mcp -server <server-url> -token <access-token>
```

## Configuration

### Config File Location
`$HOME/.tod/config`

### Config Format (INI)
```ini
[exec]
project=https://onedev.example.com/my/project
token=<generated-access-token>
workdir=/path/to/project
param.db=postgres
```

### Required Files in Working Directory
- `.git/`: Git repository
- `.onedev-buildspec.yml`: OneDev build specification

## API Integration

### OneDev API Endpoints Used
- `/~api/version/compatible-tod-versions`: Version compatibility check
- `/~api/projects`: Project lookup
- `/~api/job-runs`: Job execution
- `/~api/streaming/build-logs/{buildId}`: Log streaming
- `/~api/mcp-helper/*`: MCP tool endpoints

### Authentication
All API calls use Bearer token authentication via the `Authorization` header.

## Key Implementation Details

### Binary Log Protocol
The streaming log endpoint uses a binary protocol where:
- Positive length integers precede log entry JSON
- Negative length integers precede build status strings
- Log entries contain styled message arrays with ANSI formatting

### Error Handling
- Version compatibility is checked before job execution
- HTTP errors are wrapped with detailed context
- Build cancellation is handled via goroutines and signal channels

### MCP Server Features
- JSON-RPC 2.0 compliant
- Tools: `getLoginName`, `queryIssues`
- Dynamic parameter descriptions from API
- Structured error responses with API context

## Testing

The codebase includes comprehensive unit tests in `mcp_command_test.go` covering:
- API call functions
- MCP protocol handling
- Error scenarios with mock HTTP servers
- Tool parameter validation

Run tests to verify changes don't break existing functionality.