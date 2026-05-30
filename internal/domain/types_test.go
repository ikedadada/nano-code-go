package domain_test

import (
	"reflect"
	"testing"

	"nano-code-go/internal/domain"
)

func TestLLMAPIError(t *testing.T) {
	t.Parallel()

	raw := map[string]any{"requestId": "req-1"}
	err := &domain.LLMAPIError{
		Status:   429,
		Provider: "openai",
		Code:     "rate_limit",
		Raw:      raw,
	}

	if got, want := err.Error(), "LLM API Error: openai responded with status 429"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if err.Status != 429 {
		t.Fatalf("Status = %d, want 429", err.Status)
	}
	if err.Provider != "openai" {
		t.Fatalf("Provider = %q, want openai", err.Provider)
	}
	if err.Code != "rate_limit" {
		t.Fatalf("Code = %q, want rate_limit", err.Code)
	}
	if !reflect.DeepEqual(err.Raw, raw) {
		t.Fatalf("Raw = %#v, want original raw value", err.Raw)
	}
}
