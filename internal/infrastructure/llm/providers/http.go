package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"nano-code-go/internal/domain"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Config struct {
	APIKey  string
	BaseURL string
	Client  HTTPDoer
}

func httpClient(client HTTPDoer) HTTPDoer {
	if client == nil {
		return http.DefaultClient
	}
	return client
}

func postJSON(ctx context.Context, client HTTPDoer, url string, headers map[string]string, requestBody any, responseBody any, provider string) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("encode %s request: %w", provider, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create %s request: %w", provider, err)
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		if value != "" {
			request.Header.Set(key, value)
		}
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send %s request: %w", provider, err)
	}
	defer response.Body.Close()

	responseBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read %s response: %w", provider, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &domain.LLMAPIError{
			Status:   response.StatusCode,
			Provider: provider,
			Message:  fmt.Sprintf("LLM API Error: %s responded with status %d", provider, response.StatusCode),
			Raw:      string(responseBytes),
		}
	}
	if err := json.Unmarshal(responseBytes, responseBody); err != nil {
		return fmt.Errorf("decode %s response: %w", provider, err)
	}
	return nil
}

func unsupportedStream(provider string) (<-chan domain.StreamChunk, error) {
	return nil, fmt.Errorf("%s streaming is not implemented", provider)
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
