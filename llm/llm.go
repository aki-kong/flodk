package llm

import "context"

// Client interface defines a common API for calling LLM Endpoints.
type Client interface {
	// GenerateContent generates content for the passed chat request.
	GenerateContent(
		ctx context.Context,
		req ChatRequest,
	) (*ChatResponse, error)
}
