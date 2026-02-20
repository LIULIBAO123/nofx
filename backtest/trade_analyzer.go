package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nofx/logger"
	"nofx/mcp"
	"nofx/store"
)

// TradeAnalyzer handles AI-powered trade analysis
type TradeAnalyzer struct {
	aiClient mcp.AIClient
	config   BacktestConfig
}

// NewTradeAnalyzer creates a new trade analyzer
func NewTradeAnalyzer(aiClient mcp.AIClient, config BacktestConfig) *TradeAnalyzer {
	return &TradeAnalyzer{
		aiClient: aiClient,
		config:   config,
	}
}

// AnalyzeTradeContext contains context for trade analysis
type AnalyzeTradeContext struct {
	Trade          store.TradeEvent
	Equity         float64
	MaxDrawdown    float64
	TotalTrades    int
	WinRate        float64
	StrategyConfig string
}

// AnalyzeTrade performs AI analysis on a trade
func (a *TradeAnalyzer) AnalyzeTrade(ctx context.Context, tradeCtx AnalyzeTradeContext) (*store.TradeAnalysis, error) {
	prompt := a.buildAnalysisPrompt(tradeCtx)
	
	// Call AI model
	response, err := a.aiClient.Chat(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}
	
	// Parse response
	analysis, err := a.parseAnalysisResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}
	
	analysis.AnalyzedAt = time.Now()
	return analysis, nil
}

// buildAnalysisPrompt builds the prompt for trade analysis
func (a *TradeAnalyzer) buildAnalysisPrompt(ctx AnalyzeTradeContext) string {
	trade := ctx.Trade
	
	// Calculate PnL percentage
	pnlPct := 0.0
	if trade.OrderValue > 0 {
		pnlPct = (trade.RealizedPnL / trade.OrderValue) * 100
	}
	
	// Determine action description
	actionDesc := trade.Action
	if trade.Side != "" {
		actionDesc = fmt.Sprintf("%s %s", trade.Action, trade.Side)
	}
	
	// Format timestamp
	tradeTime := time.Unix(trade.Timestamp/1000, 0).Format("2006-01-02 15:04:05")
	
	prompt := fmt.Sprintf(`你是一位专业的量化交易分析师。请分析以下交易并提供改进建议。

## 交易信息
- 交易对: %s
- 操作: %s
- 价格: %.8f
- 数量: %.8f
- 订单价值: %.2f USDT
- 已实现盈亏: %.2f USDT (%.2f%%)
- 手续费: %.4f USDT
- 滑点: %.4f USDT
- 杠杆: %dx
- 时间: %s
- 持仓后: %.8f

## 账户状态
- 当前周期: %d
- 账户权益: %.2f USDT
- 最大回撤: %.2f%%
- 总交易次数: %d
- 胜率: %.2f%%

## 策略上下文
%s

请按以下JSON格式输出分析结果：
{
  "rating": "excellent|good|fair|poor",
  "summary": "一句话总结这次交易（20字以内）",
  "profit_analysis": "详细分析盈亏原因（100-200字）",
  "improvements": ["改进建议1", "改进建议2", "改进建议3"],
  "risk_warnings": ["风险警告1", "风险警告2"]
}

评级标准：
- excellent: 盈利>5%% 且时机完美，符合策略逻辑
- good: 盈利1-5%% 或时机较好
- fair: 盈亏在±1%% 之间或有小问题
- poor: 亏损>1%% 或存在明显错误

要求：
1. 客观评估交易质量，不要过度乐观或悲观
2. 分析盈亏的根本原因（入场时机、出场时机、市场环境等）
3. 提供3条可操作的改进建议
4. 识别1-2个潜在风险（如果有）
5. 保持专业和建设性，避免空话套话
6. 必须返回有效的JSON格式

只返回JSON，不要有其他文字。`,
		trade.Symbol,
		actionDesc,
		trade.Price,
		trade.Quantity,
		trade.OrderValue,
		trade.RealizedPnL,
		pnlPct,
		trade.Fee,
		trade.Slippage,
		trade.Leverage,
		tradeTime,
		trade.PositionAfter,
		trade.Cycle,
		ctx.Equity,
		ctx.MaxDrawdown,
		ctx.TotalTrades,
		ctx.WinRate,
		ctx.StrategyConfig,
	)
	
	return prompt
}

// parseAnalysisResponse parses AI response into TradeAnalysis
func (a *TradeAnalyzer) parseAnalysisResponse(response string) (*store.TradeAnalysis, error) {
	// Clean response - extract JSON if wrapped in markdown
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}
	
	var result struct {
		Rating         string   `json:"rating"`
		Summary        string   `json:"summary"`
		ProfitAnalysis string   `json:"profit_analysis"`
		Improvements   []string `json:"improvements"`
		RiskWarnings   []string `json:"risk_warnings"`
	}
	
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		logger.Warnf("Failed to parse AI analysis response: %v\nResponse: %s", err, response)
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}
	
	// Validate rating
	validRatings := map[string]bool{
		"excellent": true,
		"good":      true,
		"fair":      true,
		"poor":      true,
	}
	if !validRatings[result.Rating] {
		result.Rating = "fair" // default
	}
	
	// Ensure arrays are not nil
	if result.Improvements == nil {
		result.Improvements = []string{}
	}
	if result.RiskWarnings == nil {
		result.RiskWarnings = []string{}
	}
	
	analysis := &store.TradeAnalysis{
		Rating:         result.Rating,
		Summary:        result.Summary,
		ProfitAnalysis: result.ProfitAnalysis,
		Improvements:   result.Improvements,
		RiskWarnings:   result.RiskWarnings,
	}
	
	return analysis, nil
}

// AnalyzeTradeAsync analyzes a trade asynchronously
func (a *TradeAnalyzer) AnalyzeTradeAsync(ctx context.Context, tradeCtx AnalyzeTradeContext, callback func(*store.TradeAnalysis, error)) {
	go func() {
		analysis, err := a.AnalyzeTrade(ctx, tradeCtx)
		if callback != nil {
			callback(analysis, err)
		}
	}()
}

// ShouldAnalyzeTrade determines if a trade should be analyzed based on config
func (a *TradeAnalyzer) ShouldAnalyzeTrade(trade store.TradeEvent, mode string) bool {
	switch mode {
	case "realtime":
		return true
	case "selective":
		// Only analyze significant trades
		return a.isSignificantTrade(trade)
	case "batch":
		return false // Will be analyzed in batch after backtest
	default:
		return false
	}
}

// isSignificantTrade checks if a trade is significant enough to analyze
func (a *TradeAnalyzer) isSignificantTrade(trade store.TradeEvent) bool {
	// Analyze if:
	// 1. Large position (>10% of typical order value)
	// 2. Large profit/loss (>3%)
	// 3. Liquidation
	// 4. First/last trade of a cycle
	
	if trade.LiquidationFlag {
		return true
	}
	
	if trade.OrderValue > 0 {
		pnlPct := (trade.RealizedPnL / trade.OrderValue) * 100
		if pnlPct > 3 || pnlPct < -3 {
			return true
		}
	}
	
	// Could add more criteria based on config
	return false
}

