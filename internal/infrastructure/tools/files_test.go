package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nano-code-go/internal/infrastructure/tools"
)

func TestReadFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "example.txt"), []byte("hello workspace"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := tools.ReadFile(workspace).Execute(context.Background(), map[string]any{"path": "example.txt"})
	if err != nil {
		t.Fatalf("readFile error = %v", err)
	}
	if result != "hello workspace" {
		t.Fatalf("readFile = %q, want hello workspace", result)
	}
}

func TestReadFileRejectsUnsafePaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, err := tools.ReadFile(workspace).Execute(context.Background(), map[string]any{"path": "../outside.txt"})
	if err == nil || !strings.Contains(err.Error(), "Access denied") {
		t.Fatalf("readFile error = %v, want access denied", err)
	}
}

func TestReadFileRejectsLargeFiles(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	content := strings.Repeat("x", 100*1024+1)
	if err := os.WriteFile(filepath.Join(workspace, "large.txt"), []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := tools.ReadFile(workspace).Execute(context.Background(), map[string]any{"path": "large.txt"})
	if err == nil || !strings.Contains(err.Error(), "File size exceeds") {
		t.Fatalf("readFile error = %v, want file size error", err)
	}
}

func TestWriteFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	result, err := tools.WriteFile(workspace).Execute(context.Background(), map[string]any{
		"path":    "nested/example.txt",
		"content": "hello workspace",
	})
	if err != nil {
		t.Fatalf("writeFile error = %v", err)
	}
	if result != "File written successfully to nested/example.txt" {
		t.Fatalf("writeFile = %q", result)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "nested", "example.txt"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(content) != "hello workspace" {
		t.Fatalf("written content = %q", content)
	}
}

func TestWriteFileCreatesMissingWorkspaceRoot(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(t.TempDir(), "workspace")
	result, err := tools.WriteFile(workspace).Execute(context.Background(), map[string]any{
		"path":    "nested/example.txt",
		"content": "hello fresh checkout",
	})
	if err != nil {
		t.Fatalf("writeFile error = %v", err)
	}
	if result != "File written successfully to nested/example.txt" {
		t.Fatalf("writeFile = %q", result)
	}
	content, err := os.ReadFile(filepath.Join(workspace, "nested", "example.txt"))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(content) != "hello fresh checkout" {
		t.Fatalf("written content = %q", content)
	}
}

func TestWriteFileRejectsUnsafePaths(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, err := tools.WriteFile(workspace).Execute(context.Background(), map[string]any{
		"path":    "../outside.txt",
		"content": "nope",
	})
	if err == nil || !strings.Contains(err.Error(), "Access denied") {
		t.Fatalf("writeFile error = %v, want access denied", err)
	}
}

func TestWriteFileRejectsSymlinkedParentOutsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(workspace, "link")); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	_, err := tools.WriteFile(workspace).Execute(context.Background(), map[string]any{
		"path":    "link/nested/example.txt",
		"content": "nope",
	})
	if err == nil || !strings.Contains(err.Error(), "Access denied") {
		t.Fatalf("writeFile error = %v, want access denied", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "nested")); !os.IsNotExist(err) {
		t.Fatalf("outside nested directory err = %v, want not exist", err)
	}
}

func TestEditFile(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, "example.txt")
	if err := os.WriteFile(path, []byte("before TARGET after"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := tools.EditFile(workspace).Execute(context.Background(), map[string]any{
		"path":    "example.txt",
		"oldText": "TARGET",
		"newText": "updated",
	})
	if err != nil {
		t.Fatalf("editFile error = %v", err)
	}
	if result != "File edited successfully at example.txt" {
		t.Fatalf("editFile = %q", result)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read edited file: %v", err)
	}
	if string(content) != "before updated after" {
		t.Fatalf("edited content = %q", content)
	}
}

func TestEditFileRejectsAmbiguousReplacement(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "example.txt"), []byte("same same"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, err := tools.EditFile(workspace).Execute(context.Background(), map[string]any{
		"path":    "example.txt",
		"oldText": "same",
		"newText": "updated",
	})
	if err == nil || !strings.Contains(err.Error(), "found multiple times") {
		t.Fatalf("editFile error = %v, want multiple matches error", err)
	}
}
