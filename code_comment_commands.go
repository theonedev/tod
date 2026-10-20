package main

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

var codeCommentCmd = &cobra.Command{
	Use:   "code-comment",
	Short: "Interact with OneDev code comments (use 'tod pr get-code-comments <pr-reference>' to discover comment IDs)",
}

var codeCommentAddReplyCmd = &cobra.Command{
	Use:   "add-reply <comment-id> [content]",
	Short: "Add a Markdown reply to a code comment",
	Args:  markdownContentArgs("file"),
	RunE: func(cmd *cobra.Command, args []string) error {
		commentId, err := parseCommentId(args[0])
		if err != nil {
			return err
		}
		content, err := readMarkdownContent(cmd, args, "file")
		if err != nil {
			return err
		}
		body, err := postText("add-code-comment-reply", url.Values{
			"commentId": {commentId},
		}, content)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var codeCommentResolveCmd = &cobra.Command{
	Use:   "resolve <comment-id>",
	Short: "Mark a unresolved code comment as resolved",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		commentId, err := parseCommentId(args[0])
		if err != nil {
			return err
		}
		note, err := readMarkdownFlag(cmd, "note")
		if err != nil {
			return err
		}
		body, err := postText("resolve-code-comment", url.Values{
			"commentId": {commentId},
		}, note)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

var codeCommentUnresolveCmd = &cobra.Command{
	Use:   "unresolve <comment-id>",
	Short: "Mark a resolved code comment as unresolved",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		commentId, err := parseCommentId(args[0])
		if err != nil {
			return err
		}
		note, err := readMarkdownFlag(cmd, "note")
		if err != nil {
			return err
		}
		body, err := postText("unresolve-code-comment", url.Values{
			"commentId": {commentId},
		}, note)
		if err != nil {
			return err
		}
		emit(body)
		return nil
	},
}

func parseCommentId(raw string) (string, error) {
	if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
		return "", fmt.Errorf("<comment-id> must be a positive integer (run 'tod pr get-code-comments <pr-reference>' to list IDs)")
	}
	return raw, nil
}

func initCodeCommentCommands() {
	codeCommentAddReplyCmd.Flags().String("file", "", "Read the Markdown reply from a file (use - for stdin) instead of <content>")
	codeCommentResolveCmd.Flags().String("note", "", "Optional Markdown note explaining the resolution")
	codeCommentResolveCmd.Flags().String("file", "", "Read the Markdown note from a file (use - for stdin); cannot be combined with --note")
	codeCommentUnresolveCmd.Flags().String("note", "", "Optional Markdown note explaining why the comment is unresolved")
	codeCommentUnresolveCmd.Flags().String("file", "", "Read the Markdown note from a file (use - for stdin); cannot be combined with --note")

	codeCommentCmd.AddCommand(
		codeCommentAddReplyCmd,
		codeCommentResolveCmd,
		codeCommentUnresolveCmd,
	)
}
