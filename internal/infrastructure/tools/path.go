package tools

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errAccessDenied = errors.New("Access denied: Path must be within the workspace")

func resolveExistingWorkspacePath(workspaceRoot, requestedPath string) (string, error) {
	absolutePath, realRoot, err := prepareWorkspacePath(workspaceRoot, requestedPath)
	if err != nil {
		return "", err
	}

	realPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("File not found")
		}
		return "", err
	}
	if !isWithin(realRoot, realPath) {
		return "", errAccessDenied
	}
	return absolutePath, nil
}

func resolveWritableWorkspacePath(workspaceRoot, requestedPath string) (string, error) {
	absolutePath, realRoot, err := prepareWorkspacePath(workspaceRoot, requestedPath)
	if err != nil {
		return "", err
	}

	parent := filepath.Dir(absolutePath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create parent directory: %w", err)
	}

	if _, err := os.Lstat(absolutePath); err == nil {
		realPath, err := filepath.EvalSymlinks(absolutePath)
		if err != nil {
			return "", err
		}
		if !isWithin(realRoot, realPath) {
			return "", errAccessDenied
		}
		return absolutePath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if !isWithin(realRoot, realParent) {
		return "", errAccessDenied
	}
	return absolutePath, nil
}

func prepareWorkspacePath(workspaceRoot, requestedPath string) (absolutePath string, realRoot string, err error) {
	root, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", "", err
	}
	realRoot, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}

	if filepath.IsAbs(requestedPath) {
		absolutePath = filepath.Clean(requestedPath)
	} else {
		absolutePath = filepath.Join(root, requestedPath)
	}
	absolutePath, err = filepath.Abs(absolutePath)
	if err != nil {
		return "", "", err
	}
	if !isWithin(root, absolutePath) {
		return "", "", errAccessDenied
	}
	return absolutePath, realRoot, nil
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
