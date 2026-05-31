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

	"nano-code-go/internal/application/generation"
	"nano-code-go/internal/domain"
	"nano-code-go/internal/infrastructure/llm/providers"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testHTTPClient(fn func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{Transport: roundTripFunc(fn)}
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
				if got := request.Header.Get("x-goog-api-key"); got != "generic-key" {
					t.Fatalf("x-goog-api-key = %q", got)
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
				Client: testHTTPClient(func(request *http.Request) (*http.Response, error) {
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
		Client: testHTTPClient(func(request *http.Request) (*http.Response, error) {
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
		Client: testHTTPClient(func(request *http.Request) (*http.Response, error) {
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
		Client: testHTTPClient(func(request *http.Request) (*http.Response, error) {
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
	if !strings.Contains(requestURL, "/models/gemini-test:generateContent") {
		t.Fatalf("request URL = %q", requestURL)
	}
	systemInstruction := requestBody["systemInstruction"].(map[string]any)
	systemParts := systemInstruction["parts"].([]any)
	if systemParts[0].(map[string]any)["text"] != "system" {
		t.Fatalf("systemInstruction = %#v", systemInstruction)
	}
	if result.Text != "hello" || result.FinishReason != domain.FinishReasonToolCall || result.Usage.TotalTokens != 3 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolCallID != "call_0" {
		t.Fatalf("tool calls = %#v", result.ToolCalls)
	}
}

func TestProviderRequestGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		model    domain.LanguageModel
		response map[string]any
		wantJSON string
	}{
		{
			name:     "openai",
			model:    providers.NewOpenAI("gpt-test", providers.Config{APIKey: "test-key", BaseURL: "https://openai.test/v1"}),
			response: openAITestResponse(),
			wantJSON: `{
				"max_tokens": 512,
				"messages": [
					{"content": "system", "role": "system"},
					{
						"content": "using tool",
						"role": "assistant",
						"tool_calls": [
							{
								"function": {"arguments": "{\"path\":\"a.txt\"}", "name": "readFile"},
								"id": "call-0",
								"type": "function"
							}
						]
					},
					{"content": "file content", "role": "tool", "tool_call_id": "call-0"},
					{"content": "hello", "role": "user"}
				],
				"model": "gpt-test",
				"temperature": 0.2,
				"tools": [
					{
						"function": {
							"description": "Read a file",
							"name": "readFile",
							"parameters": {
								"properties": {"path": {"description": "The path", "type": "string"}},
								"required": ["path"],
								"type": "object"
							}
						},
						"type": "function"
					}
				]
			}`,
		},
		{
			name:     "anthropic",
			model:    providers.NewAnthropic("claude-test", providers.Config{APIKey: "test-key", BaseURL: "https://anthropic.test/v1"}),
			response: anthropicTestResponse(),
			wantJSON: `{
				"max_tokens": 512,
				"messages": [
					{
						"content": [
							{"text": "using tool", "type": "text"},
							{"id": "call-0", "input": {"path": "a.txt"}, "name": "readFile", "type": "tool_use"}
						],
						"role": "assistant"
					},
					{
						"content": [
							{
								"content": [{"text": "file content", "type": "text"}],
								"is_error": false,
								"tool_use_id": "call-0",
								"type": "tool_result"
							}
						],
						"role": "user"
					},
					{"content": [{"text": "hello", "type": "text"}], "role": "user"}
				],
				"model": "claude-test",
				"system": [{"text": "system", "type": "text"}],
				"temperature": 0.2,
				"tools": [
					{
						"description": "Read a file",
						"input_schema": {
							"properties": {"path": {"description": "The path", "type": "string"}},
							"required": ["path"],
							"type": "object"
						},
						"name": "readFile"
					}
				]
			}`,
		},
		{
			name:     "google",
			model:    providers.NewGoogle("gemini-test", providers.Config{APIKey: "test-key", BaseURL: "https://google.test/v1beta"}),
			response: googleTestResponse(),
			wantJSON: `{
				"contents": [
					{
						"parts": [
							{"text": "using tool"},
							{"functionCall": {"args": {"path": "a.txt"}, "name": "readFile"}}
						],
						"role": "model"
					},
					{
						"parts": [
							{"functionResponse": {"name": "readFile", "response": {"result": {"result": "file content"}}}}
						],
						"role": "user"
					},
					{"parts": [{"text": "hello"}], "role": "user"}
				],
				"generationConfig": {
					"maxOutputTokens": 512,
					"temperature": 0.2
				},
				"systemInstruction": {"parts": [{"text": "system"}], "role": "user"},
				"tools": [
					{
						"functionDeclarations": [
							{
								"description": "Read a file",
								"name": "readFile",
								"parametersJsonSchema": {
									"properties": {"path": {"description": "The path", "type": "string"}},
									"required": ["path"],
									"type": "object"
								}
							}
						]
					}
				]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotJSON []byte
			model := withClient(tt.model, testHTTPClient(func(request *http.Request) (*http.Response, error) {
				var err error
				gotJSON, err = io.ReadAll(request.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				return jsonResponse(t, http.StatusOK, tt.response), nil
			}))
			if _, err := model.Generate(context.Background(), goldenParams()); err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			assertCanonicalJSON(t, string(gotJSON), tt.wantJSON)
		})
	}
}

func TestProviderStreams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		model      func(*http.Client) domain.LanguageModel
		streamBody string
		assertURL  string
		want       domain.GenerateTextResult
	}{
		{
			name: "openai",
			model: func(client *http.Client) domain.LanguageModel {
				return providers.NewOpenAI("gpt-test", providers.Config{APIKey: "test-key", BaseURL: "https://openai.test/v1", Client: client})
			},
			assertURL: "https://openai.test/v1/chat/completions",
			streamBody: strings.Join([]string{
				`data: {"choices":[{"delta":{"content":"hel"}}]}`,
				`data: {"choices":[{"delta":{"content":"lo"}}]}`,
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","function":{"name":"readFile","arguments":"{\"path\""}}]}}]}`,
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"a.txt\"}"}}]},"finish_reason":"tool_calls"}]}`,
				`data: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
				`data: [DONE]`,
				``,
			}, "\n"),
			want: domain.GenerateTextResult{
				Text:         "hello",
				FinishReason: domain.FinishReasonToolCall,
				ToolCalls:    []domain.ToolCall{{ToolCallID: "call-1", Name: "readFile", Args: map[string]any{"path": "a.txt"}}},
				Usage:        domain.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
			},
		},
		{
			name: "anthropic",
			model: func(client *http.Client) domain.LanguageModel {
				return providers.NewAnthropic("claude-test", providers.Config{APIKey: "test-key", BaseURL: "https://anthropic.test/v1", Client: client})
			},
			assertURL: "https://anthropic.test/v1/messages",
			streamBody: strings.Join([]string{
				`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hel"}}`,
				`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}`,
				`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call-1","name":"readFile","input":{}}}`,
				`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"a.txt\"}"}}`,
				`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":1,"output_tokens":2}}`,
				`event: message_stop` + "\n" + `data: {"type":"message_stop"}`,
			}, "\n\n"),
			want: domain.GenerateTextResult{
				Text:         "hello",
				FinishReason: domain.FinishReasonToolCall,
				ToolCalls:    []domain.ToolCall{{ToolCallID: "call-1", Name: "readFile", Args: map[string]any{"path": "a.txt"}}},
				Usage:        domain.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
			},
		},
		{
			name: "google",
			model: func(client *http.Client) domain.LanguageModel {
				return providers.NewGoogle("gemini-test", providers.Config{APIKey: "test-key", BaseURL: "https://google.test/v1beta", Client: client})
			},
			assertURL: "https://google.test/v1beta/models/gemini-test:streamGenerateContent?alt=sse",
			streamBody: strings.Join([]string{
				`data: {"candidates":[{"content":{"parts":[{"text":"hel"}]}}]}`,
				`data: {"candidates":[{"content":{"parts":[{"text":"lo"},{"functionCall":{"name":"readFile","args":{"path":"a.txt"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2,"totalTokenCount":3}}`,
				``,
			}, "\n"),
			want: domain.GenerateTextResult{
				Text:         "hello",
				FinishReason: domain.FinishReasonToolCall,
				ToolCalls:    []domain.ToolCall{{ToolCallID: "readFile", Name: "readFile", Args: map[string]any{"path": "a.txt"}}},
				Usage:        domain.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requestBody map[string]any
			model := tt.model(testHTTPClient(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != tt.assertURL {
					t.Fatalf("url = %q, want %q", request.URL.String(), tt.assertURL)
				}
				if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				return streamResponse(http.StatusOK, tt.streamBody), nil
			}))

			result, err := generation.CollectStreamResult(context.Background(), generation.CollectStreamParams{
				Model:          model,
				GenerateParams: sampleParams(),
			})
			if err != nil {
				t.Fatalf("CollectStreamResult() error = %v", err)
			}
			if requestBody["stream"] != true && tt.name != "google" {
				t.Fatalf("stream = %#v", requestBody["stream"])
			}
			if !reflect.DeepEqual(result, tt.want) {
				t.Fatalf("result = %#v, want %#v", result, tt.want)
			}
		})
	}
}

