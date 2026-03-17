package coinglass

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// ETFFlowPoint ETF 资金流单点
type ETFFlowPoint struct {
	Timestamp int64   `json:"timestamp"`
	Flow      float64 `json:"flow"`      // 净流入/流出（美元）
	Inflow    float64 `json:"inflow,omitempty"`
	Outflow   float64 `json:"outflow,omitempty"`
	NetAssets float64 `json:"net_assets,omitempty"`
}

// ETFFlowHistoryResponse flow-history 响应
type ETFFlowHistoryResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data []ETFFlowPoint  `json:"data"`
}

// GetETFBitcoinFlowHistory 获取 Bitcoin ETF 资金流历史；limit 条数。
func (c *Client) GetETFBitcoinFlowHistory(limit int) (*ETFFlowHistoryResponse, error) {
	path := "/api/etf/bitcoin/flow-history"
	if limit > 0 {
		path += "?" + url.Values{"limit": []string{fmt.Sprintf("%d", limit)}}.Encode()
	}
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out ETFFlowHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass etf bitcoin flow-history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass etf bitcoin flow-history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}

// GetETFEthereumFlowHistory 获取 Ethereum ETF 资金流历史。
func (c *Client) GetETFEthereumFlowHistory(limit int) (*ETFFlowHistoryResponse, error) {
	path := "/api/etf/ethereum/flow-history"
	if limit > 0 {
		path += "?" + url.Values{"limit": []string{fmt.Sprintf("%d", limit)}}.Encode()
	}
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out ETFFlowHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass etf ethereum flow-history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass etf ethereum flow-history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}
