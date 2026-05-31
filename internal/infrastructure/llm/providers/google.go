package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nano-code-go/internal/domain"
)

type GoogleProvider struct {
	modelID string
	apiKey  string
	baseURL string
	client  HTTPDoer
}

func NewGoogle(modelID string, config Config) *GoogleProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &GoogleProvider{
		modelID: modelID,
		apiKey:  config.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  httpClient(config.Client),
	}
}

func (p *GoogleProvider) Generate(ctx context.Context, params domain.GenerateParams) (domain.GenerateTextResult, error) {
	config := map[string]any{
		"systemInstruction": googleSystemInstruction(params.Messages),
	}
	if params.Temperature != nil {
		config["temperature"] = *params.Temperature
	}
	if params.MaxTokens != nil {
		config["maxOutputTokens"] = *params.MaxTokens
	}
	if len(params.Tools) > 0 {
		var declarations []map[string]any
		for _, tool := range params.Tools {
			declarations = append(declarations, map[string]any{
				"name":                 tool.Name,
				"description":          tool.Description,
				"parametersJsonSchema": toolSchema(tool),
			})
		}
		config["tools"] = []map[string]any{{"functionDeclarations": declarations}}
	}
	request := map[string]any{
		"contents": googleMessages(params.Messages),
		"config":   config,
	}

	var response struct {
		Candidates []struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string         `json:"name"`
						Args map[string]any `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, p.modelID, p.apiKey)
	if err := postJSON(ctx, p.client, endpoint, nil, request, &response, "google"); err != nil {
		return domain.GenerateTextResult{}, err
	}
	if len(response.Candidates) == 0 {
		return domain.GenerateTextResult{}, &domain.LLMAPIError{Status: 500, Provider: "google", Message: "No candidates returned from Google API", Raw: response}
	}
	candidate := response.Candidates[0]
	var text strings.Builder
	var toolCalls []domain.ToolCall
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			text.WriteString(part.Text)
		}
		if part.FunctionCall != nil {
			name := part.FunctionCall.Name
			if name == "" {
				name = "unknown_tool"
			}
			toolCalls = append(toolCalls, domain.ToolCall{
				ToolCallID: fmt.Sprintf("call_%d", len(toolCalls)),
				Name:       name,
				Args:       part.FunctionCall.Args,
			})
		}
	}

	return domain.GenerateTextResult{
		Text:         text.String(),
		FinishReason: googleFinishReason(candidate.FinishReason, len(toolCalls) > 0),
		ToolCalls:    toolCalls,
		Usage: domain.Usage{
			PromptTokens:     response.UsageMetadata.PromptTokenCount,
			CompletionTokens: response.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      response.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (p *GoogleProvider) Stream(ctx context.Context, params domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	config := map[string]any{
		"systemInstruction": googleSystemInstruction(params.Messages),
	}
	if params.Temperature != nil {
		config["temperature"] = *params.Temperature
	}
	if params.MaxTokens != nil {
		config["maxOutputTokens"] = *params.MaxTokens
	}
	if len(params.Tools) > 0 {
		var declarations []map[string]any
		for _, tool := range params.Tools {
			declarations = append(declarations, map[string]any{
				"name":                 tool.Name,
				"description":          tool.Description,
				"parametersJsonSchema": toolSchema(tool),
			})
		}
		config["tools"] = []map[string]any{{"functionDeclarations": declarations}}
	}
	request := map[string]any{
		"contents": googleMessages(params.Messages),
		"config":   config,
	}

	decoder := &googleStreamDecoder{}
	endpoint := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", p.baseURL, p.modelID, p.apiKey)
	return streamJSON(ctx, p.client, endpoint, nil, request, "google", decoder.decode)
}

type googleStreamDecoder struct {
	toolCalls map[string]domain.ToolCall
}

func (d *googleStreamDecoder) decode(data []byte) ([]domain.StreamChunk, bool, error) {
	var event struct {
		Candidates []struct {
			FinishReason string `json:"finishReason"`
			Content      struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall *struct {
						Name string         `json:"name"`
						Args map[string]any `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata *struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, false, fmt.Errorf("decode google stream event: %w", err)
	}

	var chunks []domain.StreamChunk
	var finishReason domain.FinishReason
	if len(event.Candidates) > 0 {
		candidate := event.Candidates[0]
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				chunks = append(chunks, domain.StreamChunk{Kind: domain.StreamKindDelta, Text: part.Text})
			}
			if part.FunctionCall != nil {
				if d.toolCalls == nil {
					d.toolCalls = map[string]domain.ToolCall{}
				}
				name := part.FunctionCall.Name
				if name == "" {
					name = "unknown_tool"
				}
				d.toolCalls[name] = domain.ToolCall{ToolCallID: name, Name: name, Args: part.FunctionCall.Args}
			}
		}
		if candidate.FinishReason != "" {
			finishReason = googleFinishReason(candidate.FinishReason, len(d.toolCalls) > 0)
		}
	}
	if finishReason != "" || event.UsageMetadata != nil {
		done := domain.StreamChunk{Kind: domain.StreamKindDone, FinishReason: finishReason, ToolCalls: d.toolCallResults()}
		if event.UsageMetadata != nil {
			done.Usage = domain.Usage{
				PromptTokens:     event.UsageMetadata.PromptTokenCount,
				CompletionTokens: event.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      event.UsageMetadata.TotalTokenCount,
			}
		}
		chunks = append(chunks, done)
	}
	return chunks, false, nil
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

func googleSystemInstruction(messages []domain.Message) string {
	var parts []string
	for _, message := range messages {
		if message.Role == domain.MessageRoleSystem {
			parts = append(parts, message.Content)
		}
	}
	return strings.Join(parts, "\n")
}

func googleMessages(messages []domain.Message) []map[string]any {
	var result []map[string]any
	for _, message := range messages {
		switch message.Role {
		case domain.MessageRoleSystem:
			continue
		case domain.MessageRoleTool:
			result = append(result, map[string]any{
				"role": "tool",
				"parts": []map[string]any{{
					"functionResponse": map[string]any{
						"name": message.Name,
						"response": map[string]any{
							"result": map[string]any{"result": message.Content},
						},
					},
				}},
			})
		case domain.MessageRoleAssistant:
			parts := []map[string]any{}
			if message.Content != "" {
				parts = append(parts, map[string]any{"text": message.Content})
			}
			for _, call := range message.ToolCalls {
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": call.Name,
						"args": call.Args,
					},
				})
			}
			result = append(result, map[string]any{"role": "model", "parts": parts})
		default:
			result = append(result, map[string]any{
				"role":  string(message.Role),
				"parts": []map[string]any{{"text": message.Content}},
			})
		}
	}
	return result
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
