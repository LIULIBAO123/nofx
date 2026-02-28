package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Claude 缓存说明（Prompt 缓存读取/创建一直为 0 时）：
// 1. 本端已正确发送 cache_control: { type: "ephemeral" }，且使用 Anthropic /v1/messages 格式。
// 2. 若通过 Claude 中转站：中转必须将 cache_control 原样转发给 Anthropic，否则 Anthropic 不会启用缓存，返回的 cache_read/cache_creation 为 0。
// 3. 中转计费若走 default 分组而非 Default-cache 分组，可能不统计缓存，响应里也可能不包含缓存相关字段。
// 4. 建议在中转站确认：请求体含 cache_control；转发到 anthropic /v1/messages；响应 usage 含 cache_read_input_tokens、cache_creation_input_tokens。

const (
	ProviderClaude       = "claude"
	DefaultClaudeBaseURL = "https://api.anthropic.com/v1"
	DefaultClaudeModel   = "claude-opus-4-6"
)

type ClaudeClient struct {
	*Client
}

// NewClaudeClient creates Claude client (backward compatible)
func NewClaudeClient() AIClient {
	return NewClaudeClientWithOptions()
}

// NewClaudeClientWithOptions creates Claude client (supports options pattern)
func NewClaudeClientWithOptions(opts ...ClientOption) AIClient {
	// 1. Create Claude preset options
	claudeOpts := []ClientOption{
		WithProvider(ProviderClaude),
		WithModel(DefaultClaudeModel),
		WithBaseURL(DefaultClaudeBaseURL),
	}

	// 2. Merge user options (user options have higher priority)
	allOpts := append(claudeOpts, opts...)

	// 3. Create base client
	baseClient := NewClient(allOpts...).(*Client)

	// 4. Create Claude client
	claudeClient := &ClaudeClient{
		Client: baseClient,
	}

	// 5. Set hooks to point to ClaudeClient (implement dynamic dispatch)
	baseClient.hooks = claudeClient

	return claudeClient
}

func (c *ClaudeClient) SetAPIKey(apiKey string, customURL string, customModel string) {
	c.APIKey = apiKey

	if len(apiKey) > 8 {
		c.logger.Infof("🔧 [MCP] Claude API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
	if customURL != "" {
		c.BaseURL = customURL
		c.logger.Infof("🔧 [MCP] Claude using custom BaseURL: %s", customURL)
	} else {
		c.logger.Infof("🔧 [MCP] Claude using default BaseURL: %s", c.BaseURL)
	}
	if customModel != "" {
		c.Model = customModel
		c.logger.Infof("🔧 [MCP] Claude using custom Model: %s", customModel)
	} else {
		c.logger.Infof("🔧 [MCP] Claude using default Model: %s", c.Model)
	}
}

// setAuthHeader Claude uses x-api-key header instead of Authorization Bearer
func (c *ClaudeClient) setAuthHeader(reqHeaders http.Header) {
	reqHeaders.Set("x-api-key", c.APIKey)
	reqHeaders.Set("anthropic-version", "2023-06-01")
}

// buildUrl Claude uses /messages endpoint
func (c *ClaudeClient) buildUrl() string {
	return fmt.Sprintf("%s/messages", c.BaseURL)
}

// buildMCPRequestBody Claude request format with cache_control so API returns cache_read/cache_creation in usage.
// Without cache_control, Anthropic does not include cache stats and frontend shows "读取 0 · 创建 0".
func (c *ClaudeClient) buildMCPRequestBody(systemPrompt, userPrompt string) map[string]any {
	requestBody := map[string]any{
		"model":         c.Model,
		"max_tokens":    c.MaxTokens,
		"system":        systemPrompt,
		"messages":      []map[string]string{{"role": "user", "content": userPrompt}},
		"cache_control": map[string]string{"type": "ephemeral"}, // required for usage.cache_read_input_tokens / cache_creation_input_tokens
	}
	return requestBody
}

// buildRequestBodyFromRequest builds Claude API body with top-level system + cache_control for prompt caching.
// Anthropic returns cache_read_input_tokens / cache_creation_input_tokens only when this format is used.
func (c *ClaudeClient) buildRequestBodyFromRequest(req *Request) map[string]any {
	var systemContent, userContent string
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			systemContent = msg.Content
		case "user":
			userContent = msg.Content
		}
	}
	model := req.Model
	if model == "" {
		model = c.Model
	}
	maxTok := c.MaxTokens
	if req.MaxTokens != nil {
		maxTok = *req.MaxTokens
	}
	body := map[string]any{
		"model":          model,
		"max_tokens":     maxTok,
		"system":         systemContent,
		"messages":       []map[string]string{{"role": "user", "content": userContent}},
		"cache_control":  map[string]string{"type": "ephemeral"}, // required for Claude to return cache_read/cache_creation in usage
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	} else if c.config != nil {
		body["temperature"] = c.config.Temperature
	}
	return body
}

