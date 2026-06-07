package process

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
)

const defaultPath = "/usr/local/bin:/usr/bin:/bin"

type SandboxRunner struct {
	base         CommandRunner
	env          map[string]string
	allowNetwork bool
}

func NewSandboxRunner(env map[string]string, allowNetwork bool, base CommandRunner) SandboxRunner {
	if base == nil {
		base = OSCommandRunner{}
	}
	return SandboxRunner{
		base:         base,
		env:          copyEnv(env),
		allowNetwork: allowNetwork,
	}
}

func (r SandboxRunner) Run(ctx context.Context, commandName string, commandArgs []string, options RunOptions) (RunResult, error) {
	if runtime.GOOS != "linux" {
		return RunResult{Stderr: "Sandbox Error: sandbox is only supported on linux", ExitCode: 126}, nil
	}

	cwd := options.WorkspaceRoot
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return RunResult{}, fmt.Errorf("get current directory for sandbox: %w", err)
		}
	}

	result, err := r.base.Run(ctx, "bwrap", r.bwrapArgs(cwd, commandName, commandArgs), options)
	if err != nil && result.ExitCode == 0 && result.Stdout == "" && result.Stderr == "" {
		return RunResult{
			Stderr:   fmt.Sprintf("Sandbox Error: %s\n(Hint: check the --cap-add=SYS_ADMIN option for docker run)", err.Error()),
			ExitCode: 126,
		}, nil
	}
	return result, err
}

func (r SandboxRunner) bwrapArgs(cwd, commandName string, commandArgs []string) []string {
	args := []string{
		"--ro-bind", "/", "/",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--bind", cwd, cwd,
		"--chdir", cwd,
		"--die-with-parent",
		"--clearenv",
	}

	env := sandboxEnv(r.env)
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--setenv", key, env[key])
	}

	if !r.allowNetwork {
		args = append(args, "--unshare-net")
	}

	args = append(args, "--", commandName)
	args = append(args, commandArgs...)
	return args
}

func sandboxEnv(extra map[string]string) map[string]string {
	path := os.Getenv("PATH")
	if path == "" {
		path = defaultPath
	}
	env := map[string]string{
		"HOME": "/tmp",
		"PATH": path,
	}
	for key, value := range extra {
		env[key] = value
	}
	return env
}

func copyEnv(env map[string]string) map[string]string {
	result := make(map[string]string, len(env))
	for key, value := range env {
		result[key] = value
	}
	return result
}
