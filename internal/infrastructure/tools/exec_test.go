package tools_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"nano-code-go/internal/infrastructure/tools"
)

type recordingRunner struct {
	calls  []runCall
	result tools.RunResult
	err    error
}

type runCall struct {
	commandName string
	commandArgs []string
	options     tools.RunOptions
}

func (r *recordingRunner) Run(_ context.Context, commandName string, commandArgs []string, options tools.RunOptions) (tools.RunResult, error) {
	r.calls = append(r.calls, runCall{
		commandName: commandName,
		commandArgs: append([]string(nil), commandArgs...),
		options:     options,
	})
	return r.result, r.err
}

func TestExecCommand(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	runner := &recordingRunner{result: tools.RunResult{Stdout: "ok\n"}}
	result, err := tools.ExecCommand(workspace, runner).Execute(context.Background(), map[string]any{
		"command": `ls "nested dir"`,
	})
	if err != nil {
		t.Fatalf("execCommand error = %v", err)
	}
	if result != "ok" {
		t.Fatalf("execCommand = %q, want ok", result)
	}
	wantCalls := []runCall{{
		commandName: "ls",
		commandArgs: []string{"nested dir"},
		options:     tools.RunOptions{WorkspaceRoot: workspace, Timeout: 30 * time.Second},
	}}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("runner.calls = %#v, want %#v", runner.calls, wantCalls)
	}
}

func TestExecCommandRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "dangerous chars",
			args: map[string]any{"command": "ls; rm -rf /"},
			want: "dangerous characters",
		},
		{
			name: "not allowlisted",
			args: map[string]any{"command": "rm file"},
			want: "not allowed",
		},
		{
			name: "path outside workspace",
			args: map[string]any{"commandName": "ls", "commandArgs": []string{"../outside"}},
			want: "resolves outside the workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tools.ExecCommand(t.TempDir(), &recordingRunner{}).Execute(context.Background(), tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("execCommand error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestExecCommandSurfacesRunnerFailures(t *testing.T) {
	t.Parallel()

	runner := &recordingRunner{
		result: tools.RunResult{Stderr: "bad", ExitCode: 2},
		err:    errors.New("exit status 2"),
	}
	_, err := tools.ExecCommand(t.TempDir(), runner).Execute(context.Background(), map[string]any{
		"commandName": "git",
		"commandArgs": []string{"status"},
	})
	if err == nil || !strings.Contains(err.Error(), "Command failed (2): git status") || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("execCommand error = %v", err)
	}
}
