package services

import (
	"YourPlace/src/core"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const OllamaModel = "phi-2"
const OllamaPort = "11434"

func OllamaSetup() bool {
	err := OllamaHealthCheck()
	if err != nil {
		core.LogError("Ollama health check failed: " + err.Error())
		return false
	}
	isDownloaded, err := OllamaIsModelDownloaded(OllamaModel)
	if isDownloaded {
		return true
	}
	err = OllamaDownloadModel(OllamaModel)
	if err != nil {
		core.LogError("Failed to download model: " + err.Error())
		return false
	}
	return true
}
func OllamaHealthCheck() error {
	resp, err := http.Get("http://localhost:" + OllamaPort + "/api/ps")
	if err != nil {
		return core.LogErrorReturn("Failed to check ollama health: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return core.LogErrorReturn("Ollama health check failed: " + resp.Status)
	}
	return nil
}
func OllamaDownloadModel(modelName string) error {
	core.LogDebug("Downloading Ollama model: " + modelName)
	url := "http://localhost:" + OllamaPort + "/api/pull"
	requestBody := map[string]string{
		"name": modelName,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return core.LogErrorReturn("Ollama model download, failed to marshal json: " + err.Error())
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return core.LogErrorReturn("Ollama model download, failed to create request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return core.LogErrorReturn("Ollama model download, failed to download model: " + err.Error())
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var progressResponse struct {
			Status string `json:"status"`
			Digest string `json:"digest,omitempty"`
			Total  int64  `json:"total,omitempty"`
			Done   int64  `json:"completed,omitempty"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &progressResponse); err != nil {
			continue // skip malformed json
		}
		if progressResponse.Total > 0 {
			progress := float64(progressResponse.Done) / float64(progressResponse.Total) * 100
			core.LogDebug(fmt.Sprintf("Downloading %s: %.2f%%\n", modelName, progress))
		} else {
			core.LogDebug("Ollama Download Progress: " + progressResponse.Status)
		}
	}
	if err := scanner.Err(); err != nil {
		return core.LogErrorReturn("Ollama model download, failed to read response: " + err.Error())
	}
	return nil
}
func OllamaIsModelDownloaded(modelName string) (bool, error) {
	resp, err := http.Get("http://localhost:" + OllamaPort + "/api/tags")
	if err != nil {
		return false, core.LogErrorReturn("failed to connect to Ollama API: " + err.Error())
	}
	defer resp.Body.Close()
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, core.LogErrorReturn("failed to decode response: " + err.Error())
	}
	for _, model := range result.Models {
		if model.Name == modelName {
			return true, nil
		}
	}
	return false, nil
}
func OllamaPromptModel(modelName string, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := "http://localhost:" + OllamaPort + "/api/generate"
	requestBody := map[string]interface{}{
		"model":  modelName,
		"prompt": prompt,
		"stream": true,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", core.LogErrorReturn("Error marshaling Ollama request: " + err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", core.LogErrorReturn("Error creating Ollama request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", core.LogErrorReturn("Error sending Ollama request: " + err.Error())
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	responseString := ""
	for scanner.Scan() {
		var response struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &response); err != nil {
			continue // Skip malformed JSON
		}
		responseString += response.Response
		if response.Done {
			break
		}
	}
	if err = scanner.Err(); err != nil {
		return "", core.LogErrorReturn("Error reading Ollama response: " + err.Error())
	}
	return responseString, nil
}
