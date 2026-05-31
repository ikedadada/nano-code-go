package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"nano-code-go/internal/domain"
)

type AnthropicProvider struct {
	modelID string
	apiKey  string
	baseURL string
	client  HTTPDoer
}

func NewAnthropic(modelID string, config Config) *AnthropicProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	return &AnthropicProvider{
		modelID: modelID,
		apiKey:  config.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  httpClient(config.Client),
	}
}

func (p *AnthropicProvider) Generate(ctx context.Context, params domain.GenerateParams) (domain.GenerateTextResult, error) {
	request := map[string]any{
		"model":      p.modelID,
		"system":     anthropicSystem(params.Messages),
		"messages":   anthropicMessages(params.Messages),
		"max_tokens": 4096,
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
				"name":         tool.Name,
				"description":  tool.Description,
				"input_schema": toolSchema(tool),
			})
		}
		request["tools"] = tools
	}

	var response struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := postJSON(ctx, p.client, p.baseURL+"/messages", map[string]string{
		"x-api-key":         p.apiKey,
		"anthropic-version": "2023-06-01",
	}, request, &response, "anthropic"); err != nil {
		return domain.GenerateTextResult{}, err
	}

	var text strings.Builder
	var toolCalls []domain.ToolCall
	for _, block := range response.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, domain.ToolCall{ToolCallID: block.ID, Name: block.Name, Args: block.Input})
		}
	}
	return domain.GenerateTextResult{
		Text:         text.String(),
		FinishReason: anthropicFinishReason(response.StopReason),
		ToolCalls:    toolCalls,
		Usage: domain.Usage{
			PromptTokens:     response.Usage.InputTokens,
			CompletionTokens: response.Usage.OutputTokens,
			TotalTokens:      response.Usage.InputTokens + response.Usage.OutputTokens,
		},
	}, nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, params domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	request := map[string]any{
		"model":      p.modelID,
		"system":     anthropicSystem(params.Messages),
		"messages":   anthropicMessages(params.Messages),
		"max_tokens": 4096,
		"stream":     true,
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
				"name":         tool.Name,
				"description":  tool.Description,
				"input_schema": toolSchema(tool),
			})
		}
		request["tools"] = tools
	}

	decoder := &anthropicStreamDecoder{
		toolCalls:      map[string]domain.ToolCall{},
		partialJSON:    map[string]string{},
		contentIndexID: map[int]string{},
	}
	return streamJSON(ctx, p.client, p.baseURL+"/messages", map[string]string{
		"x-api-key":         p.apiKey,
		"anthropic-version": "2023-06-01",
	}, request, "anthropic", decoder.decode)
}

type anthropicStreamDecoder struct {
	toolCalls      map[string]domain.ToolCall
	partialJSON    map[string]string
	contentIndexID map[int]string
}

func (d *anthropicStreamDecoder) decode(data []byte) ([]domain.StreamChunk, bool, error) {
	var event struct {
		Type         string `json:"type"`
		Index        int    `json:"index"`
		ContentBlock struct {
			Type  string         `json:"type"`
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		} `json:"content_block"`
		Delta struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
			StopReason  string `json:"stop_reason"`
		} `json:"delta"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, false, fmt.Errorf("decode anthropic stream event: %w", err)
	}

	switch event.Type {
	case "content_block_start":
		if event.ContentBlock.Type == "tool_use" {
			id := event.ContentBlock.ID
			d.contentIndexID[event.Index] = id
			d.toolCalls[id] = domain.ToolCall{ToolCallID: id, Name: event.ContentBlock.Name, Args: event.ContentBlock.Input}
			d.partialJSON[id] = ""
		}
	case "content_block_delta":
		switch event.Delta.Type {
		case "text_delta":
			if event.Delta.Text != "" {
				return []domain.StreamChunk{{Kind: domain.StreamKindDelta, Text: event.Delta.Text}}, false, nil
			}
		case "input_json_delta":
			id := d.contentIndexID[event.Index]
			if id != "" {
				d.partialJSON[id] += event.Delta.PartialJSON
				if args, err := parseJSONObject(d.partialJSON[id]); err == nil {
					call := d.toolCalls[id]
					call.Args = args
					d.toolCalls[id] = call
				}
			}
		}
	case "message_delta":
		done := domain.StreamChunk{
			Kind:         domain.StreamKindDone,
			FinishReason: anthropicFinishReason(event.Delta.StopReason),
			ToolCalls:    d.toolCallResults(),
		}
		if event.Usage != nil {
			done.Usage = domain.Usage{
				PromptTokens:     event.Usage.InputTokens,
				CompletionTokens: event.Usage.OutputTokens,
				TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
			}
		}
		return []domain.StreamChunk{done}, false, nil
	case "message_stop":
		return nil, true, nil
	}
	return nil, false, nil
}

func (d *anthropicStreamDecoder) toolCallResults() []domain.ToolCall {
	result := make([]domain.ToolCall, 0, len(d.toolCalls))
	for _, call := range d.toolCalls {
		if call.Args == nil {
			call.Args = map[string]any{}
		}
		result = append(result, call)
	}
	return result
}

func anthropicSystem(messages []domain.Message) []map[string]any {
	var system []map[string]any
	for _, message := range messages {
		if message.Role == domain.MessageRoleSystem {
			system = append(system, map[string]any{"type": "text", "text": message.Content})
		}
	}
	return system
}

func anthropicMessages(messages []domain.Message) []map[string]any {
	var result []map[string]any
	for _, message := range messages {
		switch message.Role {
		case domain.MessageRoleSystem:
			continue
		case domain.MessageRoleTool:
			result = append(result, map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type":        "tool_result",
					"tool_use_id": message.ToolCallID,
					"content":     message.Content,
				}},
			})
		case domain.MessageRoleAssistant:
			if message.ToolCalls != nil {
				content := []map[string]any{}
				if message.Content != "" {
					content = append(content, map[string]any{"type": "text", "text": message.Content})
				}
				for _, call := range message.ToolCalls {
					content = append(content, map[string]any{
						"type":  "tool_use",
						"id":    call.ToolCallID,
						"name":  call.Name,
						"input": call.Args,
					})
				}
				result = append(result, map[string]any{"role": "assistant", "content": content})
				continue
			}
			result = append(result, map[string]any{"role": "assistant", "content": message.Content})
		default:
			result = append(result, map[string]any{"role": string(message.Role), "content": message.Content})
		}
	}
	return result
}

func anthropicFinishReason(reason string) domain.FinishReason {
	switch reason {
	case "end_turn":
		return domain.FinishReasonStop
	case "max_tokens":
		return domain.FinishReasonLength
	case "tool_use":
		return domain.FinishReasonToolCall
	case "refusal":
		return domain.FinishReasonContentFilter
	default:
		return domain.FinishReasonStop
	}
}
