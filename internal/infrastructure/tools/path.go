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
	root, absolutePath, err := prepareWorkspacePathLexical(workspaceRoot, requestedPath)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create workspace directory: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}

	parent := filepath.Dir(absolutePath)
	if err := mkdirAllWithinWorkspace(root, realRoot, parent); err != nil {
		return "", err
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
	root, absolutePath, err := prepareWorkspacePathLexical(workspaceRoot, requestedPath)
	if err != nil {
		return "", "", err
	}

	realRoot, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}
	return absolutePath, realRoot, nil
}

func prepareWorkspacePathLexical(workspaceRoot, requestedPath string) (root string, absolutePath string, err error) {
	root, err = filepath.Abs(workspaceRoot)
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
	return root, absolutePath, nil
}

func mkdirAllWithinWorkspace(root, realRoot, dir string) error {
	if !isWithin(root, dir) {
		return errAccessDenied
	}

	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}

	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)

		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o755); err != nil {
				return fmt.Errorf("create parent directory: %w", err)
			}
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := filepath.EvalSymlinks(current)
			if err != nil {
				return err
			}
			if !isWithin(realRoot, realPath) {
				return errAccessDenied
			}
			info, err = os.Stat(current)
			if err != nil {
				return err
			}
		}
		if !info.IsDir() {
			return fmt.Errorf("create parent directory: %s is not a directory", current)
		}
	}

	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	if !isWithin(realRoot, realDir) {
		return errAccessDenied
	}
	return nil
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
