package agent

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"nano-code-go/internal/domain"
)

type queuedModel struct {
	generateResults []domain.GenerateTextResult
	streamChunks    []domain.StreamChunk
	seenMessages    [][]domain.Message
}

func (m *queuedModel) Generate(_ context.Context, params domain.GenerateParams) (domain.GenerateTextResult, error) {
	m.seenMessages = append(m.seenMessages, append([]domain.Message(nil), params.Messages...))
	if len(m.generateResults) == 0 {
		return domain.GenerateTextResult{}, errors.New("unexpected Generate call")
	}
	result := m.generateResults[0]
	m.generateResults = m.generateResults[1:]
	return result, nil
}

func (m *queuedModel) Stream(context.Context, domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	ch := make(chan domain.StreamChunk, len(m.streamChunks))
	for _, chunk := range m.streamChunks {
		ch <- chunk
	}
	close(ch)
	return ch, nil
}

func result(text string, toolCalls ...domain.ToolCall) domain.GenerateTextResult {
	finishReason := domain.FinishReasonStop
	if len(toolCalls) > 0 {
		finishReason = domain.FinishReasonToolCall
	}
	return domain.GenerateTextResult{
		Text:         text,
		FinishReason: finishReason,
		ToolCalls:    toolCalls,
	}
}

func TestAgent_GenerateExecutesApprovedTool(t *testing.T) {
	t.Parallel()

	model := &queuedModel{
		generateResults: []domain.GenerateTextResult{
			result("need tool", domain.ToolCall{
				ToolCallID: "call-1",
				Name:       "echo",
				Args:       map[string]any{"value": "hello"},
			}),
			result("done"),
		},
	}

	var toolCalls []map[string]any
	tool := domain.Tool{
		Name:          "echo",
		Description:   "Echoes a value",
		NeedsApproval: true,
		Parameters:    domain.ToolParameters{Type: "object"},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			toolCalls = append(toolCalls, args)
			return "echo:" + args["value"].(string), nil
		},
	}

	agent := New(Config{
		Name:         "test-agent",
		Instructions: "You are a test agent.",
		Model:        model,
		Tools:        []domain.Tool{tool},
		MaxSteps:     3,
		Approval: func(context.Context, string, map[string]any) (bool, error) {
			return true, nil
		},
	})

	got, err := agent.Generate(context.Background(), "run")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got.Text != "done" {
		t.Fatalf("Generate().Text = %q, want done", got.Text)
	}
	if !reflect.DeepEqual(toolCalls, []map[string]any{{"value": "hello"}}) {
		t.Fatalf("tool calls = %#v", toolCalls)
	}

	secondCallMessages := model.seenMessages[1]
	wantLast := domain.Message{
		Role:       domain.MessageRoleTool,
		ToolCallID: "call-1",
		Name:       "echo",
		Content:    "echo:hello",
	}
	if !reflect.DeepEqual(secondCallMessages[len(secondCallMessages)-1], wantLast) {
		t.Fatalf("last message = %#v, want %#v", secondCallMessages[len(secondCallMessages)-1], wantLast)
	}
}

func TestAgent_GenerateHandlesDeniedTool(t *testing.T) {
	t.Parallel()

	model := &queuedModel{
		generateResults: []domain.GenerateTextResult{
			result("need tool", domain.ToolCall{ToolCallID: "call-1", Name: "writeFile", Args: map[string]any{}}),
			result("done"),
		},
	}
	tool := domain.Tool{
		Name:          "writeFile",
		NeedsApproval: true,
		Execute: func(context.Context, map[string]any) (string, error) {
			t.Fatal("denied tool should not execute")
			return "", nil
		},
	}

	agent := New(Config{
		Model:    model,
		Tools:    []domain.Tool{tool},
		MaxSteps: 2,
		Approval: func(context.Context, string, map[string]any) (bool, error) {
			return false, nil
		},
	})

	if _, err := agent.Generate(context.Background(), "run"); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	last := model.seenMessages[1][len(model.seenMessages[1])-1]
	if got, want := last.Content, "Tool call denied by user."; got != want {
		t.Fatalf("denied tool content = %q, want %q", got, want)
	}
}