func TestOpenAIProviderStreamPreservesToolCallIndexOrder(t *testing.T) {
	t.Parallel()

	streamBody := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"z-call","function":{"name":"firstTool","arguments":"{\"seq\""}},{"index":1,"id":"a-call","function":{"name":"secondTool","arguments":"{\"seq\""}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":1}"}},{"index":1,"function":{"arguments":":2}"}}]},"finish_reason":"tool_calls"}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	model := providers.NewOpenAI("gpt-test", providers.Config{
		APIKey:  "test-key",
		BaseURL: "https://openai.test/v1",
		Client: testHTTPClient(func(request *http.Request) (*http.Response, error) {
			return streamResponse(http.StatusOK, streamBody), nil
		}),
	})

	result, err := generation.CollectStreamResult(context.Background(), generation.CollectStreamParams{
		Model:          model,
		GenerateParams: sampleParams(),
	})
	if err != nil {
		t.Fatalf("CollectStreamResult() error = %v", err)
	}

	want := []domain.ToolCall{
		{ToolCallID: "z-call", Name: "firstTool", Args: map[string]any{"seq": float64(1)}},
		{ToolCallID: "a-call", Name: "secondTool", Args: map[string]any{"seq": float64(2)}},
	}
	if !reflect.DeepEqual(result.ToolCalls, want) {
		t.Fatalf("tool calls = %#v, want %#v", result.ToolCalls, want)
	}
}

