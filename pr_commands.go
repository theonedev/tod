package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage OneDev pull requests",
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

var prShowCmd = &cobra.Command{
	Use:   "show <pr-reference>",
	Short: "Get information about a single pull request",
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

var prCommentsCmd = &cobra.Command{
	Use:   "comments <pr-reference>",
	Short: "Get comments on a pull request",
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

var prCodeCommentsCmd = &cobra.Command{
	Use:   "code-comments <pr-reference>",
	Short: "Get code comments on a pull request",
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

var prFileChangesCmd = &cobra.Command{
	Use:   "file-changes <pr-reference>",
	Short: "Get pull request file changes in patch format",
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

var prFileContentCmd = &cobra.Command{
	Use:   "file-content <pr-reference>",
	Short: "Get the content of a file at a specific revision of a pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, _ := cmd.Flags().GetString("path")
		oldRevision, _ := cmd.Flags().GetBool("old-revision")
		if path == "" {
			return fmt.Errorf("--path is required")
		}

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
	Use:   "create",
	Short: "Create a new pull request",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceBranch, _ := cmd.Flags().GetString("source-branch")
		targetBranch, _ := cmd.Flags().GetString("target-branch")
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		sourceProject, _ := cmd.Flags().GetString("source-project")
		targetProject, _ := cmd.Flags().GetString("target-project")
		assignees, _ := cmd.Flags().GetStringArray("assignee")
		reviewers, _ := cmd.Flags().GetStringArray("reviewer")
		mergeStrategy, _ := cmd.Flags().GetString("merge-strategy")
		fields, _ := cmd.Flags().GetStringArray("field")

		payload, err := collectFieldFlags(fields)
		if err != nil {
			return err
		}
		if sourceBranch == "" {
			if _, ok := payload["sourceBranch"]; !ok {
				return fmt.Errorf("--source-branch (or --field sourceBranch=...) is required")
			}
		} else {
			payload["sourceBranch"] = sourceBranch
		}
		if targetBranch != "" {
			payload["targetBranch"] = targetBranch
		}
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
			payload["assignees"] = stringSlice(assignees)
		}
		if len(reviewers) > 0 {
			payload["reviewers"] = stringSlice(reviewers)
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
		fields, _ := cmd.Flags().GetStringArray("field")

		payload, err := collectFieldFlags(fields)
		if err != nil {
			return err
		}
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
			payload["assignees"] = stringSlice(assignees)
		}
		if len(addReviewers) > 0 {
			payload["addReviewers"] = stringSlice(addReviewers)
		}
		if len(removeReviewers) > 0 {
			payload["removeReviewers"] = stringSlice(removeReviewers)
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

var prProcessCmd = &cobra.Command{
	Use:   "process <pr-reference>",
	Short: "Run an operation on a pull request (approve, requestChanges, merge, discard, reopen, deleteSourceBranch, restoreSourceBranch)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		operation, _ := cmd.Flags().GetString("operation")
		comment, _ := cmd.Flags().GetString("comment")
		if operation == "" {
			return fmt.Errorf("--operation is required")
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := postText("process-pull-request", url.Values{
			"currentProject": {currentProject},
			"reference":      {args[0]},
			"operation":      {operation},
		}, comment)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var prCommentCmd = &cobra.Command{
	Use:   "comment <pr-reference>",
	Short: "Add a comment to a pull request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content, _ := cmd.Flags().GetString("content")
		if content == "" {
			return fmt.Errorf("--content is required")
		}

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

// stringSlice converts a []string to []interface{} for inclusion in a JSON
// payload.
func stringSlice(in []string) []interface{} {
	out := make([]interface{}, 0, len(in))
	for _, s := range in {
		out = append(out, s)
	}
	return out
}

func initPullRequestCommands() {
	prCmd.PersistentFlags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")

	prListCmd.Flags().String("project", "", "Project to query (defaults to the current project)")
	prListCmd.Flags().String("query", "", "OneDev pull request query string (run 'tod schema show query-pull-requests' for valid keys)")
	prListCmd.Flags().Int("offset", 0, "Starting offset")
	prListCmd.Flags().Int("count", DefaultQueryCount, "Number of pull requests to return")

	prFileChangesCmd.Flags().Bool("for-code-review", false, "If set, return only changes relevant for code review")

	prFileContentCmd.Flags().String("path", "", "Path of the file relative to repository root (required)")
	prFileContentCmd.Flags().Bool("old-revision", false, "If set, return the file as it existed before the pull request")

	prCreateCmd.Flags().String("source-branch", "", "Source branch (required)")
	prCreateCmd.Flags().String("target-branch", "", "Target branch (defaults to the project's default branch)")
	prCreateCmd.Flags().String("title", "", "Pull request title")
	prCreateCmd.Flags().String("description", "", "Pull request description")
	prCreateCmd.Flags().String("source-project", "", "Source project (defaults to current project)")
	prCreateCmd.Flags().String("target-project", "", "Target project (defaults to original project for forks)")
	prCreateCmd.Flags().StringArray("assignee", nil, "Assignee login name (repeatable)")
	prCreateCmd.Flags().StringArray("reviewer", nil, "Reviewer login name (repeatable)")
	prCreateCmd.Flags().String("merge-strategy", "", "CREATE_MERGE_COMMIT | CREATE_MERGE_COMMIT_IF_NECESSARY | SQUASH_SOURCE_BRANCH_COMMITS | REBASE_SOURCE_BRANCH_COMMITS")
	prCreateCmd.Flags().StringArray("field", nil, "Additional field in form key=value (repeatable; run 'tod schema show create-pull-request' for valid keys)")

	prEditCmd.Flags().String("title", "", "New pull request title")
	prEditCmd.Flags().String("description", "", "New pull request description")
	prEditCmd.Flags().StringArray("assignee", nil, "Assignee login name (repeatable)")
	prEditCmd.Flags().StringArray("add-reviewer", nil, "Reviewer login name to add (repeatable)")
	prEditCmd.Flags().StringArray("remove-reviewer", nil, "Reviewer login name to remove (repeatable)")
	prEditCmd.Flags().String("merge-strategy", "", "Merge strategy")
	prEditCmd.Flags().StringArray("field", nil, "Additional field in form key=value (repeatable; run 'tod schema show edit-pull-request' for valid keys)")

	prProcessCmd.Flags().String("operation", "", "Operation to run (required)")
	prProcessCmd.Flags().String("comment", "", "Comment accompanying the operation")

	prCommentCmd.Flags().String("content", "", "Comment body (required)")

	prCmd.AddCommand(
		prListCmd,
		prShowCmd,
		prCommentsCmd,
		prCodeCommentsCmd,
		prFileChangesCmd,
		prFileContentCmd,
		prCreateCmd,
		prEditCmd,
		prProcessCmd,
		prCommentCmd,
		prCheckoutCmd,
	)
}
