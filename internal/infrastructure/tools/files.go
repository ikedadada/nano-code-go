package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"nano-code-go/internal/domain"
)

const maxReadFileSize = 100 * 1024

type Options struct {
	WorkspaceRoot  string
	AllowedDomains []string
	HTTPClient     HTTPDoer
	CommandRunner  CommandRunner
}

func ReadFile(workspaceRoot string) domain.Tool {
	return domain.Tool{
		Name:          "readFile",
		Description:   "Reads the contents of a file at the specified path within the workspace as a string.",
		NeedsApproval: false,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The relative path to the file within the workspace",
				},
			},
			Required: []string{"path"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			_ = ctx
			path, err := stringArg(args, "path")
			if err != nil {
				return "", err
			}
			return readFileExecute(workspaceRoot, path)
		},
	}
}

func WriteFile(workspaceRoot string) domain.Tool {
	return domain.Tool{
		Name:          "writeFile",
		Description:   "Writes the provided content to a file at the specified path within the workspace.",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The relative path to the file within the workspace",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to write to the file",
				},
			},
			Required: []string{"path", "content"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			_ = ctx
			path, err := stringArg(args, "path")
			if err != nil {
				return "", err
			}
			content, err := stringArg(args, "content")
			if err != nil {
				return "", err
			}
			return writeFileExecute(workspaceRoot, path, content)
		},
	}
}

func EditFile(workspaceRoot string) domain.Tool {
	return domain.Tool{
		Name:          "editFile",
		Description:   "Edits part of a file by replacing the section specified by oldText with newText.",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "The path to the file to edit",
				},
				"oldText": map[string]any{
					"type":        "string",
					"description": "The text to replace",
				},
				"newText": map[string]any{
					"type":        "string",
					"description": "The new text to insert",
				},
			},
			Required: []string{"path", "oldText", "newText"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			_ = ctx
			path, err := stringArg(args, "path")
			if err != nil {
				return "", err
			}
			oldText, err := stringArg(args, "oldText")
			if err != nil {
				return "", err
			}
			newText, err := stringArg(args, "newText")
			if err != nil {
				return "", err
			}
			return editFileExecute(workspaceRoot, path, oldText, newText)
		},
	}
}

func readFileExecute(workspaceRoot, requestedPath string) (string, error) {
	path, err := resolveExistingWorkspacePath(workspaceRoot, requestedPath)
	if err != nil {
		return "", err
	}
	stat, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("File not found")
		}
		return "", err
	}
	if !stat.Mode().IsRegular() {
		return "", errors.New("The specified path is not a file")
	}
	if stat.Size() > maxReadFileSize {
		return "", errors.New("File size exceeds the maximum allowed limit of 100 KB")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func writeFileExecute(workspaceRoot, requestedPath, content string) (string, error) {
	path, err := resolveWritableWorkspacePath(workspaceRoot, requestedPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("File written successfully to %s", requestedPath), nil
}

func editFileExecute(workspaceRoot, requestedPath, oldText, newText string) (string, error) {
	path, err := resolveExistingWorkspacePath(workspaceRoot, requestedPath)
	if err != nil {
		return "", err
	}
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(contentBytes)
	matches := strings.Count(content, oldText)
	if matches == 0 {
		return "", errors.New("The specified oldText was not found in the file")
	}
	if matches > 1 {
		return "", errors.New("The specified oldText was found multiple times in the file. Please provide a unique oldText.")
	}
	newContent := strings.Replace(content, oldText, newText, 1)
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("File edited successfully at %s", requestedPath), nil
}

func stringArg(args map[string]any, name string) (string, error) {
	value, ok := args[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", name)
	}
	return text, nil
}
