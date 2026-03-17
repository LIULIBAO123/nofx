package coinglass

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// bitcoinDominanceDataPoint V4 返回 data 为数组时的单元素
type bitcoinDominanceDataPoint struct {
	Timestamp         int64   `json:"timestamp"`
	Price             float64 `json:"price"`
	BitcoinDominance  float64 `json:"bitcoin_dominance"`
	MarketCap         float64 `json:"market_cap"`
}

// FearGreedPoint 恐惧贪婪指数单点
type FearGreedPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     int     `json:"value"`     // 0-100
	ValueCN   string  `json:"value_cn,omitempty"`
	Price     float64 `json:"price,omitempty"`
}

// FearGreedHistoryResponse fear-greed-history 响应（data 可能为数组或对象内嵌数组）
type FearGreedHistoryResponse struct {
	Code string            `json:"code"`
	Msg  string            `json:"msg"`
	Data []FearGreedPoint  `json:"data"`
}

// GetFearGreedHistory 获取恐惧贪婪指数历史；limit 条数。兼容 data 为数组或对象（如 { "list": [...] }）。
func (c *Client) GetFearGreedHistory(limit int) (*FearGreedHistoryResponse, error) {
	path := "/api/index/fear-greed-history"
	if limit > 0 {
		path += "?" + url.Values{"limit": []string{fmt.Sprintf("%d", limit)}}.Encode()
	}
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("coinglass fear-greed-history parse: %w", err)
	}
	if raw.Code != "0" && raw.Msg != "success" {
		return nil, fmt.Errorf("coinglass fear-greed-history: code=%s msg=%s", raw.Code, raw.Msg)
	}
	var list []FearGreedPoint
	if len(raw.Data) > 0 && raw.Data[0] == '[' {
		if err := json.Unmarshal(raw.Data, &list); err != nil {
			return nil, fmt.Errorf("coinglass fear-greed-history data array: %w", err)
		}
	} else {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw.Data, &obj); err != nil {
			return nil, fmt.Errorf("coinglass fear-greed-history data object: %w", err)
		}
		for _, key := range []string{"data", "list", "history"} {
			if b, ok := obj[key]; ok && len(b) > 0 && b[0] == '[' {
				if err := json.Unmarshal(b, &list); err == nil {
					break
				}
			}
		}
	}
	return &FearGreedHistoryResponse{Code: raw.Code, Msg: raw.Msg, Data: list}, nil
}

// BitcoinDominanceResponse bitcoin-dominance 响应；V4 的 data 为数组，取最新一条的 bitcoin_dominance
type BitcoinDominanceResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// GetBitcoinDominance 获取 BTC 市值占比（百分比，如 54.5）。兼容 data 为 number 或 array。
func (c *Client) GetBitcoinDominance() (float64, error) {
	body, err := c.DoRequest("/api/index/bitcoin-dominance")
	if err != nil {
		return 0, err
	}
	var out struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("coinglass bitcoin-dominance parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return 0, fmt.Errorf("coinglass bitcoin-dominance: code=%s msg=%s", out.Code, out.Msg)
	}
	// data 可能为 number 或 array
	var pct float64
	if len(out.Data) > 0 && out.Data[0] == '[' {
		var arr []bitcoinDominanceDataPoint
		if err := json.Unmarshal(out.Data, &arr); err != nil || len(arr) == 0 {
			return 0, fmt.Errorf("coinglass bitcoin-dominance parse array: %w", err)
		}
		pct = arr[len(arr)-1].BitcoinDominance
	} else {
		if err := json.Unmarshal(out.Data, &pct); err != nil {
			return 0, fmt.Errorf("coinglass bitcoin-dominance parse number: %w", err)
		}
	}
	return pct, nil
}

// AltcoinSeasonPoint 山寨季指数单点
type AltcoinSeasonPoint struct {
	Timestamp       int64   `json:"timestamp"`
	AltcoinIndex    int     `json:"altcoin_index"`    // 0-100
	AltcoinMarketcap float64 `json:"altcoin_marketcap,omitempty"`
}

// AltcoinSeasonResponse altcoin-season 响应（端点可能为 altcoin-season 或 altcoin-season-index，按文档）
type AltcoinSeasonResponse struct {
	Code string               `json:"code"`
	Msg  string               `json:"msg"`
	Data []AltcoinSeasonPoint `json:"data"`
}

// GetAltcoinSeasonHistory 获取山寨季指数历史；limit 条数。
func (c *Client) GetAltcoinSeasonHistory(limit int) (*AltcoinSeasonResponse, error) {
	path := "/api/index/altcoin-season"
	if limit > 0 {
		path += "?" + url.Values{"limit": []string{fmt.Sprintf("%d", limit)}}.Encode()
	}
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out AltcoinSeasonResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass altcoin-season parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass altcoin-season: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}
