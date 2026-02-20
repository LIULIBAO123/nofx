# AI 交易分析功能设计文档

## 功能概述

在回测实验室中，每次交易执行后，自动调用 AI 模型分析该交易，提供：
- 交易决策评估（好/中/差）
- 盈亏原因分析
- 策略改进建议
- 风险提示

## 技术架构

### 1. 后端实现

#### 1.1 数据库扩展

在 `backtest_trades` 表中添加 AI 分析字段：

```sql
ALTER TABLE backtest_trades ADD COLUMN ai_analysis TEXT DEFAULT '';
ALTER TABLE backtest_trades ADD COLUMN ai_analysis_ts BIGINT DEFAULT 0;
ALTER TABLE backtest_trades ADD COLUMN analysis_rating VARCHAR(20) DEFAULT '';
```

#### 1.2 AI 分析结构

```go
// TradeAnalysis AI 交易分析结果
type TradeAnalysis struct {
    TradeID         int64     `json:"trade_id"`
    Rating          string    `json:"rating"`           // "excellent", "good", "fair", "poor"
    Summary         string    `json:"summary"`          // 简短总结
    ProfitAnalysis  string    `json:"profit_analysis"`  // 盈亏原因分析
    Improvements    []string  `json:"improvements"`     // 改进建议列表
    RiskWarnings    []string  `json:"risk_warnings"`    // 风险警告
    AnalyzedAt      time.Time `json:"analyzed_at"`
}
```

#### 1.3 分析触发时机

在 `AppendTradeEvent` 后自动触发：

```go
// 在 backtest/runner.go 中
func (r *Runner) recordTrade(event TradeEvent) error {
    // 保存交易记录
    if err := r.store.AppendTradeEvent(r.runID, event); err != nil {
        return err
    }
    
    // 异步触发 AI 分析（如果启用）
    if r.config.EnableTradeAnalysis {
        go r.analyzeTradeAsync(event)
    }
    
    return nil
}
```

#### 1.4 AI 分析提示词模板

```go
const tradeAnalysisPrompt = `你是一位专业的量化交易分析师。请分析以下交易并提供改进建议。

## 交易信息
- 交易对: {{.Symbol}}
- 操作: {{.Action}} ({{.Side}})
- 价格: {{.Price}}
- 数量: {{.Quantity}}
- 已实现盈亏: {{.RealizedPnL}} ({{.PnLPct}}%)
- 时间: {{.Timestamp}}
- 持仓后: {{.PositionAfter}}

## 市场环境
- 当前周期: {{.Cycle}}
- 账户权益: {{.Equity}}
- 最大回撤: {{.MaxDrawdown}}%

## 策略上下文
{{.StrategyContext}}

请按以下格式输出分析结果（JSON格式）：
{
  "rating": "excellent|good|fair|poor",
  "summary": "一句话总结这次交易",
  "profit_analysis": "详细分析盈亏原因",
  "improvements": ["改进建议1", "改进建议2", "改进建议3"],
  "risk_warnings": ["风险警告1", "风险警告2"]
}

要求：
1. 客观评估交易质量
2. 分析盈亏的根本原因（技术面、基本面、时机等）
3. 提供可操作的改进建议
4. 识别潜在风险
5. 保持专业和建设性
`
```

### 2. API 接口

#### 2.1 获取交易分析

```
GET /api/backtest/trade-analysis?run_id=xxx&trade_id=xxx
```

响应：
```json
{
  "trade_id": 123,
  "rating": "good",
  "summary": "在支撑位附近买入，时机较好",
  "profit_analysis": "该交易在价格触及关键支撑位后反弹时买入，符合策略逻辑。盈利3.5%主要得益于准确的入场时机和良好的风险回报比。",
  "improvements": [
    "可以考虑在更低的位置分批建仓，降低平均成本",
    "止损位置可以设置得更紧一些，提高风险回报比",
    "建议增加成交量确认，避免假突破"
  ],
  "risk_warnings": [
    "当前持仓占比较高，注意分散风险",
    "市场波动加剧，建议降低杠杆"
  ],
  "analyzed_at": "2024-02-20T10:30:00Z"
}
```

#### 2.2 批量获取分析

```
GET /api/backtest/trades?run_id=xxx&include_analysis=true
```

#### 2.3 重新分析交易

```
POST /api/backtest/reanalyze-trade
{
  "run_id": "bt_20240220_100000",
  "trade_id": 123
}
```

### 3. 前端实现

#### 3.1 交易列表增强

在交易列表中显示 AI 分析评级：

```tsx
<div className="trade-item">
  <div className="trade-info">
    {/* 现有交易信息 */}
  </div>
  
  {/* AI 分析评级徽章 */}
  {trade.ai_analysis && (
    <div className={`rating-badge rating-${trade.analysis_rating}`}>
      <Brain className="w-4 h-4" />
      <span>{getRatingText(trade.analysis_rating)}</span>
    </div>
  )}
</div>
```

#### 3.2 交易详情弹窗

