package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nano-code-go/internal/a2aprotocol"
	"nano-code-go/internal/agentruntime"
	appa2a "nano-code-go/internal/application/a2a"
)

type Env = agentruntime.Env

func EnvFromOS() Env {
	env := make(Env)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

type Options struct {
	Env           Env
	RunAgent      appa2a.RunAgent
	WorkspaceRoot string
}

type App struct {
	handler http.Handler
	service *appa2a.Service
	token   string
}

func NewApp(options Options) *App {
	env := options.Env
	if env == nil {
		env = Env{}
	}
	runAgent := options.RunAgent
	if runAgent == nil {
		runAgent = func(context.Context, appa2a.RunAgentRequest) (appa2a.RunAgentResponse, error) {
			return appa2a.RunAgentResponse{}, errors.New("agent runner is not configured")
		}
	}
	workspaceRoot := options.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = filepath.Join(".", "workspace")
	}

	port := envInt(env, "PORT", 3000)
	host := envString(env, "HOST", "localhost")
	agentURL := envString(env, "A2A_AGENT_URL", fmt.Sprintf("http://%s:%d/a2a", host, port))
	token := envString(env, "A2A_AUTH_TOKEN", "")

	service := appa2a.NewService(appa2a.ServiceConfig{
		AgentURL:       agentURL,
		RunAgent:       runAgent,
		WorkspaceRoot:  workspaceRoot,
		AuthRequired:   token != "",
		Sandbox:        envString(env, "A2A_SANDBOX", "") == "true",
		AllowedDomains: parseCSV(envString(env, "A2A_ALLOWED_DOMAINS", "")),
	})

	app := &App{service: service, token: token}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent-card.json", app.handleAgentCard)
	mux.HandleFunc("POST /a2a", app.handleMessageSend)
	mux.HandleFunc("GET /docs", app.handleDocs)
	app.handler = mux
	return app
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.handler.ServeHTTP(w, r)
}