// parseMCPResponse Claude has different response format
func (c *ClaudeClient) parseMCPResponse(body []byte) (string, *TokenUsage, error) {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens               int `json:"input_tokens"`
			OutputTokens              int `json:"output_tokens"`
			CacheReadInputTokens      int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens  int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", nil, fmt.Errorf("failed to parse Claude response: %w, body: %s", err, string(body))
	}

	if response.Error != nil {
		return "", nil, fmt.Errorf("Claude API error: %s - %s", response.Error.Type, response.Error.Message)
	}

	if len(response.Content) == 0 {
		return "", nil, fmt.Errorf("Claude returned empty content, body: %s", string(body))
	}

	inputTok := response.Usage.InputTokens
	outputTok := response.Usage.OutputTokens
	totalTokens := inputTok + outputTok
	// 兼容中转站：若标准字段为 0，尝试从 usage 原始对象读取（部分中转用 prompt_tokens/completion_tokens 或嵌套 usage）
	if totalTokens == 0 {
		var raw map[string]any
		if json.Unmarshal(body, &raw) == nil {
			if u, _ := raw["usage"].(map[string]any); u != nil {
				if v, _ := u["input_tokens"].(float64); v > 0 {
					inputTok = int(v)
				}
				if v, _ := u["prompt_tokens"].(float64); v > 0 && inputTok == 0 {
					inputTok = int(v)
				}
				if v, _ := u["output_tokens"].(float64); v > 0 {
					outputTok = int(v)
				}
				if v, _ := u["completion_tokens"].(float64); v > 0 && outputTok == 0 {
					outputTok = int(v)
				}
				totalTokens = inputTok + outputTok
			}
		}
	}
	var usage *TokenUsage
	if totalTokens > 0 {
		cacheRead := response.Usage.CacheReadInputTokens
		cacheCreate := response.Usage.CacheCreationInputTokens
		if cacheRead == 0 && cacheCreate == 0 {
			var raw map[string]any
			if json.Unmarshal(body, &raw) == nil {
				if u, _ := raw["usage"].(map[string]any); u != nil {
					if v, _ := u["cache_read_input_tokens"].(float64); v > 0 {
						cacheRead = int(v)
					}
					if v, _ := u["cache_creation_input_tokens"].(float64); v > 0 {
						cacheCreate = int(v)
					}
				}
			}
		}
		usage = &TokenUsage{
			Provider:                 c.Provider,
			Model:                    c.Model,
			PromptTokens:             inputTok,
			CompletionTokens:         outputTok,
			TotalTokens:              totalTokens,
			CacheReadInputTokens:     cacheRead,
			CacheCreationInputTokens: cacheCreate,
		}
	}

	for _, content := range response.Content {
		if content.Type == "text" {
			return content.Text, usage, nil
		}
	}

	return "", nil, fmt.Errorf("no text content in Claude response")
}
