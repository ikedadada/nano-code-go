package a2a_test

import (
	"context"
	"reflect"
	"testing"

	appa2a "nano-code-go/internal/application/a2a"
)

func TestService_AgentCard(t *testing.T) {
	t.Parallel()

	service := appa2a.NewService(appa2a.ServiceConfig{
		AgentURL:     "http://localhost:3000/a2a",
		AuthRequired: true,
	})

	card := service.AgentCard()
	if card.ProtocolVersion != "0.3.0" {
		t.Fatalf("ProtocolVersion = %q, want 0.3.0", card.ProtocolVersion)
	}
	if card.Name != "nano-code" {
		t.Fatalf("Name = %q, want nano-code", card.Name)
	}
	if card.URL != "http://localhost:3000/a2a" {
		t.Fatalf("URL = %q", card.URL)
	}
	if card.PreferredTransport != "JSONRPC" {
		t.Fatalf("PreferredTransport = %q, want JSONRPC", card.PreferredTransport)
	}
	if !card.AuthRequired {
		t.Fatalf("AuthRequired = false, want true")
	}
	if len(card.Skills) != 1 || card.Skills[0].ID != "coding-agent" {
		t.Fatalf("Skills = %#v", card.Skills)
	}
}

func TestService_SendMessage(t *testing.T) {
	t.Parallel()

	var requests []appa2a.RunAgentRequest
	service := appa2a.NewService(appa2a.ServiceConfig{
		WorkspaceRoot:  "/workspace",
		Sandbox:        true,
		AllowedDomains: []string{"example.com"},
		RunAgent: func(_ context.Context, request appa2a.RunAgentRequest) (appa2a.RunAgentResponse, error) {
			requests = append(requests, request)
			return appa2a.RunAgentResponse{Text: "answer:" + request.Prompt}, nil
		},
	})

	message, err := service.SendMessage(context.Background(), []appa2a.TextPart{
		{Text: "hello"},
		{Text: "world"},
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	wantRequest := appa2a.RunAgentRequest{
		Prompt:         "hello\nworld",
		IssueDriven:    false,
		Streaming:      false,
		Yolo:           true,
		Sandbox:        true,
		AllowedDomains: []string{"example.com"},
		WorkspaceRoot:  "/workspace",
	}
	if !reflect.DeepEqual(requests, []appa2a.RunAgentRequest{wantRequest}) {
		t.Fatalf("requests = %#v, want %#v", requests, []appa2a.RunAgentRequest{wantRequest})
	}
	if message.Role != "agent" {
		t.Fatalf("message = %#v", message)
	}
	if len(message.Parts) != 1 || message.Parts[0].Text != "answer:hello\nworld" {
		t.Fatalf("message.Parts = %#v", message.Parts)
	}
	if message.MessageID == "" {
		t.Fatalf("message.MessageID is empty")
	}
}

func TestService_SendMessageRejectsEmptyText(t *testing.T) {
	t.Parallel()

	service := appa2a.NewService(appa2a.ServiceConfig{
		RunAgent: func(context.Context, appa2a.RunAgentRequest) (appa2a.RunAgentResponse, error) {
			t.Fatal("RunAgent should not be called")
			return appa2a.RunAgentResponse{}, nil
		},
	})

	_, err := service.SendMessage(context.Background(), []appa2a.TextPart{{Text: "  "}})
	if err == nil || err.Error() != "text part is required" {
		t.Fatalf("SendMessage() error = %v, want text part is required", err)
	}
}
