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
	Use:   "add-reply <comment-id> <content>",
	Short: "Add a reply to a code comment",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		commentId, err := parseCommentId(args[0])
		if err != nil {
			return err
		}
		body, err := postText("add-code-comment-reply", url.Values{
			"commentId": {commentId},
		}, args[1])
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
		note, _ := cmd.Flags().GetString("note")
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
		note, _ := cmd.Flags().GetString("note")
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
	codeCommentResolveCmd.Flags().String("note", "", "Optional note explaining the resolution")
	codeCommentUnresolveCmd.Flags().String("note", "", "Optional note explaining why the comment is unresolved")

	codeCommentCmd.AddCommand(
		codeCommentAddReplyCmd,
		codeCommentResolveCmd,
		codeCommentUnresolveCmd,
	)
}
