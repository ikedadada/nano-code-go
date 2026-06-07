package a2a

import (
	"context"
	"fmt"
	"io"

	"nano-code-go/internal/a2aprotocol"
)

type RegisteredAgent struct {
	ID          string
	CardURL     string
	EndpointURL string
	BearerToken string
	Card        a2aprotocol.AgentCard
}

type Registry struct {
	agents []RegisteredAgent
}

func NewRegistry(agents []RegisteredAgent) *Registry {
	return &Registry{agents: append([]RegisteredAgent(nil), agents...)}
}

func (r *Registry) List() []RegisteredAgent {
	return append([]RegisteredAgent(nil), r.agents...)
}

type AgentCardFetcher interface {
	FetchAgentCard(ctx context.Context, agentCardURL, bearerToken string) (a2aprotocol.AgentCard, error)
}

func Discover(ctx context.Context, sources []AgentSource, client AgentCardFetcher, warnings io.Writer) *Registry {
	if client == nil {
		client = NewClient(nil)
	}
	if warnings == nil {
		warnings = io.Discard
	}

	agents := make([]RegisteredAgent, 0, len(sources))
	for _, source := range sources {
		card, err := client.FetchAgentCard(ctx, source.AgentCardURL, source.BearerToken)
		if err != nil {
			_, _ = fmt.Fprintf(warnings, "A2A agent '%s' skipped: failed to fetch Agent Card from %s. %s\n", source.ID, source.AgentCardURL, err.Error())
			continue
		}

		endpointURL := source.EndpointURL
		if endpointURL == "" {
			endpointURL = card.URL
		}
		if endpointURL == "" {
			_, _ = fmt.Fprintf(warnings, "A2A agent '%s' skipped: Agent Card does not define an endpoint URL.\n", source.ID)
			continue
		}

		agents = append(agents, RegisteredAgent{
			ID:          source.ID,
			CardURL:     source.AgentCardURL,
			EndpointURL: endpointURL,
			BearerToken: source.BearerToken,
			Card:        card,
		})
	}

	return NewRegistry(agents)
}
