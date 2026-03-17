// Package binancedata 提供币安 U 本位合约公开行情与衍生数据（多空比、资金费率、Taker 买卖量），用于增强市场判断与 AI 辅助分析。
// 无需 API Key，仅调用 Binance fapi 公开接口。
package binancedata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	DefaultBaseURL = "https://fapi.binance.com"
	DefaultTimeout = 15 * time.Second
)

// Client 币安衍生数据客户端（仅读公开接口）
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

func (c *Client) get(path string, params url.Values) ([]byte, error) {
	u := c.BaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("binance api %s: %s %s", path, resp.Status, string(body))
	}
	return io.ReadAll(resp.Body)
}

// LongShortItem 多空比单条
type LongShortItem struct {
	Symbol        string  `json:"symbol"`
	LongShortRatio float64 `json:"longShortRatio,string"`
	LongAccount   float64 `json:"longAccount,string"`
	ShortAccount  float64 `json:"shortAccount,string"`
	Timestamp     int64   `json:"timestamp"`
}

// GetGlobalLongShortAccountRatio 全账户多空比，symbol 如 BTCUSDT，period 如 5m/15m/30m/1h/4h/1d，limit 默认 1 取最新
func (c *Client) GetGlobalLongShortAccountRatio(symbol, period string, limit int) ([]LongShortItem, error) {
	if limit <= 0 {
		limit = 1
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("period", period)
	params.Set("limit", strconv.Itoa(limit))
	body, err := c.get("/futures/data/globalLongShortAccountRatio", params)
	if err != nil {
		return nil, err
	}
	var out []LongShortItem
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTopLongShortAccountRatio 大户账户多空比
func (c *Client) GetTopLongShortAccountRatio(symbol, period string, limit int) ([]LongShortItem, error) {
	if limit <= 0 {
		limit = 1
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("period", period)
	params.Set("limit", strconv.Itoa(limit))
	body, err := c.get("/futures/data/topLongShortAccountRatio", params)
	if err != nil {
		return nil, err
	}
	var out []LongShortItem
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FundingRateItem 资金费率单条
type FundingRateItem struct {
	Symbol      string  `json:"symbol"`
	FundingRate float64 `json:"fundingRate,string"`
	FundingTime int64   `json:"fundingTime"`
}

// GetFundingRateHistory 资金费率历史，limit 最大 1000
func (c *Client) GetFundingRateHistory(symbol string, limit int) ([]FundingRateItem, error) {
	if limit <= 0 {
		limit = 24
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("limit", strconv.Itoa(limit))
	body, err := c.get("/fapi/v1/fundingRate", params)
	if err != nil {
		return nil, err
	}
	var out []FundingRateItem
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PremiumIndex 当前/下一档资金费率与标记价
type PremiumIndex struct {
	Symbol               string  `json:"symbol"`
	MarkPrice            float64 `json:"markPrice,string"`
	LastFundingRate      float64 `json:"lastFundingRate,string"`
	NextFundingTime      int64   `json:"nextFundingTime"`
	Time                 int64   `json:"time"`
	EstimatedSettlePrice float64 `json:"estimatedSettlePrice,string"`
}

// GetPremiumIndex 当前标记价与下一档资金费率
func (c *Client) GetPremiumIndex(symbol string) (*PremiumIndex, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	body, err := c.get("/fapi/v1/premiumIndex", params)
	if err != nil {
		return nil, err
	}
	var out PremiumIndex
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TakerBuySellItem Taker 买卖量单条
type TakerBuySellItem struct {
	Symbol           string  `json:"symbol"`
	BuySellRatio     float64 `json:"buySellRatio,string"`
	SellVol          float64 `json:"sellVol,string"`
	BuyVol           float64 `json:"buyVol,string"`
	Timestamp        int64   `json:"timestamp"`
}

// GetTakerLongShortRatio Taker 多空比（U 本位：takerlongshortRatio），period 5m/15m/30m/1h/2h/4h/1d
func (c *Client) GetTakerLongShortRatio(symbol, period string, limit int) ([]TakerBuySellItem, error) {
	if limit <= 0 {
		limit = 1
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("period", period)
	params.Set("limit", strconv.Itoa(limit))
	body, err := c.get("/futures/data/takerlongshortRatio", params)
	if err != nil {
		return nil, err
	}
	var out []TakerBuySellItem
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SpotClient 币安现货 API（用于 Basis 计算：永续价 - 现货价）
const SpotBaseURL = "https://api.binance.com"

// GetSpotPrice 获取现货最新价，symbol 如 BTCUSDT
func GetSpotPrice(symbol string) (float64, error) {
	client := &http.Client{Timeout: DefaultTimeout}
	u := SpotBaseURL + "/api/v3/ticker/price?symbol=" + url.QueryEscape(symbol)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("binance spot %s: %s", resp.Status, string(body))
	}
	var out struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(out.Price, 64)
}
