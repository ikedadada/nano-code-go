package providers

import (
	"strings"
	"testing"
)

func TestParseJSONObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    map[string]any
		wantErr string
	}{
		{name: "object", input: `{"name":"nano","count":2}`, want: map[string]any{"name": "nano", "count": float64(2)}},
		{name: "empty", input: "", want: map[string]any{}},
		{name: "array", input: `[1,2,3]`, wantErr: "JSON value must be an object"},
		{name: "invalid", input: `{`, wantErr: "Invalid JSON:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseJSONObject(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseJSONObject() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseJSONObject() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseJSONObject() = %#v, want %#v", got, tt.want)
			}
			for key, want := range tt.want {
				if got[key] != want {
					t.Fatalf("parseJSONObject()[%q] = %#v, want %#v", key, got[key], want)
				}
			}
		})
	}
}
