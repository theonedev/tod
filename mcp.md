# TOD MCP (Model Context Protocol) Documentation

## Overview

TOD offers a comprehensive Model Context Protocol (MCP) server with tools and prompts, enabling you to interact with OneDev 13+ through AI assistants in an intelligent and natural way.

## Installation

To install tod, just put tod binary into your PATH. 

### Download Pre-built Binaries

https://code.onedev.io/onedev/tod/~builds?query=%22Job%22+is+%22Release%22

### Build Binary from Source

**Requirements:**
- Go 1.22.1 or higher

**Steps:**
1. Clone the repository:
   ```bash
   git clone https://code.onedev.io/onedev/tod.git
   cd tod
   ```

2. Build the binary:
   ```bash
   go build
   ```

## Configuration

The MCP server uses the same configuration as other TOD commands. Ensure your `~/.todconfig` file is properly configured:

```ini
server-url=https://onedev.example.com
access-token=your-personal-access-token
```

## Start MCP Server

To start the MCP server:

```bash
# Start MCP server (uses default configuration)
tod mcp

# Start with debug logging
tod mcp --log-file /tmp/tod-mcp.log
```

Before integrating this MCP server with your AI assistant, it is highly recommended to run above command from your terminal first to make sure there isn't any errors.

> [!IMPORTANT] Working directory 
> This MCP server has a notion of working directory, which is set to project root initially for many 
> AI assistants. This directory is expected to be inside a git repository, with one of the remote 
> pointing to a OneDev project. Tool `getWorkingDir` and `setWorkingDir` is available to get 
> and set the working directory

## Available Tools

TOD's MCP server provides tools organized into the following categories:

### Issue Management Tools

#### `queryIssues`
Query issues in current project.

**Parameters:**
- `query` (optional) - Query string using OneDev's issue query syntax
- `project` (optional) - Project to query issues in. Leave empty for current project
- `count` (optional) - Number of issues to return (default: 25, max: 100)
- `offset` (optional) - Start position for the query (default: 0)

#### `getIssue`
Get detailed information about a specific issue.

**Parameters:**
- `issueReference` (required) - Issue reference in form `#<number>`, `<project>#<number>`, or `<project key>-<number>`

#### `getIssueComments`
Get comments for a specific issue.

**Parameters:**
- `issueReference` (required) - Issue reference in form `#<number>`, `<project>#<number>`, or `<project key>-<number>`

#### `createIssue`
Create a new issue.

**Parameters:**
- `title` (required) - Title of the issue
- `description` (optional) - Description of the issue
- `project` (optional) - Project to create issue in. Leave empty for current project
- `iterations` (optional) - Iterations to schedule the issue in
- `confidential` (optional) - Whether the issue is confidential
- various custom fields depending on OneDev issue settings

#### `editIssue`
Edit an existing issue.

**Parameters:**
- `issueReference` (required) - Reference of the issue to update
- `title` (optional) - Title of the issue
- `description` (optional) - Description of the issue
- `iterations` (optional) - Iterations to schedule the issue in
- `confidential` (optional) - Whether the issue is confidential
- various custom fields depending on OneDev issue settings

#### `changeIssueState`
Change state of specified issue.

**Parameters:**
- `issueReference` (required) - Reference of the issue to change state
- `state` (required) - New state for the issue
- `comment` (optional) - Comment for the state change

#### `linkIssues`
Set up links between two issues.

**Parameters:**
- `sourceIssueReference` (required) - Issue reference as source of the link
- `targetIssueReference` (required) - Issue reference as target of the link
- `linkName` (required) - Name of the link. Must be one of: Sub Issues, Parent Issue, Related

#### `addIssueComment`
Add a comment to an issue. For issue state change (work on issue, set issue done, submit issue for review, etc.), use the changeIssueState tool instead.

**Parameters:**
- `issueReference` (required) - Issue reference
- `commentContent` (required) - Content of the comment to add

#### `logWork`
Log spent time on an issue.

**Parameters:**
- `issueReference` (required) - Issue reference
- `spentHours` (required) - Spent time in hours
- `comment` (optional) - Comment to add to the work log

### Pull Request Management Tools

#### `queryPullRequests`
Query pull requests in current project.

