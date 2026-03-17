package coinglass

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// LiquidationAggregatedPoint 强平聚合历史单点（多空强平金额）
type LiquidationAggregatedPoint struct {
	Timestamp  int64   `json:"timestamp"`
	LongVol    float64 `json:"long_vol"`   // 多头强平量/金额
	ShortVol   float64 `json:"short_vol"`  // 空头强平量/金额
	Volume     float64 `json:"volume,omitempty"`
	BuyVol     float64 `json:"buy_vol,omitempty"`
	SellVol    float64 `json:"sell_vol,omitempty"`
}

// LiquidationAggregatedHistoryResponse aggregated-history 响应
type LiquidationAggregatedHistoryResponse struct {
	Code string                        `json:"code"`
	Msg  string                        `json:"msg"`
	Data []LiquidationAggregatedPoint `json:"data"`
}

// GetLiquidationAggregatedHistory 强平聚合历史；symbol 如 BTC，interval 如 1h、4h，limit 条数。
// Coinglass 要求 exchange_list（如 Binance 或 ALL），缺省时报 Required String parameter 'exchange_list' is not present。
func (c *Client) GetLiquidationAggregatedHistory(symbol, interval string, limit int) (*LiquidationAggregatedHistoryResponse, error) {
	q := url.Values{"symbol": []string{symbol}, "exchange_list": []string{"Binance"}}
	if interval != "" {
		q.Set("interval", interval)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/futures/liquidation/aggregated-history?" + q.Encode()
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out LiquidationAggregatedHistoryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass liquidation aggregated-history parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass liquidation aggregated-history: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}

// LiquidationHeatmapResponse 强平热力图响应（aggregated-heatmap 或 heatmap/model2）；用于提取关键价位
type LiquidationHeatmapResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		YAxis    []float64 `json:"y_axis"`     // 价格轴
		YAxisAlt []float64 `json:"yAxis"`      // 兼容驼峰
		Liq      []struct {
			X int     `json:"x"` // 或索引
			Y int     `json:"y"`
			V float64 `json:"v"` // 强平量/金额
		} `json:"liq"`
		LiquidationLeverageData [][]interface{} `json:"liquidation_leverage_data,omitempty"`
	} `json:"data"`
}

// GetLiquidationAggregatedHeatmap 获取全所聚合强平热力图；symbol 如 BTC，range 如 24h、7d、30d。返回可解析的关键价位与量。
// Coinglass V4 已弃用旧路径 aggregated-heatmap（返回 404），改用官方文档的 aggregated-heatmap/model2，与 KeyStore V4 路由一致。
func (c *Client) GetLiquidationAggregatedHeatmap(symbol, rangeParam string) (*LiquidationHeatmapResponse, error) {
	q := url.Values{"symbol": []string{symbol}}
	if rangeParam != "" {
		q.Set("range", rangeParam)
	}
	path := "/api/futures/liquidation/aggregated-heatmap/model2?" + q.Encode()
	body, err := c.DoRequest(path)
	if err != nil {
		return nil, err
	}
	var out LiquidationHeatmapResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("coinglass liquidation aggregated-heatmap/model2 parse: %w", err)
	}
	if out.Code != "0" && out.Msg != "success" {
		return nil, fmt.Errorf("coinglass liquidation aggregated-heatmap/model2: code=%s msg=%s", out.Code, out.Msg)
	}
	return &out, nil
}

// LiquidationKeyLevels 从热力图数据提取的关键价位摘要（供 AI 与方向池使用）
type LiquidationKeyLevels struct {
	Symbol    string   `json:"symbol"`
	Range     string   `json:"range"`
	KeyPrices []string `json:"key_prices"` // 关键价位字符串，如 "92k long cluster", "94k short cluster"
	Summary   string   `json:"summary"`    // 一句摘要
}

// HeatmapToKeyLevelsSummary 从热力图响应生成一句关键价位摘要；若无数据返回空字符串。
func (r *LiquidationHeatmapResponse) HeatmapToKeyLevelsSummary(symbol, rangeParam string) string {
	if r == nil || r.Data == nil {
		return ""
	}
	d := r.Data
	ys := d.YAxis
	if len(ys) == 0 {
		ys = d.YAxisAlt
	}
	if len(ys) == 0 {
		return ""
	}
	// 取最低、最高及中间若干档位作为关键价位提示
	const maxLevels = 8
	var parts []string
	step := 1
	if len(ys) > maxLevels {
		step = len(ys) / maxLevels
	}
	for i := 0; i < len(ys) && len(parts) < maxLevels; i += step {
		p := ys[i]
		if p >= 1000 {
			parts = append(parts, fmt.Sprintf("%.0f", p))
		} else {
			parts = append(parts, fmt.Sprintf("%.2f", p))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("%s %s key levels (sample): %s", symbol, rangeParam, joinStrings(parts, ", "))
}

func joinStrings(a []string, sep string) string {
	if len(a) == 0 {
		return ""
	}
	s := a[0]
	for i := 1; i < len(a); i++ {
		s += sep + a[i]
	}
	return s
}
