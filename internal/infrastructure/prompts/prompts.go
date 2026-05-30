package prompts

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed baseInstructions.md issueInstructions.md
var promptFiles embed.FS

func LoadInstructions(workspaceRoot string, issueDriven bool) (string, error) {
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
		instructions += "\n\n" + string(issue)
	}

	return instructions, nil
}
