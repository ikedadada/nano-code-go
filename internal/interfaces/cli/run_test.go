package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"nano-code-go/internal/application/agent"
	"nano-code-go/internal/domain"
	"nano-code-go/internal/infrastructure/approval"
)

func TestRunWithRunnerParsesOptions(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	var got RunAgentRequest
	err := runWithRunner(
		context.Background(),
		[]string{"-y", "--verbose", "-s", "-S", "-d", "example.com, api.example.com", "hello", "world"},
		&stdout,
		&stderr,
		Env{},
		func(_ context.Context, request RunAgentRequest) (RunAgentResponse, error) {
			got = request
			return RunAgentResponse{Text: "answer"}, nil
		},
	)
	if err != nil {
		t.Fatalf("runWithRunner() error = %v", err)
	}

	if got.Prompt != "hello world" {
		t.Fatalf("Prompt = %q", got.Prompt)
	}
	if !got.Yolo || !got.Sandbox || !got.Streaming {
		t.Fatalf("flags = yolo:%v sandbox:%v streaming:%v", got.Yolo, got.Sandbox, got.Streaming)
	}
	if !reflect.DeepEqual(got.AllowedDomains, []string{"example.com", "api.example.com"}) {
		t.Fatalf("AllowedDomains = %#v", got.AllowedDomains)
	}
	if !strings.HasSuffix(got.WorkspaceRoot, filepath.Join("internal", "interfaces", "cli", "workspace")) {
		t.Fatalf("WorkspaceRoot = %q", got.WorkspaceRoot)
	}
	if stdout.String() != "answer\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "[debug] User Prompt: hello world") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunWithRunnerUsesIssuePrompt(t *testing.T) {
	t.Parallel()

	var got RunAgentRequest
	err := runWithRunner(
		context.Background(),
		nil,
		nil,
		nil,
		Env{"ISSUE_TEXT": "issue text"},
		func(_ context.Context, request RunAgentRequest) (RunAgentResponse, error) {
			got = request
			return RunAgentResponse{Text: "answer"}, nil
		},
	)
	if err != nil {
		t.Fatalf("runWithRunner() error = %v", err)
	}
	if got.Prompt != "issue text" || !got.IssueDriven {
		t.Fatalf("request = %#v", got)
	}
}

func TestRunWithRunnerUsesLogLevelDebug(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	err := runWithRunner(
		context.Background(),
		[]string{"prompt"},
		nil,
		&stderr,
		Env{"LOG_LEVEL": "debug"},
		func(_ context.Context, _ RunAgentRequest) (RunAgentResponse, error) {
			return RunAgentResponse{Text: "answer"}, nil
		},
	)
	if err != nil {
		t.Fatalf("runWithRunner() error = %v", err)
	}
	if !strings.Contains(stderr.String(), "[debug] User Prompt: prompt") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunWithRunnerSmokeYoloFakeAgent(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var toolRan bool
	err := runWithRunner(
		context.Background(),
		[]string{"--yolo", "touch", "file"},
		&stdout,
		nil,
		Env{},
		func(ctx context.Context, request RunAgentRequest) (RunAgentResponse, error) {
			if !request.Yolo {
				t.Fatalf("Yolo = false")
			}
			model := &smokeModel{}
			smokeAgent := agent.New(agent.Config{
				Name:         "smoke",
				Instructions: "use fakeTool",
				Model:        model,
				Tools: []domain.Tool{{
					Name:          "fakeTool",
					Description:   "fake smoke-test tool",
					NeedsApproval: true,
					Parameters:    domain.ToolParameters{Type: "object"},
					Execute: func(_ context.Context, args map[string]any) (string, error) {
						if args["path"] != "a.txt" {
							t.Fatalf("tool args = %#v", args)
						}
						toolRan = true
						return "tool ok", nil
					},
				}},
				MaxSteps: 3,
				Approval: approval.AllowAll,
			})

			result, err := smokeAgent.Generate(ctx, request.Prompt)
			if err != nil {
				return RunAgentResponse{}, err
			}
			return RunAgentResponse{Text: result.Text}, nil
		},
	)
	if err != nil {
		t.Fatalf("runWithRunner() error = %v", err)
	}
	if !toolRan {
		t.Fatalf("fake tool was not executed")
	}
	if stdout.String() != "finished\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunWithRunnerIssueBodyPrecedence(t *testing.T) {
	t.Parallel()

	var got RunAgentRequest
	err := runWithRunner(
		context.Background(),
		nil,
		nil,
		nil,
		Env{"ISSUE_BODY": "issue body", "ISSUE_TEXT": "issue text"},
		func(_ context.Context, request RunAgentRequest) (RunAgentResponse, error) {
			got = request
			return RunAgentResponse{Text: "answer"}, nil
		},
	)
	if err != nil {
		t.Fatalf("runWithRunner() error = %v", err)
	}
	if got.Prompt != "issue body" {
		t.Fatalf("Prompt = %q", got.Prompt)
	}
}

func TestRunWithRunnerMissingPromptIsUsageError(t *testing.T) {
	t.Parallel()

	err := runWithRunner(
		context.Background(),
		nil,
		nil,
		nil,
		Env{},
		func(context.Context, RunAgentRequest) (RunAgentResponse, error) {
			t.Fatal("runner should not be called")
			return RunAgentResponse{}, nil
		},
	)
	if err == nil {
		t.Fatalf("runWithRunner() error = nil")
	}
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("error = %T, want UsageError", err)
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode() = %d", ExitCode(err))
	}
}

func TestRunWithRunnerPropagatesRunnerError(t *testing.T) {
	t.Parallel()

	want := errors.New("runner failed")
	err := runWithRunner(
		context.Background(),
		[]string{"prompt"},
		nil,
		nil,
		Env{},
		func(context.Context, RunAgentRequest) (RunAgentResponse, error) {
			return RunAgentResponse{}, want
		},
	)
	if !errors.Is(err, want) {
		t.Fatalf("runWithRunner() error = %v, want %v", err, want)
	}
}

type smokeModel struct {
	calls int
}

func (m *smokeModel) Generate(_ context.Context, params domain.GenerateParams) (domain.GenerateTextResult, error) {
	m.calls++
	switch m.calls {
	case 1:
		if len(params.Messages) < 2 || params.Messages[1].Content != "touch file" {
			return domain.GenerateTextResult{}, errors.New("prompt was not forwarded to model")
		}
		return domain.GenerateTextResult{
			ToolCalls: []domain.ToolCall{{
				ToolCallID: "call-1",
				Name:       "fakeTool",
				Args:       map[string]any{"path": "a.txt"},
			}},
			FinishReason: domain.FinishReasonToolCall,
		}, nil
	case 2:
		for _, message := range params.Messages {
			if message.Role == domain.MessageRoleTool && message.Content == "tool ok" {
				return domain.GenerateTextResult{
					Text:         "finished",
					FinishReason: domain.FinishReasonStop,
				}, nil
			}
		}
		return domain.GenerateTextResult{}, errors.New("tool result was not forwarded to model")
	default:
		return domain.GenerateTextResult{}, errors.New("model called too many times")
	}
}

func (m *smokeModel) Stream(context.Context, domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	return nil, errors.New("streaming is not used by this test")
}
