package domain

import (
	"context"
)

// ToolParameters is the JSON Schema object used by providers to describe a
// tool's accepted input.
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
