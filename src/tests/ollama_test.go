package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

// OllamaRequest represents the request to the Ollama API
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaResponse represents the response from the Ollama API
type OllamaResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	CreatedAt string `json:"created_at,omitempty"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
}

func TestOllamaAPI(t *testing.T) {
	// Configuration
	ollamaEndpoint := "http://localhost:11434/api/generate"
	model := "llama3.2:3b"
	prompt := "What is the capital of France?"

	// Create request
	requestData := OllamaRequest{
		Model:  model,
		Prompt: prompt,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Set timeout for request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Send request to Ollama API
	resp, err := client.Post(ollamaEndpoint, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatalf("Failed to send request to Ollama API: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ollama API returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// For Ollama API, the response is actually a stream of JSON objects
	// Let's split by newlines and check the last complete object to verify completion
	lines := bytes.Split(body, []byte("\n"))

	var lastResponse OllamaResponse
	var foundComplete bool

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var response OllamaResponse
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("Failed to unmarshal response line: %v", err)
		}

		lastResponse = response
		if response.Done {
			foundComplete = true
		}
	}

	// Verify we got a complete response
	if !foundComplete {
		t.Fatalf("Did not receive a complete response from Ollama API")
	}

	// Verify the model is correct
	if lastResponse.Model != model {
		t.Errorf("Expected model %s, got %s", model, lastResponse.Model)
	}

	// Verify we got some response text (simple check)
	if lastResponse.Response == "" {
		t.Error("Received empty response text")
	}

	t.Logf("Ollama API test passed. Response received from %s model.", model)
	t.Logf("Prompt: %s", prompt)
	t.Logf("Response excerpt: %s", lastResponse.Response[:min(50, len(lastResponse.Response))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
