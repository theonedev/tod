package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage OneDev issues",
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

var issueShowCmd = &cobra.Command{
	Use:   "show <issue-reference>",
	Short: "Get information about a single issue",
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

var issueCommentsCmd = &cobra.Command{
	Use:   "comments <issue-reference>",
	Short: "Get comments for an issue",
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
	Use:   "create",
	Short: "Create a new issue",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		description, _ := cmd.Flags().GetString("description")
		project, _ := cmd.Flags().GetString("project")
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
		if _, hasTitle := payload["title"]; !hasTitle {
			return fmt.Errorf("--title (or --field title=...) is required")
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

var issueTransitionCmd = &cobra.Command{
	Use:   "transition <issue-reference>",
	Short: "Change the state of an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		state, _ := cmd.Flags().GetString("state")
		comment, _ := cmd.Flags().GetString("comment")
		fields, _ := cmd.Flags().GetStringArray("field")

		payload, err := collectFieldFlags(fields)
		if err != nil {
			return err
		}
		if state != "" {
			payload["state"] = state
		}
		if _, hasState := payload["state"]; !hasState {
			return fmt.Errorf("--state (or --field state=...) is required")
		}
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
	Use:   "link <source-issue> <target-issue>",
	Short: "Add <target-issue> as <link-name> of <source-issue>",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		linkName, _ := cmd.Flags().GetString("link-name")
		if linkName == "" {
			return fmt.Errorf("--link-name is required")
		}

		currentProject, err := currentProjectFor(cmd)
		if err != nil {
			return err
		}

		body, err := apiGetBytes("link-issues", url.Values{
			"currentProject":  {currentProject},
			"sourceReference": {args[0]},
			"targetReference": {args[1]},
			"linkName":        {linkName},
		})
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var issueCommentCmd = &cobra.Command{
	Use:   "comment <issue-reference>",
	Short: "Add a comment to an issue",
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

var issueLogWorkCmd = &cobra.Command{
	Use:   "log-work <issue-reference>",
	Short: "Log spent time against an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		hours, _ := cmd.Flags().GetInt("hours")
		comment, _ := cmd.Flags().GetString("comment")
		if hours <= 0 {
			return fmt.Errorf("--hours must be a positive integer")
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

	issueListCmd.Flags().String("project", "", "Project to query (defaults to the current project)")
	issueListCmd.Flags().String("query", "", "OneDev issue query string (run 'tod schema show query-issues' for valid keys)")
	issueListCmd.Flags().Int("offset", 0, "Starting offset")
	issueListCmd.Flags().Int("count", DefaultQueryCount, "Number of issues to return")

	issueCreateCmd.Flags().String("title", "", "Issue title (required)")
	issueCreateCmd.Flags().String("description", "", "Issue description")
	issueCreateCmd.Flags().String("project", "", "Project to create the issue in (defaults to the current project)")
	issueCreateCmd.Flags().StringArray("field", nil, "Additional field in form key=value (repeatable; JSON values are parsed; run 'tod schema show create-issue' for valid keys)")

	issueEditCmd.Flags().String("title", "", "New issue title")
	issueEditCmd.Flags().String("description", "", "New issue description")
	issueEditCmd.Flags().StringArray("field", nil, "Field to update in form key=value (repeatable; JSON values are parsed; run 'tod schema show edit-issue' for valid keys)")

	issueTransitionCmd.Flags().String("state", "", "Target state (required; run 'tod schema show change-issue-state' for valid states)")
	issueTransitionCmd.Flags().String("comment", "", "Optional comment for the transition")
	issueTransitionCmd.Flags().StringArray("field", nil, "Additional state-specific field in form key=value (repeatable; run 'tod schema show change-issue-state' for valid keys)")

	issueLinkCmd.Flags().String("link-name", "", "Link name as configured on the OneDev server (required; run 'tod schema show link-issues' for valid names)")

	issueCommentCmd.Flags().String("content", "", "Comment body (required)")

	issueLogWorkCmd.Flags().Int("hours", 0, "Hours spent (required, positive integer)")
	issueLogWorkCmd.Flags().String("comment", "", "Optional comment for the work log entry")

	issueCmd.AddCommand(
		issueListCmd,
		issueShowCmd,
		issueCommentsCmd,
		issueCreateCmd,
		issueEditCmd,
		issueTransitionCmd,
		issueLinkCmd,
		issueCommentCmd,
		issueLogWorkCmd,
	)
}
