package generation_test

import (
	"context"
	"reflect"
	"testing"

	"nano-code-go/internal/application/generation"
	"nano-code-go/internal/domain"
)

type streamModel struct {
	chunks []domain.StreamChunk
}

func (m streamModel) Generate(context.Context, domain.GenerateParams) (domain.GenerateTextResult, error) {
	return domain.GenerateTextResult{}, nil
}

func (m streamModel) Stream(context.Context, domain.GenerateParams) (<-chan domain.StreamChunk, error) {
	ch := make(chan domain.StreamChunk, len(m.chunks))
	for _, chunk := range m.chunks {
		ch <- chunk
	}
	close(ch)
	return ch, nil
}

func TestCollectStreamResult(t *testing.T) {
	t.Parallel()

	chunks := []domain.StreamChunk{
		{Kind: domain.StreamKindDelta, Text: "hello"},
		{Kind: domain.StreamKindDelta, Text: " world"},
		{
			Kind:         domain.StreamKindDone,
			FinishReason: domain.FinishReasonToolCall,
			ToolCalls: []domain.ToolCall{{
				ToolCallID: "call-1",
				Name:       "readFile",
				Args:       map[string]any{},
			}},
			Usage: domain.Usage{TotalTokens: 12},
		},
	}

	var seen []domain.StreamChunk
	got, err := generation.CollectStreamResult(context.Background(), generation.CollectStreamParams{
		Model: streamModel{chunks: chunks},
		OnChunk: func(chunk domain.StreamChunk) {
			seen = append(seen, chunk)
		},
	})
	if err != nil {
		t.Fatalf("CollectStreamResult() error = %v", err)
	}

	want := domain.GenerateTextResult{
		Text:         "hello world",
		FinishReason: domain.FinishReasonToolCall,
		ToolCalls: []domain.ToolCall{{
			ToolCallID: "call-1",
			Name:       "readFile",
			Args:       map[string]any{},
		}},
		Usage: domain.Usage{TotalTokens: 12},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectStreamResult() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(seen, chunks) {
		t.Fatalf("seen chunks = %#v, want %#v", seen, chunks)
	}
}
