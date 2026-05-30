package tools_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"nano-code-go/internal/domain"
	"nano-code-go/internal/infrastructure/tools"
)

type fakeExec struct {
	calls   []map[string]any
	results []string
}

func (f *fakeExec) tool() domain.Tool {
	return domain.Tool{
		Name: "execCommand",
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			copied := map[string]any{}
			for key, value := range args {
				copied[key] = value
			}
			f.calls = append(f.calls, copied)
			if len(f.results) == 0 {
				return "ok", nil
			}
			result := f.results[0]
			f.results = f.results[1:]
			return result, nil
		},
	}
}

func TestCreateBranch(t *testing.T) {
	t.Parallel()

	exec := &fakeExec{}
	result, err := tools.CreateBranch(exec.tool()).Execute(context.Background(), map[string]any{"branchName": "fix/test"})
	if err != nil {
		t.Fatalf("createBranch error = %v", err)
	}
	if !strings.Contains(result, "Branch created: fix/test") {
		t.Fatalf("createBranch = %q", result)
	}
	want := []map[string]any{{"commandName": "git", "commandArgs": []string{"switch", "-c", "fix/test"}}}
	if !reflect.DeepEqual(exec.calls, want) {
		t.Fatalf("calls = %#v, want %#v", exec.calls, want)
	}
}

func TestCreateBranchRejectsInvalidNames(t *testing.T) {
	t.Parallel()

	_, err := tools.CreateBranch((&fakeExec{}).tool()).Execute(context.Background(), map[string]any{"branchName": "-bad"})
	if err == nil || !strings.Contains(err.Error(), "cannot start") {
		t.Fatalf("createBranch error = %v", err)
	}
}

func TestCommit(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	exec := &fakeExec{results: []string{" M file.txt", "added", "committed"}}
	result, err := tools.Commit(workspace, exec.tool()).Execute(context.Background(), map[string]any{
		"message": "Update file",
		"files":   []string{"file.txt"},
	})
	if err != nil {
		t.Fatalf("commit error = %v", err)
	}
	if !strings.Contains(result, "Committed: Update file") {
		t.Fatalf("commit = %q", result)
	}
	if len(exec.calls) != 3 {
		t.Fatalf("calls = %#v, want 3 calls", exec.calls)
	}
	if !reflect.DeepEqual(exec.calls[0], map[string]any{"commandName": "git", "commandArgs": []string{"status", "--porcelain"}}) {
		t.Fatalf("status call = %#v", exec.calls[0])
	}
	if !reflect.DeepEqual(exec.calls[1], map[string]any{"commandName": "git", "commandArgs": []string{"add", "--", "file.txt"}}) {
		t.Fatalf("add call = %#v", exec.calls[1])
	}
	commitArgs := exec.calls[2]["commandArgs"].([]string)
	if len(commitArgs) != 3 || commitArgs[0] != "commit" || commitArgs[1] != "-F" || !strings.Contains(commitArgs[2], ".commit-message-") {
		t.Fatalf("commit args = %#v", commitArgs)
	}
}

func TestCommitSkipsWhenNoChanges(t *testing.T) {
	t.Parallel()

	exec := &fakeExec{results: []string{"[no output]"}}
	result, err := tools.Commit(t.TempDir(), exec.tool()).Execute(context.Background(), map[string]any{
		"message": "Update file",
		"files":   []string{"file.txt"},
	})
	if err != nil {
		t.Fatalf("commit error = %v", err)
	}
	if result != "No changes to commit (already up to date)" {
		t.Fatalf("commit = %q", result)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("calls = %#v, want only status", exec.calls)
	}
}

func TestPushBranch(t *testing.T) {
	t.Parallel()

	exec := &fakeExec{}
	result, err := tools.PushBranch(exec.tool()).Execute(context.Background(), map[string]any{"branchName": "fix/test"})
	if err != nil {
		t.Fatalf("pushBranch error = %v", err)
	}
	if !strings.Contains(result, "Branch pushed: fix/test") {
		t.Fatalf("pushBranch = %q", result)
	}
	want := []map[string]any{{"commandName": "git", "commandArgs": []string{"push", "-u", "origin", "fix/test"}}}
	if !reflect.DeepEqual(exec.calls, want) {
		t.Fatalf("calls = %#v, want %#v", exec.calls, want)
	}
}
