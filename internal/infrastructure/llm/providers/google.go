package providers

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"nano-code-go/internal/domain"
)

type GoogleProvider struct {
	modelID string
	apiKey  string
	baseURL string
	client  *genai.Client
	initErr error
}

func NewGoogle(modelID string, config Config) *GoogleProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	clientConfig := &genai.ClientConfig{
		APIKey:     config.APIKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: config.Client,
	}
	if config.BaseURL != "" {
		clientConfig.HTTPOptions = googleHTTPOptions(config.BaseURL)
	}
	client, err := genai.NewClient(context.Background(), clientConfig)
	return &GoogleProvider{
		modelID: modelID,
		apiKey:  config.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
		initErr: err,
	}
}

func (p *GoogleProvider) Generate(ctx context.Context, params domain.GenerateParams) (domain.GenerateTextResult, error) {
	if p.initErr != nil {
		return domain.GenerateTextResult{}, fmt.Errorf("initialize google client: %w", p.initErr)
	}
	response, err := p.client.Models.GenerateContent(ctx, p.modelID, googleMessages(params.Messages), googleConfig(params))
	if err != nil {
		return domain.GenerateTextResult{}, sdkError("google", err)
	}
	if len(response.Candidates) == 0 {
		return domain.GenerateTextResult{}, &domain.LLMAPIError{Status: 500, Provider: "google", Message: "No candidates returned from Google API", Raw: response}
	}

	candidate := response.Candidates[0]
	toolCalls := make([]domain.ToolCall, 0, len(response.FunctionCalls()))
	for _, call := range response.FunctionCalls() {
		name := call.Name
		if name == "" {
			name = "unknown_tool"
		}
		id := call.ID
		if id == "" {
			id = fmt.Sprintf("call_%d", len(toolCalls))
		}
		toolCalls = append(toolCalls, domain.ToolCall{ToolCallID: id, Name: name, Args: call.Args})
	}

	return domain.GenerateTextResult{
		Text:         response.Text(),
		FinishReason: googleFinishReason(string(candidate.FinishReason), len(toolCalls) > 0),
		ToolCalls:    toolCalls,
		Usage:        googleUsage(response.UsageMetadata),
	}, nil
}

func (p *GoogleProvider) Stream(ctx context.Context, params domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	if p.initErr != nil {
		return nil, fmt.Errorf("initialize google client: %w", p.initErr)
	}
	stream := p.client.Models.GenerateContentStream(ctx, p.modelID, googleMessages(params.Messages), googleConfig(params))

	chunks := make(chan domain.StreamChunk)
	go func() {
		defer close(chunks)

		decoder := &googleStreamDecoder{}
		for response, err := range stream {
			if err != nil {
				sendStreamChunk(ctx, chunks, domain.StreamChunk{Err: sdkError("google", err)})
				return
			}
			decoded, err := decoder.decode(response)
			if err != nil {
				sendStreamChunk(ctx, chunks, domain.StreamChunk{Err: err})
				return
			}
			for _, chunk := range decoded {
				if !sendStreamChunk(ctx, chunks, chunk) {
					return
				}
			}
		}
	}()
	return chunks, nil
}

type googleStreamDecoder struct {
	toolCalls []domain.ToolCall
}

func (d *googleStreamDecoder) decode(event *genai.GenerateContentResponse) ([]domain.StreamChunk, error) {
	var chunks []domain.StreamChunk
	var finishReason domain.FinishReason
	if len(event.Candidates) > 0 {
		candidate := event.Candidates[0]
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					chunks = append(chunks, domain.StreamChunk{Kind: domain.StreamKindDelta, Text: part.Text})
				}
				if part.FunctionCall != nil {
					name := part.FunctionCall.Name
					if name == "" {
						name = "unknown_tool"
					}
					id := part.FunctionCall.ID
					if id == "" {
						id = name
					}
					d.toolCalls = append(d.toolCalls, domain.ToolCall{ToolCallID: id, Name: name, Args: part.FunctionCall.Args})
				}
			}
		}
		if candidate.FinishReason != "" {
			finishReason = googleFinishReason(string(candidate.FinishReason), len(d.toolCalls) > 0)
		}
	}
	if finishReason != "" || event.UsageMetadata != nil {
		done := domain.StreamChunk{Kind: domain.StreamKindDone, FinishReason: finishReason, ToolCalls: d.toolCallResults()}
		if event.UsageMetadata != nil {
			done.Usage = googleUsage(event.UsageMetadata)
		}
		chunks = append(chunks, done)
	}
	return chunks, nil
}

