package ports

import "context"

type ApprovalPolicy func(ctx context.Context, toolName string, args map[string]any) (bool, error)
