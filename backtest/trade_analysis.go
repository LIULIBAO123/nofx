package backtest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nofx/kernel"
	"nofx/logger"
)

// TradeAnalysisRequest contains all context needed for AI to analyze a trade
type TradeAnalysisRequest struct {
	Trade           TradeEvent                       `json:"trade"`
	AccountBefore   kernel.AccountInfo               `json:"account_before"`
	AccountAfter    kernel.AccountInfo               `json:"account_after"`
	MarketData      map[string]interface{}           `json:"market_data,omitempty"`
	PositionHistory []PositionSnapshot               `json:"position_history,omitempty"`
	RecentTrades    []TradeEvent                     `json:"recent_trades,omitempty"`
	DecisionContext string                           `json:"decision_context,omitempty"`
}

// AnalyzeTrade uses AI to analyze a completed trade and provide insights
func (r *Runner) AnalyzeTrade(trade TradeEvent, accountBefore, accountAfter kernel.AccountInfo, marketData map[string]interface{}, decisionContext string) *TradeAnalysis {
	// Skip analysis for hold/wait actions
	if trade.Action == "hold" || trade.Action == "wait" {
		return nil
	}

	// Skip analysis if AI client is not available
	if r.mcpClient == nil {
		return &TradeAnalysis{
			AnalysisError: "AI client not available",
			GeneratedAt:   time.Now().Unix(),
		}
	}

	// Build analysis request
	req := TradeAnalysisRequest{
		Trade:           trade,
		AccountBefore:   accountBefore,
		AccountAfter:    accountAfter,
		MarketData:      marketData,
		DecisionContext: decisionContext,
	}

	// Get recent trades for context (last 5 trades)
	recentTrades, _ := r.getRecentTrades(5)
	req.RecentTrades = recentTrades

	// Generate analysis prompt
	prompt := r.buildTradeAnalysisPrompt(req)

	// Call AI (no timeout needed as CallWithMessages handles it internally)
	response, err := r.mcpClient.CallWithMessages("You are a professional trading analyst.", prompt)
	if err != nil {
		logger.Infof("⚠️ Trade analysis failed: %v", err)
		return &TradeAnalysis{
			AnalysisError: fmt.Sprintf("AI call failed: %v", err),
			GeneratedAt:   time.Now().Unix(),
		}
	}

	// Parse AI response
	analysis := r.parseTradeAnalysis(response, trade)
	analysis.GeneratedAt = time.Now().Unix()

	return analysis
}

