package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"nano-code-go/internal/domain"
)

type RemoteAgentEndpoint struct {
	ID          string
	Name        string
	URL         string
	BearerToken string
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	httpClient HTTPDoer
}

func NewClient(httpClient HTTPDoer) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient}
}

func (c *Client) FetchAgentCard(ctx context.Context, agentCardURL, bearerToken string) (domain.A2AAgentCard, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, agentCardURL, nil)
	if err != nil {
		return domain.A2AAgentCard{}, fmt.Errorf("create agent card request: %w", err)
	}
	if bearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return domain.A2AAgentCard{}, fmt.Errorf("fetch A2A agent card: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.A2AAgentCard{}, fmt.Errorf("A2A Agent Card fetch failed with HTTP %d %s", response.StatusCode, response.Status)
	}

	var card domain.A2AAgentCard
	if err := json.NewDecoder(response.Body).Decode(&card); err != nil {
		return domain.A2AAgentCard{}, fmt.Errorf("decode A2A agent card: %w", err)
	}
	return card, nil
}

func (c *Client) SendMessage(ctx context.Context, agent RemoteAgentEndpoint, prompt string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      uuid.NewString(),
		"method":  "message/send",
		"params": map[string]any{
			"message": map[string]any{
				"role":      "user",
				"messageId": uuid.NewString(),
				"parts":     []map[string]string{{"kind": "text", "text": prompt}},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("encode A2A message/send request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, agent.URL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create A2A message/send request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if agent.BearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+agent.BearerToken)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("send A2A message: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("A2A agent '%s' responded with HTTP %d %s", agent.ID, response.StatusCode, response.Status)
	}

	var rpc struct {
		Result any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&rpc); err != nil {
		return "", fmt.Errorf("decode A2A message/send response: %w", err)
	}
	if rpc.Error != nil {
		return "", fmt.Errorf("A2A agent '%s' returned JSON-RPC error %d: %s", agent.ID, rpc.Error.Code, rpc.Error.Message)
	}

	text, resultErr, ok := extractMessageSendResultText(rpc.Result)
	if resultErr != "" {
		return "", fmt.Errorf("A2A agent '%s' failed: %s", agent.ID, resultErr)
	}
	if ok {
		return text, nil
	}
	return "", fmt.Errorf("A2A agent '%s' returned an invalid message: %s", agent.ID, formatInvalidResult(rpc.Result))
}

func extractMessageSendResultText(result any) (text string, resultErr string, ok bool) {
	record, ok := result.(map[string]any)
	if !ok {
		return "", "", false
	}

	if record["kind"] == "message" && record["role"] == "agent" {
		text := strings.TrimSpace(strings.Join(textFromParts(record["parts"]), "\n"))
		return text, "", text != ""
	}

	var statusText string
	if status, _ := record["status"].(map[string]any); status != nil {
		state := normalizeTaskState(status["state"])
		statusText = textFromMessage(status["message"])
		if isTerminalFailureState(state) {
			if statusText != "" {
				return "", statusText, true
			}
			return "", state, true
		}
		if isNonTerminalState(state) {
			return "", "task is not completed: " + state, true
		}
	}

	if text := textFromArtifacts(record["artifacts"]); text != "" {
		return text, "", true
	}
	if statusText != "" {
		return statusText, "", true
	}
	if text := textFromHistory(record["history"]); text != "" {
		return text, "", true
	}

	return "", "", false
}

func textFromParts(value any) []string {
	parts, ok := value.([]any)
	if !ok {
		return nil
	}
	var texts []string
	for _, part := range parts {
		record, ok := part.(map[string]any)
		if !ok {
			continue
		}
		text, ok := record["text"].(string)
		if !ok {
			continue
		}
		kind, hasKind := record["kind"].(string)
		partType, hasType := record["type"].(string)
		if kind == "text" || partType == "text" || (!hasKind && !hasType) {
			texts = append(texts, text)
		}
	}
	return texts
}

func textFromMessage(value any) string {
	record, ok := value.(map[string]any)
	if !ok || record["role"] != "agent" {
		return ""
	}
	return strings.TrimSpace(strings.Join(textFromParts(record["parts"]), "\n"))
}

func textFromArtifacts(value any) string {
	artifacts, ok := value.([]any)
	if !ok {
		return ""
	}
	var texts []string
	for _, artifact := range artifacts {
		record, ok := artifact.(map[string]any)
		if !ok {
			continue
		}
		texts = append(texts, textFromParts(record["parts"])...)
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}

func textFromHistory(value any) string {
	history, ok := value.([]any)
	if !ok {
		return ""
	}
	var texts []string
	for _, item := range history {
		if text := textFromMessage(item); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}

func normalizeTaskState(value any) string {
	state := strings.ToLower(fmt.Sprint(value))
	state = strings.TrimPrefix(state, "task_state_")
	return strings.ReplaceAll(state, "_", "-")
}

func isTerminalFailureState(state string) bool {
	switch state {
	case "failed", "canceled", "cancelled", "rejected":
		return true
	default:
		return false
	}
}

func isNonTerminalState(state string) bool {
	switch state {
	case "submitted", "working", "input-required", "auth-required", "unknown", "unspecified":
		return true
	default:
		return false
	}
}

func formatInvalidResult(result any) string {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprint(result)
	}
	const max = 500
	if len(body) > max {
		body = body[:max]
	}
	return string(body)
}
