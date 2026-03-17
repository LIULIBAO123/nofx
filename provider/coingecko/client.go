// Package coingecko 提供 CoinGecko 公开 API，用于 BTC/ETH 市值占比等宏观数据（数据补强）。
package coingecko

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultBaseURL = "https://api.coingecko.com"
	DefaultTimeout = 15 * time.Second
)

// Client CoinGecko 客户端（公开接口，无需 API Key；免费额度内使用）
type Client struct {
	BaseURL string
	Client  *http.Client
}

// NewClient 创建客户端
func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		Client:  &http.Client{Timeout: DefaultTimeout},
	}
}

// GlobalResponse /api/v3/global 响应（仅取所需字段）
type GlobalResponse struct {
	Data struct {
		MarketCapPercentage map[string]float64 `json:"market_cap_percentage"` // btc, eth, ...
	} `json:"data"`
}

// GetBTCDominance 返回 BTC 市值占比，如 54.5 表示 54.5%；失败返回 0 与 error
func (c *Client) GetBTCDominance() (float64, error) {
	u := c.BaseURL + "/api/v3/global"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("coingecko global %s: %s", resp.Status, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var out GlobalResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, err
	}
	if v, ok := out.Data.MarketCapPercentage["btc"]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("btc dominance not found in response")
}
