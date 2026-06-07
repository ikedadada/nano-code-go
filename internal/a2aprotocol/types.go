package a2aprotocol

type TextPart struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type Part = TextPart

type Message struct {
	Kind      string `json:"kind"`
	MessageID string `json:"messageId"`
	Role      string `json:"role"`
	Parts     []Part `json:"parts"`
}

type Artifact struct {
	ArtifactID string         `json:"artifactId,omitempty"`
	Parts      []any          `json:"parts,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type TaskStatus struct {
	State     string `json:"state"`
	Message   any    `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type Task struct {
	Kind      string         `json:"kind,omitempty"`
	ID        string         `json:"id"`
	ContextID string         `json:"contextId,omitempty"`
	Status    *TaskStatus    `json:"status,omitempty"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	History   []any          `json:"history,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	InputModes  []string `json:"inputModes"`
	OutputModes []string `json:"outputModes"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

type AgentCard struct {
	ProtocolVersion    string                    `json:"protocolVersion"`
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	URL                string                    `json:"url"`
	PreferredTransport string                    `json:"preferredTransport"`
	SecuritySchemes    map[string]SecurityScheme `json:"securitySchemes,omitempty"`
	Security           []map[string][]string     `json:"security,omitempty"`
	Capabilities       AgentCapabilities         `json:"capabilities"`
	DefaultInputModes  []string                  `json:"defaultInputModes"`
	DefaultOutputModes []string                  `json:"defaultOutputModes"`
	Skills             []AgentSkill              `json:"skills"`
}

type AgentCapabilities struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

type JSONRPCSuccess struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result"`
}

type JSONRPCError struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Error   JSONRPCErrorObj `json:"error"`
}

type JSONRPCErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type MessageSendParams struct {
	Message MessageSendMessage `json:"message"`
}

type MessageSendMessage struct {
	Role      string `json:"role"`
	Parts     []Part `json:"parts"`
	MessageID string `json:"messageId"`
}
