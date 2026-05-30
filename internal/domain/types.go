package domain

import (
	"context"
	"fmt"
)

type ToolParameters struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
}

type Tool struct {
	Name          string
	Description   string
	NeedsApproval bool
	Parameters    ToolParameters
	Execute       func(ctx context.Context, args map[string]any) (string, error)
}

type ToolCall struct {
	ToolCallID string
	Name       string
	Args       map[string]any
}

type ToolResult struct {
	ToolCallID string
	Result     string
}

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleSystem    MessageRole = "system"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"
)

type Message struct {
	Role       MessageRole
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonContentFilter FinishReason = "content_filter"
	FinishReasonToolCall      FinishReason = "tool_call"
	FinishReasonError         FinishReason = "error"
)

type GenerateTextResult struct {
	Text         string
	FinishReason FinishReason
	ToolCalls    []ToolCall
	Usage        Usage
}

type GenerateParams struct {
	Messages    []Message
	Tools       []Tool
	Temperature *float64
	MaxTokens   *int
}

type StreamKind string

const (
	StreamKindDelta StreamKind = "delta"
	StreamKindEvent StreamKind = "event"
	StreamKindDone  StreamKind = "done"
)

type StreamChunk struct {
	Kind         StreamKind
	Text         string
	FinishReason FinishReason
	ToolCalls    []ToolCall
	Usage        Usage
	Err          error
}

type LanguageModel interface {
	Generate(ctx context.Context, params GenerateParams) (GenerateTextResult, error)
	Stream(ctx context.Context, params GenerateParams) (<-chan StreamChunk, error)
}

type LLMAPIError struct {
	Status   int
	Provider string
	Code     string
	Message  string
	Raw      any
}

func (e *LLMAPIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("LLM API Error: %s responded with status %d", e.Provider, e.Status)
}
