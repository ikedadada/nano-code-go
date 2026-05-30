package a2a

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
)

//go:embed agents.json
var catalogFS embed.FS

type AgentSource struct {
	ID             string
	AgentCardURL   string
	EndpointURL    string
	BearerToken    string
	BearerTokenEnv string
}

type catalogFile struct {
	Agents []catalogAgent `json:"agents"`
}

type catalogAgent struct {
	ID             string `json:"id"`
	AgentCardURL   string `json:"agentCardUrl"`
	EndpointURL    string `json:"endpointUrl,omitempty"`
	BearerToken    string `json:"bearerToken,omitempty"`
	BearerTokenEnv string `json:"bearerTokenEnv,omitempty"`
}

func LoadAgentSources(path string, env map[string]string) ([]AgentSource, error) {
	content, err := readCatalog(path)
	if err != nil {
		return nil, err
	}

	var catalog catalogFile
	if err := json.Unmarshal(content, &catalog); err != nil {
		return nil, fmt.Errorf("parse A2A agent catalog: %w", err)
	}

	sources := make([]AgentSource, 0, len(catalog.Agents))
	for _, agent := range catalog.Agents {
		if agent.ID == "" || agent.AgentCardURL == "" {
			return nil, fmt.Errorf("invalid A2A agent catalog entry: Expected id and agentCardUrl")
		}

		source := AgentSource{
			ID:             agent.ID,
			AgentCardURL:   agent.AgentCardURL,
			EndpointURL:    agent.EndpointURL,
			BearerToken:    agent.BearerToken,
			BearerTokenEnv: agent.BearerTokenEnv,
		}
		if source.BearerTokenEnv != "" && env != nil && env[source.BearerTokenEnv] != "" {
			source.BearerToken = env[source.BearerTokenEnv]
		}
		sources = append(sources, source)
	}

	return sources, nil
}

func readCatalog(path string) ([]byte, error) {
	if path == "" {
		content, err := catalogFS.ReadFile("agents.json")
		if err != nil {
			return nil, fmt.Errorf("read embedded A2A agent catalog: %w", err)
		}
		return content, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read A2A agent catalog: %w", err)
	}
	return content, nil
}
