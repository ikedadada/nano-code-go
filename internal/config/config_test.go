package config_test

import (
	"reflect"
	"testing"

	"nano-code-go/internal/config"
)

func TestDefault(t *testing.T) {
	t.Parallel()

	got := config.Default()
	if got.Sandbox {
		t.Fatalf("Default().Sandbox = true, want false")
	}
	wantDomains := []string{"api.github.com", "github.com"}
	if !reflect.DeepEqual(got.AllowedDomains, wantDomains) {
		t.Fatalf("Default().AllowedDomains = %#v, want %#v", got.AllowedDomains, wantDomains)
	}
}

func TestConfig_WithAllowedDomains(t *testing.T) {
	t.Parallel()

	base := config.Default()
	got := base.WithAllowedDomains([]string{"example.com"})

	if !reflect.DeepEqual(base.AllowedDomains, []string{"api.github.com", "github.com"}) {
		t.Fatalf("base config was mutated: %#v", base.AllowedDomains)
	}
	want := []string{"api.github.com", "github.com", "example.com"}
	if !reflect.DeepEqual(got.AllowedDomains, want) {
		t.Fatalf("WithAllowedDomains() = %#v, want %#v", got.AllowedDomains, want)
	}
}
