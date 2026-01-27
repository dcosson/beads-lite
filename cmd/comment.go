// Package cmd implements CLI commands for beads.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"beads2/storage"
)

// CommentAddOptions configures the comment add command.
type CommentAddOptions struct {
	IssueID string
	Body    string
	Author  string
	JSON    bool
	Stdin   io.Reader // for testing; defaults to os.Stdin
}

// CommentAddResult is returned when JSON output is enabled.
type CommentAddResult struct {
	CommentID string `json:"comment_id"`
	IssueID   string `json:"issue_id"`
}

// CommentAdd adds a comment to an issue.
func CommentAdd(ctx context.Context, store storage.Storage, opts CommentAddOptions) error {
	if opts.IssueID == "" {
		return fmt.Errorf("issue ID is required")
	}

	body := opts.Body
	if body == "-" {
		stdin := opts.Stdin
		if stdin == nil {
			stdin = os.Stdin
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("reading from stdin: %w", err)
		}
		body = string(data)
	}

	if body == "" {
		return fmt.Errorf("comment body is required")
	}

	author := opts.Author
	if author == "" {
		author = os.Getenv("USER")
	}
	if author == "" {
		author = "unknown"
	}

	comment := &storage.Comment{
		Author: author,
		Body:   body,
	}

	if err := store.AddComment(ctx, opts.IssueID, comment); err != nil {
		return err
	}

	if opts.JSON {
		result := CommentAddResult{
			CommentID: comment.ID,
			IssueID:   opts.IssueID,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Println(comment.ID)
	return nil
}

// CommentListOptions configures the comment list command.
type CommentListOptions struct {
	IssueID string
	JSON    bool
	Reverse bool // if true, show oldest first (chronological); default is newest first
}

// CommentListResult is returned when JSON output is enabled.
type CommentListResult struct {
	Comments []CommentOutput `json:"comments"`
}

// CommentOutput represents a comment in output.
type CommentOutput struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// CommentList lists comments on an issue.
func CommentList(ctx context.Context, store storage.Storage, opts CommentListOptions) error {
	if opts.IssueID == "" {
		return fmt.Errorf("issue ID is required")
	}

	issue, err := store.Get(ctx, opts.IssueID)
	if err != nil {
		return err
	}

	comments := make([]CommentOutput, len(issue.Comments))
	for i, c := range issue.Comments {
		comments[i] = CommentOutput{
			ID:        c.ID,
			Author:    c.Author,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
		}
	}

	// Default is newest first (reverse order of storage)
	if !opts.Reverse {
		for i, j := 0, len(comments)-1; i < j; i, j = i+1, j-1 {
			comments[i], comments[j] = comments[j], comments[i]
		}
	}

	if opts.JSON {
		result := CommentListResult{Comments: comments}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if len(comments) == 0 {
		fmt.Println("No comments")
		return nil
	}

	for i, c := range comments {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s  %s  %s\n", c.Author, c.CreatedAt.Format(time.RFC3339), c.ID)
		fmt.Println(c.Body)
	}

	return nil
}