点击交易显示详细分析：

```tsx
<Modal>
  <div className="trade-analysis-modal">
    <h3>AI 交易分析</h3>
    
    {/* 评级 */}
    <div className="rating-section">
      <RatingStars rating={analysis.rating} />
      <p>{analysis.summary}</p>
    </div>
    
    {/* 盈亏分析 */}
    <div className="analysis-section">
      <h4>盈亏分析</h4>
      <p>{analysis.profit_analysis}</p>
    </div>
    
    {/* 改进建议 */}
    <div className="improvements-section">
      <h4>改进建议</h4>
      <ul>
        {analysis.improvements.map((item, i) => (
          <li key={i}>
            <CheckCircle2 className="w-4 h-4" />
            {item}
          </li>
        ))}
      </ul>
    </div>
    
    {/* 风险警告 */}
    {analysis.risk_warnings.length > 0 && (
      <div className="warnings-section">
        <h4>风险警告</h4>
        <ul>
          {analysis.risk_warnings.map((item, i) => (
            <li key={i}>
              <AlertTriangle className="w-4 h-4" />
              {item}
            </li>
          ))}
        </ul>
      </div>
    )}
  </div>
</Modal>
```

#### 3.3 分析统计面板

在回测概览中显示 AI 分析统计：

```tsx
<div className="analysis-stats">
  <h4>AI 分析统计</h4>
  <div className="stats-grid">
    <StatCard
      icon={CheckCircle2}
      label="优秀交易"
      value={excellentCount}
      color="#0ECB81"
    />
    <StatCard
      icon={Activity}
      label="良好交易"
      value={goodCount}
      color="#F0B90B"
    />
    <StatCard
      icon={AlertTriangle}
      label="需改进"
      value={poorCount}
      color="#F6465D"
    />
  </div>
</div>
```

### 4. 配置选项

在回测配置中添加 AI 分析开关：

```tsx
<div className="config-section">
  <label>
    <input
      type="checkbox"
      checked={config.enableTradeAnalysis}
      onChange={(e) => setConfig({
        ...config,
        enableTradeAnalysis: e.target.checked
      })}
    />
    启用 AI 交易分析
  </label>
  
  {config.enableTradeAnalysis && (
    <div className="analysis-options">
      <label>
        分析模式:
        <select
          value={config.analysisMode}
          onChange={(e) => setConfig({
            ...config,
            analysisMode: e.target.value
          })}
        >
          <option value="realtime">实时分析（每笔交易）</option>
          <option value="batch">批量分析（回测结束后）</option>
          <option value="selective">选择性分析（仅重要交易）</option>
        </select>
      </label>
      
      <label>
        分析详细程度:
        <select
          value={config.analysisDetail}
          onChange={(e) => setConfig({
            ...config,
            analysisDetail: e.target.value
          })}
        >
          <option value="brief">简要（快速）</option>
          <option value="standard">标准</option>
          <option value="detailed">详细（深度分析）</option>
        </select>
      </label>
    </div>
  )}
</div>
```

## 实现优先级

### Phase 1: 核心功能（MVP）
- [ ] 数据库表结构扩展
- [ ] AI 分析核心逻辑
- [ ] 基础 API 接口
- [ ] 前端交易列表显示评级

### Phase 2: 增强功能
- [ ] 交易详情弹窗
- [ ] 分析统计面板
- [ ] 批量分析功能
- [ ] 重新分析功能

### Phase 3: 高级功能
- [ ] 分析历史对比
- [ ] 策略优化建议汇总
- [ ] AI 学习和改进
- [ ] 导出分析报告

## 性能考虑

1. **异步分析**: 交易记录后异步调用 AI，不阻塞回测进程
2. **批量模式**: 回测结束后批量分析，减少 API 调用
3. **缓存机制**: 相同交易场景的分析结果可复用
4. **限流控制**: 控制 AI API 调用频率，避免超限

## 成本优化

1. **选择性分析**: 只分析重要交易（大额、亏损、关键点位）
2. **使用便宜模型**: 默认使用 DeepSeek 等性价比高的模型
3. **本地缓存**: 缓存常见场景的分析结果
4. **用户配额**: 限制每日/每月分析次数

## 安全考虑

1. **数据脱敏**: 发送给 AI 的数据不包含敏感信息
2. **权限控制**: 只有回测所有者可以查看分析
3. **API 密钥保护**: AI API 密钥安全存储
4. **速率限制**: 防止滥用

## 用户体验

1. **渐进式加载**: 分析结果异步加载，不阻塞界面
2. **加载状态**: 显示分析进度和状态
3. **错误处理**: 分析失败时友好提示
4. **可选功能**: 用户可以选择是否启用

## 未来扩展

1. **多模型对比**: 使用多个 AI 模型分析同一交易，对比结果
2. **AI 辩论**: 多个 AI 角色（多头/空头/分析师）辩论交易决策
3. **策略进化**: 基于 AI 分析自动优化策略参数
4. **实时建议**: 实盘交易时实时提供 AI 建议

