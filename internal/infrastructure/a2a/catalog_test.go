package a2a_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	infraa2a "nano-code-go/internal/infrastructure/a2a"
)

func TestLoadAgentSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		env     map[string]string
		want    []infraa2a.AgentSource
		wantErr string
	}{
		{
			name:    "loads card URLs and endpoint overrides",
			content: `{"agents":[{"id":"pirate","agentCardUrl":"http://localhost:8082/.well-known/agent-card.json","endpointUrl":"http://localhost:8082/invoke"}]}`,
			want: []infraa2a.AgentSource{{
				ID:           "pirate",
				AgentCardURL: "http://localhost:8082/.well-known/agent-card.json",
				EndpointURL:  "http://localhost:8082/invoke",
			}},
		},
		{
			name:    "loads bearer token from env",
			content: `{"agents":[{"id":"private-agent","agentCardUrl":"http://localhost:3001/.well-known/agent-card.json","bearerTokenEnv":"PRIVATE_A2A_TOKEN"}]}`,
			env:     map[string]string{"PRIVATE_A2A_TOKEN": "test-token"},
			want: []infraa2a.AgentSource{{
				ID:             "private-agent",
				AgentCardURL:   "http://localhost:3001/.well-known/agent-card.json",
				BearerToken:    "test-token",
				BearerTokenEnv: "PRIVATE_A2A_TOKEN",
			}},
		},
		{
			name:    "rejects malformed entries",
			content: `{"agents":[{"id":"missing-url"}]}`,
			wantErr: "Expected id and agentCardUrl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := writeCatalog(t, tt.content)
			got, err := infraa2a.LoadAgentSources(path, tt.env)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("LoadAgentSources() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadAgentSources() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("LoadAgentSources() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func writeCatalog(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "agents.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	return path
}
