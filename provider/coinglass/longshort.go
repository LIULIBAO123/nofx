package coinglass

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// LongShortHistoryPoint 多空比历史单点（兼容 Coinglass V4：time, global_account_long_percent, global_account_short_percent）
type LongShortHistoryPoint struct {
	Timestamp    int64   `json:"time"`                          // V4 为 time(ms)
	LongAccount  float64 `json:"global_account_long_percent"`   // V4 字段；兼容 long_account
	ShortAccount float64 `json:"global_account_short_percent"`  // V4 字段；兼容 short_account
	LongShortRatio float64 `json:"global_account_long_short_ratio,omitempty"` // V4
}

// GlobalLongShortHistoryResponse global-long-short-account-ratio/history 响应
type GlobalLongShortHistoryResponse struct {
	Code string                    `json:"code"`
	Msg  string                    `json:"msg"`
	Data []LongShortHistoryPoint   `json:"data"`
}

// symbolToPair 将币种或交易对转为 Coinglass 要求的交易对（如 BTC -> BTCUSDT，1000PEPE -> 1000PEPEUSDT）；已以 USDT 结尾则不变。
func symbolToPair(symbol string) string {
	s := strings.TrimSpace(strings.ToUpper(symbol))
	if s == "" {
		return symbol
	}
	if strings.HasSuffix(s, "USDT") {
		return s
	}
	return s + "USDT"
}

// GetGlobalLongShortAccountRatioHistory 全账户多空比历史。exchange 如 Binance，symbol 如 BTC、1000PEPE 或 BTCUSDT（自动补 USDT），interval 如 1h，limit 条数。
// Coinglass V4 要求必填 exchange 与交易对 symbol（如 BTCUSDT），否则返回 pair does not exist。
func (c *Client) GetGlobalLongShortAccountRatioHistory(symbol, interval string, limit int) (*GlobalLongShortHistoryResponse, error) {
	exchange := "Binance"
	pair := symbolToPair(symbol)
	q := url.Values{"exchange": []string{exchange}, "symbol": []string{pair}}
	if interval != "" {
		q.Set("interval", interval)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/futures/global-long-short-account-ratio/history?" + q.Encode()
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out GlobalLongShortHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass global-long-short history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass global-long-short history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}

// TopLongShortHistoryResponse top-long-short-account-ratio/history 响应
type TopLongShortHistoryResponse struct {
	Code string                    `json:"code"`
	Msg  string                    `json:"msg"`
	Data []LongShortHistoryPoint   `json:"data"`
}

// GetTopLongShortAccountRatioHistory 大户多空比历史。V4 要求 exchange + 交易对 symbol，使用 symbolToPair 统一转成交易对。
func (c *Client) GetTopLongShortAccountRatioHistory(symbol, interval string, limit int) (*TopLongShortHistoryResponse, error) {
	exchange := "Binance"
	pair := symbolToPair(symbol)
	q := url.Values{"exchange": []string{exchange}, "symbol": []string{pair}}
	if interval != "" {
		q.Set("interval", interval)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/futures/top-long-short-account-ratio/history?" + q.Encode()
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out TopLongShortHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass top-long-short history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass top-long-short history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}