// buildTradeAnalysisPrompt creates a detailed prompt for trade analysis
func (r *Runner) buildTradeAnalysisPrompt(req TradeAnalysisRequest) string {
	var sb strings.Builder

	sb.WriteString("# 交易分析任务\n\n")
	sb.WriteString("请对以下交易进行全面分析，提供专业的交易复盘和改进建议。\n\n")

	// Trade details
	sb.WriteString("## 交易详情\n")
	sb.WriteString(fmt.Sprintf("- **交易动作**: %s\n", req.Trade.Action))
	sb.WriteString(fmt.Sprintf("- **交易品种**: %s\n", req.Trade.Symbol))
	sb.WriteString(fmt.Sprintf("- **交易方向**: %s\n", req.Trade.Side))
	sb.WriteString(fmt.Sprintf("- **交易数量**: %.4f\n", req.Trade.Quantity))
	sb.WriteString(fmt.Sprintf("- **成交价格**: %.4f\n", req.Trade.Price))
	sb.WriteString(fmt.Sprintf("- **杠杆倍数**: %dx\n", req.Trade.Leverage))
	sb.WriteString(fmt.Sprintf("- **订单价值**: $%.2f\n", req.Trade.OrderValue))
	sb.WriteString(fmt.Sprintf("- **手续费**: $%.2f\n", req.Trade.Fee))
	sb.WriteString(fmt.Sprintf("- **滑点**: %.4f\n", req.Trade.Slippage))

	if strings.Contains(req.Trade.Action, "close") {
		sb.WriteString(fmt.Sprintf("- **实现盈亏**: $%.2f (%.2f%%)\n", 
			req.Trade.RealizedPnL, 
			(req.Trade.RealizedPnL/req.Trade.OrderValue)*100))
	}

	sb.WriteString("\n")

	// Account status
	sb.WriteString("## 账户状态\n")
	sb.WriteString("### 交易前\n")
	sb.WriteString(fmt.Sprintf("- 总权益: $%.2f\n", req.AccountBefore.TotalEquity))
	sb.WriteString(fmt.Sprintf("- 可用余额: $%.2f\n", req.AccountBefore.AvailableBalance))
	sb.WriteString(fmt.Sprintf("- 保证金使用率: %.2f%%\n", req.AccountBefore.MarginUsedPct))
	sb.WriteString(fmt.Sprintf("- 持仓数量: %d\n", req.AccountBefore.PositionCount))

	sb.WriteString("\n### 交易后\n")
	sb.WriteString(fmt.Sprintf("- 总权益: $%.2f (变化: $%.2f)\n", 
		req.AccountAfter.TotalEquity, 
		req.AccountAfter.TotalEquity-req.AccountBefore.TotalEquity))
	sb.WriteString(fmt.Sprintf("- 可用余额: $%.2f\n", req.AccountAfter.AvailableBalance))
	sb.WriteString(fmt.Sprintf("- 保证金使用率: %.2f%%\n", req.AccountAfter.MarginUsedPct))
	sb.WriteString(fmt.Sprintf("- 持仓数量: %d\n", req.AccountAfter.PositionCount))

	sb.WriteString("\n")

	// Market data context
	if len(req.MarketData) > 0 {
		sb.WriteString("## 市场数据\n")
		if mdJSON, err := json.MarshalIndent(req.MarketData, "", "  "); err == nil {
			sb.WriteString(fmt.Sprintf("```json\n%s\n```\n\n", string(mdJSON)))
		}
	}

	// Recent trades context
	if len(req.RecentTrades) > 0 {
		sb.WriteString("## 近期交易历史\n")
		for i, t := range req.RecentTrades {
			pnlStr := ""
			if strings.Contains(t.Action, "close") {
				pnlStr = fmt.Sprintf(" | PnL: $%.2f", t.RealizedPnL)
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s @ $%.4f%s\n", 
				i+1, t.Action, t.Symbol, t.Price, pnlStr))
		}
		sb.WriteString("\n")
	}

	// Decision context
	if req.DecisionContext != "" {
		sb.WriteString("## 决策上下文\n")
		sb.WriteString(req.DecisionContext)
		sb.WriteString("\n\n")
	}

	// Analysis requirements
	sb.WriteString("## 分析要求\n\n")
	sb.WriteString("请按照以下结构提供JSON格式的分析结果：\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	
	if strings.Contains(req.Trade.Action, "open") {
		sb.WriteString("  \"entry_quality\": \"入场时机质量评估（优秀/良好/一般/较差），包括技术面、基本面、市场情绪等因素\",\n")
	}
	
	if strings.Contains(req.Trade.Action, "close") {
		sb.WriteString("  \"exit_quality\": \"出场时机质量评估（优秀/良好/一般/较差），是否达到预期目标或及时止损\",\n")
	}
	
	sb.WriteString("  \"risk_management\": \"风险管理评估，包括仓位大小、杠杆使用、止损止盈设置是否合理\",\n")
	sb.WriteString("  \"market_condition\": \"市场环境匹配度，当前市场状态是否适合该交易策略\",\n")
	sb.WriteString("  \"profit_loss_reason\": \"盈亏原因分析，解释为什么盈利或亏损\",\n")
	sb.WriteString("  \"improvement\": \"改进建议，针对性的优化方向和具体措施\",\n")
	sb.WriteString("  \"overall_score\": 8.5  // 综合评分 0-10分\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	
	sb.WriteString("**注意**: 请直接返回JSON对象，不要包含其他文字说明。\n")

	return sb.String()
}

// parseTradeAnalysis extracts structured analysis from AI response
func (r *Runner) parseTradeAnalysis(response string, trade TradeEvent) *TradeAnalysis {
	analysis := &TradeAnalysis{}

	// Try to parse as JSON first
	response = strings.TrimSpace(response)
	
	// Remove markdown code blocks if present
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimPrefix(response, "```")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	// Try JSON parsing
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(response), &jsonData); err == nil {
		// Successfully parsed JSON
		if v, ok := jsonData["entry_quality"].(string); ok {
			analysis.EntryQuality = v
		}
		if v, ok := jsonData["exit_quality"].(string); ok {
			analysis.ExitQuality = v
		}
		if v, ok := jsonData["risk_management"].(string); ok {
			analysis.RiskManagement = v
		}
		if v, ok := jsonData["market_condition"].(string); ok {
			analysis.MarketCondition = v
		}
		if v, ok := jsonData["profit_loss_reason"].(string); ok {
			analysis.ProfitLossReason = v
		}
		if v, ok := jsonData["improvement"].(string); ok {
			analysis.Improvement = v
		}
		if v, ok := jsonData["overall_score"].(float64); ok {
			analysis.OverallScore = v
		}
	} else {
		// Fallback: parse as plain text
		analysis.EntryQuality = extractSection(response, "entry_quality", "入场")
		analysis.ExitQuality = extractSection(response, "exit_quality", "出场")
		analysis.RiskManagement = extractSection(response, "risk_management", "风险")
		analysis.MarketCondition = extractSection(response, "market_condition", "市场")
		analysis.ProfitLossReason = extractSection(response, "profit_loss_reason", "盈亏")
		analysis.Improvement = extractSection(response, "improvement", "改进")
		
		// Try to extract score
		if score := extractScore(response); score > 0 {
			analysis.OverallScore = score
		} else {
			// Default score based on PnL
			if trade.RealizedPnL > 0 {
				analysis.OverallScore = 7.0
			} else if trade.RealizedPnL < 0 {
				analysis.OverallScore = 4.0
			} else {
				analysis.OverallScore = 5.0
			}
		}
	}

	return analysis
}

