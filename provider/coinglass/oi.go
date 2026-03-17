package coinglass

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// OIExchangeItem 单交易所/汇总的 OI 数据（Coinglass API 返回）
type OIExchangeItem struct {
	Exchange                            string  `json:"exchange"`
	Symbol                              string  `json:"symbol"`
	OpenInterestUSD                     float64 `json:"open_interest_usd"`
	OpenInterestQuantity                float64 `json:"open_interest_quantity"`
	OpenInterestChangePercent5m         float64 `json:"open_interest_change_percent_5m"`
	OpenInterestChangePercent15m        float64 `json:"open_interest_change_percent_15m"`
	OpenInterestChangePercent1h         float64 `json:"open_interest_change_percent_1h"`
	OpenInterestChangePercent4h        float64 `json:"open_interest_change_percent_4h"`
	OpenInterestChangePercent24h        float64 `json:"open_interest_change_percent_24h"`
}

// OIExchangeListResponse API 响应
type OIExchangeListResponse struct {
	Code string            `json:"code"`
	Msg  string            `json:"msg"`
	Data []OIExchangeItem  `json:"data"`
}

// GetOpenInterestExchangeList 获取指定币种在各交易所的 OI 列表（经 KeyStore 代理）。
// symbol 如 BTC、ETH。
func (c *Client) GetOpenInterestExchangeList(symbol string) (*OIExchangeListResponse, error) {
	path := "/api/futures/open-interest/exchange-list?" + url.Values{"symbol": []string{symbol}}.Encode()
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out OIExchangeListResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass oi exchange-list parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass oi exchange-list: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}
