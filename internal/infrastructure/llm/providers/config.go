package providers

import "net/http"

type Config struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}
