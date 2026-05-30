package a2a_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	appa2a "nano-code-go/internal/application/a2a"
	a2ahttp "nano-code-go/internal/interfaces/a2a"
)

func newTestApp(t *testing.T, runAgent appa2a.RunAgent) http.Handler {
	t.Helper()
	if runAgent == nil {
		runAgent = func(_ context.Context, request appa2a.RunAgentRequest) (appa2a.RunAgentResponse, error) {
			return appa2a.RunAgentResponse{Text: "answer:" + request.Prompt}, nil
		}
	}
	return a2ahttp.NewApp(a2ahttp.Options{
		Env: a2ahttp.Env{
			"A2A_AUTH_TOKEN": "secret-token",
			"A2A_AGENT_URL":  "http://localhost:3000/a2a",
		},
		WorkspaceRoot: "/workspace",
		RunAgent:      runAgent,
	})
}

func TestAppServesAgentCard(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)

	newTestApp(t, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var card map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &card); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if card["name"] != "nano-code" {
		t.Fatalf("card name = %#v", card["name"])
	}
	if card["url"] != "http://localhost:3000/a2a" {
		t.Fatalf("card url = %#v", card["url"])
	}
	if card["preferredTransport"] != "JSONRPC" {
		t.Fatalf("preferredTransport = %#v", card["preferredTransport"])
	}
}

func TestAppHandlesMessageSend(t *testing.T) {
	t.Parallel()

	body := `{"jsonrpc":"2.0","id":"req-1","method":"message/send","params":{"message":{"role":"user","messageId":"msg-1","parts":[{"kind":"text","text":"hello"}]}}}`
	request := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer secret-token")
	response := httptest.NewRecorder()

	newTestApp(t, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["jsonrpc"] != "2.0" {
		t.Fatalf("jsonrpc = %#v", result["jsonrpc"])
	}
	if result["id"] != "req-1" {
		t.Fatalf("id = %#v", result["id"])
	}
	message := result["result"].(map[string]any)
	if message["kind"] != "message" || message["role"] != "agent" {
		t.Fatalf("message = %#v", message)
	}
	parts := message["parts"].([]any)
	if parts[0].(map[string]any)["text"] != "answer:hello" {
		t.Fatalf("parts = %#v", parts)
	}
}

func TestAppPassesEnvOptionsToRunner(t *testing.T) {
	t.Parallel()

	var requests []appa2a.RunAgentRequest
	app := a2ahttp.NewApp(a2ahttp.Options{
		Env: a2ahttp.Env{
			"PORT":                "8787",
			"HOST":                "127.0.0.1",
			"A2A_AUTH_TOKEN":      "secret-token",
			"A2A_SANDBOX":         "true",
			"A2A_ALLOWED_DOMAINS": "example.com, docs.example.com",
		},
		WorkspaceRoot: "/custom-workspace",
		RunAgent: func(_ context.Context, request appa2a.RunAgentRequest) (appa2a.RunAgentResponse, error) {
			requests = append(requests, request)
			return appa2a.RunAgentResponse{Text: "configured-answer"}, nil
		},
	})

	cardResponse := httptest.NewRecorder()
	app.ServeHTTP(cardResponse, httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil))
	var card map[string]any
	if err := json.Unmarshal(cardResponse.Body.Bytes(), &card); err != nil {
		t.Fatalf("decode card: %v", err)
	}
	if card["url"] != "http://127.0.0.1:8787/a2a" {
		t.Fatalf("card URL = %#v", card["url"])
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","messageId":"msg-1","parts":[{"kind":"text","text":"hello"}]}}}`
	request := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer secret-token")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	want := []appa2a.RunAgentRequest{{
		Prompt:         "hello",
		IssueDriven:    false,
		Streaming:      false,
		Yolo:           true,
		Sandbox:        true,
		AllowedDomains: []string{"example.com", "docs.example.com"},
		WorkspaceRoot:  "/custom-workspace",
	}}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
}

func TestAppJSONRPCErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		headers    map[string]string
		body       string
		wantStatus int
		wantCode   float64
	}{
		{
			name:       "unauthorized",
			headers:    map[string]string{"Content-Type": "application/json"},
			body:       `{"jsonrpc":"2.0","id":"req-1","method":"message/send"}`,
			wantStatus: http.StatusUnauthorized,
			wantCode:   -32001,
		},
		{
			name:       "non json",
			headers:    map[string]string{"Authorization": "Bearer secret-token"},
			body:       `not-json`,
			wantStatus: http.StatusUnsupportedMediaType,
			wantCode:   -32600,
		},
		{
			name: "unsupported method",
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer secret-token",
			},
			body:       `{"jsonrpc":"2.0","id":1,"method":"tasks/get"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   -32601,
		},
		{
			name: "invalid params",
			headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer secret-token",
			},
			body:       `{"jsonrpc":"2.0","id":2,"method":"message/send","params":{}}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   -32602,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodPost, "/a2a", strings.NewReader(tt.body))
			for key, value := range tt.headers {
				request.Header.Set(key, value)
			}
			response := httptest.NewRecorder()

			newTestApp(t, nil).ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tt.wantStatus, response.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			errObj := body["error"].(map[string]any)
			if errObj["code"] != tt.wantCode {
				t.Fatalf("error code = %#v, want %#v", errObj["code"], tt.wantCode)
			}
		})
	}
}

func TestAppServesDocs(t *testing.T) {
	t.Parallel()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	newTestApp(t, nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content-type = %q", response.Header().Get("Content-Type"))
	}
	body := response.Body.String()
	for _, want := range []string{"nano-code A2A API", "/.well-known/agent-card.json", "/a2a", "Send an A2A message"} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs missing %q in %s", want, body)
		}
	}
}
