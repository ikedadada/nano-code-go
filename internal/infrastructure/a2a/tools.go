package a2a

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"nano-code-go/internal/domain"
)

type MessageSender interface {
	SendMessage(ctx context.Context, agent RemoteAgentEndpoint, prompt string) (string, error)
}

func CreateTools(registry *Registry, client MessageSender) []domain.Tool {
	if registry == nil {
		registry = NewRegistry(nil)
	}
	if client == nil {
		client = NewClient(nil)
	}

	var tools []domain.Tool
	for _, agent := range registry.List() {
		for _, skill := range agent.Card.Skills {
			registeredAgent := agent
			registeredSkill := skill
			tools = append(tools, domain.Tool{
				Name:          toolName(registeredAgent, registeredSkill.ID),
				Description:   toolDescription(registeredAgent, registeredSkill),
				NeedsApproval: true,
				Parameters: domain.ToolParameters{
					Type: "object",
					Properties: map[string]any{
						"prompt": map[string]any{
							"type":        "string",
							"description": fmt.Sprintf("Task prompt for remote A2A agent '%s' skill '%s'", registeredAgent.Card.Name, registeredSkill.Name),
						},
					},
					Required: []string{"prompt"},
				},
				Execute: func(ctx context.Context, args map[string]any) (string, error) {
					prompt, ok := args["prompt"].(string)
					if !ok || prompt == "" {
						return "", fmt.Errorf("prompt is required")
					}
					return client.SendMessage(ctx, RemoteAgentEndpoint{
						ID:          registeredAgent.ID,
						Name:        registeredAgent.Card.Name,
						URL:         registeredAgent.EndpointURL,
						BearerToken: registeredAgent.BearerToken,
					}, prompt)
				},
			})
		}
	}
	return tools
}

var unsafeToolNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
var repeatedUnderscores = regexp.MustCompile(`_+`)

func toolName(agent RegisteredAgent, skillID string) string {
	name := "a2a_" + agent.ID + "_" + skillID
	name = unsafeToolNameChars.ReplaceAllString(name, "_")
	name = repeatedUnderscores.ReplaceAllString(name, "_")
	if len(name) > 64 {
		name = name[:64]
	}
	if name == "" {
		return "a2a_remote_agent"
	}
	return name
}

func toolDescription(agent RegisteredAgent, skill domain.A2AAgentSkill) string {
	parts := []string{
		fmt.Sprintf("Delegate to remote A2A agent '%s' for skill '%s'.", agent.Card.Name, skill.Name),
		skill.Description,
	}
	if len(skill.Tags) > 0 {
		parts = append(parts, "Tags: "+strings.Join(skill.Tags, ", ")+".")
	}
	return strings.Join(parts, " ")
}
