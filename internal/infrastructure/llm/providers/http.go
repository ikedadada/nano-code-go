package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

type streamDecoder func([]byte) ([]domain.StreamChunk, bool, error)

func streamJSON(ctx context.Context, client HTTPDoer, url string, headers map[string]string, requestBody any, provider string, decode streamDecoder) (<-chan domain.StreamChunk, error) {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("encode %s stream request: %w", provider, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create %s stream request: %w", provider, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	for key, value := range headers {
		if value != "" {
			request.Header.Set(key, value)
		}
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send %s stream request: %w", provider, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		responseBytes, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read %s stream error response: %w", provider, readErr)
		}
		return nil, &domain.LLMAPIError{
			Status:   response.StatusCode,
			Provider: provider,
			Message:  fmt.Sprintf("LLM API Error: %s responded with status %d", provider, response.StatusCode),
			Raw:      string(responseBytes),
		}
	}

	chunks := make(chan domain.StreamChunk)
	go func() {
		defer close(chunks)
		defer response.Body.Close()

		scanner := bufio.NewScanner(response.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				sendStreamChunk(ctx, chunks, domain.StreamChunk{Err: ctx.Err()})
				return
			default:
			}

			data, ok := eventData(scanner.Text())
			if !ok {
				continue
			}
			if string(data) == "[DONE]" {
				return
			}
			decoded, done, err := decode(data)
			if err != nil {
				sendStreamChunk(ctx, chunks, domain.StreamChunk{Err: err})
				return
			}
			for _, chunk := range decoded {
				if !sendStreamChunk(ctx, chunks, chunk) {
					return
				}
			}
			if done {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			sendStreamChunk(ctx, chunks, domain.StreamChunk{Err: fmt.Errorf("read %s stream: %w", provider, err)})
		}
	}()
	return chunks, nil
}

func eventData(line string) ([]byte, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return nil, false
	}
	return []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), true
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
