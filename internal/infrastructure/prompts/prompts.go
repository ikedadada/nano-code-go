package prompts

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed baseInstructions.md issueInstructions.md
var promptFiles embed.FS

func LoadInstructions(workspaceRoot string, issueDriven bool) (string, error) {
	return LoadInstructionsWithEnv(workspaceRoot, issueDriven, osEnv())
}

func LoadInstructionsWithEnv(workspaceRoot string, issueDriven bool, env map[string]string) (string, error) {
	base, err := promptFiles.ReadFile("baseInstructions.md")
	if err != nil {
		return "", fmt.Errorf("read base instructions: %w", err)
	}

	instructions := string(base)
	agentsPath := filepath.Join(workspaceRoot, "AGENTS.md")
	agents, err := os.ReadFile(agentsPath)
	if err == nil {
		instructions += "\n\n# Project-Specific Instructions\n\n" + string(agents)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read workspace instructions: %w", err)
	}

	if issueDriven {
		issue, err := promptFiles.ReadFile("issueInstructions.md")
		if err != nil {
			return "", fmt.Errorf("read issue instructions: %w", err)
		}
		instructions += "\n\n" + renderIssueInstructions(string(issue), env)
	}

	return instructions, nil
}

func renderIssueInstructions(template string, env map[string]string) string {
	issueNumber := "(none)"
	if env != nil && env["ISSUE_NUMBER"] != "" {
		issueNumber = env["ISSUE_NUMBER"]
	}
	return strings.ReplaceAll(template, "{{ISSUE_NUMBER}}", issueNumber)
}

func osEnv() map[string]string {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}
