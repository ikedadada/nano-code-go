package a2a

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type RunAgent func(ctx context.Context, request RunAgentRequest) (RunAgentResponse, error)

type ServiceConfig struct {
	AgentURL       string
	RunAgent       RunAgent
	WorkspaceRoot  string
	AuthRequired   bool
	Sandbox        bool
	AllowedDomains []string
}

type Service struct {
	config ServiceConfig
}

func NewService(config ServiceConfig) *Service {
	return &Service{config: config}
}

func (s *Service) AgentCard() AgentCard {
	return AgentCard{
		ProtocolVersion:    "0.3.0",
		Name:               "nano-code",
		Description:        "A Go coding agent exposed over A2A JSON-RPC.",
		URL:                s.config.AgentURL,
		PreferredTransport: "JSONRPC",
		AuthRequired:       s.config.AuthRequired,
		Capabilities: AgentCapabilities{
			Streaming:              false,
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []AgentSkill{
			{
				ID:          "coding-agent",
				Name:        "Coding Agent",
				Description: "Helps with coding tasks in the configured workspace.",
				Tags:        []string{"coding", "go", "automation"},
				InputModes:  []string{"text/plain"},
				OutputModes: []string{"text/plain"},
			},
		},
	}
}

func (s *Service) SendMessage(ctx context.Context, parts []TextPart) (Message, error) {
	prompt := extractTextPrompt(parts)
	if prompt == "" {
		return Message{}, errors.New("Text part is required")
	}
	if s.config.RunAgent == nil {
		return Message{}, errors.New("agent runner is not configured")
	}

	result, err := s.config.RunAgent(ctx, RunAgentRequest{
		Prompt:         prompt,
		IssueDriven:    false,
		Streaming:      false,
		Yolo:           true,
		Sandbox:        s.config.Sandbox,
		AllowedDomains: append([]string(nil), s.config.AllowedDomains...),
		WorkspaceRoot:  s.config.WorkspaceRoot,
	})
	if err != nil {
		return Message{}, err
	}

	return Message{
		MessageID: uuid.NewString(),
		Role:      "agent",
		Parts:     []TextPart{{Text: result.Text}},
	}, nil
}

func extractTextPrompt(parts []TextPart) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		texts = append(texts, part.Text)
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}
