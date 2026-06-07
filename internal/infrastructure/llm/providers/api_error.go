package providers

import "fmt"

type APIError struct {
	Status   int
	Provider string
	Code     string
	Message  string
	Raw      any
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("llm api error: %s responded with status %d", e.Provider, e.Status)
}
