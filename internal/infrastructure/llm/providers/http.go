package providers

import (
	"context"
	"net/http"

	"nano-code-go/internal/domain"
)

type Config struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

func sendStreamChunk(ctx context.Context, chunks chan<- domain.StreamChunk, chunk domain.StreamChunk) bool {
	select {
	case <-ctx.Done():
		return false
	case chunks <- chunk:
		return true
	}
}

func toolSchema(tool domain.Tool) map[string]any {
	schema := map[string]any{"type": tool.Parameters.Type}
	if tool.Parameters.Properties != nil {
		schema["properties"] = tool.Parameters.Properties
	}
	if tool.Parameters.Required != nil {
		schema["required"] = tool.Parameters.Required
	}
	return schema
}
