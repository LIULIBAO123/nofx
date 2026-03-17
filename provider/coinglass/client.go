// Package coinglass 通过 KeyStore 中转站访问 Coinglass API，获取 OI、资金费率、强平、多空比等市场数据。
// 请求经 KeyStore 网关转发，认证使用 X-Api-Key（KeyStore API Key），路径与参数与官方 Coinglass 一致。
// 支持按「每分钟请求数」限速，以及按机制错峰（系统周期/主周期/止盈止损调整）错开首次请求时间。
package coinglass

import (
	"io"
	"net/http"
	"nofx/logger"
	"nofx/security"
	"strings"
	"sync"
	"time"
)

const (
	// KeyStore 代理默认 Base URL（V4）
	DefaultKeyStoreBaseURL = "https://www.keystore.com.cn/api/v1/proxy/coinglass/v4"
	DefaultTimeout         = 30 * time.Second
	// DefaultRateLimitPerMin 中转站常见限制（每分钟请求数），0 表示不限速
	DefaultRateLimitPerMin = 10
)

// Client Coinglass API 客户端（经 KeyStore 代理）
type Client struct {
	BaseURL            string
	APIKey             string
	Timeout            time.Duration
	mu                 sync.RWMutex
	rateLimitPerMin    int           // 每分钟最多请求数，0=不限
	limiterMu          sync.Mutex   // 限速锁
	lastRequestTime    time.Time     // 上次请求时间，用于间隔
}

// NewClient 创建客户端。baseURL 为空时使用 DefaultKeyStoreBaseURL；apiKey 为 KeyStore 的 X-Api-Key。
// rateLimitPerMin 为每分钟最大请求数（如 10），≤0 表示不限速。
func NewClient(baseURL, apiKey string, rateLimitPerMin int) *Client {
	if baseURL == "" {
		baseURL = DefaultKeyStoreBaseURL
	}
	if rateLimitPerMin <= 0 {
		rateLimitPerMin = 0
	}
	return &Client{
		BaseURL:         strings.TrimSuffix(baseURL, "/"),
		APIKey:          apiKey,
		Timeout:         DefaultTimeout,
		rateLimitPerMin: rateLimitPerMin,
	}
}

// SetConfig 更新 Base URL 与 API Key
func (c *Client) SetConfig(baseURL, apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if baseURL != "" {
		c.BaseURL = strings.TrimSuffix(baseURL, "/")
	}
	if apiKey != "" {
		c.APIKey = apiKey
	}
}

// WaitFlowStagger 按机制错峰：在发起该机制的首批请求前等待一段时间，避免多机制同时打满限速。
// 系统周期=0，主周期=20s，止盈止损调整=40s；其他 flow 不等待。
func (c *Client) WaitFlowStagger(flow string) {
	var d time.Duration
	switch flow {
	case "系统周期":
		d = 0
	case "主周期":
		d = 20 * time.Second
	case "止盈止损调整":
		d = 40 * time.Second
	default:
		d = 0
	}
	if d > 0 {
		time.Sleep(d)
	}
}

// waitRateLimit 在限速条件下等待到可发起下一请求的时刻（每 6s 一发等价于 10/min）
func (c *Client) waitRateLimit() {
	if c.rateLimitPerMin <= 0 {
		return
	}
	interval := 60 * time.Second / time.Duration(c.rateLimitPerMin)
	c.limiterMu.Lock()
	elapsed := time.Since(c.lastRequestTime)
	if elapsed < interval {
		wait := interval - elapsed
		c.limiterMu.Unlock()
		time.Sleep(wait)
		c.limiterMu.Lock()
	}
	c.lastRequestTime = time.Now()
	c.limiterMu.Unlock()
}

// DoRequest 发起 GET 请求，path 为完整路径（如 /api/futures/open-interest/exchange-list?symbol=BTC）。
// 请求头自动添加 X-Api-Key，由 KeyStore 网关消费并注入上游 Coinglass 凭证。
// 若设置了 rateLimitPerMin，会在上次请求后间隔 60/rateLimitPerMin 再发本次请求。
func (c *Client) DoRequest(path string) ([]byte, error) {
	c.waitRateLimit()

	c.mu.RLock()
	baseURL := c.BaseURL
	apiKey := c.APIKey
	timeout := c.Timeout
	c.mu.RUnlock()

	rawURL := baseURL + path
	if err := security.ValidateURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", apiKey)
	client := security.SafeHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		logger.Warnf("Coinglass proxy %s: status %d body %s", path, resp.StatusCode, string(body))
		return body, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}
	return body, nil
}

// APIError 接口错误
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}