func TestAnthropicProviderStreamPreservesMultipleToolCallOrder(t *testing.T) {
	t.Parallel()

	model := providers.NewAnthropic("claude-test", providers.Config{
		APIKey:  "test-key",
		BaseURL: "https://anthropic.test/v1",
		Client: testHTTPClient(func(request *http.Request) (*http.Response, error) {
			return streamResponse(http.StatusOK, strings.Join([]string{
				`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"plan"}}`,
				`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call-z","name":"searchFiles","input":{}}}`,
				`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"first\"}"}}`,
				`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"call-a","name":"readFile","input":{}}}`,
				`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"second.txt\"}"}}`,
				`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":1,"output_tokens":2}}`,
				`event: message_stop` + "\n" + `data: {"type":"message_stop"}`,
			}, "\n\n")), nil
		}),
	})

	result, err := generation.CollectStreamResult(context.Background(), generation.CollectStreamParams{
		Model:          model,
		GenerateParams: sampleParams(),
	})
	if err != nil {
		t.Fatalf("CollectStreamResult() error = %v", err)
	}

	want := []domain.ToolCall{
		{ToolCallID: "call-z", Name: "searchFiles", Args: map[string]any{"query": "first"}},
		{ToolCallID: "call-a", Name: "readFile", Args: map[string]any{"path": "second.txt"}},
	}
	if !reflect.DeepEqual(result.ToolCalls, want) {
		t.Fatalf("tool calls = %#v, want %#v", result.ToolCalls, want)
	}
	if result.FinishReason != domain.FinishReasonToolCall {
		t.Fatalf("finish reason = %q, want %q", result.FinishReason, domain.FinishReasonToolCall)
	}
}

