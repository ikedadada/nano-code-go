package providers

import (
	"errors"
	"fmt"
	"net/http"

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

func httpClientFromDoer(client HTTPDoer) *http.Client {
	if client == nil {
		return http.DefaultClient
	}
	if typed, ok := client.(*http.Client); ok {
		return typed
	}
	return &http.Client{Transport: doerRoundTripper{client: client}}
}

type doerRoundTripper struct {
	client HTTPDoer
}

func (r doerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.client == nil {
		return nil, fmt.Errorf("nil HTTP client")
	}
	return r.client.Do(request)
}
