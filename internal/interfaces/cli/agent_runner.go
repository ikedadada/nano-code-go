package cli

import (
	"context"
	"io"

	"nano-code-go/internal/agentruntime"
)

func runAgentWithIO(
	ctx context.Context,
	request RunAgentRequest,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	env Env,
) (RunAgentResponse, error) {
	return RunAgentWithIO(ctx, request, stdin, stdout, stderr, env)
}

func RunAgentWithIO(
	ctx context.Context,
	request RunAgentRequest,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	env Env,
) (RunAgentResponse, error) {
	result, err := agentruntime.RunAgentWithIO(ctx, agentruntime.RunAgentRequest{
		Prompt:         request.Prompt,
		IssueDriven:    request.IssueDriven,
		Streaming:      request.Streaming,
		Yolo:           request.Yolo,
		Sandbox:        request.Sandbox,
		AllowedDomains: request.AllowedDomains,
		WorkspaceRoot:  request.WorkspaceRoot,
	}, stdin, stdout, stderr, agentRuntimeEnv(env))
	if err != nil {
		return RunAgentResponse{}, err
	}
	return RunAgentResponse{Text: result.Text, Streamed: result.Streamed}, nil
}

func agentRuntimeEnv(env Env) agentruntime.Env {
	result := make(agentruntime.Env, len(env))
	for key, value := range env {
		result[key] = value
	}
	return result
}
