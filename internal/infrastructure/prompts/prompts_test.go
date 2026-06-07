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
				"You are a Go coding agent.",
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
				"You are a Go coding agent.",
				"You are a Go coding agent running on GitHub Actions.",
				"The triggering Issue number is (none)",
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

func TestLoadInstructionsWithEnvRendersIssueNumber(t *testing.T) {
	t.Parallel()

	got, err := prompts.LoadInstructionsWithEnv(t.TempDir(), true, map[string]string{
		"ISSUE_NUMBER": "123",
	})
	if err != nil {
		t.Fatalf("LoadInstructionsWithEnv() error = %v", err)
	}

	for _, forbidden := range []string{"${process.env", "{{ISSUE_NUMBER}}"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("LoadInstructionsWithEnv() contains unresolved template %q in:\n%s", forbidden, got)
		}
	}
	if !strings.Contains(got, "The triggering Issue number is 123") {
		t.Fatalf("LoadInstructionsWithEnv() did not render issue number in:\n%s", got)
	}
}
