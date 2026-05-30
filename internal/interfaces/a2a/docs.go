package a2a

func openAPIDocument() map[string]any {
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "nano-code A2A API",
			"version":     "0.1.0",
			"description": "A2A Agent Card discovery and JSON-RPC message/send endpoint for nano-code.",
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "opaque",
				},
			},
		},
		"paths": map[string]any{
			"/.well-known/agent-card.json": map[string]any{
				"get": map[string]any{
					"tags":    []string{"A2A"},
					"summary": "Get the A2A Agent Card",
				},
			},
			"/a2a": map[string]any{
				"post": map[string]any{
					"tags":     []string{"A2A"},
					"summary":  "Send an A2A message",
					"security": []map[string][]string{{"bearerAuth": {}}},
				},
			},
		},
	}
}
