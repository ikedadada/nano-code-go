package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"nano-code-go/internal/infrastructure/logger"
)

type UsageError struct {
	Err error
}

func (e *UsageError) Error() string {
	if e == nil || e.Err == nil {
		return "usage error"
	}
	return e.Err.Error()
}

func (e *UsageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type Env map[string]string

type RunAgentRequest struct {
	Prompt         string
	IssueDriven    bool
	Streaming      bool
	Yolo           bool
	Sandbox        bool
	AllowedDomains []string
	WorkspaceRoot  string
}

type RunAgentResponse struct {
	Text     string
	Streamed bool
}

type AgentRunner func(ctx context.Context, request RunAgentRequest) (RunAgentResponse, error)

func EnvFromOS() Env {
	env := make(Env)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer, env Env) error {
	return runWithRunner(ctx, args, stdout, stderr, env, func(ctx context.Context, request RunAgentRequest) (RunAgentResponse, error) {
		return runAgentWithIO(ctx, request, os.Stdin, stdout, stderr, env)
	})
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return 2
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	return 1
}

func runWithRunner(ctx context.Context, args []string, stdout, stderr io.Writer, env Env, runner AgentRunner) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if env == nil {
		env = Env{}
	}
	if runner == nil {
		return errors.New("agent runner is not configured")
	}

	options, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}
	prompt, issueDriven, err := resolvePrompt(options.promptParts, env)
	if err != nil {
		return err
	}

	workspaceRoot, err := defaultWorkspaceRoot()
	if err != nil {
		return err
	}

	log := logger.New(stdout, stderr)
	log.SetDebug(options.verbose || strings.EqualFold(env["LOG_LEVEL"], "debug"))
	log.Debug("Start agent")
	log.Debug("User Prompt:", prompt)

	result, err := runner(ctx, RunAgentRequest{
		Prompt:         prompt,
		IssueDriven:    issueDriven,
		Streaming:      options.streaming,
		Yolo:           options.yolo,
		Sandbox:        options.sandbox,
		AllowedDomains: options.allowedDomains,
		WorkspaceRoot:  workspaceRoot,
	})
	if err != nil {
		return err
	}
	if !result.Streamed {
		log.Output(result.Text)
	}
	return nil
}

type cliOptions struct {
	yolo           bool
	verbose        bool
	sandbox        bool
	streaming      bool
	allowedDomains []string
	promptParts    []string
}

func parseArgs(args []string, stderr io.Writer) (cliOptions, error) {
	flags := flag.NewFlagSet("nano-code", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var yoloLong, yoloShort bool
	var verboseLong, verboseShort bool
	var sandboxLong, sandboxShort bool
	var streamingLong, streamingShort bool
	var domainsLong, domainsShort string

	flags.BoolVar(&yoloLong, "yolo", false, "approve all tool calls")
	flags.BoolVar(&yoloShort, "y", false, "approve all tool calls")
	flags.BoolVar(&verboseLong, "verbose", false, "show debug logs")
	flags.BoolVar(&verboseShort, "v", false, "show debug logs")
	flags.BoolVar(&sandboxLong, "sandbox", false, "run commands in sandbox")
	flags.BoolVar(&sandboxShort, "s", false, "run commands in sandbox")
	flags.BoolVar(&streamingLong, "streaming", false, "stream model output")
	flags.BoolVar(&streamingShort, "S", false, "stream model output")
	flags.StringVar(&domainsLong, "allowed-domains", "", "comma-separated domains allowed for web fetch")
	flags.StringVar(&domainsShort, "d", "", "comma-separated domains allowed for web fetch")

	if err := flags.Parse(args); err != nil {
		return cliOptions{}, &UsageError{Err: err}
	}

	return cliOptions{
		yolo:           yoloLong || yoloShort,
		verbose:        verboseLong || verboseShort,
		sandbox:        sandboxLong || sandboxShort,
		streaming:      streamingLong || streamingShort,
		allowedDomains: parseAllowedDomains(domainsLong, domainsShort),
		promptParts:    flags.Args(),
	}, nil
}

func resolvePrompt(promptParts []string, env Env) (string, bool, error) {
	issueBody, hasIssueBody := env["ISSUE_BODY"]
	issueText, hasIssueText := env["ISSUE_TEXT"]
	isIssueDriven := hasIssueBody || hasIssueText
	if isIssueDriven {
		if issueBody != "" {
			return issueBody, true, nil
		}
		return issueText, true, nil
	}

	if len(promptParts) == 0 {
		return "", false, &UsageError{Err: errors.New("error: missing required argument 'prompt'")}
	}
	return strings.Join(promptParts, " "), false, nil
}

func parseAllowedDomains(values ...string) []string {
	var result []string
	for _, value := range values {
		if value == "" {
			continue
		}
		for _, domain := range strings.Split(value, ",") {
			domain = strings.TrimSpace(domain)
			if domain != "" {
				result = append(result, domain)
			}
		}
	}
	return result
}

func defaultWorkspaceRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current directory: %w", err)
	}
	return filepath.Join(wd, "workspace"), nil
}
