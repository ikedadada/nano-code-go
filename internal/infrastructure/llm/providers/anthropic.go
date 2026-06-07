package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"

	"nano-code-go/internal/domain"
)

type AnthropicProvider struct {
	modelID string
	client  anthropic.Client
}

func NewAnthropic(modelID string, config Config) *AnthropicProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")

	options := []anthropicoption.RequestOption{
		anthropicoption.WithAPIKey(config.APIKey),
		anthropicoption.WithBaseURL(baseURL),
	}
	if config.Client != nil {
		options = append(options, anthropicoption.WithHTTPClient(config.Client))
	}

	return &AnthropicProvider{
		modelID: modelID,
		client:  anthropic.NewClient(options...),
	}
}

func (p *AnthropicProvider) Generate(ctx context.Context, params domain.GenerateParams) (domain.GenerateTextResult, error) {
	response, err := p.client.Messages.New(ctx, p.requestParams(params))
	if err != nil {
		return domain.GenerateTextResult{}, sdkError("anthropic", err)
	}

	var text strings.Builder
	var toolCalls []domain.ToolCall
	for _, block := range response.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			args, err := rawJSONObject(block.Input)
			if err != nil {
				return domain.GenerateTextResult{}, err
			}
			toolCalls = append(toolCalls, domain.ToolCall{ToolCallID: block.ID, Name: block.Name, Args: args})
		}
	}
	return domain.GenerateTextResult{
		Text:         text.String(),
		FinishReason: anthropicFinishReason(string(response.StopReason)),
		ToolCalls:    toolCalls,
		Usage: domain.Usage{
			PromptTokens:     int(response.Usage.InputTokens),
			CompletionTokens: int(response.Usage.OutputTokens),
			TotalTokens:      int(response.Usage.InputTokens + response.Usage.OutputTokens),
		},
	}, nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, params domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	stream := p.client.Messages.NewStreaming(ctx, p.requestParams(params))

	chunks := make(chan domain.StreamChunk)
	go func() {
		defer close(chunks)
		defer stream.Close()

		decoder := &anthropicStreamDecoder{
			toolCalls:      map[string]domain.ToolCall{},
			partialJSON:    map[string]string{},
			contentIndexID: map[int]string{},
		}
		for stream.Next() {
			decoded, done, err := decoder.decode(stream.Current())
			if err != nil {
				sendStreamChunk(ctx, chunks, domain.StreamChunk{Err: err})
				return
			}
			for _, chunk := range decoded {
				if !sendStreamChunk(ctx, chunks, chunk) {
					return
				}
			}
			if done {
				return
			}
		}
		if err := stream.Err(); err != nil {
			sendStreamChunk(ctx, chunks, domain.StreamChunk{Err: sdkError("anthropic", err)})
		}
	}()
	return chunks, nil
}

func (p *AnthropicProvider) requestParams(params domain.GenerateParams) anthropic.MessageNewParams {
	maxTokens := int64(4096)
	if params.MaxTokens != nil {
		maxTokens = int64(*params.MaxTokens)
	}
	request := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.modelID),
		System:    anthropicSystem(params.Messages),
		Messages:  anthropicMessages(params.Messages),
		MaxTokens: maxTokens,
	}
	if params.Temperature != nil {
		request.Temperature = anthropic.Float(*params.Temperature)
	}
	if len(params.Tools) > 0 {
		request.Tools = anthropicTools(params.Tools)
	}
	return request
}

type anthropicStreamDecoder struct {
	toolCalls       map[string]domain.ToolCall
	toolCallIndexes []int
	partialJSON     map[string]string
	contentIndexID  map[int]string
}

