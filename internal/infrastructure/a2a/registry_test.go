package a2a_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"nano-code-go/internal/a2aprotocol"
	infraa2a "nano-code-go/internal/infrastructure/a2a"
)

type fakeFetcher struct {
	cards map[string]a2aprotocol.AgentCard
	errs  map[string]error
}

func (f fakeFetcher) FetchAgentCard(_ context.Context, url, _ string) (a2aprotocol.AgentCard, error) {
	if err := f.errs[url]; err != nil {
		return a2aprotocol.AgentCard{}, err
	}
	return f.cards[url], nil
}

func TestRegistry(t *testing.T) {
	t.Parallel()

	if got := infraa2a.NewRegistry(nil).List(); len(got) != 0 {
		t.Fatalf("empty registry list = %#v, want empty", got)
	}
}

func TestDiscover(t *testing.T) {
	t.Parallel()

	card := testCard("Remote Agent", "http://remote.example/a2a")
	registry := infraa2a.Discover(context.Background(), []infraa2a.AgentSource{{
		ID:           "remote-agent",
		AgentCardURL: "http://remote.example/.well-known/agent-card.json",
		BearerToken:  "secret-token",
	}}, fakeFetcher{
		cards: map[string]a2aprotocol.AgentCard{
			"http://remote.example/.well-known/agent-card.json": card,
		},
	}, nil)

	want := []infraa2a.RegisteredAgent{{
		ID:          "remote-agent",
		CardURL:     "http://remote.example/.well-known/agent-card.json",
		EndpointURL: "http://remote.example/a2a",
		BearerToken: "secret-token",
		Card:        card,
	}}
	if !reflect.DeepEqual(registry.List(), want) {
		t.Fatalf("registry.List() = %#v, want %#v", registry.List(), want)
	}
}

func TestDiscoverUsesEndpointOverrideAndSkipsFailures(t *testing.T) {
	t.Parallel()

	card := testCard("Docker Agent", "http://localhost:9000/unused")
	var warnings bytes.Buffer
	registry := infraa2a.Discover(context.Background(), []infraa2a.AgentSource{
		{ID: "offline", AgentCardURL: "http://offline.example/card"},
		{ID: "docker", AgentCardURL: "http://localhost:9000/card", EndpointURL: "http://localhost:9000/invoke"},
	}, fakeFetcher{
		cards: map[string]a2aprotocol.AgentCard{
			"http://localhost:9000/card": card,
		},
		errs: map[string]error{
			"http://offline.example/card": errors.New("connection refused"),
		},
	}, &warnings)

	got := registry.List()
	if len(got) != 1 {
		t.Fatalf("registry.List() = %#v, want 1 agent", got)
	}
	if got[0].EndpointURL != "http://localhost:9000/invoke" {
		t.Fatalf("EndpointURL = %q", got[0].EndpointURL)
	}
	if !strings.Contains(warnings.String(), "offline") || !strings.Contains(warnings.String(), "connection refused") {
		t.Fatalf("warnings = %q", warnings.String())
	}
}

func testCard(name, url string) a2aprotocol.AgentCard {
	return a2aprotocol.AgentCard{
		ProtocolVersion:    "0.3.0",
		Name:               name,
		Description:        "Remote A2A agent.",
		URL:                url,
		PreferredTransport: "JSONRPC",
		Capabilities:       a2aprotocol.AgentCapabilities{},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             []a2aprotocol.AgentSkill{},
	}
}
