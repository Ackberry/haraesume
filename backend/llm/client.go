package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"backend/config"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func RunLLM(systemPrompt, userPrompt string, maxTokens int) (string, error) {
	return RunLLMWithTemperature(systemPrompt, userPrompt, maxTokens, nil)
}

func RunLLMWithTemperature(systemPrompt, userPrompt string, maxTokens int, temperature *float64) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		return "", errors.New("missing API key: set OPENROUTER_API_KEY environment variable")
	}

	requestBody := ChatRequest{
		Model: config.OpenRouterModel,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to serialize LLM request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, config.OpenRouterBaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read LLM response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed ChatResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return "", errors.New("no response from model")
	}

	content, err := parseMessageContent(parsed.Choices[0].Message.Content)
	if err != nil {
		return "", err
	}

	return content, nil
}

func parseMessageContent(content any) (string, error) {
	switch value := content.(type) {
	case string:
		return value, nil
	case []any:
		var chunks []string
		for _, item := range value {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, ok := m["text"].(string)
			if ok && strings.TrimSpace(text) != "" {
				chunks = append(chunks, text)
			}
		}
		joined := strings.TrimSpace(strings.Join(chunks, "\n"))
		if joined == "" {
			return "", errors.New("no response from model")
		}
		return joined, nil
	default:
		return "", errors.New("no response from model")
	}
}
