package main

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/spf13/cobra"
)

// issueBranchPattern matches a branch ending with
// '[<optional-prefix>/]issue-<number>[-<optional-suffix>]'. The capture group
// is the issue number.
var issueBranchPattern = regexp.MustCompile(`(?:^|/)issue-(\d+)(?:-[^/]*)?$`)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Interact with OneDev issues",
}

var issueListCmd = &cobra.Command{
	Use:   "list",
	Short: "Query issues in the current (or a specified) project",
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
		body, err := queryEntities("query-issues", project, currentProject, query, offset, count)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueGetCmd = &cobra.Command{
	Use:   "get <issue-reference>",
	Short: "Get detail information of a single issue except comments",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := getEntityData("get-issue", args[0], currentProject)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueGetCommentsCmd = &cobra.Command{
	Use:   "get-comments <issue-reference>",
	Short: "Get comments of an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := getEntityData("get-issue-comments", args[0], currentProject)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		description, _ := cmd.Flags().GetString("description")
		project, _ := cmd.Flags().GetString("project")
		fields, _ := cmd.Flags().GetStringArray("field")
		iterations, _ := cmd.Flags().GetStringArray("iteration")
		ownEstimatedTime, _ := cmd.Flags().GetFloat64("own-estimated-time")
		confidential, _ := cmd.Flags().GetBool("confidential")

		payload, err := collectFieldFlags(fields)
		if err != nil {
			return err
		}
		payload["title"] = args[0]
		if description != "" {
			payload["description"] = description
		}
		if len(iterations) > 0 {
			payload["iterations"] = iterations
		}
		if cmd.Flags().Changed("own-estimated-time") {
			payload["ownEstimatedTime"] = ownEstimatedTime
		}
		if cmd.Flags().Changed("confidential") {
			payload["confidential"] = confidential
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := postJSON("create-issue", url.Values{
			"project":        {project},
			"currentProject": {currentProject},
		}, payload)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueEditCmd = &cobra.Command{
	Use:   "edit <issue-reference>",
	Short: "Edit an existing issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		fields, _ := cmd.Flags().GetStringArray("field")
		iterations, _ := cmd.Flags().GetStringArray("iteration")
		ownEstimatedTime, _ := cmd.Flags().GetFloat64("own-estimated-time")
		confidential, _ := cmd.Flags().GetBool("confidential")

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
		if len(iterations) > 0 {
			payload["iterations"] = iterations
		}
		if cmd.Flags().Changed("own-estimated-time") {
			payload["ownEstimatedTime"] = ownEstimatedTime
		}
		if cmd.Flags().Changed("confidential") {
			payload["confidential"] = confidential
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := postJSON("edit-issue", url.Values{
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

var issueChangeStateCmd = &cobra.Command{
	Use:   "change-state <issue-reference> <state>",
	Short: "Change the state of an issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		comment, _ := cmd.Flags().GetString("comment")
		fields, _ := cmd.Flags().GetStringArray("field")

		payload, err := collectFieldFlags(fields)
		if err != nil {
			return err
		}
		payload["state"] = args[1]
		if comment != "" {
			payload["comment"] = comment
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := postJSON("change-issue-state", url.Values{
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

var issueLinkCmd = &cobra.Command{
	Use:   "link <source-issue-reference> <link-name> <target-issue-reference>",
	Short: "Add <target-issue-reference> as <link-name> of <source-issue-reference>",
	Long: `Add <target-issue-reference> as <link-name> of <source-issue-reference>.

Run 'tod issue get-valid-links' for valid link names.`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := apiGetBytes("link-issues", url.Values{
			"currentProject":  {currentProject},
			"sourceReference": {args[0]},
			"linkName":        {args[1]},
			"targetReference": {args[2]},
		})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueAddCommentCmd = &cobra.Command{
	Use:   "add-comment <issue-reference> <content>",
	Short: "Add a comment to an issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		content := args[1]

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}
		body, err := postText("add-issue-comment", url.Values{
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

var issueGetQueryDescriptionCmd = &cobra.Command{
	Use:   "get-query-description",
	Short: "Get the OneDev issue query DSL description (DSL for `--query` of `issue list`)",
	Long: `Get the OneDev issue query DSL description so you know what syntax
'tod issue list --query' accepts (operators, ordering, fuzzy matching, the
set of supported field/criteria keys, etc.).

The description is fetched from the OneDev server endpoint
/~api/tod/get-issue-query-description, which returns the canonical issue
query syntax reference for this server.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-issue-query-description", nil)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueGetValidFieldsCmd = &cobra.Command{
	Use:   "get-valid-fields",
	Short: "Get valid issue fields and their allowed values",
	Long: `Get valid issue fields and their allowed values for
this OneDev server. Use this to discover which field names and values are
accepted by --field when running 'tod issue create', 'tod issue edit', or
'tod issue change-state'.

The description is fetched from the OneDev server endpoint
/~api/tod/get-valid-issue-fields.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-valid-issue-fields", nil)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueGetValidLinksCmd = &cobra.Command{
	Use:   "get-valid-links",
	Short: "Get valid issue link names",
	Long: `Get valid issue link names for this OneDev server. Use this to
discover which link names are accepted as the <link-name> argument when
running 'tod issue link'.

The description is fetched from the OneDev server endpoint
/~api/tod/get-valid-issue-links.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := apiGetBytes("get-valid-issue-links", nil)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueCurrentReferenceCmd = &cobra.Command{
	Use:           "current-reference",
	Short:         "Print the issue number inferred from the current branch",
	SilenceErrors: true,
	Long: `Print the issue number inferred from the current git branch in the
working directory.

A current issue number exists when the current branch matches a pattern of
the form '[<optional-prefix>/]issue-<number>[-<optional-suffix>]'. In that
case the number is printed. If no issue number can be inferred, the command
prints an error and exits non-zero.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		wd := workingDirOf(cmd)
		branch, err := currentBranch(wd)
		if err != nil {
			return currentReferenceError(cmd, err)
		}
		if branch == "" {
			return currentReferenceError(cmd, fmt.Errorf("could not detect current branch (detached HEAD)"))
		}
		matches := issueBranchPattern.FindStringSubmatch(branch)
		if len(matches) < 2 {
			return currentReferenceError(cmd, fmt.Errorf("could not infer issue number from current branch %q", branch))
		}
		fmt.Println(matches[1])
		return nil
	},
}

var issueCheckoutCmd = &cobra.Command{
	Use:   "checkout <issue-reference>",
	Short: "Checkout an issue branch into the working directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wd := workingDirOf(cmd)
		logger := cliLogger("[checkout] ")
		if err := checkoutIssue(wd, args[0], logger); err != nil {
			return fmt.Errorf("failed to checkout issue: %v", err)
		}
		logger.Printf("Checked out issue %s successfully", args[0])
		return nil
	},
}

var issueLogWorkCmd = &cobra.Command{
	Use:   "log-work <issue-reference> <hours>",
	Short: "Log spent time against an issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		comment, _ := cmd.Flags().GetString("comment")

		var hours int
		if _, err := fmt.Sscanf(args[1], "%d", &hours); err != nil || hours <= 0 {
			return fmt.Errorf("<hours> must be a positive integer")
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := postText("log-work", url.Values{
			"currentProject": {currentProject},
			"reference":      {args[0]},
			"spentHours":     {fmt.Sprintf("%d", hours)},
		}, comment)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

func initIssueCommands() {
	issueCmd.PersistentFlags().String("working-dir", "", "Working directory used to infer the OneDev project (defaults to current directory)")

	issueListCmd.Flags().String("project", "", "Project to query issues in. Leave empty to query in current project")
	issueListCmd.Flags().String("query", "", "OneDev issue query string (run 'tod issue get-query-description' for the supported query DSL)")
	issueListCmd.Flags().Int("offset", 0, "start position for the query (optional, defaults to 0)")
	issueListCmd.Flags().Int("count", DefaultQueryCount, fmt.Sprintf("number of issues to return (optional, defaults to %d, max %d)", DefaultQueryCount, MaxQueryCount))

	issueCreateCmd.Flags().String("description", "", "Issue description")
	issueCreateCmd.Flags().String("project", "", "Project to create issue in. Leave empty to create issue in current project")
	issueCreateCmd.Flags().StringArray("field", nil, "Additional field in form key=value (repeatable; JSON values are parsed; run 'tod issue get-valid-fields' for valid field names and values)")
	issueCreateCmd.Flags().StringArray("iteration", nil, "Iteration name to assign (repeatable)")
	issueCreateCmd.Flags().Float64("own-estimated-time", 0, "Estimated time in hours for this issue only (not including linked issues)")
	issueCreateCmd.Flags().Bool("confidential", false, "whether the issue is confidential")

	issueEditCmd.Flags().String("title", "", "New issue title")
	issueEditCmd.Flags().String("description", "", "New issue description")
	issueEditCmd.Flags().StringArray("field", nil, "Field to update in form key=value (repeatable; JSON values are parsed; run 'tod issue get-valid-fields' for valid field names and values)")
	issueEditCmd.Flags().StringArray("iteration", nil, "Iteration name to assign (repeatable)")
	issueEditCmd.Flags().Float64("own-estimated-time", 0, "Estimated time in hours for this issue only (not including linked issues)")
	issueEditCmd.Flags().Bool("confidential", false, "Whether the issue is confidential")

	issueChangeStateCmd.Flags().String("comment", "", "Optional comment for the state change")
	issueChangeStateCmd.Flags().StringArray("field", nil, "Additional state-specific field in form key=value (repeatable; run 'tod issue get-valid-fields' for valid field names and values)")

	issueLogWorkCmd.Flags().String("comment", "", "Optional comment for the work log entry")

	issueCmd.AddCommand(
		issueListCmd,
		issueGetCmd,
		issueGetCommentsCmd,
		issueCreateCmd,
		issueEditCmd,
		issueChangeStateCmd,
		issueLinkCmd,
		issueAddCommentCmd,
		issueLogWorkCmd,
		issueCheckoutCmd,
		issueCurrentReferenceCmd,
		issueGetQueryDescriptionCmd,
		issueGetValidFieldsCmd,
		issueGetValidLinksCmd,
	)
}
