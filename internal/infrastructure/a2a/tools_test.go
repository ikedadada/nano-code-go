package a2a_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"nano-code-go/internal/domain"
	infraa2a "nano-code-go/internal/infrastructure/a2a"
)

type fakeSender struct {
	calls []sendCall
}

type sendCall struct {
	agent  infraa2a.RemoteAgentEndpoint
	prompt string
}

func (s *fakeSender) SendMessage(_ context.Context, agent infraa2a.RemoteAgentEndpoint, prompt string) (string, error) {
	s.calls = append(s.calls, sendCall{agent: agent, prompt: prompt})
	return "review result", nil
}

func TestCreateTools(t *testing.T) {
	t.Parallel()

	if got := infraa2a.CreateTools(infraa2a.NewRegistry(nil), nil); len(got) != 0 {
		t.Fatalf("CreateTools(empty) = %#v, want empty", got)
	}

	card := domain.A2AAgentCard{
		Name: "Code Reviewer",
		URL:  "http://reviewer.example/a2a",
		Skills: []domain.A2AAgentSkill{{
			ID:          "review-code",
			Name:        "Review Code",
			Description: "Reviews code and reports actionable findings.",
			Tags:        []string{"review", "code"},
			InputModes:  []string{"text/plain"},
			OutputModes: []string{"text/plain"},
		}},
	}
	registry := infraa2a.NewRegistry([]infraa2a.RegisteredAgent{{
		ID:          "reviewer",
		CardURL:     "http://reviewer.example/.well-known/agent-card.json",
		EndpointURL: card.URL,
		BearerToken: "secret-token",
		Card:        card,
	}})
	sender := &fakeSender{}

	tools := infraa2a.CreateTools(registry, sender)
	if len(tools) != 1 {
		t.Fatalf("CreateTools() length = %d, want 1", len(tools))
	}
	tool := tools[0]
	if tool.Name != "a2a_reviewer_review-code" {
		t.Fatalf("tool.Name = %q", tool.Name)
	}
	if !tool.NeedsApproval {
		t.Fatalf("tool.NeedsApproval = false, want true")
	}
	if !reflect.DeepEqual(tool.Parameters.Required, []string{"prompt"}) {
		t.Fatalf("tool.Parameters.Required = %#v", tool.Parameters.Required)
	}
	if !strings.Contains(tool.Description, "Code Reviewer") || !strings.Contains(tool.Description, "Reviews code") {
		t.Fatalf("tool.Description = %q", tool.Description)
	}

	result, err := tool.Execute(context.Background(), map[string]any{"prompt": "review this"})
	if err != nil {
		t.Fatalf("tool.Execute() error = %v", err)
	}
	if result != "review result" {
		t.Fatalf("tool.Execute() = %q", result)
	}
	wantCalls := []sendCall{{
		agent: infraa2a.RemoteAgentEndpoint{
			ID:          "reviewer",
			Name:        "Code Reviewer",
			URL:         "http://reviewer.example/a2a",
			BearerToken: "secret-token",
		},
		prompt: "review this",
	}}
	if !reflect.DeepEqual(sender.calls, wantCalls) {
		t.Fatalf("sender.calls = %#v, want %#v", sender.calls, wantCalls)
	}
}
