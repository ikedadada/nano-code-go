//go:build integration

package providers_test

import (
	"context"
	"os"
	"testing"
	"time"

	"nano-code-go/internal/domain"
	"nano-code-go/internal/infrastructure/llm/providers"
)

func TestIntegrationOpenAIProviderGenerate(t *testing.T) {
	t.Parallel()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}
	modelID := envOrDefault("OPENAI_INTEGRATION_MODEL", "gpt-4o-mini")
	assertLiveGenerate(t, providers.NewOpenAI(modelID, providers.Config{APIKey: apiKey}))
}

func TestIntegrationAnthropicProviderGenerate(t *testing.T) {
	t.Parallel()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY is not set")
	}
	modelID := envOrDefault("ANTHROPIC_INTEGRATION_MODEL", "claude-3-5-haiku-latest")
	assertLiveGenerate(t, providers.NewAnthropic(modelID, providers.Config{APIKey: apiKey}))
}

func TestIntegrationGoogleProviderGenerate(t *testing.T) {
	t.Parallel()

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		t.Skip("GOOGLE_API_KEY is not set")
	}
	modelID := envOrDefault("GOOGLE_INTEGRATION_MODEL", "gemini-1.5-flash")
	assertLiveGenerate(t, providers.NewGoogle(modelID, providers.Config{APIKey: apiKey}))
}

func assertLiveGenerate(t *testing.T, model domain.LanguageModel) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	maxTokens := 16
	result, err := model.Generate(ctx, domain.GenerateParams{
		Messages:  []domain.Message{{Role: domain.MessageRoleUser, Content: "Reply with exactly: ok"}},
		MaxTokens: &maxTokens,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Text == "" {
		t.Fatalf("Generate() returned empty text: %#v", result)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
