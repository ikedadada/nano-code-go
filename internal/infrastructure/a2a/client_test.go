package a2a_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	infraa2a "nano-code-go/internal/infrastructure/a2a"
)

func TestClient_FetchAgentCard(t *testing.T) {
	t.Parallel()

	client := infraa2a.NewClient(httpClient(func(r *http.Request) (*http.Response, error) {
		if got, want := r.Header.Get("Authorization"), "Bearer secret-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		return jsonHTTPResponse(t, http.StatusOK, map[string]any{
			"protocolVersion":    "0.3.0",
			"name":               "Remote Agent",
			"description":        "Remote A2A agent.",
			"url":                "http://remote.example/a2a",
			"preferredTransport": "JSONRPC",
			"capabilities": map[string]any{
				"streaming":              false,
				"pushNotifications":      false,
				"stateTransitionHistory": false,
			},
			"defaultInputModes":  []string{"text/plain"},
			"defaultOutputModes": []string{"text/plain"},
			"skills":             []any{},
		}), nil
	}))

	card, err := client.FetchAgentCard(context.Background(), "http://remote.example/card", "secret-token")
	if err != nil {
		t.Fatalf("FetchAgentCard() error = %v", err)
	}
	if card.Name != "Remote Agent" || card.URL != "http://remote.example/a2a" {
		t.Fatalf("card = %#v", card)
	}
}

func TestClient_SendMessage(t *testing.T) {
	t.Parallel()

	client := infraa2a.NewClient(httpClient(func(r *http.Request) (*http.Response, error) {
		if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
			t.Fatalf("Content-Type = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer secret-token"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request["jsonrpc"] != "2.0" || request["method"] != "message/send" {
			t.Fatalf("request = %#v", request)
		}
		params := request["params"].(map[string]any)
		message := params["message"].(map[string]any)
		parts := message["parts"].([]any)
		if parts[0].(map[string]any)["text"] != "hello remote" {
			t.Fatalf("parts = %#v", parts)
		}

		return jsonHTTPResponse(t, http.StatusOK, map[string]any{
			"jsonrpc": "2.0",
			"id":      request["id"],
			"result": map[string]any{
				"kind":      "message",
				"messageId": "agent-message-1",
				"role":      "agent",
				"parts": []map[string]string{
					{"kind": "text", "text": "hello"},
					{"kind": "text", "text": "from remote"},
				},
			},
		}), nil
	}))

	got, err := client.SendMessage(context.Background(), infraa2a.RemoteAgentEndpoint{
		ID:          "remote-agent",
		Name:        "Remote Agent",
		URL:         "http://remote.example/a2a",
		BearerToken: "secret-token",
	}, "hello remote")
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if got != "hello\nfrom remote" {
		t.Fatalf("SendMessage() = %q, want hello\\nfrom remote", got)
	}
}

func TestClient_SendMessageExtractsTaskResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result map[string]any
		want   string
	}{
		{
			name: "artifact response",
			result: map[string]any{
				"artifacts": []map[string]any{{
					"parts": []map[string]string{{"kind": "text", "text": "Ahoy from Docker Agent."}},
				}},
			},
			want: "Ahoy from Docker Agent.",
		},
		{
			name: "status message",
			result: map[string]any{
				"kind": "task",
				"id":   "task-1",
				"status": map[string]any{
					"state": "completed",
					"message": map[string]any{
						"kind":  "message",
						"role":  "agent",
						"parts": []map[string]string{{"text": "Ahoy, matey! Hello."}},
					},
				},
			},
			want: "Ahoy, matey! Hello.",
		},
		{
			name: "artifact response preferred over status message",
			result: map[string]any{
				"kind": "task",
				"id":   "task-1",
				"status": map[string]any{
					"state": "completed",
					"message": map[string]any{
						"kind":  "message",
						"role":  "agent",
						"parts": []map[string]string{{"kind": "text", "text": "Still working..."}},
					},
				},
				"artifacts": []map[string]any{{
					"parts": []map[string]string{{"kind": "text", "text": "Final artifact answer."}},
				}},
			},
			want: "Final artifact answer.",
		},
		{
			name: "agent history",
			result: map[string]any{
				"kind": "task",
				"history": []map[string]any{
					{"kind": "message", "role": "user", "parts": []map[string]string{{"kind": "text", "text": "Say hello."}}},
					{"kind": "message", "role": "agent", "parts": []map[string]string{{"kind": "text", "text": "Ahoy, matey! Hello."}}},
				},
			},
			want: "Ahoy, matey! Hello.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := infraa2a.NewClient(httpClient(func(r *http.Request) (*http.Response, error) {
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				return jsonHTTPResponse(t, http.StatusOK, map[string]any{
					"jsonrpc": "2.0",
					"id":      request["id"],
					"result":  tt.result,
				}), nil
			}))
			got, err := client.SendMessage(context.Background(), infraa2a.RemoteAgentEndpoint{
				ID:  "pirate",
				URL: "http://localhost:9000/invoke",
			}, "say hello")
			if err != nil {
				t.Fatalf("SendMessage() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("SendMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClient_SendMessageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    map[string]any
		wantErr string
	}{
		{
			name: "failed task",
			body: map[string]any{
				"jsonrpc": "2.0",
				"id":      "req-1",
				"result": map[string]any{
					"kind": "task",
					"status": map[string]any{
						"state": "failed",
						"message": map[string]any{
							"kind":  "message",
							"role":  "agent",
							"parts": []map[string]string{{"kind": "text", "text": "agent run failed"}},
						},
					},
				},
			},
			wantErr: "A2A agent 'pirate' failed: agent run failed",
		},
		{
			name: "json-rpc error",
			body: map[string]any{
				"jsonrpc": "2.0",
				"id":      "req-1",
				"error":   map[string]any{"code": -32602, "message": "Invalid params"},
			},
			wantErr: "A2A agent 'pirate' returned JSON-RPC error -32602: Invalid params",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := infraa2a.NewClient(httpClient(func(*http.Request) (*http.Response, error) {
				return jsonHTTPResponse(t, http.StatusOK, tt.body), nil
			}))

			_, err := client.SendMessage(context.Background(), infraa2a.RemoteAgentEndpoint{
				ID:  "pirate",
				URL: "http://localhost:9000/invoke",
			}, "hello")
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("SendMessage() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func httpClient(do func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{Transport: doerFunc(do)}
}

func jsonHTTPResponse(t *testing.T, status int, value any) *http.Response {
	t.Helper()

	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
}
