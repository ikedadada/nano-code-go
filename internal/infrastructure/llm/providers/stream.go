package providers

import (
	"context"

	"nano-code-go/internal/domain"
)

func sendStreamChunk(ctx context.Context, chunks chan<- domain.StreamChunk, chunk domain.StreamChunk) bool {
	select {
	case <-ctx.Done():
		return false
	case chunks <- chunk:
		return true
	}
}
