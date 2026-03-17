package coinglass

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// WhaleIndexPoint 鲸鱼指数历史单点
type WhaleIndexPoint struct {
	Time            int64   `json:"time"`
	WhaleIndexValue float64 `json:"whale_index_value"`
}

// WhaleIndexHistoryResponse whale-index/history 响应
type WhaleIndexHistoryResponse struct {
	Code string             `json:"code"`
	Msg  string             `json:"msg"`
	Data []WhaleIndexPoint  `json:"data"`
}

// GetWhaleIndexHistory 获取鲸鱼指数历史；exchange 如 Binance，symbol 如 BTC 或 BTCUSDT，interval 如 1h，limit 条数。
// Coinglass V4 要求 symbol 为交易对（如 BTCUSDT），传 BTC 时易触发 500，与 KeyStore 转发规则一致：仅改 path/query，此处补全为 BTCUSDT。
func (c *Client) GetWhaleIndexHistory(exchange, symbol, interval string, limit int) (*WhaleIndexHistoryResponse, error) {
	if exchange == "" {
		exchange = "Binance"
	}
	pair := symbol
	if len(symbol) <= 4 && symbol != "" {
		pair = symbol + "USDT"
	}
	q := url.Values{"exchange": []string{exchange}, "symbol": []string{pair}}
	if interval != "" {
		q.Set("interval", interval)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/futures/whale-index/history?" + q.Encode()
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out WhaleIndexHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass whale-index history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass whale-index history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}

// CGDIIndexPoint CGDI 指数历史单点（API 可能返回 cgdi_index_value 或 cgdiIndexValue）
type CGDIIndexPoint struct {
	Time           int64   `json:"time"`
	CGDIIndexValue float64 `json:"cgdi_index_value"`
}

// CGDIIndexHistoryResponse cgdi-index/history 响应
type CGDIIndexHistoryResponse struct {
	Code string            `json:"code"`
	Msg  string            `json:"msg"`
	Data []CGDIIndexPoint  `json:"data"`
}

// GetCGDIIndexHistory 获取 CGDI（多空扩散）指数历史；symbol 如 BTC，limit 条数。
func (c *Client) GetCGDIIndexHistory(symbol string, limit int) (*CGDIIndexHistoryResponse, error) {
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/futures/cgdi-index/history"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out CGDIIndexHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass cgdi-index history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass cgdi-index history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}

// CDRIIndexPoint CDRI 指数历史单点（API 返回 cdri_index_value）
type CDRIIndexPoint struct {
	Time           int64   `json:"time"`
	CDRIIndexValue float64 `json:"cdri_index_value"`
}

// CDRIIndexHistoryResponse cdri-index/history 响应
type CDRIIndexHistoryResponse struct {
	Code string            `json:"code"`
	Msg  string            `json:"msg"`
	Data []CDRIIndexPoint  `json:"data"`
}

// GetCDRIIndexHistory 获取 CDRI（衍生品风险）指数历史；symbol 如 BTC，limit 条数。
func (c *Client) GetCDRIIndexHistory(symbol string, limit int) (*CDRIIndexHistoryResponse, error) {
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/futures/cdri-index/history"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out CDRIIndexHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass cdri-index history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass cdri-index history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}
