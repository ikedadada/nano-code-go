package prompts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nano-code-go/internal/infrastructure/prompts"
)

func TestLoadInstructions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		agents      string
		issueDriven bool
		wantParts   []string
	}{
		{
			name: "base only",
			wantParts: []string{
				"You are a TypeScript coding agent.",
			},
		},
		{
			name:   "with workspace agents",
			agents: "Use project rules.",
			wantParts: []string{
				"# Project-Specific Instructions",
				"Use project rules.",
			},
		},
		{
			name:        "with issue instructions",
			issueDriven: true,
			wantParts: []string{
				"You are a TypeScript coding agent.",
				"You are a TypeScript coding agent running on GitHub Actions.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			workspace := t.TempDir()
			if tt.agents != "" {
				if err := os.WriteFile(filepath.Join(workspace, "AGENTS.md"), []byte(tt.agents), 0o600); err != nil {
					t.Fatalf("write AGENTS.md: %v", err)
				}
			}

			got, err := prompts.LoadInstructions(workspace, tt.issueDriven)
			if err != nil {
				t.Fatalf("LoadInstructions() error = %v", err)
			}

			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Fatalf("LoadInstructions() missing %q in:\n%s", part, got)
				}
			}
		})
	}
}
