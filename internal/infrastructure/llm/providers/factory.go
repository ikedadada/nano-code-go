package providers

import (
	"fmt"
	"strings"

	"nano-code-go/internal/domain"
)

type Env map[string]string

type FactoryOptions struct {
	Env    Env
	Client HTTPDoer
}

func CreateModelFromEnv(options FactoryOptions) (domain.LanguageModel, error) {
	env := options.Env
	provider := env["LLM_PROVIDER"]
	modelName := env["LLM_MODEL"]
	apiKey := env["LLM_API_KEY"]

	if provider == "" {
		return nil, fmt.Errorf("LLM_PROVIDER environment variable is not set")
	}
	if modelName == "" {
		return nil, fmt.Errorf("LLM_MODEL environment variable is not set")
	}

	switch strings.ToLower(provider) {
	case "openai":
		if env["OPENAI_API_KEY"] != "" {
			apiKey = env["OPENAI_API_KEY"]
		}
		return NewOpenAI(modelName, Config{APIKey: apiKey, Client: options.Client}), nil
	case "anthropic":
		if env["ANTHROPIC_API_KEY"] != "" {
			apiKey = env["ANTHROPIC_API_KEY"]
		}
		return NewAnthropic(modelName, Config{APIKey: apiKey, Client: options.Client}), nil
	case "google":
		if env["GOOGLE_API_KEY"] != "" {
			apiKey = env["GOOGLE_API_KEY"]
		}
		return NewGoogle(modelName, Config{APIKey: apiKey, Client: options.Client}), nil
	default:
		return nil, fmt.Errorf("Unsupported LLM provider: %s", provider)
	}
}