// extractSection extracts a section from text response
func extractSection(text, jsonKey, keyword string) string {
	lines := strings.Split(text, "\n")
	var result []string
	capturing := false

	for _, line := range lines {
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, jsonKey) || strings.Contains(line, keyword) {
			capturing = true
			// Extract content after colon if present
			if idx := strings.Index(line, ":"); idx >= 0 {
				content := strings.TrimSpace(line[idx+1:])
				if content != "" && content != "\"" {
					result = append(result, content)
				}
			}
			continue
		}
		if capturing {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "**") {
				break
			}
			result = append(result, trimmed)
		}
	}

	return strings.Join(result, " ")
}

// extractScore tries to extract numerical score from text
func extractScore(text string) float64 {
	var score float64
	// Try to find patterns like "8.5分", "score: 8.5", "overall_score: 8.5"
	// Simple pattern matching (avoiding regex for simplicity)
	
	if strings.Contains(text, "overall_score") {
		parts := strings.Split(text, "overall_score")
		if len(parts) > 1 {
			// Extract number after colon
			afterColon := strings.Split(parts[1], ":")
			if len(afterColon) > 1 {
				numStr := strings.TrimSpace(afterColon[1])
				numStr = strings.Split(numStr, ",")[0]
				numStr = strings.Split(numStr, "\n")[0]
				numStr = strings.TrimSpace(numStr)
				fmt.Sscanf(numStr, "%f", &score)
				if score >= 0 && score <= 10 {
					return score
				}
			}
		}
	}
	
	return 0
}

// getRecentTrades retrieves recent trade events for context
func (r *Runner) getRecentTrades(limit int) ([]TradeEvent, error) {
	trades, err := LoadTradeEvents(r.cfg.RunID)
	if err != nil {
		return nil, err
	}

	// Return last N trades
	if len(trades) <= limit {
		return trades, nil
	}

	return trades[len(trades)-limit:], nil
}





