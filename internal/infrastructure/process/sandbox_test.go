package process

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"nano-code-go/internal/infrastructure/tools"
)

type recordingCommandRunner struct {
	commandName string
	commandArgs []string
	options     tools.RunOptions
	result      tools.RunResult
	err         error
}

func (r *recordingCommandRunner) Run(_ context.Context, commandName string, commandArgs []string, options tools.RunOptions) (tools.RunResult, error) {
	r.commandName = commandName
	r.commandArgs = append([]string(nil), commandArgs...)
	r.options = options
	return r.result, r.err
}

func TestSandboxRunnerWrapsCommandWithBwrap(t *testing.T) {
	t.Setenv("PATH", "/bin")

	base := &recordingCommandRunner{result: tools.RunResult{Stdout: "ok", ExitCode: 0}}
	runner := NewSandboxRunner(map[string]string{"CUSTOM": "value"}, false, base)
	options := tools.RunOptions{WorkspaceRoot: "/workspace", Timeout: 5 * time.Second}

	result, err := runner.Run(context.Background(), "bun", []string{"test"}, options)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "ok" {
		t.Fatalf("result = %#v", result)
	}
	if base.commandName != "bwrap" {
		t.Fatalf("commandName = %q", base.commandName)
	}
	wantArgs := []string{
		"--ro-bind", "/", "/",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--bind", "/workspace", "/workspace",
		"--chdir", "/workspace",
		"--die-with-parent",
		"--clearenv",
		"--setenv", "CUSTOM", "value",
		"--setenv", "HOME", "/tmp",
		"--setenv", "PATH", "/bin",
		"--unshare-net",
		"--", "bun", "test",
	}
	if !reflect.DeepEqual(base.commandArgs, wantArgs) {
		t.Fatalf("commandArgs = %#v, want %#v", base.commandArgs, wantArgs)
	}
	if !reflect.DeepEqual(base.options, options) {
		t.Fatalf("options = %#v, want %#v", base.options, options)
	}
}

func TestSandboxRunnerAllowsNetwork(t *testing.T) {
	t.Setenv("PATH", "/bin")

	base := &recordingCommandRunner{}
	runner := NewSandboxRunner(nil, true, base)
	_, err := runner.Run(context.Background(), "curl", []string{"https://example.com"}, tools.RunOptions{WorkspaceRoot: "/workspace"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(strings.Join(base.commandArgs, " "), "--unshare-net") {
		t.Fatalf("commandArgs = %#v", base.commandArgs)
	}
}

func TestSandboxRunnerReportsSpawnFailure(t *testing.T) {
	t.Parallel()

	base := &recordingCommandRunner{err: errors.New("executable file not found")}
	runner := NewSandboxRunner(nil, false, base)
	result, err := runner.Run(context.Background(), "ls", nil, tools.RunOptions{WorkspaceRoot: "/workspace"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 126 || !strings.Contains(result.Stderr, "Sandbox Error: executable file not found") {
		t.Fatalf("result = %#v", result)
	}
}