func TestGoogleProviderStreamPreservesMultipleToolCallOrder(t *testing.T) {
	t.Parallel()

	model := providers.NewGoogle("gemini-test", providers.Config{
		APIKey:  "test-key",
		BaseURL: "https://google.test/v1beta",
		Client: testHTTPClient(func(request *http.Request) (*http.Response, error) {
			return streamResponse(http.StatusOK, strings.Join([]string{
				`data: {"candidates":[{"content":{"parts":[{"text":"plan"},{"functionCall":{"id":"call-z","name":"searchFiles","args":{"query":"first"}}},{"functionCall":{"id":"call-a","name":"readFile","args":{"path":"second.txt"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2,"totalTokenCount":3}}`,
				``,
			}, "\n")), nil
		}),
	})

	result, err := generation.CollectStreamResult(context.Background(), generation.CollectStreamParams{
		Model:          model,
		GenerateParams: sampleParams(),
	})
	if err != nil {
		t.Fatalf("CollectStreamResult() error = %v", err)
	}

	want := []domain.ToolCall{
		{ToolCallID: "call-z", Name: "searchFiles", Args: map[string]any{"query": "first"}},
		{ToolCallID: "call-a", Name: "readFile", Args: map[string]any{"path": "second.txt"}},
	}
	if !reflect.DeepEqual(result.ToolCalls, want) {
		t.Fatalf("tool calls = %#v, want %#v", result.ToolCalls, want)
	}
	if result.FinishReason != domain.FinishReasonToolCall {
		t.Fatalf("finish reason = %q, want %q", result.FinishReason, domain.FinishReasonToolCall)
	}
}

func withClient(model domain.LanguageModel, client *http.Client) domain.LanguageModel {
	switch typed := model.(type) {
	case *providers.OpenAIProvider:
		_ = typed
		return providers.NewOpenAI("gpt-test", providers.Config{APIKey: "test-key", BaseURL: "https://openai.test/v1", Client: client})
	case *providers.AnthropicProvider:
		_ = typed
		return providers.NewAnthropic("claude-test", providers.Config{APIKey: "test-key", BaseURL: "https://anthropic.test/v1", Client: client})
	case *providers.GoogleProvider:
		_ = typed
		return providers.NewGoogle("gemini-test", providers.Config{APIKey: "test-key", BaseURL: "https://google.test/v1beta", Client: client})
	default:
		return model
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

func goldenParams() domain.GenerateParams {
	temperature := 0.2
	maxTokens := 512
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
			{Role: domain.MessageRoleUser, Content: "hello"},
		},
		Tools: []domain.Tool{{
			Name:        "readFile",
			Description: "Read a file",
			Parameters: domain.ToolParameters{
				Type: "object",
				Properties: map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "The path",
					},
				},
				Required: []string{"path"},
			},
		}},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
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

func streamResponse(status int, body string) *http.Response {
	body = strings.TrimSpace(body)
	if !strings.Contains(body, "\n\n") {
		body = strings.Join(strings.Split(body, "\n"), "\n\n")
	}
	body += "\n\n"
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func assertCanonicalJSON(t *testing.T, got string, want string) {
	t.Helper()

	gotCanonical := canonicalJSON(t, got)
	wantCanonical := canonicalJSON(t, want)
	if gotCanonical != wantCanonical {
		t.Fatalf("JSON mismatch\ngot:  %s\nwant: %s", gotCanonical, wantCanonical)
	}
}

func canonicalJSON(t *testing.T, input string) string {
	t.Helper()

	var value any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		t.Fatalf("unmarshal JSON: %v\n%s", err, input)
	}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal canonical JSON: %v", err)
	}
	return string(body)
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
