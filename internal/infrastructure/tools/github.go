package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"nano-code-go/internal/domain"
)

func CreatePullRequest(workspaceRoot string, execTool domain.Tool) domain.Tool {
	return domain.Tool{
		Name:          "createPullRequest",
		Description:   "Create a PR using GitHub CLI. If an existing PR is found, update it.",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"title": map[string]any{"type": "string", "description": "PR title"},
				"body":  map[string]any{"type": "string", "description": "PR body"},
				"head":  map[string]any{"type": "string", "description": "Source branch name"},
				"base":  map[string]any{"type": "string", "description": "Target branch name"},
			},
			Required: []string{"title", "body", "head", "base"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			title, err := stringArg(args, "title")
			if err != nil {
				return "", err
			}
			body, err := stringArg(args, "body")
			if err != nil {
				return "", err
			}
			head, err := stringArg(args, "head")
			if err != nil {
				return "", err
			}
			base, err := stringArg(args, "base")
			if err != nil {
				return "", err
			}
			return createPullRequestExecute(ctx, workspaceRoot, execTool, title, body, head, base)
		},
	}
}

func CreateIssueComment(workspaceRoot string, execTool domain.Tool) domain.Tool {
	return domain.Tool{
		Name:          "createIssueComment",
		Description:   "Post a comment on a specified Issue using GitHub CLI",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"issueNumber": map[string]any{"type": "number", "description": "Number of the Issue to comment on"},
				"body":        map[string]any{"type": "string", "description": "Comment body"},
			},
			Required: []string{"issueNumber", "body"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			issueNumber, err := positiveIntArg(args, "issueNumber")
			if err != nil {
				return "", err
			}
			body, err := stringArg(args, "body")
			if err != nil {
				return "", err
			}
			return createIssueCommentExecute(ctx, workspaceRoot, execTool, issueNumber, body)
		},
	}
}

func createPullRequestExecute(ctx context.Context, workspaceRoot string, execTool domain.Tool, title, body, head, base string) (string, error) {
	if err := validateTitle(title); err != nil {
		return "", err
	}
	if err := validateBranchName(head); err != nil {
		return "", err
	}
	if err := validateBranchName(base); err != nil {
		return "", err
	}

	listResult, err := execTool.Execute(ctx, map[string]any{
		"commandName": "gh",
		"commandArgs": []string{"pr", "list", "--head", head, "--base", base, "--state", "open", "--json", "number"},
	})
	if err != nil {
		return "", err
	}

	bodyFile, cleanup, err := writeTempFile(workspaceRoot, body, "pr-body")
	if err != nil {
		return "", err
	}
	defer cleanup()

	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal([]byte(listResult), &prs); err == nil && len(prs) > 0 {
		prNumber := strconv.Itoa(prs[0].Number)
		if _, err := execTool.Execute(ctx, map[string]any{
			"commandName": "gh",
			"commandArgs": []string{"pr", "edit", prNumber, "--body-file", bodyFile},
		}); err != nil {
			return "", err
		}
		return "Updated existing PR #" + prNumber, nil
	}

	result, err := execTool.Execute(ctx, map[string]any{
		"commandName": "gh",
		"commandArgs": []string{"pr", "create", "--title", title, "--body-file", bodyFile, "--base", base, "--head", head},
	})
	if err != nil {
		return "", err
	}
	return "Created PR: " + result, nil
}

func createIssueCommentExecute(ctx context.Context, workspaceRoot string, execTool domain.Tool, issueNumber int, body string) (string, error) {
	bodyFile, cleanup, err := writeTempFile(workspaceRoot, body, "comment-body")
	if err != nil {
		return "", err
	}
	defer cleanup()

	if _, err := execTool.Execute(ctx, map[string]any{
		"commandName": "gh",
		"commandArgs": []string{"issue", "comment", strconv.Itoa(issueNumber), "--body-file", bodyFile},
	}); err != nil {
		return "", err
	}
	return "Comment posted", nil
}

func validateTitle(title string) error {
	if title == "" || len(title) > 200 {
		return errors.New("Invalid PR title")
	}
	if strings.ContainsAny(title, "\r\n\x00") {
		return errors.New("PR title cannot contain newlines or control characters")
	}
	return nil
}

func positiveIntArg(args map[string]any, name string) (int, error) {
	value, ok := args[name]
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed, nil
		}
	case int64:
		if typed > 0 {
			return int(typed), nil
		}
	case float64:
		asInt := int(typed)
		if typed > 0 && typed == float64(asInt) {
			return asInt, nil
		}
	}
	return 0, fmt.Errorf("%s must be a positive integer", name)
}