**Parameters:**
- `query` (optional) - Query string using OneDev's pull request query syntax
- `project` (optional) - Project to query pull requests in. Leave empty for current project
- `count` (optional) - Number of pull requests to return (default: 25, max: 100)
- `offset` (optional) - Start position for the query (default: 0)

#### `getPullRequest`
Get detailed information about a specific pull request.

**Parameters:**
- `pullRequestReference` (required) - Pull request reference in form `#<number>`, `<project>#<number>`, or `<project key>-<number>`

#### `getPullRequestComments`
Get comments for a specific pull request.

**Parameters:**
- `pullRequestReference` (required) - Pull request reference

#### `getPullRequestCodeComments`
Get code comments for a specific pull request.

**Parameters:**
- `pullRequestReference` (required) - Pull request reference

#### `getPullRequestFileChanges`
Get pull request file changes in patch format.

**Parameters:**
- `pullRequestReference` (required) - Pull request reference
- `sinceLastReview` (required) - If true, only changes since last review will be returned

#### `getPullRequestFileContent`
Get content of specified file in pull request.

**Parameters:**
- `pullRequestReference` (required) - Pull request reference
- `filePath` (required) - Path of the file relative to repository root
- `revision` (required) - Must be one of: initial, latest, lastReviewed

#### `createPullRequest`
Create a new pull request.

**Parameters:**
- `sourceBranch` (required) - A branch in source project to be used as source branch
- `title` (required) - Title of the pull request. Leave empty to use default title
- `description` (optional) - Description of the pull request
- `sourceProject` (optional) - Source project. Leave empty to use current project
- `targetProject` (optional) - Target project. If left empty, defaults to original project when source is a fork
- `targetBranch` (optional) - Target branch. Leave empty to use default branch
- `assignees` (optional) - Array of assignee user login names
- `reviewers` (optional) - Array of reviewer user login names
- `mergeStrategy` (optional) - One of: CREATE_MERGE_COMMIT, CREATE_MERGE_COMMIT_IF_NECESSARY, SQUASH_SOURCE_BRANCH_COMMITS, REBASE_SOURCE_BRANCH_COMMITS

#### `editPullRequest`
Edit an existing pull request.

**Parameters:**
- `pullRequestReference` (required) - Reference of the pull request to edit
- `title` (optional) - Title of the pull request
- `description` (optional) - Description of the pull request
- `assignees` (optional) - Array of assignee user login names
- `addReviewers` (optional) - Array of reviewer user login names to add
- `removeReviewers` (optional) - Array of reviewer user login names to remove
- `mergeStrategy` (optional) - Merge strategy
- `autoMerge` (optional) - Whether to enable auto merge
- `autoMergeCommitMessage` (optional) - Preset commit message for auto merge

#### `processPullRequest`
Process a pull request (approve, merge, etc.).

**Parameters:**
- `pullRequestReference` (required) - Pull request reference
- `operation` (required) - One of: approve, requestChanges, merge, discard, reopen, deleteSourceBranch, restoreSourceBranch
- `comment` (optional) - Comment for the operation

#### `addPullRequestComment`
Add a comment to a pull request.

**Parameters:**
- `pullRequestReference` (required) - Pull request reference
- `commentContent` (required) - Content of the comment to add

#### `checkoutPullRequest`
Checkout specified pull request in current working directory.

**Parameters:**
- `pullRequestReference` (required) - Pull request reference

### Build Management Tools

#### `queryBuilds`
Query builds in current project.

**Parameters:**
- `query` (optional) - Query string using OneDev's build query syntax
- `project` (optional) - Project to query builds in. Leave empty for current project
- `count` (optional) - Number of builds to return (default: 25, max: 100)
- `offset` (optional) - Start position for the query (default: 0)

#### `getBuild`
Get detailed information about a specific build.

**Parameters:**
- `buildReference` (required) - Build reference in form `#<number>`, `<project>#<number>`, or `<project key>-<number>`

#### `getBuildLog`
Get build log for a specific build.

**Parameters:**
- `buildReference` (required) - Build reference

#### `getBuildFileContent`
Get content of specified file in a build.

**Parameters:**
- `buildReference` (required) - Build reference
- `filePath` (required) - Path of the file relative to repository root

#### `getFileChangesSincePreviousSuccessfulSimilarBuild`
Get file changes since previous successful build similar to specified build.

