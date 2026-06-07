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
	return agentruntime.RunAgentWithIO(ctx, request, stdin, stdout, stderr, env)
}
