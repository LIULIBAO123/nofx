package coinglass

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// FundingExchangeItem 单交易所资金费率（exchange-list 中一项）
type FundingExchangeItem struct {
	Exchange             string  `json:"exchange"`
	FundingRate          float64 `json:"funding_rate"`
	FundingRateInterval  int     `json:"funding_rate_interval"`
	NextFundingTime      int64   `json:"next_funding_time"`
}

// FundingSymbolItem exchange-list 按币种返回
type FundingSymbolItem struct {
	Symbol               string                 `json:"symbol"`
	StablecoinMarginList []FundingExchangeItem  `json:"stablecoin_margin_list"`
	TokenMarginList      []FundingExchangeItem  `json:"token_margin_list"`
}

// FundingExchangeListResponse API 响应
type FundingExchangeListResponse struct {
	Code string              `json:"code"`
	Msg  string              `json:"msg"`
	Data []FundingSymbolItem `json:"data"`
}

// GetFundingRateExchangeList 获取多所资金费率列表；symbol 如 BTC、ETH，空则返回全部。
func (c *Client) GetFundingRateExchangeList(symbol string) (*FundingExchangeListResponse, error) {
	path := "/api/futures/funding-rate/exchange-list"
	if symbol != "" {
		path += "?" + url.Values{"symbol": []string{symbol}}.Encode()
	}
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out FundingExchangeListResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass funding exchange-list parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass funding exchange-list: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}

// OIWeightHistoryPoint OI 加权资金费率历史单点
type OIWeightHistoryPoint struct {
	Timestamp     int64   `json:"timestamp"`
	FundingRate   float64 `json:"funding_rate"`
	OpenInterest  float64 `json:"open_interest,omitempty"`
}

// FundingRateOIWeightHistoryResponse oi-weight-history 响应（结构按实际 API 调整）
type FundingRateOIWeightHistoryResponse struct {
	Code string                   `json:"code"`
	Msg  string                   `json:"msg"`
	Data []OIWeightHistoryPoint   `json:"data"`
}

// GetFundingRateOIWeightHistory 获取 OI 加权资金费率历史；symbol 如 BTC，interval 如 1h、8h，limit 条数。
func (c *Client) GetFundingRateOIWeightHistory(symbol, interval string, limit int) (*FundingRateOIWeightHistoryResponse, error) {
	q := url.Values{"symbol": []string{symbol}}
	if interval != "" {
		q.Set("interval", interval)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/futures/funding-rate/oi-weight-history?" + q.Encode()
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out FundingRateOIWeightHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass funding oi-weight-history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass funding oi-weight-history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}
