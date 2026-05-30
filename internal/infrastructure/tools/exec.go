package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"nano-code-go/internal/domain"
)

const maxCommandOutputLength = 2024

var allowedCommands = map[string]struct{}{
	"bun":  {},
	"ls":   {},
	"git":  {},
	"gh":   {},
	"curl": {},
}

type CommandRunner interface {
	Run(ctx context.Context, commandName string, commandArgs []string, options RunOptions) (RunResult, error)
}

type RunOptions struct {
	WorkspaceRoot string
	Timeout       time.Duration
}

type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type OSCommandRunner struct{}

func (OSCommandRunner) Run(ctx context.Context, commandName string, commandArgs []string, options RunOptions) (RunResult, error) {
	timeout := options.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(ctx, commandName, commandArgs...)
	command.Dir = options.WorkspaceRoot

	var stdout, stderr limitedBuffer
	stdout.limit = maxCommandOutputLength
	stderr.limit = maxCommandOutputLength
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	result := RunResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if command.ProcessState != nil {
		result.ExitCode = command.ProcessState.ExitCode()
	}
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, err
	}
	return result, nil
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	_, _ = b.buffer.Write(p)
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	text := b.buffer.String()
	if b.truncated {
		return text + "\n...[output truncated]"
	}
	return text
}

func ExecCommand(workspaceRoot string, runner CommandRunner) domain.Tool {
	if runner == nil {
		runner = OSCommandRunner{}
	}

	return domain.Tool{
		Name:          "execCommand",
		Description:   "Executes a shell command and returns its output as a string.",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The shell command to execute",
				},
			},
			Required: []string{"command"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			commandName, commandArgs, err := commandFromArgs(args)
			if err != nil {
				return "", err
			}
			return execCommandExecute(ctx, workspaceRoot, runner, commandName, commandArgs)
		},
	}
}

func execCommandExecute(ctx context.Context, workspaceRoot string, runner CommandRunner, commandName string, commandArgs []string) (string, error) {
	if _, ok := allowedCommands[commandName]; !ok {
		return "", fmt.Errorf("Command %q is not allowed. Allowed commands are: bun, ls, git, gh, curl", commandName)
	}
	for _, arg := range commandArgs {
		if strings.Contains(arg, "/") || strings.Contains(arg, "\\") {
			if _, _, err := prepareWorkspacePath(workspaceRoot, arg); err != nil {
				return "", fmt.Errorf("Argument %q is not allowed because it resolves outside the workspace", arg)
			}
		}
	}

	result, err := runner.Run(ctx, commandName, commandArgs, RunOptions{WorkspaceRoot: workspaceRoot, Timeout: 30 * time.Second})
	stdout := strings.TrimSpace(result.Stdout)
	stderr := strings.TrimSpace(result.Stderr)
	if err != nil || result.ExitCode != 0 {
		message := stderr
		if message == "" {
			message = stdout
		}
		if message == "" && err != nil {
			message = err.Error()
		}
		if message == "" {
			message = "Unknown error"
		}
		code := result.ExitCode
		if code == 0 {
			code = -1
		}
		return "", fmt.Errorf("Command failed (%d): %s %s\n%s", code, commandName, strings.Join(commandArgs, " "), message)
	}
	if stdout != "" {
		return stdout, nil
	}
	if stderr != "" {
		return stderr, nil
	}
	return "[no output]", nil
}

func commandFromArgs(args map[string]any) (string, []string, error) {
	if commandValue, ok := args["command"]; ok {
		command, ok := commandValue.(string)
		if !ok {
			return "", nil, errors.New("command must be a string")
		}
		if strings.ContainsAny(command, ";&`$") {
			return "", nil, errors.New("Command contains dangerous characters that are not allowed")
		}
		parts, err := parseCommand(command)
		if err != nil {
			return "", nil, err
		}
		if len(parts) == 0 {
			return "", nil, errors.New("No command provided")
		}
		return parts[0], parts[1:], nil
	}

	commandName, err := stringArg(args, "commandName")
	if err != nil {
		return "", nil, errors.New("No command provided")
	}
	commandArgs, err := stringSliceArg(args, "commandArgs")
	if err != nil {
		return "", nil, err
	}
	return commandName, commandArgs, nil
}

func parseCommand(input string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	var quote rune
	escaped := false

	for _, ch := range input {
		if quote != 0 {
			if escaped {
				current.WriteRune(ch)
				escaped = false
				continue
			}
			if ch == '\\' && quote == '"' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
				continue
			}
			current.WriteRune(ch)
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}

	if quote != 0 {
		return nil, fmt.Errorf("Unclosed quote: %c", quote)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

func stringSliceArg(args map[string]any, name string) ([]string, error) {
	value, ok := args[name]
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), nil
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s must contain only strings", name)
			}
			result = append(result, text)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings", name)
	}
}
