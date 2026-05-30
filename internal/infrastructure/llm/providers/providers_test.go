package providers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"nano-code-go/internal/domain"
	"nano-code-go/internal/infrastructure/llm/providers"
)

type llmDoer func(*http.Request) (*http.Response, error)

func (f llmDoer) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCreateModelFromEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     providers.Env
		wantErr string
	}{
		{name: "requires provider", env: providers.Env{"LLM_MODEL": "model"}, wantErr: "LLM_PROVIDER environment variable is not set"},
		{name: "requires model", env: providers.Env{"LLM_PROVIDER": "openai"}, wantErr: "LLM_MODEL environment variable is not set"},
		{name: "unsupported provider", env: providers.Env{"LLM_PROVIDER": "unsupported", "LLM_MODEL": "model"}, wantErr: "Unsupported LLM provider: unsupported"},
		{name: "openai", env: providers.Env{"LLM_PROVIDER": "openai", "LLM_MODEL": "model"}},
		{name: "anthropic", env: providers.Env{"LLM_PROVIDER": "anthropic", "LLM_MODEL": "model"}},
		{name: "google", env: providers.Env{"LLM_PROVIDER": "google", "LLM_MODEL": "model"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model, err := providers.CreateModelFromEnv(providers.FactoryOptions{Env: tt.env})
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("CreateModelFromEnv() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateModelFromEnv() error = %v", err)
			}
			if model == nil {
				t.Fatalf("CreateModelFromEnv() model = nil")
			}
		})
	}
}

func TestCreateModelFromEnvAPIKeyFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		provider  string
		extraEnv  providers.Env
		assertKey func(t *testing.T, request *http.Request)
		response  map[string]any
	}{
		{
			name:     "openai uses generic key",
			provider: "openai",
			assertKey: func(t *testing.T, request *http.Request) {
				t.Helper()
				if got := request.Header.Get("Authorization"); got != "Bearer generic-key" {
					t.Fatalf("Authorization = %q", got)
				}
			},
			response: openAITestResponse(),
		},
		{
			name:     "openai provider key wins",
			provider: "openai",
			extraEnv: providers.Env{"OPENAI_API_KEY": "provider-key"},
			assertKey: func(t *testing.T, request *http.Request) {
				t.Helper()
				if got := request.Header.Get("Authorization"); got != "Bearer provider-key" {
					t.Fatalf("Authorization = %q", got)
				}
			},
			response: openAITestResponse(),
		},
		{
			name:     "anthropic uses generic key",
			provider: "anthropic",
			assertKey: func(t *testing.T, request *http.Request) {
				t.Helper()
				if got := request.Header.Get("x-api-key"); got != "generic-key" {
					t.Fatalf("x-api-key = %q", got)
				}
			},
			response: anthropicTestResponse(),
		},
		{
			name:     "google uses generic key",
			provider: "google",
			assertKey: func(t *testing.T, request *http.Request) {
				t.Helper()
				if got := request.URL.Query().Get("key"); got != "generic-key" {
					t.Fatalf("key query = %q", got)
				}
			},
			response: googleTestResponse(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := providers.Env{
				"LLM_PROVIDER": tt.provider,
				"LLM_MODEL":    "test-model",
				"LLM_API_KEY":  "generic-key",
			}
			for key, value := range tt.extraEnv {
				env[key] = value
			}
			model, err := providers.CreateModelFromEnv(providers.FactoryOptions{
				Env: env,
				Client: llmDoer(func(request *http.Request) (*http.Response, error) {
					tt.assertKey(t, request)
					return jsonResponse(t, http.StatusOK, tt.response), nil
				}),
			})
			if err != nil {
				t.Fatalf("CreateModelFromEnv() error = %v", err)
			}
			if _, err := model.Generate(context.Background(), domain.GenerateParams{}); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
		})
	}
}

