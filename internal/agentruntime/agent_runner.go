package agentruntime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"nano-code-go/internal/application/agent"
	"nano-code-go/internal/config"
	"nano-code-go/internal/infrastructure/a2a"
	"nano-code-go/internal/infrastructure/approval"
	"nano-code-go/internal/infrastructure/llm/providers"
	"nano-code-go/internal/infrastructure/process"
	"nano-code-go/internal/infrastructure/prompts"
	"nano-code-go/internal/infrastructure/tools"
)

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

func RunAgentWithIO(
	ctx context.Context,
	request RunAgentRequest,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	env Env,
) (RunAgentResponse, error) {
	cfg := config.Default()
	cfg.Sandbox = request.Sandbox
	cfg = cfg.WithAllowedDomains(request.AllowedDomains)

	model, err := providers.CreateModelFromEnv(providers.FactoryOptions{
		Env: providerEnv(env),
	})
	if err != nil {
		return RunAgentResponse{}, err
	}

	instructions, err := prompts.LoadInstructionsWithEnv(request.WorkspaceRoot, request.IssueDriven, stringEnv(env))
	if err != nil {
		return RunAgentResponse{}, err
	}

	sources, err := a2a.LoadAgentSources("", stringEnv(env))
	if err != nil {
		return RunAgentResponse{}, err
	}

	registry := a2a.Discover(ctx, sources, a2a.NewClient(&http.Client{Timeout: 5 * time.Second}), stderr)
	approvalPolicy := approval.ReadlinePolicy{In: stdin, Out: stderr}.Request
	if request.Yolo {
		approvalPolicy = approval.AllowAll
	}
	var commandRunner process.CommandRunner
	if request.Sandbox {
		commandRunner = process.NewSandboxRunner(safeSandboxEnv(env), false, nil)
	}

	nanoAgent := agent.New(agent.Config{
		Name:         "nano-code",
		Model:        model,
		Instructions: instructions,
		Tools: tools.CreateTools(tools.Options{
			WorkspaceRoot:  request.WorkspaceRoot,
			AllowedDomains: cfg.AllowedDomains,
			CommandRunner:  commandRunner,
		}, registry),
		MaxSteps:     20,
		UseStreaming: request.Streaming,
		Approval:     approvalPolicy,
		Output:       stdout,
	})

	result, err := nanoAgent.Generate(ctx, request.Prompt)
	if err != nil {
		return RunAgentResponse{}, fmt.Errorf("run agent: %w", err)
	}
	return RunAgentResponse{Text: result.Text, Streamed: request.Streaming}, nil
}

func providerEnv(env Env) providers.Env {
	result := make(providers.Env, len(env))
	for key, value := range env {
		result[key] = value
	}
	return result
}

func stringEnv(env Env) map[string]string {
	result := make(map[string]string, len(env))
	for key, value := range env {
		result[key] = value
	}
	return result
}

func safeSandboxEnv(env Env) map[string]string {
	result := map[string]string{}
	if env["PATH"] != "" {
		result["PATH"] = env["PATH"]
	}
	if env["LANG"] != "" {
		result["LANG"] = env["LANG"]
	} else {
		result["LANG"] = "C.UTF-8"
	}
	return result
}
