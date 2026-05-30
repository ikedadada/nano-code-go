package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"nano-code-go/internal/domain"
)

var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

func CreateBranch(execTool domain.Tool) domain.Tool {
	return domain.Tool{
		Name:          "createBranch",
		Description:   "Create a new Git branch. Fails if the branch already exists.",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"branchName": map[string]any{"type": "string", "description": "Branch name to create"},
			},
			Required: []string{"branchName"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			branchName, err := stringArg(args, "branchName")
			if err != nil {
				return "", err
			}
			if err := validateBranchName(branchName); err != nil {
				return "", err
			}
			result, err := execTool.Execute(ctx, map[string]any{
				"commandName": "git",
				"commandArgs": []string{"switch", "-c", branchName},
			})
			if err != nil {
				return "", fmt.Errorf("Branch creation failed: %w", err)
			}
			return fmt.Sprintf("Branch created: %s\n%s", branchName, result), nil
		},
	}
}

func Commit(workspaceRoot string, execTool domain.Tool) domain.Tool {
	return domain.Tool{
		Name:          "commit",
		Description:   "Commit changes with a message. If there are no changes, do not commit.",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"message": map[string]any{"type": "string", "description": "Commit message"},
				"files": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "List of file paths to commit",
				},
			},
			Required: []string{"message", "files"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			message, err := stringArg(args, "message")
			if err != nil {
				return "", err
			}
			files, err := stringSliceArg(args, "files")
			if err != nil {
				return "", err
			}
			return commitExecute(ctx, workspaceRoot, execTool, message, files)
		},
	}
}

func PushBranch(execTool domain.Tool) domain.Tool {
	return domain.Tool{
		Name:          "pushBranch",
		Description:   "Push the current branch to the remote repository. If it is a new branch, set the upstream.",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"branchName": map[string]any{"type": "string", "description": "Branch name to push"},
			},
			Required: []string{"branchName"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			branchName, err := stringArg(args, "branchName")
			if err != nil {
				return "", err
			}
			if err := validateBranchName(branchName); err != nil {
				return "", err
			}
			result, err := execTool.Execute(ctx, map[string]any{
				"commandName": "git",
				"commandArgs": []string{"push", "-u", "origin", branchName},
			})
			if err != nil {
				return "", fmt.Errorf("Push failed: %w", err)
			}
			return fmt.Sprintf("Branch pushed: %s\n%s", branchName, result), nil
		},
	}
}

func commitExecute(ctx context.Context, workspaceRoot string, execTool domain.Tool, message string, files []string) (string, error) {
	if message == "" || strings.ContainsRune(message, 0) {
		return "", errors.New("Invalid commit message")
	}

	status, err := execTool.Execute(ctx, map[string]any{
		"commandName": "git",
		"commandArgs": []string{"status", "--porcelain"},
	})
	if err != nil {
		return "", fmt.Errorf("Commit failed: %w", err)
	}
	if strings.TrimSpace(status) == "" || strings.Contains(status, "no output") {
		return "No changes to commit (already up to date)", nil
	}

	for _, file := range files {
		if err := validateFilePath(file); err != nil {
			return "", err
		}
		if _, err := execTool.Execute(ctx, map[string]any{
			"commandName": "git",
			"commandArgs": []string{"add", "--", file},
		}); err != nil {
			return "", fmt.Errorf("Commit failed: %w", err)
		}
	}

	messageFile, cleanup, err := writeTempFile(workspaceRoot, message, "commit-message")
	if err != nil {
		return "", err
	}
	defer cleanup()

	result, err := execTool.Execute(ctx, map[string]any{
		"commandName": "git",
		"commandArgs": []string{"commit", "-F", messageFile},
	})
	if err != nil {
		return "", fmt.Errorf("Commit failed: %w", err)
	}
	return fmt.Sprintf("Committed: %s\n%s", message, result), nil
}

func validateBranchName(name string) error {
	if name == "" || len(name) > 120 {
		return errors.New("Invalid branch name")
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, ":") {
		return errors.New("Branch name cannot start with '-' or ':'")
	}
	if strings.ContainsAny(name, " \t\r\n") {
		return errors.New("Branch name cannot contain whitespace")
	}
	if !branchNamePattern.MatchString(name) {
		return errors.New("Branch name contains invalid characters")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "//") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") {
		return errors.New("Invalid branch name format")
	}
	return nil
}

func validateFilePath(filePath string) error {
	if filePath == "" {
		return errors.New("File path is empty")
	}
	if strings.HasPrefix(filePath, "-") {
		return errors.New("File path cannot start with '-'")
	}
	if strings.ContainsAny(filePath, "\r\n\x00") {
		return errors.New("File path contains invalid control characters")
	}
	return nil
}

func writeTempFile(workspaceRoot, content, prefix string) (string, func(), error) {
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return "", func() {}, err
	}
	tempDir, err := os.MkdirTemp(workspaceRoot, "."+prefix+"-")
	if err != nil {
		return "", func() {}, err
	}
	tempPath := filepath.Join(tempDir, "content.txt")
	if err := os.WriteFile(tempPath, []byte(content), 0o600); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", func() {}, err
	}
	return tempPath, func() { _ = os.RemoveAll(tempDir) }, nil
}
