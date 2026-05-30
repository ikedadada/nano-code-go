package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"nano-code-go/internal/domain"
)

const maxWebFetchSize = 1024 * 1024

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func WebFetch(allowedDomains []string, client HTTPDoer) domain.Tool {
	if client == nil {
		client = &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	return domain.Tool{
		Name:          "webFetch",
		Description:   "Fetches the web page from the specified URL",
		NeedsApproval: true,
		Parameters: domain.ToolParameters{
			Type: "object",
			Properties: map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to fetch",
				},
			},
			Required: []string{"url"},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			rawURL, err := stringArg(args, "url")
			if err != nil {
				return "", err
			}
			return webFetchExecute(ctx, allowedDomains, client, rawURL)
		},
	}
}

func webFetchExecute(ctx context.Context, allowedDomains []string, client HTTPDoer, rawURL string) (string, error) {
	targetURL, err := url.Parse(rawURL)
	if err != nil || targetURL.Scheme == "" || targetURL.Hostname() == "" {
		return "", errors.New("Invalid URL format")
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		return "", errors.New("Invalid URL format")
	}

	if !domainAllowed(targetURL.Hostname(), allowedDomains) {
		return "", fmt.Errorf("Security Error: Access to domain '%s' is not allowed.\nAllowed domains: %s", targetURL.Hostname(), strings.Join(allowedDomains, ", "))
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 && response.StatusCode < 400 {
		return "", fmt.Errorf("HTTP Error: %d %s", response.StatusCode, response.Status)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP Error: %d %s", response.StatusCode, response.Status)
	}

	limited := io.LimitReader(response.Body, maxWebFetchSize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(body) > maxWebFetchSize {
		return "", fmt.Errorf("Response size exceeds the maximum allowed limit of %d bytes", maxWebFetchSize)
	}
	return string(body), nil
}

func domainAllowed(hostname string, allowedDomains []string) bool {
	for _, domain := range allowedDomains {
		if hostname == domain || strings.HasSuffix(hostname, "."+domain) {
			return true
		}
	}
	return false
}
