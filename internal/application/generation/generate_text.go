package generation

import (
	"context"
	"fmt"

	"nano-code-go/internal/domain"
)

type GenerateTextParams struct {
	Model domain.LanguageModel
	domain.GenerateParams
}

func GenerateText(ctx context.Context, params GenerateTextParams) (domain.GenerateTextResult, error) {
	return params.Model.Generate(ctx, params.GenerateParams)
}

type CollectStreamParams struct {
	Model domain.LanguageModel
	domain.GenerateParams
	OnChunk func(domain.StreamChunk)
}

func CollectStreamResult(ctx context.Context, params CollectStreamParams) (domain.GenerateTextResult, error) {
	chunks, err := params.Model.Stream(ctx, params.GenerateParams)
	if err != nil {
		return domain.GenerateTextResult{}, fmt.Errorf("start stream: %w", err)
	}

	var text string
	finishReason := domain.FinishReasonStop
	var usage domain.Usage
	var toolCalls []domain.ToolCall

	for {
		select {
		case <-ctx.Done():
			return domain.GenerateTextResult{}, ctx.Err()
		case chunk, ok := <-chunks:
			if !ok {
				return domain.GenerateTextResult{
					Text:         text,
					FinishReason: finishReason,
					ToolCalls:    toolCalls,
					Usage:        usage,
				}, nil
			}

			if params.OnChunk != nil {
				params.OnChunk(chunk)
			}
			if chunk.Err != nil {
				return domain.GenerateTextResult{}, chunk.Err
			}
			if chunk.Kind == domain.StreamKindDelta {
				text += chunk.Text
			}
			if chunk.Kind == domain.StreamKindDone {
				if chunk.FinishReason != "" {
					finishReason = chunk.FinishReason
				}
				toolCalls = chunk.ToolCalls
				usage = chunk.Usage
			}
		}
	}
}
