package tools_test

import (
	"testing"

	"nano-code-go/internal/infrastructure/tools"
)

func TestCreateToolsRegistersImplementedToolsInOrder(t *testing.T) {
	t.Parallel()

	registered := tools.CreateTools(tools.Options{
		WorkspaceRoot:  t.TempDir(),
		AllowedDomains: []string{"example.com"},
	}, nil)

	var names []string
	for _, tool := range registered {
		names = append(names, tool.Name)
	}

	want := []string{
		"readFile",
		"writeFile",
		"editFile",
		"execCommand",
		"createBranch",
		"commit",
		"pushBranch",
		"createPullRequest",
		"createIssueComment",
		"webFetch",
	}
	for i, name := range want {
		if len(names) <= i || names[i] != name {
			t.Fatalf("tool order = %#v, want prefix %#v", names, want)
		}
	}
}
