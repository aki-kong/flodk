package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aki-kong/flodk/llm"
)

// OllamaClient handles requests to Ollama LLM
type OllamaClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// OllamaChatRequest represents the Ollama chat completion request format
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Think    bool            `json:"think"`
	Options  OllamaOptions   `json:"options"`
}

// OllamaMessage represents a chat message for Ollama
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaOptions represents optional parameters for Ollama
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// OllamaChatResponse represents the Ollama chat completion response
type OllamaChatResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            OllamaMessage `json:"message"`
	Done               bool          `json:"done"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

// GenerateContent sends a chat completion request to Ollama
func (c *OllamaClient) GenerateContent(
	ctx context.Context,
	req llm.ChatRequest,
) (*llm.ChatResponse, error) {
	// Convert llm.ChatRequest to OllamaChatRequest
	ollamaReq := OllamaChatRequest{
		Model:    req.Model,
		Stream:   req.Stream,
		Messages: make([]OllamaMessage, len(req.Messages)),
		Options: OllamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	for i, msg := range req.Messages {
		ollamaReq.Messages[i] = OllamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Marshal the request to JSON
	jsonData, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Unmarshal response
	var ollamaResp OllamaChatResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert OllamaChatResponse to llm.ChatResponse
	chatResp := &llm.ChatResponse{
		Model:   ollamaResp.Model,
		Created: parseCreatedAt(ollamaResp.CreatedAt),
		Choices: []llm.Choice{
			{
				Index: 0,
				Message: llm.Message{
					Role:    ollamaResp.Message.Role,
					Content: ollamaResp.Message.Content,
				},
				FinishReason: "stop",
			},
		},
		Usage: llm.Usage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}

	return chatResp, nil
}

// parseCreatedAt converts Ollama's created_at string to Unix timestamp
func parseCreatedAt(createdAt string) int64 {
	// Ollama returns RFC3339 format: 2024-01-15T10:30:45.123456789Z
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		// Return current timestamp if parsing fails
		return time.Now().Unix()
	}
	return t.Unix()
}
