package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Interact with OneDev pull requests",
}

var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "Query pull requests in the current (or a specified) project",
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
		body, err := queryEntities("query-pull-requests", project, currentProject, query, offset, count)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prGetCmd = &cobra.Command{
	Use:   "get <pr-reference>",
	Short: "Get detail information of a single pull request except comments and code comments",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := getEntityData("get-pull-request", args[0], currentProject)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prGetCommentsCmd = &cobra.Command{
	Use:   "get-comments <pr-reference>",
	Short: "Get comments of a pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := getEntityData("get-pull-request-comments", args[0], currentProject)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prGetCodeCommentsCmd = &cobra.Command{
	Use:   "get-code-comments <pr-reference>",
	Short: "Get code comments of a pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := getEntityData("get-pull-request-code-comments", args[0], currentProject)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prGetPatchCmd = &cobra.Command{
	Use:   "get-patch <pr-reference>",
	Short: "Get pull request patch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		forCodeReview, _ := cmd.Flags().GetBool("for-code-review")

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		info, err := getPullRequestPatchInfo(args[0], currentProject)
		if err != nil {
			return err
		}

		projectId, _ := info["projectId"].(string)
		oldCommit, _ := info["oldCommitHash"].(string)
		newCommit, _ := info["newCommitHash"].(string)

		patchURL := config.ServerUrl + "/~downloads/projects/" + projectId + "/patch?" + url.Values{
			"old-commit":      {oldCommit},
			"new-commit":      {newCommit},
			"for-code-review": {fmt.Sprintf("%t", forCodeReview)},
		}.Encode()

		body, err := apiGetAbsolute(patchURL)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prGetFileContentCmd = &cobra.Command{
	Use:   "get-file-content <pr-reference> <path>",
	Short: "Get the content of a file at a specific revision of a pull request",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[1]
		oldRevision, _ := cmd.Flags().GetBool("old-revision")

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		pr, err := getPullRequestDetail(args[0], currentProject)
		if err != nil {
			return err
		}

		var commitHash string
		if oldRevision {
			info, err := getPullRequestPatchInfo(args[0], currentProject)
			if err != nil {
				return err
			}
			commitHash, _ = info["oldCommitHash"].(string)
		} else {
			commitHash, _ = pr["headCommitHash"].(string)
		}
		targetProject, _ := pr["targetProject"].(string)

		body, err := apiGetAbsolute(fmt.Sprintf("%s/%s/~raw/%s/%s", config.ServerUrl, targetProject, commitHash, path))
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := args[0]
		sourceBranch, _ := cmd.Flags().GetString("source-branch")
		targetBranch, _ := cmd.Flags().GetString("target-branch")
		description, _ := cmd.Flags().GetString("description")
		sourceProject, _ := cmd.Flags().GetString("source-project")
		targetProject, _ := cmd.Flags().GetString("target-project")
		assignees, _ := cmd.Flags().GetStringArray("assignee")
		reviewers, _ := cmd.Flags().GetStringArray("reviewer")
		labels, _ := cmd.Flags().GetStringArray("label")
		mergeStrategy, _ := cmd.Flags().GetString("merge-strategy")

		payload := map[string]interface{}{}
		payload["title"] = title

		if sourceBranch == "" {
			branch, err := currentBranch(workingDirOf(cmd))
			if err != nil {
				return err
			}
			if branch == "" {
				return fmt.Errorf("--source-branch is required: could not detect current branch (detached HEAD)")
			}
			sourceBranch = branch
		}
		payload["sourceBranch"] = sourceBranch

		if targetBranch != "" {
			payload["targetBranch"] = targetBranch
		}
		if description != "" {
			payload["description"] = description
		}
		if mergeStrategy != "" {
			payload["mergeStrategy"] = mergeStrategy
		}
		if len(assignees) > 0 {
			payload["assignees"] = assignees
		}
		if len(reviewers) > 0 {
			payload["reviewers"] = reviewers
		}
		if len(labels) > 0 {
			payload["labels"] = labels
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := postJSON("create-pull-request", url.Values{
			"sourceProject":  {sourceProject},
			"targetProject":  {targetProject},
			"currentProject": {currentProject},
		}, payload)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prEditCmd = &cobra.Command{
	Use:   "edit <pr-reference>",
	Short: "Edit a pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		assignees, _ := cmd.Flags().GetStringArray("assignee")
		addReviewers, _ := cmd.Flags().GetStringArray("add-reviewer")
		removeReviewers, _ := cmd.Flags().GetStringArray("remove-reviewer")
		mergeStrategy, _ := cmd.Flags().GetString("merge-strategy")
		labels, _ := cmd.Flags().GetStringArray("label")

		payload := map[string]interface{}{}

		if title != "" {
			payload["title"] = title
		}
		if description != "" {
			payload["description"] = description
		}
		if mergeStrategy != "" {
			payload["mergeStrategy"] = mergeStrategy
		}
		if len(assignees) > 0 {
			payload["assignees"] = assignees
		}
		if len(addReviewers) > 0 {
			payload["addReviewers"] = addReviewers
		}
		if len(removeReviewers) > 0 {
			payload["removeReviewers"] = removeReviewers
		}
		if len(labels) > 0 {
			payload["labels"] = labels
		}
		if cmd.Flags().Changed("auto-merge") {
			autoMerge, _ := cmd.Flags().GetBool("auto-merge")
			payload["autoMerge"] = autoMerge
		}
		if cmd.Flags().Changed("auto-merge-commit-message") {
			autoMergeCommitMessage, _ := cmd.Flags().GetString("auto-merge-commit-message")
			payload["autoMergeCommitMessage"] = autoMergeCommitMessage
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := postJSON("edit-pull-request", url.Values{
			"currentProject": {currentProject},
			"reference":      {args[0]},
		}, payload)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

// runPullRequestOperation posts the (optional) text body to the given tod
// endpoint for the supplied pull request reference. It is the shared
// implementation used by `tod pr approve`, `request-changes`, `merge`,
// and `discard`.
func runPullRequestOperation(cmd *cobra.Command, endpointSuffix, reference, body string) error {
	currentProject, err := currentProjectFor(cmd)
	if err != nil {
		return err
	}
	respBody, err := postText(endpointSuffix, url.Values{
		"currentProject": {currentProject},
		"reference":      {reference},
	}, body)
	if err != nil {
		return err
	}
	emit(respBody)
	return nil
}

var prApproveCmd = &cobra.Command{
	Use:   "approve <pr-reference>",
	Short: "Approve a pull request as a pending reviewer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		comment, _ := cmd.Flags().GetString("comment")
		return runPullRequestOperation(cmd, "approve-pull-request", args[0], comment)
	},
}

var prRequestChangesCmd = &cobra.Command{
	Use:   "request-changes <pr-reference>",
	Short: "Request changes on a pull request as a pending reviewer",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		comment, _ := cmd.Flags().GetString("comment")
		return runPullRequestOperation(cmd, "request-changes-on-pull-request", args[0], comment)
	},
}

var prMergeCmd = &cobra.Command{
	Use:   "merge <pr-reference>",
	Short: "Merge a pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		commitMessage, _ := cmd.Flags().GetString("commit-message")
		return runPullRequestOperation(cmd, "merge-pull-request", args[0], commitMessage)
	},
}

var prDiscardCmd = &cobra.Command{
	Use:   "discard <pr-reference>",
	Short: "Discard (close without merging) a pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		comment, _ := cmd.Flags().GetString("comment")
		return runPullRequestOperation(cmd, "discard-pull-request", args[0], comment)
	},
}

var prAddCommentCmd = &cobra.Command{
	Use:   "add-comment <pr-reference> <content>",
	Short: "Add a comment to a pull request",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		content := args[1]

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := postText("add-pull-request-comment", url.Values{
			"currentProject": {currentProject},
			"reference":      {args[0]},
		}, content)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prAddCodeCommentCmd = &cobra.Command{
	Use:   "add-code-comment <pr-reference> <content>",
	Short: "Add a code comment to pull request patch",
	Long: `Add a code comment to a line range of a file in the pull request patch. 
The line range must be visible in right side (added lines or equal lines inside context) of the pull request patch.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		content := args[1]
		filePath, _ := cmd.Flags().GetString("file")
		fromLine, _ := cmd.Flags().GetInt("from-line")
		toLine, _ := cmd.Flags().GetInt("to-line")

		if filePath == "" {
			return fmt.Errorf("--file is required")
		}
		if fromLine <= 0 {
			return fmt.Errorf("--from-line must be greater than 0")
		}
		if !cmd.Flags().Changed("to-line") {
			toLine = fromLine
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := postText("add-pull-request-code-comment", url.Values{
			"currentProject": {currentProject},
			"reference":      {args[0]},
			"filePath":       {filePath},
			"fromLineNumber": {fmt.Sprintf("%d", fromLine)},
			"toLineNumber":   {fmt.Sprintf("%d", toLine)},
		}, content)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prGetQueryDescriptionCmd = &cobra.Command{
	Use:   "get-query-description",
	Short: "Get the OneDev pull request query DSL description (DSL for `--query` of `pr list`)",
	Long: `Get the OneDev pull request query DSL description so you know what syntax
'tod pr list --query' accepts (operators, ordering, fuzzy matching, the
set of supported field/criteria keys, etc.).

The description is fetched from the OneDev server endpoint
/~api/tod/get-pull-request-query-description, which returns the canonical
pull request query syntax reference for this server.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-pull-request-query-description", nil)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prCheckoutCmd = &cobra.Command{
	Use:   "checkout <pr-reference>",
	Short: "Checkout a pull request into the working directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd := workingDirOf(cmd)
		logger := cliLogger("[checkout] ")
		if err := checkoutPullRequest(wd, args[0], logger); err != nil {
			return fmt.Errorf("failed to checkout pull request: %v", err)
		}
		logger.Printf("Checked out pull request %s successfully", args[0])
		return nil
	},
}

func initPullRequestCommands() {
	prCmd.PersistentFlags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")

	prListCmd.Flags().String("project", "", "Project to query pull requests in. Leave empty to query in current project")
	prListCmd.Flags().String("query", "", "OneDev pull request query string (run 'tod pr get-query-description' for the supported query DSL)")
	prListCmd.Flags().Int("offset", 0, "start position for the query (optional, defaults to 0)")
	prListCmd.Flags().Int("count", DefaultQueryCount, fmt.Sprintf("number of pull requests to return (optional, defaults to %d, max %d)", DefaultQueryCount, MaxQueryCount))

	prGetPatchCmd.Flags().Bool("for-code-review", false, "If set, return only changes relevant for code review")

	prGetFileContentCmd.Flags().Bool("old-revision", false, "If set, return the file content before pull request change")

	prCreateCmd.Flags().String("source-branch", "", "Source branch (defaults to the current git branch)")
	prCreateCmd.Flags().String("target-branch", "", "Target branch (defaults to the project's default branch)")
	prCreateCmd.Flags().String("description", "", "Pull request description")
	prCreateCmd.Flags().String("source-project", "", "Source project (defaults to current project)")
	prCreateCmd.Flags().String("target-project", "", "Target project (defaults to original project for forks)")
	prCreateCmd.Flags().StringArray("assignee", nil, "Assignee login name (repeatable)")
	prCreateCmd.Flags().StringArray("reviewer", nil, "Reviewer login name (repeatable)")
	prCreateCmd.Flags().StringArray("label", nil, "Label name (repeatable; run 'tod get-valid-labels' for accepted values)")
	prCreateCmd.Flags().String("merge-strategy", "", "CREATE_MERGE_COMMIT | CREATE_MERGE_COMMIT_IF_NECESSARY | SQUASH_SOURCE_BRANCH_COMMITS | REBASE_SOURCE_BRANCH_COMMITS")

	prEditCmd.Flags().String("title", "", "New pull request title")
	prEditCmd.Flags().String("description", "", "New pull request description")
	prEditCmd.Flags().StringArray("assignee", nil, "Assignee login name (repeatable)")
	prEditCmd.Flags().StringArray("add-reviewer", nil, "Reviewer login name to add (repeatable)")
	prEditCmd.Flags().StringArray("remove-reviewer", nil, "Reviewer login name to remove (repeatable)")
	prEditCmd.Flags().StringArray("label", nil, "Label name (repeatable; run 'tod get-valid-labels' for accepted values)")
	prEditCmd.Flags().String("merge-strategy", "", "Merge strategy")
	prEditCmd.Flags().Bool("auto-merge", false, "Whether or not to enable auto-merge")
	prEditCmd.Flags().String("auto-merge-commit-message", "", "Preset commit message for auto merge")

	prApproveCmd.Flags().String("comment", "", "Optional review comment")
	prRequestChangesCmd.Flags().String("comment", "", "Optional review comment")
	prMergeCmd.Flags().String("commit-message", "", "Optional merge commit message (must satisfy server-side validation if provided)")
	prDiscardCmd.Flags().String("comment", "", "Optional comment explaining the discard")

	prAddCodeCommentCmd.Flags().String("file", "", "Path of the file to comment on (required)")
	prAddCodeCommentCmd.Flags().Int("from-line", 0, "Start line number of the comment range, 1-based (required)")
	prAddCodeCommentCmd.Flags().Int("to-line", 0, "End line number of the comment range, 1-based (defaults to --from-line)")

	prCmd.AddCommand(
		prListCmd,
		prGetCmd,
		prGetCommentsCmd,
		prGetCodeCommentsCmd,
		prGetPatchCmd,
		prGetFileContentCmd,
		prCreateCmd,
		prEditCmd,
		prApproveCmd,
		prRequestChangesCmd,
		prMergeCmd,
		prDiscardCmd,
		prAddCommentCmd,
		prAddCodeCommentCmd,
		prCheckoutCmd,
		prGetQueryDescriptionCmd,
	)
}