func (d *anthropicStreamDecoder) decode(event anthropic.MessageStreamEventUnion) ([]domain.StreamChunk, bool, error) {
	switch event.Type {
	case "content_block_start":
		if event.ContentBlock.Type == "tool_use" {
			index := int(event.Index)
			id := event.ContentBlock.ID
			d.contentIndexID[index] = id
			args, err := objectFromAny(event.ContentBlock.Input)
			if err != nil {
				return nil, false, err
			}
			if _, exists := d.toolCalls[id]; !exists {
				d.toolCallIndexes = append(d.toolCallIndexes, index)
			}
			d.toolCalls[id] = domain.ToolCall{ToolCallID: id, Name: event.ContentBlock.Name, Args: args}
			d.partialJSON[id] = ""
		}
	case "content_block_delta":
		switch event.Delta.Type {
		case "text_delta":
			if event.Delta.Text != "" {
				return []domain.StreamChunk{{Kind: domain.StreamKindDelta, Text: event.Delta.Text}}, false, nil
			}
		case "input_json_delta":
			id := d.contentIndexID[int(event.Index)]
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
			FinishReason: anthropicFinishReason(string(event.Delta.StopReason)),
			ToolCalls:    d.toolCallResults(),
		}
		done.Usage = domain.Usage{
			PromptTokens:     int(event.Usage.InputTokens),
			CompletionTokens: int(event.Usage.OutputTokens),
			TotalTokens:      int(event.Usage.InputTokens + event.Usage.OutputTokens),
		}
		return []domain.StreamChunk{done}, false, nil
	case "message_stop":
		return nil, true, nil
	}
	return nil, false, nil
}

func (d *anthropicStreamDecoder) toolCallResults() []domain.ToolCall {
	result := make([]domain.ToolCall, 0, len(d.toolCalls))
	indexes := append([]int(nil), d.toolCallIndexes...)
	sort.Ints(indexes)
	for _, index := range indexes {
		id := d.contentIndexID[index]
		call, ok := d.toolCalls[id]
		if !ok {
			continue
		}
		if call.Args == nil {
			call.Args = map[string]any{}
		}
		result = append(result, call)
	}
	return result
}

func anthropicSystem(messages []domain.Message) []anthropic.TextBlockParam {
	var system []anthropic.TextBlockParam
	for _, message := range messages {
		if message.Role == domain.MessageRoleSystem {
			system = append(system, anthropic.TextBlockParam{Text: message.Content})
		}
	}
	return system
}

func anthropicMessages(messages []domain.Message) []anthropic.MessageParam {
	var result []anthropic.MessageParam
	for _, message := range messages {
		switch message.Role {
		case domain.MessageRoleSystem:
			continue
		case domain.MessageRoleTool:
			result = append(result, anthropic.NewUserMessage(anthropic.NewToolResultBlock(message.ToolCallID, message.Content, false)))
		case domain.MessageRoleAssistant:
			if message.ToolCalls != nil {
				content := []anthropic.ContentBlockParamUnion{}
				if message.Content != "" {
					content = append(content, anthropic.NewTextBlock(message.Content))
				}
				for _, call := range message.ToolCalls {
					content = append(content, anthropic.NewToolUseBlock(call.ToolCallID, call.Args, call.Name))
				}
				result = append(result, anthropic.NewAssistantMessage(content...))
				continue
			}
			result = append(result, anthropic.NewAssistantMessage(anthropic.NewTextBlock(message.Content)))
		default:
			result = append(result, anthropic.NewUserMessage(anthropic.NewTextBlock(message.Content)))
		}
	}
	return result
}

func anthropicTools(tools []domain.Tool) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		converted := anthropic.ToolUnionParamOfTool(anthropic.ToolInputSchemaParam{
			Properties: tool.Parameters.Properties,
			Required:   tool.Parameters.Required,
		}, tool.Name)
		converted.OfTool.Description = anthropic.String(tool.Description)
		result = append(result, converted)
	}
	return result
}

func anthropicFinishReason(reason string) domain.FinishReason {
	switch reason {
	case "end_turn", "stop_sequence":
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

func rawJSONObject(raw json.RawMessage) (map[string]any, error) {
	return parseJSONObject(string(raw))
}

func objectFromAny(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode JSON object: %w", err)
	}
	return parseJSONObject(string(body))
}
