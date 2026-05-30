package agent

import (
	"context"
	"fmt"
	"io"

	"nano-code-go/internal/application/generation"
	"nano-code-go/internal/application/ports"
	"nano-code-go/internal/domain"
)

const defaultMaxSteps = 5

type Config struct {
	Name         string
	Instructions string
	Model        domain.LanguageModel
	Tools        []domain.Tool
	MaxSteps     int
	UseStreaming bool
	Approval     ports.ApprovalPolicy
	Output       io.Writer
}

type Agent struct {
	name         string
	instructions string
	model        domain.LanguageModel
	tools        []domain.Tool
	maxSteps     int
	useStreaming bool
	approval     ports.ApprovalPolicy
	output       io.Writer
}

type Result struct {
	Text string
}

func New(config Config) *Agent {
	maxSteps := config.MaxSteps
	if maxSteps == 0 {
		maxSteps = defaultMaxSteps
	}

	return &Agent{
		name:         config.Name,
		instructions: config.Instructions,
		model:        config.Model,
		tools:        config.Tools,
		maxSteps:     maxSteps,
		useStreaming: config.UseStreaming,
		approval:     config.Approval,
		output:       config.Output,
	}
}

func (a *Agent) Generate(ctx context.Context, userPrompt string) (Result, error) {
	messages := []domain.Message{
		{Role: domain.MessageRoleSystem, Content: a.instructions},
		{Role: domain.MessageRoleUser, Content: userPrompt},
	}

	var finalText string
	for currentStep := 0; currentStep < a.maxSteps; currentStep++ {
		messages = manageContext(messages)

		response, err := a.generateStep(ctx, messages)
		if err != nil {
			return Result{}, err
		}
		if response.Text != "" {
			finalText = response.Text
		}

		if len(response.ToolCalls) > 0 {
			messages = append(messages, domain.Message{
				Role:      domain.MessageRoleAssistant,
				Content:   response.Text,
				ToolCalls: response.ToolCalls,
			})

			for _, toolCall := range response.ToolCalls {
				messages = append(messages, a.executeToolCall(ctx, toolCall))
			}
			continue
		}

		messages = append(messages, domain.Message{
			Role:    domain.MessageRoleAssistant,
			Content: response.Text,
		})
		break
	}

	_ = a.name
	return Result{Text: finalText}, nil
}

func (a *Agent) generateStep(ctx context.Context, messages []domain.Message) (domain.GenerateTextResult, error) {
	params := domain.GenerateParams{Messages: messages, Tools: a.tools}
	if a.useStreaming {
		return generation.CollectStreamResult(ctx, generation.CollectStreamParams{
			Model:          a.model,
			GenerateParams: params,
			OnChunk: func(chunk domain.StreamChunk) {
				if chunk.Kind == domain.StreamKindDelta && chunk.Text != "" && a.output != nil {
					_, _ = io.WriteString(a.output, chunk.Text)
				}
			},
		})
	}

	return generation.GenerateText(ctx, generation.GenerateTextParams{
		Model:          a.model,
		GenerateParams: params,
	})
}

func (a *Agent) executeToolCall(ctx context.Context, toolCall domain.ToolCall) domain.Message {
	tool, ok := a.findTool(toolCall.Name)
	if !ok {
		return toolMessage(toolCall, fmt.Sprintf("Error: Tool not found - %s", toolCall.Name))
	}

	if tool.NeedsApproval {
		if a.approval == nil {
			return toolMessage(toolCall, "Tool call denied by user.")
		}
		approved, err := a.approval(ctx, tool.Name, toolCall.Args)
		if err != nil {
			return toolMessage(toolCall, fmt.Sprintf("Error requesting approval for tool %s: %s", tool.Name, err.Error()))
		}
		if !approved {
			return toolMessage(toolCall, "Tool call denied by user.")
		}
	}

	if tool.Execute == nil {
		return toolMessage(toolCall, fmt.Sprintf("Error executing tool %s: tool has no execute function", tool.Name))
	}

	result, err := tool.Execute(ctx, toolCall.Args)
	if err != nil {
		return toolMessage(toolCall, fmt.Sprintf("Error executing tool %s: %s", tool.Name, err.Error()))
	}

	return toolMessage(toolCall, result)
}

func (a *Agent) findTool(name string) (domain.Tool, bool) {
	for _, tool := range a.tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return domain.Tool{}, false
}

func toolMessage(toolCall domain.ToolCall, content string) domain.Message {
	return domain.Message{
		Role:       domain.MessageRoleTool,
		ToolCallID: toolCall.ToolCallID,
		Name:       toolCall.Name,
		Content:    content,
	}
}

func manageContext(messages []domain.Message) []domain.Message {
	const charLimit = 30000

	totalLength := messageContentLength(messages)
	if totalLength < charLimit {
		return messages
	}
	if len(messages) == 0 {
		return messages
	}

	systemMessage := messages[0]
	recentStart := len(messages) - 4
	if recentStart < 1 {
		recentStart = 1
	}

	middleMessages := append([]domain.Message(nil), messages[1:recentStart]...)
	for i, message := range middleMessages {
		if message.Role == domain.MessageRoleTool && len(message.Content) > 2000 {
			message.Content = fmt.Sprintf("(Previous tool execution results were omitted: %d characters)", len(message.Content))
			middleMessages[i] = message
		}
	}

	recentMessages := append([]domain.Message(nil), messages[recentStart:]...)
	totalLength = len(systemMessage.Content) + messageContentLength(middleMessages) + messageContentLength(recentMessages)
	for totalLength > charLimit && len(middleMessages) > 0 {
		totalLength -= len(middleMessages[0].Content)
		middleMessages = middleMessages[1:]
	}

	result := make([]domain.Message, 0, 1+len(middleMessages)+len(recentMessages))
	result = append(result, systemMessage)
	result = append(result, middleMessages...)
	result = append(result, recentMessages...)
	return result
}

func messageContentLength(messages []domain.Message) int {
	total := 0
	for _, message := range messages {
		total += len(message.Content)
	}
	return total
}