func (d *googleStreamDecoder) toolCallResults() []domain.ToolCall {
	result := make([]domain.ToolCall, 0, len(d.toolCalls))
	for _, call := range d.toolCalls {
		if call.Args == nil {
			call.Args = map[string]any{}
		}
		result = append(result, call)
	}
	return result
}

func googleConfig(params domain.GenerateParams) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{
		SystemInstruction: googleSystemInstruction(params.Messages),
	}
	if params.Temperature != nil {
		temperature := float32(*params.Temperature)
		config.Temperature = &temperature
	}
	if params.MaxTokens != nil {
		config.MaxOutputTokens = int32(*params.MaxTokens)
	}
	if len(params.Tools) > 0 {
		config.Tools = googleTools(params.Tools)
	}
	return config
}

func googleSystemInstruction(messages []domain.Message) *genai.Content {
	var parts []string
	for _, message := range messages {
		if message.Role == domain.MessageRoleSystem {
			parts = append(parts, message.Content)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return genai.NewContentFromText(strings.Join(parts, "\n"), genai.RoleUser)
}

func googleMessages(messages []domain.Message) []*genai.Content {
	var result []*genai.Content
	for _, message := range messages {
		switch message.Role {
		case domain.MessageRoleSystem:
			continue
		case domain.MessageRoleTool:
			result = append(result, genai.NewContentFromParts([]*genai.Part{
				genai.NewPartFromFunctionResponse(message.Name, map[string]any{
					"result": map[string]any{"result": message.Content},
				}),
			}, genai.RoleUser))
		case domain.MessageRoleAssistant:
			parts := []*genai.Part{}
			if message.Content != "" {
				parts = append(parts, genai.NewPartFromText(message.Content))
			}
			for _, call := range message.ToolCalls {
				parts = append(parts, genai.NewPartFromFunctionCall(call.Name, call.Args))
			}
			result = append(result, genai.NewContentFromParts(parts, genai.RoleModel))
		default:
			result = append(result, genai.NewContentFromText(message.Content, genai.RoleUser))
		}
	}
	return result
}

func googleTools(tools []domain.Tool) []*genai.Tool {
	declarations := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		declarations = append(declarations, &genai.FunctionDeclaration{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParametersJsonSchema: googleToolSchema(tool),
		})
	}
	return []*genai.Tool{{FunctionDeclarations: declarations}}
}

func googleToolSchema(tool domain.Tool) map[string]any {
	schema := map[string]any{"type": tool.Parameters.Type}
	if tool.Parameters.Properties != nil {
		schema["properties"] = tool.Parameters.Properties
	}
	if tool.Parameters.Required != nil {
		schema["required"] = tool.Parameters.Required
	}
	return schema
}

func googleUsage(usage *genai.GenerateContentResponseUsageMetadata) domain.Usage {
	if usage == nil {
		return domain.Usage{}
	}
	return domain.Usage{
		PromptTokens:     int(usage.PromptTokenCount),
		CompletionTokens: int(usage.CandidatesTokenCount),
		TotalTokens:      int(usage.TotalTokenCount),
	}
}

func googleHTTPOptions(baseURL string) genai.HTTPOptions {
	baseURL = strings.TrimRight(baseURL, "/")
	version := ""
	for _, suffix := range []string{"/v1beta", "/v1"} {
		if strings.HasSuffix(baseURL, suffix) {
			version = strings.TrimPrefix(suffix, "/")
			baseURL = strings.TrimSuffix(baseURL, suffix)
			break
		}
	}
	options := genai.HTTPOptions{BaseURL: baseURL}
	if version != "" {
		options.APIVersion = version
	}
	return options
}

func googleFinishReason(reason string, hasFunctionCall bool) domain.FinishReason {
	if hasFunctionCall {
		return domain.FinishReasonToolCall
	}
	switch reason {
	case "STOP":
		return domain.FinishReasonStop
	case "MAX_TOKENS":
		return domain.FinishReasonLength
	case "SAFETY":
		return domain.FinishReasonContentFilter
	default:
		return domain.FinishReasonStop
	}
}
