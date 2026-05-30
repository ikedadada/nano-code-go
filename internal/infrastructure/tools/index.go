package tools

import (
	"path/filepath"

	"nano-code-go/internal/domain"
	infraa2a "nano-code-go/internal/infrastructure/a2a"
)

func CreateTools(options Options, a2aRegistry *infraa2a.Registry) []domain.Tool {
	workspaceRoot := options.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(".", "workspace")
	}

	base := []domain.Tool{
		ReadFile(workspaceRoot),
		WriteFile(workspaceRoot),
		EditFile(workspaceRoot),
		WebFetch(options.AllowedDomains, options.HTTPClient),
	}
	base = append(base, infraa2a.CreateTools(a2aRegistry, nil)...)
	return base
}
