package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// parseStreamResponse reads an SSE stream from the response body (stream=true),
// accumulates content from choices[0].delta.content (OpenAI/DeepSeek format),
// and returns the full content plus usage if present in the stream.
func parseStreamResponse(body io.Reader, provider, model string) (fullContent string, usage *TokenUsage, err error) {
	scanner := bufio.NewScanner(body)
	// Allow large tokens (e.g. long lines in SSE)
	const maxCapacity = 1024 * 1024
	buf := make([]byte, 0, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var lastUsage *TokenUsage
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Message *struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			if chunk.Choices[0].Delta.Content != "" {
				fullContent += chunk.Choices[0].Delta.Content
			}
			if chunk.Choices[0].Message != nil && chunk.Choices[0].Message.Content != "" {
				fullContent += chunk.Choices[0].Message.Content
			}
		}
		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			lastUsage = &TokenUsage{
				Provider:                provider,
				Model:                   model,
				PromptTokens:            chunk.Usage.PromptTokens,
				CompletionTokens:        chunk.Usage.CompletionTokens,
				TotalTokens:             chunk.Usage.TotalTokens,
				CacheReadInputTokens:    0,
				CacheCreationInputTokens: 0,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fullContent, lastUsage, fmt.Errorf("stream read: %w", err)
	}
	return fullContent, lastUsage, nil
}
