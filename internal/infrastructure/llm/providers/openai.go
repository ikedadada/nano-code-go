package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	openai "github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"nano-code-go/internal/domain"
)

type OpenAIProvider struct {
	modelID string
	apiKey  string
	baseURL string
	client  openai.Client
}

func NewOpenAI(modelID string, config Config) *OpenAIProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	options := []openaioption.RequestOption{
		openaioption.WithAPIKey(config.APIKey),
		openaioption.WithBaseURL(baseURL),
	}
	if config.Client != nil {
		options = append(options, openaioption.WithHTTPClient(config.Client))
	}

	return &OpenAIProvider{
		modelID: modelID,
		apiKey:  config.APIKey,
		baseURL: baseURL,
		client:  openai.NewClient(options...),
	}
}

func (p *OpenAIProvider) Generate(ctx context.Context, params domain.GenerateParams) (domain.GenerateTextResult, error) {
	response, err := p.client.Chat.Completions.New(ctx, p.requestParams(params))
	if err != nil {
		return domain.GenerateTextResult{}, sdkError("openai", err)
	}
	if len(response.Choices) == 0 {
		return domain.GenerateTextResult{}, &domain.LLMAPIError{Status: 500, Provider: "openai", Message: "No choices returned from OpenAI API", Raw: response}
	}

	choice := response.Choices[0]
	toolCalls := make([]domain.ToolCall, 0, len(choice.Message.ToolCalls))
	for _, call := range choice.Message.ToolCalls {
		function := call.AsFunction()
		if function.ID == "" && function.Function.Name == "" {
			continue
		}
		args, err := parseJSONObject(function.Function.Arguments)
		if err != nil {
			return domain.GenerateTextResult{}, err
		}
		toolCalls = append(toolCalls, domain.ToolCall{ToolCallID: function.ID, Name: function.Function.Name, Args: args})
	}

	return domain.GenerateTextResult{
		Text:         choice.Message.Content,
		FinishReason: openAIFinishReason(choice.FinishReason),
		ToolCalls:    toolCalls,
		Usage: domain.Usage{
			PromptTokens:     int(response.Usage.PromptTokens),
			CompletionTokens: int(response.Usage.CompletionTokens),
			TotalTokens:      int(response.Usage.TotalTokens),
		},
	}, nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, params domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	request := p.requestParams(params)
	request.StreamOptions.IncludeUsage = openai.Bool(true)
	stream := p.client.Chat.Completions.NewStreaming(ctx, request)

	chunks := make(chan domain.StreamChunk)
	go func() {
		defer close(chunks)
		defer stream.Close()

		decoder := &openAIStreamDecoder{}
		for stream.Next() {
			decoded, err := decoder.decode(stream.Current())
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
		if err := stream.Err(); err != nil {
			sendStreamChunk(ctx, chunks, domain.StreamChunk{Err: sdkError("openai", err)})
		}
	}()
	return chunks, nil
}

func (p *OpenAIProvider) requestParams(params domain.GenerateParams) openai.ChatCompletionNewParams {
	request := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(p.modelID),
		Messages: openAIMessages(params.Messages),
	}
	if params.Temperature != nil {
		request.Temperature = openai.Float(*params.Temperature)
	}
	if params.MaxTokens != nil {
		request.MaxTokens = openai.Int(int64(*params.MaxTokens))
	}
	if len(params.Tools) > 0 {
		request.Tools = openAITools(params.Tools)
	}
	return request
}

type openAIStreamDecoder struct {
	toolCalls map[int]*openAIStreamToolCall
}

type openAIStreamToolCall struct {
	id       string
	name     string
	argsText string
}

func (d *openAIStreamDecoder) decode(event openai.ChatCompletionChunk) ([]domain.StreamChunk, error) {
	var chunks []domain.StreamChunk
	var finishReason domain.FinishReason
	if len(event.Choices) > 0 {
		choice := event.Choices[0]
		if choice.Delta.Content != "" {
			chunks = append(chunks, domain.StreamChunk{Kind: domain.StreamKindDelta, Text: choice.Delta.Content})
		}
		if len(choice.Delta.ToolCalls) > 0 {
			if d.toolCalls == nil {
				d.toolCalls = map[int]*openAIStreamToolCall{}
			}
			for _, call := range choice.Delta.ToolCalls {
				index := int(call.Index)
				existing := d.toolCalls[index]
				if existing == nil {
					existing = &openAIStreamToolCall{}
					d.toolCalls[index] = existing
				}
				if call.ID != "" {
					existing.id = call.ID
				}
				if call.Function.Name != "" {
					existing.name = call.Function.Name
				}
				existing.argsText += call.Function.Arguments
			}
		}
		if choice.FinishReason != "" {
			finishReason = openAIFinishReason(choice.FinishReason)
		}
	}
	if finishReason != "" || event.Usage.JSON.TotalTokens.Valid() {
		done := domain.StreamChunk{Kind: domain.StreamKindDone, FinishReason: finishReason, ToolCalls: d.toolCallResults()}
		if event.Usage.JSON.TotalTokens.Valid() {
			done.Usage = domain.Usage{
				PromptTokens:     int(event.Usage.PromptTokens),
				CompletionTokens: int(event.Usage.CompletionTokens),
				TotalTokens:      int(event.Usage.TotalTokens),
			}
		}
		chunks = append(chunks, done)
	}
	return chunks, nil
}

func (d *openAIStreamDecoder) toolCallResults() []domain.ToolCall {
	indices := make([]int, 0, len(d.toolCalls))
	for index := range d.toolCalls {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	result := make([]domain.ToolCall, 0, len(indices))
	for _, index := range indices {
		call := d.toolCalls[index]
		args, err := parseJSONObject(call.argsText)
		if err != nil {
			args = map[string]any{}
		}
		result = append(result, domain.ToolCall{ToolCallID: call.id, Name: call.name, Args: args})
	}
	return result
}

func openAIMessages(messages []domain.Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case domain.MessageRoleTool:
			result = append(result, openai.ToolMessage(message.Content, message.ToolCallID))
		case domain.MessageRoleAssistant:
			converted := openai.ChatCompletionMessageParamOfAssistant(message.Content)
			if message.ToolCalls != nil {
				var toolCalls []openai.ChatCompletionMessageToolCallUnionParam
				for _, call := range message.ToolCalls {
					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: call.ToolCallID,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      call.Name,
								Arguments: mustJSON(call.Args),
							},
						},
					})
				}
				converted.OfAssistant.ToolCalls = toolCalls
			}
			result = append(result, converted)
		case domain.MessageRoleSystem:
			result = append(result, openai.SystemMessage(message.Content))
		case domain.MessageRoleUser:
			result = append(result, openai.UserMessage(message.Content))
		default:
			result = append(result, openai.UserMessage(message.Content))
		}
	}
	return result
}

func openAITools(tools []domain.Tool) []openai.ChatCompletionToolUnionParam {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		result = append(result, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  shared.FunctionParameters(openAIToolSchema(tool)),
		}))
	}
	return result
}

func openAIToolSchema(tool domain.Tool) map[string]any {
	schema := map[string]any{"type": tool.Parameters.Type}
	if tool.Parameters.Properties != nil {
		schema["properties"] = tool.Parameters.Properties
	}
	if tool.Parameters.Required != nil {
		schema["required"] = tool.Parameters.Required
	}
	return schema
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
