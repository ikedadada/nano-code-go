package a2a

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"nano-code-go/internal/domain"
)

type RunAgentRequest struct {
	Prompt         string
	IssueDriven    bool
	Streaming      bool
	Yolo           bool
	Sandbox        bool
	AllowedDomains []string
	WorkspaceRoot  string
}

type RunAgentResponse struct {
	Text string
}

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

func (s *Service) AgentCard() domain.A2AAgentCard {
	card := domain.A2AAgentCard{
		ProtocolVersion:    "0.3.0",
		Name:               "nano-code",
		Description:        "A Go coding agent exposed over A2A JSON-RPC.",
		URL:                s.config.AgentURL,
		PreferredTransport: "JSONRPC",
		Capabilities: domain.A2AAgentCapabilities{
			Streaming:              false,
			PushNotifications:      false,
			StateTransitionHistory: false,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []domain.A2AAgentSkill{
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

	if s.config.AuthRequired {
		card.SecuritySchemes = map[string]domain.A2ASecurityScheme{
			"bearerAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "opaque",
				Description:  "Bearer token required for A2A JSON-RPC requests.",
			},
		}
		card.Security = []map[string][]string{{"bearerAuth": {}}}
	}

	return card
}

func (s *Service) SendMessage(ctx context.Context, command domain.A2AMessageSendCommand) (domain.A2AMessage, error) {
	prompt := extractTextPrompt(command.Parts)
	if prompt == "" {
		return domain.A2AMessage{}, errors.New("Text part is required")
	}
	if s.config.RunAgent == nil {
		return domain.A2AMessage{}, errors.New("agent runner is not configured")
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
		return domain.A2AMessage{}, err
	}

	return domain.A2AMessage{
		Kind:      "message",
		MessageID: uuid.NewString(),
		Role:      "agent",
		Parts:     []domain.A2APart{{Kind: "text", Text: result.Text}},
	}, nil
}

func extractTextPrompt(parts []domain.A2APart) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Kind == "text" {
			texts = append(texts, part.Text)
		}
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}
