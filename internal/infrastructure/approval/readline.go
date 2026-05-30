package approval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func AllowAll(context.Context, string, map[string]any) (bool, error) {
	return true, nil
}

type ReadlinePolicy struct {
	In  io.Reader
	Out io.Writer
}

func (p ReadlinePolicy) Request(ctx context.Context, toolName string, args map[string]any) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	in := p.In
	if in == nil {
		return false, fmt.Errorf("approval input is nil")
	}
	out := p.Out
	if out == nil {
		out = io.Discard
	}

	argsJSON, err := json.MarshalIndent(args, "", "  ")
	if err != nil {
		return false, fmt.Errorf("format approval args: %w", err)
	}

	if _, err := fmt.Fprintf(out, "Approve tool call %q with args:\n%s\nApprove? [y/N]: ", toolName, argsJSON); err != nil {
		return false, fmt.Errorf("write approval prompt: %w", err)
	}

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read approval response: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
