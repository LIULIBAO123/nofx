package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	ProviderDeepSeek       = "deepseek"
	DefaultDeepSeekBaseURL = "https://api.deepseek.com"
	DefaultDeepSeekModel   = "deepseek-chat"
)

type DeepSeekClient struct {
	*Client
}

// NewDeepSeekClient creates DeepSeek client (backward compatible)
//
// Deprecated: Recommend using NewDeepSeekClientWithOptions for better flexibility
func NewDeepSeekClient() AIClient {
	return NewDeepSeekClientWithOptions()
}

// NewDeepSeekClientWithOptions creates DeepSeek client (supports options pattern)
//
// Usage examples:
//   // Basic usage
//   client := mcp.NewDeepSeekClientWithOptions()
//
//   // Custom configuration
//   client := mcp.NewDeepSeekClientWithOptions(
//       mcp.WithAPIKey("sk-xxx"),
//       mcp.WithLogger(customLogger),
//       mcp.WithTimeout(60*time.Second),
//   )
func NewDeepSeekClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create DeepSeek preset options
	deepseekOpts := []ClientOption{
		WithProvider(ProviderDeepSeek),
		WithModel(DefaultDeepSeekModel),
		WithBaseURL(DefaultDeepSeekBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(deepseekOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create DeepSeek client
	dsClient := &DeepSeekClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to DeepSeekClient (implement dynamic dispatch)
	baseClient.hooks = dsClient

	return dsClient
}

func (dsClient *DeepSeekClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	dsClient.APIKey = apiKey

	if len(apiKey) > 8 {
		dsClient.logger.Infof("🔧 [MCP] DeepSeek API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		dsClient.BaseURL = customURL
		dsClient.logger.Infof("🔧 [MCP] DeepSeek using custom BaseURL: %s", customURL)
	} else {
		dsClient.logger.Infof("🔧 [MCP] DeepSeek using default BaseURL: %s", dsClient.BaseURL)
	}
	if customModel != "" {
		dsClient.Model = customModel
		dsClient.logger.Infof("🔧 [MCP] DeepSeek using custom Model: %s", customModel)
	} else {
		dsClient.logger.Infof("🔧 [MCP] DeepSeek using default Model: %s", dsClient.Model)
	}
}

func (dsClient *DeepSeekClient) setAuthHeader(reqHeaders http.Header) {
	dsClient.Client.setAuthHeader(reqHeaders)
}

// parseMCPResponse parses DeepSeek response and returns usage including prompt_cache_hit_tokens / prompt_cache_miss_tokens.
func (dsClient *DeepSeekClient) parseMCPResponse(body []byte) (string, *TokenUsage, error) {
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens           int `json:"prompt_tokens"`
			CompletionTokens       int `json:"completion_tokens"`
			TotalTokens            int `json:"total_tokens"`
			PromptCacheHitTokens   int `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens  int `json:"prompt_cache_miss_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", nil, fmt.Errorf("API returned empty response")
	}

	var usage *TokenUsage
	if result.Usage.TotalTokens > 0 {
		usage = &TokenUsage{
			Provider:                 dsClient.Provider,
			Model:                    dsClient.Model,
			PromptTokens:            result.Usage.PromptTokens,
			CompletionTokens:        result.Usage.CompletionTokens,
			TotalTokens:             result.Usage.TotalTokens,
			CacheReadInputTokens:    result.Usage.PromptCacheHitTokens,
			CacheCreationInputTokens: result.Usage.PromptCacheMissTokens,
		}
	}
	return result.Choices[0].Message.Content, usage, nil
}