func Run(ctx context.Context, stdout, stderr io.Writer, env Env) error {
	if stderr == nil {
		stderr = io.Discard
	}
	if stdout == nil {
		stdout = io.Discard
	}
	port := envInt(env, "PORT", 3000)
	addr := ":" + strconv.Itoa(port)
	server := &http.Server{
		Addr:              addr,
		Handler:           NewApp(Options{Env: env, RunAgent: defaultRunAgent(stderr, env)}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		_, _ = fmt.Fprintf(stdout, "A2A server listening on http://localhost:%d\n", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown a2a server: %w", err)
		}
		<-errCh
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func defaultRunAgent(stderr io.Writer, env Env) appa2a.RunAgent {
	return func(ctx context.Context, request appa2a.RunAgentRequest) (appa2a.RunAgentResponse, error) {
		result, err := agentruntime.RunAgentWithIO(ctx, agentruntime.RunAgentRequest{
			Prompt:         request.Prompt,
			IssueDriven:    request.IssueDriven,
			Streaming:      request.Streaming,
			Yolo:           request.Yolo,
			Sandbox:        request.Sandbox,
			AllowedDomains: append([]string(nil), request.AllowedDomains...),
			WorkspaceRoot:  request.WorkspaceRoot,
		}, strings.NewReader(""), io.Discard, stderr, env)
		if err != nil {
			return appa2a.RunAgentResponse{}, err
		}
		return appa2a.RunAgentResponse{
			Text:     result.Text,
			Streamed: result.Streamed,
		}, nil
	}
}

func (a *App) handleAgentCard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, protocolAgentCard(a.service.AgentCard()))
}

func (a *App) handleMessageSend(w http.ResponseWriter, r *http.Request) {
	if a.token != "" && r.Header.Get("Authorization") != "Bearer "+a.token {
		writeJSONRPCError(w, http.StatusUnauthorized, nil, -32001, "Unauthorized")
		return
	}

	if !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeJSONRPCError(w, http.StatusUnsupportedMediaType, nil, -32600, "Content-Type must be application/json")
		return
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeJSONRPCError(w, http.StatusBadRequest, nil, -32700, "Parse error")
		return
	}

	id := parseJSONRPCID(raw["id"])
	version := parseString(raw["jsonrpc"])
	if version != "2.0" {
		writeJSONRPCError(w, http.StatusBadRequest, id, -32600, "Invalid Request")
		return
	}
	method := parseString(raw["method"])
	if method == "" {
		writeJSONRPCError(w, http.StatusBadRequest, id, -32600, "Invalid Request")
		return
	}
	if method != "message/send" {
		writeJSONRPCError(w, http.StatusBadRequest, id, -32601, "Method not found")
		return
	}

	var params a2aprotocol.MessageSendParams
	if err := json.Unmarshal(raw["params"], &params); err != nil ||
		params.Message.Role != "user" ||
		params.Message.MessageID == "" ||
		len(params.Message.Parts) == 0 {
		writeJSONRPCError(w, http.StatusBadRequest, id, -32602, "Invalid params")
		return
	}
	for _, part := range params.Message.Parts {
		if part.Kind != "text" {
			writeJSONRPCError(w, http.StatusBadRequest, id, -32602, "Invalid params")
			return
		}
	}

	message, err := a.service.SendMessage(r.Context(), appTextParts(params.Message.Parts))
	if err != nil {
		writeJSONRPCError(w, http.StatusBadRequest, id, -32602, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, a2aprotocol.JSONRPCSuccess{
		JSONRPC: "2.0",
		ID:      id,
		Result:  protocolMessage(message),
	})
}

func (a *App) handleDocs(w http.ResponseWriter, _ *http.Request) {
	spec := openAPIDocument()
	specJSON, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		http.Error(w, "failed to render docs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>nano-code A2A API</title></head>
<body>
<h1>nano-code A2A API</h1>
<p>Static OpenAPI 3.1 document for the Go A2A endpoints.</p>
<pre id="openapi">%s</pre>
</body>
</html>`, html.EscapeString(string(specJSON)))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONRPCError(w http.ResponseWriter, status int, id any, code int, message string) {
	writeJSON(w, status, a2aprotocol.JSONRPCError{
		JSONRPC: "2.0",
		ID:      id,
		Error: a2aprotocol.JSONRPCErrorObj{
			Code:    code,
			Message: message,
		},
	})
}

func protocolAgentCard(card appa2a.AgentCard) a2aprotocol.AgentCard {
	result := a2aprotocol.AgentCard{
		ProtocolVersion:    card.ProtocolVersion,
		Name:               card.Name,
		Description:        card.Description,
		URL:                card.URL,
		PreferredTransport: card.PreferredTransport,
		Capabilities: a2aprotocol.AgentCapabilities{
			Streaming:              card.Capabilities.Streaming,
			PushNotifications:      card.Capabilities.PushNotifications,
			StateTransitionHistory: card.Capabilities.StateTransitionHistory,
		},
		DefaultInputModes:  append([]string(nil), card.DefaultInputModes...),
		DefaultOutputModes: append([]string(nil), card.DefaultOutputModes...),
		Skills:             protocolAgentSkills(card.Skills),
	}
	if card.AuthRequired {
		result.SecuritySchemes = map[string]a2aprotocol.SecurityScheme{
			"bearerAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "opaque",
				Description:  "Bearer token required for A2A JSON-RPC requests.",
			},
		}
		result.Security = []map[string][]string{{"bearerAuth": {}}}
	}
	return result
}

func protocolAgentSkills(skills []appa2a.AgentSkill) []a2aprotocol.AgentSkill {
	result := make([]a2aprotocol.AgentSkill, 0, len(skills))
	for _, skill := range skills {
		result = append(result, a2aprotocol.AgentSkill{
			ID:          skill.ID,
			Name:        skill.Name,
			Description: skill.Description,
			Tags:        append([]string(nil), skill.Tags...),
			InputModes:  append([]string(nil), skill.InputModes...),
			OutputModes: append([]string(nil), skill.OutputModes...),
		})
	}
	return result
}

func appTextParts(parts []a2aprotocol.Part) []appa2a.TextPart {
	result := make([]appa2a.TextPart, 0, len(parts))
	for _, part := range parts {
		result = append(result, appa2a.TextPart{Text: part.Text})
	}
	return result
}

func protocolMessage(message appa2a.Message) a2aprotocol.Message {
	return a2aprotocol.Message{
		Kind:      "message",
		MessageID: message.MessageID,
		Role:      message.Role,
		Parts:     protocolParts(message.Parts),
	}
}

func protocolParts(parts []appa2a.TextPart) []a2aprotocol.Part {
	result := make([]a2aprotocol.Part, 0, len(parts))
	for _, part := range parts {
		result = append(result, a2aprotocol.Part{Kind: "text", Text: part.Text})
	}
	return result
}

func parseJSONRPCID(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	switch id := value.(type) {
	case string:
		return id
	case float64:
		if id == float64(int64(id)) {
			return int64(id)
		}
		return id
	case nil:
		return nil
	default:
		return nil
	}
}

func parseString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return value
}

func envString(env Env, key, fallback string) string {
	if value, ok := env[key]; ok && value != "" {
		return value
	}
	return fallback
}

func envInt(env Env, key string, fallback int) int {
	value := envString(env, key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
