package providers

import (
	"errors"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	openai "github.com/openai/openai-go/v3"
	"google.golang.org/genai"

	"nano-code-go/internal/domain"
)

func sdkError(provider string, err error) error {
	var openAIErr *openai.Error
	if errors.As(err, &openAIErr) {
		return &domain.LLMAPIError{
			Status:   openAIErr.StatusCode,
			Provider: provider,
			Code:     openAIErr.Code,
			Message:  openAIErr.Message,
			Raw:      openAIErr.RawJSON(),
		}
	}

	var anthropicErr *anthropic.Error
	if errors.As(err, &anthropicErr) {
		return &domain.LLMAPIError{
			Status:   anthropicErr.StatusCode,
			Provider: provider,
			Code:     string(anthropicErr.Type()),
			Message:  anthropicErr.Error(),
			Raw:      anthropicErr.RawJSON(),
		}
	}

	var googleErr genai.APIError
	if errors.As(err, &googleErr) {
		return &domain.LLMAPIError{
			Status:   googleErr.Code,
			Provider: provider,
			Code:     googleErr.Status,
			Message:  googleErr.Message,
			Raw:      googleErr.Details,
		}
	}

	return err
}