func TestAgent_GenerateHandlesMissingAndFailingTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		toolCall    domain.ToolCall
		tools       []domain.Tool
		wantContent string
	}{
		{
			name:        "missing tool",
			toolCall:    domain.ToolCall{ToolCallID: "call-1", Name: "missing"},
			wantContent: "Error: Tool not found - missing",
		},
		{
			name:     "tool error",
			toolCall: domain.ToolCall{ToolCallID: "call-1", Name: "boom"},
			tools: []domain.Tool{{
				Name: "boom",
				Execute: func(context.Context, map[string]any) (string, error) {
					return "", errors.New("failed")
				},
			}},
			wantContent: "Error executing tool boom: failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := &queuedModel{
				generateResults: []domain.GenerateTextResult{
					result("need tool", tt.toolCall),
					result("done"),
				},
			}
			agent := New(Config{Model: model, Tools: tt.tools, MaxSteps: 2})

			if _, err := agent.Generate(context.Background(), "run"); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			last := model.seenMessages[1][len(model.seenMessages[1])-1]
			if got := last.Content; got != tt.wantContent {
				t.Fatalf("tool message content = %q, want %q", got, tt.wantContent)
			}
		})
	}
}

func TestAgent_GenerateStopsAtMaxSteps(t *testing.T) {
	t.Parallel()

	model := &queuedModel{
		generateResults: []domain.GenerateTextResult{
			result("step 1", domain.ToolCall{ToolCallID: "call-1", Name: "echo"}),
			result("step 2", domain.ToolCall{ToolCallID: "call-2", Name: "echo"}),
		},
	}
	tool := domain.Tool{
		Name: "echo",
		Execute: func(context.Context, map[string]any) (string, error) {
			return "ok", nil
		},
	}
	agent := New(Config{Model: model, Tools: []domain.Tool{tool}, MaxSteps: 2})

	got, err := agent.Generate(context.Background(), "run")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got.Text != "step 2" {
		t.Fatalf("Generate().Text = %q, want step 2", got.Text)
	}
	if len(model.seenMessages) != 2 {
		t.Fatalf("model calls = %d, want 2", len(model.seenMessages))
	}
}

func TestAgent_GenerateUsesStreaming(t *testing.T) {
	t.Parallel()

	model := &queuedModel{
		streamChunks: []domain.StreamChunk{
			{Kind: domain.StreamKindDelta, Text: "hello"},
			{Kind: domain.StreamKindDelta, Text: " world"},
			{Kind: domain.StreamKindDone, FinishReason: domain.FinishReasonStop},
		},
	}
	var output bytes.Buffer
	agent := New(Config{
		Model:        model,
		UseStreaming: true,
		Output:       &output,
		MaxSteps:     1,
	})

	got, err := agent.Generate(context.Background(), "run")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got.Text != "hello world" {
		t.Fatalf("Generate().Text = %q, want hello world", got.Text)
	}
	if output.String() != "hello world" {
		t.Fatalf("stream output = %q, want hello world", output.String())
	}
}

func TestAgent_GenerateCompressesOldToolResults(t *testing.T) {
	t.Parallel()

	longResult := strings.Repeat("x", 10000)
	model := &queuedModel{
		generateResults: []domain.GenerateTextResult{
			result("step 1", domain.ToolCall{ToolCallID: "call-1", Name: "echo"}),
			result("step 2", domain.ToolCall{ToolCallID: "call-2", Name: "echo"}),
			result("step 3", domain.ToolCall{ToolCallID: "call-3", Name: "echo"}),
			result("step 4", domain.ToolCall{ToolCallID: "call-4", Name: "echo"}),
			result("done"),
		},
	}
	tool := domain.Tool{
		Name: "echo",
		Execute: func(context.Context, map[string]any) (string, error) {
			return longResult, nil
		},
	}
	agent := New(Config{Model: model, Tools: []domain.Tool{tool}, MaxSteps: 5})

	if _, err := agent.Generate(context.Background(), "run"); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var compressed bool
	for _, message := range model.seenMessages[len(model.seenMessages)-1] {
		if strings.HasPrefix(message.Content, "(Previous tool execution results were omitted:") {
			compressed = true
			break
		}
	}
	if !compressed {
		t.Fatalf("expected old tool result to be compressed in final model call")
	}
}
