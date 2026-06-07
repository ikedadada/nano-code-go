package a2a

type RunAgentRequest struct {
	Prompt         string
	IssueDriven    bool
	Streaming      bool
	Yolo           bool
	Sandbox        bool
	AllowedDomains []string
	WorkspaceRoot  string
}

type RunAgentResponse struct {
	Text     string
	Streamed bool
}

type TextPart struct {
	Text string
}

type Message struct {
	MessageID string
	Role      string
	Parts     []TextPart
}

type AgentCard struct {
	ProtocolVersion    string
	Name               string
	Description        string
	URL                string
	PreferredTransport string
	AuthRequired       bool
	Capabilities       AgentCapabilities
	DefaultInputModes  []string
	DefaultOutputModes []string
	Skills             []AgentSkill
}

type AgentCapabilities struct {
	Streaming              bool
	PushNotifications      bool
	StateTransitionHistory bool
}

type AgentSkill struct {
	ID          string
	Name        string
	Description string
	Tags        []string
	InputModes  []string
	OutputModes []string
}
