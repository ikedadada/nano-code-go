package approval_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"nano-code-go/internal/infrastructure/approval"
)

func TestAllowAll(t *testing.T) {
	t.Parallel()

	approved, err := approval.AllowAll(context.Background(), "writeFile", map[string]any{"path": "a.txt"})
	if err != nil {
		t.Fatalf("AllowAll() error = %v", err)
	}
	if !approved {
		t.Fatalf("AllowAll() = false, want true")
	}
}

func TestReadlinePolicy_Request(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "yes short", in: "y\n", want: true},
		{name: "yes long", in: "yes\n", want: true},
		{name: "default no", in: "\n", want: false},
		{name: "explicit no", in: "n\n", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			policy := approval.ReadlinePolicy{
				In:  strings.NewReader(tt.in),
				Out: &out,
			}
			got, err := policy.Request(context.Background(), "writeFile", map[string]any{"path": "a.txt"})
			if err != nil {
				t.Fatalf("Request() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Request() = %t, want %t", got, tt.want)
			}
			if !strings.Contains(out.String(), "Approve tool call \"writeFile\"") {
				t.Fatalf("prompt missing tool name: %q", out.String())
			}
			if !strings.Contains(out.String(), "\"path\": \"a.txt\"") {
				t.Fatalf("prompt missing args: %q", out.String())
			}
		})
	}
}

func TestReadlinePolicy_RequestCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	policy := approval.ReadlinePolicy{In: strings.NewReader("y\n")}
	approved, err := policy.Request(ctx, "writeFile", nil)
	if err == nil {
		t.Fatalf("Request() error = nil, want context cancellation")
	}
	if approved {
		t.Fatalf("Request() approved = true, want false")
	}
}
