package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nano-code-go/internal/domain"
)

type OpenAIProvider struct {
	modelID string
	apiKey  string
	baseURL string
	client  HTTPDoer
}

func NewOpenAI(modelID string, config Config) *OpenAIProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIProvider{
		modelID: modelID,
		apiKey:  config.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  httpClient(config.Client),
	}
}

func (p *OpenAIProvider) Generate(ctx context.Context, params domain.GenerateParams) (domain.GenerateTextResult, error) {
	request := map[string]any{
		"model":    p.modelID,
		"messages": openAIMessages(params.Messages),
	}
	if params.Temperature != nil {
		request["temperature"] = *params.Temperature
	}
	if params.MaxTokens != nil {
		request["max_tokens"] = *params.MaxTokens
	}
	if len(params.Tools) > 0 {
		tools := make([]map[string]any, 0, len(params.Tools))
		for _, tool := range params.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        tool.Name,
					"description": tool.Description,
					"parameters":  toolSchema(tool),
				},
			})
		}
		request["tools"] = tools
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/chat/completions", map[string]string{
		"Authorization": "Bearer " + p.apiKey,
	}, request, &response, "openai"); err != nil {
		return domain.GenerateTextResult{}, err
	}
	if len(response.Choices) == 0 {
		return domain.GenerateTextResult{}, &domain.LLMAPIError{Status: 500, Provider: "openai", Message: "No choices returned from OpenAI API", Raw: response}
	}

	choice := response.Choices[0]
	toolCalls := make([]domain.ToolCall, 0, len(choice.Message.ToolCalls))
	for _, call := range choice.Message.ToolCalls {
		if call.Type != "function" {
			continue
		}
		args, err := parseJSONObject(call.Function.Arguments)
		if err != nil {
			return domain.GenerateTextResult{}, err
		}
		toolCalls = append(toolCalls, domain.ToolCall{ToolCallID: call.ID, Name: call.Function.Name, Args: args})
	}
	return domain.GenerateTextResult{
		Text:         choice.Message.Content,
		FinishReason: openAIFinishReason(choice.FinishReason),
		ToolCalls:    toolCalls,
		Usage: domain.Usage{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.CompletionTokens,
			TotalTokens:      response.Usage.TotalTokens,
		},
	}, nil
}

func (p *OpenAIProvider) Stream(context.Context, domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	return unsupportedStream("openai")
}

func openAIMessages(messages []domain.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case domain.MessageRoleTool:
			result = append(result, map[string]any{
				"role":         "tool",
				"tool_call_id": message.ToolCallID,
				"content":      message.Content,
			})
		case domain.MessageRoleAssistant:
			converted := map[string]any{
				"role":    "assistant",
				"content": message.Content,
			}
			if message.ToolCalls != nil {
				var toolCalls []map[string]any
				for _, call := range message.ToolCalls {
					toolCalls = append(toolCalls, map[string]any{
						"id":   call.ToolCallID,
						"type": "function",
						"function": map[string]any{
							"name":      call.Name,
							"arguments": mustJSON(call.Args),
						},
					})
				}
				converted["tool_calls"] = toolCalls
			}
			result = append(result, converted)
		default:
			result = append(result, map[string]any{
				"role":    string(message.Role),
				"content": message.Content,
			})
		}
	}
	return result
}

func openAIFinishReason(reason string) domain.FinishReason {
	switch reason {
	case "stop":
		return domain.FinishReasonStop
	case "length":
		return domain.FinishReasonLength
	case "content_filter":
		return domain.FinishReasonContentFilter
	case "tool_calls", "function_call":
		return domain.FinishReasonToolCall
	default:
		return domain.FinishReasonStop
	}
}

func parseJSONObject(jsonText string) (map[string]any, error) {
	if jsonText == "" {
		jsonText = "{}"
	}
	var value any
	if err := json.Unmarshal([]byte(jsonText), &value); err != nil {
		return nil, fmt.Errorf("Invalid JSON: %w", err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON value must be an object")
	}
	return object, nil
}

func mustJSON(value any) string {
	body, _ := json.Marshal(value)
	return string(body)
}