**Parameters:**
- `buildReference` (required) - Build reference

#### `runJob`
Run specified job against specified branch or tag in current project.

**Parameters:**
- `jobName` (required) - Name of the job to run
- `branch` (optional) - Branch to run the job against (either branch or tag, not both)
- `tag` (optional) - Tag to run the job against (either branch or tag, not both)
- `params` (optional) - Array of parameters in form key=value

#### `runLocalJob`
Run specified job against local changes in current working directory.

**Parameters:**
- `jobName` (required) - Name of the job to run
- `params` (optional) - Array of parameters in form key=value

### Build Spec Management Tools

#### `getBuildSpecSchema`
Get build spec schema to understand how to edit build spec (`.onedev-buildspec.yml`).

#### `checkBuildSpec`
Check build spec for validity and update it to latest version if needed.

### Project and System Tools

#### `getCurrentProject`
Get default OneDev project for operations.

#### `getCurrentRemote`
Get current OneDev remote for various operations.

#### `getWorkingDir`
Get working directory.

#### `setWorkingDir`
Set working directory.

**Parameters:**
- `workingDir` (required) - Absolute path in the file system

#### `getLoginName`
Returns login name of specified user or current user.

**Parameters:**
- `userName` (optional) - Name of the user. If not provided, returns current user's login name

#### `getUnixTimestamp`
Returns unix timestamp in milliseconds since epoch.

**Parameters:**
- `dateTimeDescription` (required) - Description of date/time to convert (e.g., "today", "next month", "2025-01-01")


## Available Prompts

TOD's MCP server provides prompts that guide AI assistants through complex workflows:

### `change-issue-state`
Change state of specified issue.

**Parameters:**
- `issueReference` (required) - Reference of the issue to change state, for instance #123, project#123, or projectkey-123
- `instruction` (required) - Instruction on what to do with the issue

**Description:**
This prompt guides the AI assistant through the process of changing an issue's state. The assistant will:
1. Parse the instruction to determine the desired state change
2. Call the `changeIssueState` tool with the appropriate parameters
3. Include any meaningful comment derived from the instruction
4. Follow any additional instructions returned by the tool call

### `edit-build-spec`
Create or edit OneDev build spec (.onedev-buildspec.yml).

**Parameters:**
- `instruction` (required) - Instruction to create or edit OneDev build spec

**Description:**
This prompt guides the AI assistant through the process of creating or editing a OneDev build specification file. The assistant will:
1. Get the current build spec schema to understand the structure
2. Read the existing build spec file if it exists
3. Apply the provided instruction to create or modify the build spec
4. Validate the resulting build spec
5. Save the changes to the file

### `investigate-build-problems`
Investigate problems of a build.

**Parameters:**
- `buildReference` (required) - Reference of the build to investigate problems, for instance #123, project#123, or projectkey-123
- `instruction` (optional) - Instruction to investigate problems of the build

**Description:**
This prompt guides the AI assistant through a systematic investigation of build problems. The assistant will:
1. Get the build details and status
2. Retrieve the build log to identify error messages
3. Examine the build spec file to understand the build configuration
4. Check for file changes since the previous successful build
5. Analyze the failure and provide recommendations

### `review-pull-request`
Review a pull request.

**Parameters:**
- `pullRequestReference` (required) - Reference of the pull request to review, for instance #123, project#123, or projectkey-123
- `sinceLastReview` (required) - Either true or false. If true, only changes since last review will be reviewed; otherwise all changes of the pull request will be reviewed
- `instruction` (optional) - Instruction to review the pull request

**Description:**
This prompt guides the AI assistant through a comprehensive pull request review process. The assistant will:
1. Get the pull request details including title and description
2. Retrieve file changes for review (either all changes or only since last review based on the parameter)
3. Examine file contents as needed to understand the changes
4. Check if the pull request is pending the current user's review
5. Provide an approval, request changes, or leave a comment based on the review findings

## Reference Formats

Many tools accept reference parameters in these formats:

- **Issue References**: `#123`, `myproject#123`, or `PROJ-123`
- **Pull Request References**: `#456`, `myproject#456`, or `PROJ-456`
- **Build References**: `#789`, `myproject#789`, or `PROJ-789`

