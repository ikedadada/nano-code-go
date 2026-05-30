package tools_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"nano-code-go/internal/infrastructure/tools"
)

func TestCreatePullRequestCreatesNewPR(t *testing.T) {
	t.Parallel()

	exec := &fakeExec{results: []string{"[]", "https://github.com/o/r/pull/1"}}
	result, err := tools.CreatePullRequest(t.TempDir(), exec.tool()).Execute(context.Background(), map[string]any{
		"title": "Update docs",
		"body":  "Body",
		"head":  "feature/docs",
		"base":  "main",
	})
	if err != nil {
		t.Fatalf("createPullRequest error = %v", err)
	}
	if result != "Created PR: https://github.com/o/r/pull/1" {
		t.Fatalf("createPullRequest = %q", result)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("calls = %#v, want 2", exec.calls)
	}
	wantList := map[string]any{"commandName": "gh", "commandArgs": []string{"pr", "list", "--head", "feature/docs", "--base", "main", "--state", "open", "--json", "number"}}
	if !reflect.DeepEqual(exec.calls[0], wantList) {
		t.Fatalf("list call = %#v", exec.calls[0])
	}
	createArgs := exec.calls[1]["commandArgs"].([]string)
	if len(createArgs) != 10 || createArgs[0] != "pr" || createArgs[1] != "create" || !strings.Contains(createArgs[5], ".pr-body-") {
		t.Fatalf("create args = %#v", createArgs)
	}
}

func TestCreatePullRequestUpdatesExistingPR(t *testing.T) {
	t.Parallel()

	exec := &fakeExec{results: []string{`[{"number":42}]`, "updated"}}
	result, err := tools.CreatePullRequest(t.TempDir(), exec.tool()).Execute(context.Background(), map[string]any{
		"title": "Update docs",
		"body":  "Body",
		"head":  "feature/docs",
		"base":  "main",
	})
	if err != nil {
		t.Fatalf("createPullRequest error = %v", err)
	}
	if result != "Updated existing PR #42" {
		t.Fatalf("createPullRequest = %q", result)
	}
	editArgs := exec.calls[1]["commandArgs"].([]string)
	if len(editArgs) != 5 || editArgs[0] != "pr" || editArgs[1] != "edit" || editArgs[2] != "42" {
		t.Fatalf("edit args = %#v", editArgs)
	}
}

func TestCreatePullRequestRejectsInvalidTitle(t *testing.T) {
	t.Parallel()

	_, err := tools.CreatePullRequest(t.TempDir(), (&fakeExec{}).tool()).Execute(context.Background(), map[string]any{
		"title": "bad\ntitle",
		"body":  "Body",
		"head":  "feature/docs",
		"base":  "main",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot contain newlines") {
		t.Fatalf("createPullRequest error = %v", err)
	}
}

func TestCreateIssueComment(t *testing.T) {
	t.Parallel()

	exec := &fakeExec{}
	result, err := tools.CreateIssueComment(t.TempDir(), exec.tool()).Execute(context.Background(), map[string]any{
		"issueNumber": 12,
		"body":        "comment body",
	})
	if err != nil {
		t.Fatalf("createIssueComment error = %v", err)
	}
	if result != "Comment posted" {
		t.Fatalf("createIssueComment = %q", result)
	}
	args := exec.calls[0]["commandArgs"].([]string)
	if len(args) != 5 || args[0] != "issue" || args[1] != "comment" || args[2] != "12" || !strings.Contains(args[4], ".comment-body-") {
		t.Fatalf("issue comment args = %#v", args)
	}
}
