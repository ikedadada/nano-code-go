package tools_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"nano-code-go/internal/infrastructure/tools"
)

func TestWebFetchRejectsDisallowedDomains(t *testing.T) {
	t.Parallel()

	_, err := tools.WebFetch([]string{"example.com"}, fakeDoer(func(*http.Request) (*http.Response, error) {
		t.Fatal("HTTP client should not be called")
		return nil, nil
	})).Execute(context.Background(), map[string]any{"url": "https://not-example.test/page"})
	if err == nil || !strings.Contains(err.Error(), "is not allowed") {
		t.Fatalf("webFetch error = %v, want domain error", err)
	}
}

func TestWebFetchAllowsConfiguredDomainsAndSubdomains(t *testing.T) {
	t.Parallel()

	var fetchedURL string
	tool := tools.WebFetch([]string{"example.com"}, fakeDoer(func(request *http.Request) (*http.Response, error) {
		fetchedURL = request.URL.String()
		return httpResponse(http.StatusOK, "ok"), nil
	}))

	result, err := tool.Execute(context.Background(), map[string]any{"url": "https://docs.example.com/page"})
	if err != nil {
		t.Fatalf("webFetch error = %v", err)
	}
	if result != "ok" {
		t.Fatalf("webFetch = %q, want ok", result)
	}
	if fetchedURL != "https://docs.example.com/page" {
		t.Fatalf("fetched URL = %q", fetchedURL)
	}
}

func TestWebFetchRejectsOversizedResponses(t *testing.T) {
	t.Parallel()

	tool := tools.WebFetch([]string{"example.com"}, fakeDoer(func(*http.Request) (*http.Response, error) {
		return httpResponse(http.StatusOK, strings.Repeat("x", 1024*1024+1)), nil
	}))

	_, err := tool.Execute(context.Background(), map[string]any{"url": "https://example.com/page"})
	if err == nil || !strings.Contains(err.Error(), "Response size exceeds") {
		t.Fatalf("webFetch error = %v, want response size error", err)
	}
}

func TestWebFetchRejectsHTTPErrorAndRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
	}{
		{name: "server error", status: http.StatusInternalServerError},
		{name: "redirect", status: http.StatusFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool := tools.WebFetch([]string{"example.com"}, fakeDoer(func(*http.Request) (*http.Response, error) {
				return httpResponse(tt.status, "body"), nil
			}))
			_, err := tool.Execute(context.Background(), map[string]any{"url": "https://example.com/page"})
			if err == nil || !strings.Contains(err.Error(), "HTTP Error") {
				t.Fatalf("webFetch error = %v, want HTTP Error", err)
			}
		})
	}
}

type fakeDoer func(*http.Request) (*http.Response, error)

func (f fakeDoer) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
	}
}
