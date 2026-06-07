package process

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

const maxCommandOutputLength = 2024

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