func TestOpenAIProviderGenerate(t *testing.T) {
	t.Parallel()

	var requestBody map[string]any
	var auth string
	model := providers.NewOpenAI("gpt-test", providers.Config{
		APIKey:  "test-key",
		BaseURL: "https://openai.test/v1",
		Client: llmDoer(func(request *http.Request) (*http.Response, error) {
			auth = request.Header.Get("Authorization")
			if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"choices": []map[string]any{{
					"message": map[string]any{
						"content": "hello",
						"tool_calls": []map[string]any{{
							"id":   "call-1",
							"type": "function",
							"function": map[string]any{
								"name":      "readFile",
								"arguments": `{"path":"a.txt"}`,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
				"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3},
			}), nil
		}),
	})

	result, err := model.Generate(context.Background(), sampleParams())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if auth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", auth)
	}
	if requestBody["model"] != "gpt-test" {
		t.Fatalf("model = %#v", requestBody["model"])
	}
	messages := requestBody["messages"].([]any)
	assertMapContains(t, messages[1].(map[string]any), "role", "assistant")
	tools := requestBody["tools"].([]any)
	toolFn := tools[0].(map[string]any)["function"].(map[string]any)
	if toolFn["name"] != "readFile" {
		t.Fatalf("tool function = %#v", toolFn)
	}
	want := domain.GenerateTextResult{
		Text:         "hello",
		FinishReason: domain.FinishReasonToolCall,
		ToolCalls: []domain.ToolCall{{
			ToolCallID: "call-1",
			Name:       "readFile",
			Args:       map[string]any{"path": "a.txt"},
		}},
		Usage: domain.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("Generate() = %#v, want %#v", result, want)
	}
}

func TestAnthropicProviderGenerate(t *testing.T) {
	t.Parallel()

	var requestBody map[string]any
	model := providers.NewAnthropic("claude-test", providers.Config{
		APIKey:  "test-key",
		BaseURL: "https://anthropic.test/v1",
		Client: llmDoer(func(request *http.Request) (*http.Response, error) {
			if got := request.Header.Get("x-api-key"); got != "test-key" {
				t.Fatalf("x-api-key = %q", got)
			}
			if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "hello"},
					{"type": "tool_use", "id": "call-1", "name": "readFile", "input": map[string]any{"path": "a.txt"}},
				},
				"stop_reason": "tool_use",
				"usage":       map[string]any{"input_tokens": 1, "output_tokens": 2},
			}), nil
		}),
	})

	result, err := model.Generate(context.Background(), sampleParams())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if requestBody["model"] != "claude-test" {
		t.Fatalf("model = %#v", requestBody["model"])
	}
	if len(requestBody["system"].([]any)) != 1 {
		t.Fatalf("system = %#v", requestBody["system"])
	}
	if result.Text != "hello" || result.FinishReason != domain.FinishReasonToolCall || result.Usage.TotalTokens != 3 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Name != "readFile" {
		t.Fatalf("tool calls = %#v", result.ToolCalls)
	}
}

func TestGoogleProviderGenerate(t *testing.T) {
	t.Parallel()

	var requestURL string
	var requestBody map[string]any
	model := providers.NewGoogle("gemini-test", providers.Config{
		APIKey:  "test-key",
		BaseURL: "https://google.test/v1beta",
		Client: llmDoer(func(request *http.Request) (*http.Response, error) {
			requestURL = request.URL.String()
			if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return jsonResponse(t, http.StatusOK, map[string]any{
				"candidates": []map[string]any{{
					"finishReason": "STOP",
					"content": map[string]any{"parts": []map[string]any{
						{"text": "hello"},
						{"functionCall": map[string]any{"name": "readFile", "args": map[string]any{"path": "a.txt"}}},
					}},
				}},
				"usageMetadata": map[string]any{"promptTokenCount": 1, "candidatesTokenCount": 2, "totalTokenCount": 3},
			}), nil
		}),
	})

	result, err := model.Generate(context.Background(), sampleParams())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.Contains(requestURL, "/models/gemini-test:generateContent?key=test-key") {
		t.Fatalf("request URL = %q", requestURL)
	}
	config := requestBody["config"].(map[string]any)
	if config["systemInstruction"] != "system" {
		t.Fatalf("config = %#v", config)
	}
	if result.Text != "hello" || result.FinishReason != domain.FinishReasonToolCall || result.Usage.TotalTokens != 3 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolCallID != "call_0" {
		t.Fatalf("tool calls = %#v", result.ToolCalls)
	}
}

func TestProvidersStreamUnsupported(t *testing.T) {
	t.Parallel()

	providersToTest := []domain.LanguageModel{
		providers.NewOpenAI("model", providers.Config{}),
		providers.NewAnthropic("model", providers.Config{}),
		providers.NewGoogle("model", providers.Config{}),
	}
	for _, provider := range providersToTest {
		_, err := provider.Stream(context.Background(), domain.GenerateParams{})
		if err == nil || !strings.Contains(err.Error(), "streaming is not implemented") {
			t.Fatalf("Stream() error = %v, want unsupported streaming", err)
		}
	}
}

func sampleParams() domain.GenerateParams {
	return domain.GenerateParams{
		Messages: []domain.Message{
			{Role: domain.MessageRoleSystem, Content: "system"},
			{
				Role:    domain.MessageRoleAssistant,
				Content: "using tool",
				ToolCalls: []domain.ToolCall{{
					ToolCallID: "call-0",
					Name:       "readFile",
					Args:       map[string]any{"path": "a.txt"},
				}},
			},
			{Role: domain.MessageRoleTool, ToolCallID: "call-0", Name: "readFile", Content: "file content"},
		},
		Tools: []domain.Tool{{
			Name:        "readFile",
			Description: "Read a file",
			Parameters:  domain.ToolParameters{Type: "object"},
		}},
	}
}

func jsonResponse(t *testing.T, status int, value any) *http.Response {
	t.Helper()

	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}

func assertMapContains(t *testing.T, value map[string]any, key string, want any) {
	t.Helper()
	if value[key] != want {
		t.Fatalf("%s = %#v, want %#v in %#v", key, value[key], want, value)
	}
}

func openAITestResponse() map[string]any {
	return map[string]any{
		"choices": []map[string]any{{
			"message":       map[string]any{"content": "hello"},
			"finish_reason": "stop",
		}},
	}
}

func anthropicTestResponse() map[string]any {
	return map[string]any{
		"content":     []map[string]any{{"type": "text", "text": "hello"}},
		"stop_reason": "end_turn",
	}
}

func googleTestResponse() map[string]any {
	return map[string]any{
		"candidates": []map[string]any{{
			"finishReason": "STOP",
			"content":      map[string]any{"parts": []map[string]any{{"text": "hello"}}},
		}},
	}
}
