package domain

type A2ATextPart struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type A2APart = A2ATextPart

type A2AMessage struct {
	Kind      string    `json:"kind"`
	MessageID string    `json:"messageId"`
	Role      string    `json:"role"`
	Parts     []A2APart `json:"parts"`
}

type A2AArtifact struct {
	ArtifactID string         `json:"artifactId,omitempty"`
	Parts      []any          `json:"parts,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type A2ATaskStatus struct {
	State     string `json:"state"`
	Message   any    `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type A2ATask struct {
	Kind      string         `json:"kind,omitempty"`
	ID        string         `json:"id"`
	ContextID string         `json:"contextId,omitempty"`
	Status    *A2ATaskStatus `json:"status,omitempty"`
	Artifacts []A2AArtifact  `json:"artifacts,omitempty"`
	History   []any          `json:"history,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type A2AAgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	InputModes  []string `json:"inputModes"`
	OutputModes []string `json:"outputModes"`
}

type A2ASecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

type A2AAgentCard struct {
	ProtocolVersion    string                       `json:"protocolVersion"`
	Name               string                       `json:"name"`
	Description        string                       `json:"description"`
	URL                string                       `json:"url"`
	PreferredTransport string                       `json:"preferredTransport"`
	SecuritySchemes    map[string]A2ASecurityScheme `json:"securitySchemes,omitempty"`
	Security           []map[string][]string        `json:"security,omitempty"`
	Capabilities       A2AAgentCapabilities         `json:"capabilities"`
	DefaultInputModes  []string                     `json:"defaultInputModes"`
	DefaultOutputModes []string                     `json:"defaultOutputModes"`
	Skills             []A2AAgentSkill              `json:"skills"`
}

type A2AAgentCapabilities struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

type A2AJSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type A2AJSONRPCSuccess struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result"`
}

type A2AJSONRPCError struct {
	JSONRPC string             `json:"jsonrpc"`
	ID      any                `json:"id"`
	Error   A2AJSONRPCErrorObj `json:"error"`
}

type A2AJSONRPCErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type A2AMessageSendParams struct {
	Message A2AMessageSendMessage `json:"message"`
}

type A2AMessageSendMessage struct {
	Role      string    `json:"role"`
	Parts     []A2APart `json:"parts"`
	MessageID string    `json:"messageId"`
}

type A2AMessageSendCommand struct {
	MessageID string    `json:"messageId"`
	Parts     []A2APart `json:"parts"`
}
