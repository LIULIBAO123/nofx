# AI 交易分析功能 - 快速使用指南

## 功能简介

在回测实验室中，每次交易后可以调用 AI 模型分析该交易，提供：
- ✅ 交易质量评级（优秀/良好/一般/较差）
- ✅ 盈亏原因深度分析
- ✅ 3条可操作的改进建议
- ✅ 风险警告提示

## 快速开始

### 1. API 调用示例

#### 分析单笔交易

```bash
curl -X POST http://localhost:8080/api/backtest/analyze-trade \
  -H "Content-Type: application/json" \
  -d '{
    "run_id": "bt_20240220_100000",
    "trade_id": 123
  }'
```

响应示例：
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

#### 获取已有分析

```bash
curl "http://localhost:8080/api/backtest/trade-analysis?run_id=bt_20240220_100000&trade_id=123"
```

### 2. 前端集成示例

```typescript
import { analyzeTrade, getRatingColor, getRatingText } from '@/lib/tradeAnalysis'

// 分析交易
const analysis = await analyzeTrade('bt_20240220_100000', 123)

// 显示评级
const ratingColor = getRatingColor(analysis.rating) // '#0ECB81' for excellent
const ratingText = getRatingText(analysis.rating, 'zh') // '优秀'

// 渲染分析结果
<div className="trade-analysis">
  <div className="rating" style={{ color: ratingColor }}>
    {ratingText}
  </div>
  <p className="summary">{analysis.summary}</p>
  <div className="improvements">
    {analysis.improvements.map((item, i) => (
      <li key={i}>{item}</li>
    ))}
  </div>
</div>
```

## 评级标准

| 评级 | 标准 | 颜色 |
|------|------|------|
| **优秀 (excellent)** | 盈利>5% 且时机完美，符合策略逻辑 | 🟢 绿色 |
| **良好 (good)** | 盈利1-5% 或时机较好 | 🟡 黄色 |
| **一般 (fair)** | 盈亏在±1% 之间或有小问题 | ⚪ 灰色 |
| **较差 (poor)** | 亏损>1% 或存在明显错误 | 🔴 红色 |

## 使用场景

### 场景 1：回测后批量分析

回测完成后，分析所有重要交易：

```typescript
// 获取所有交易
const trades = await api.get(`/backtest/trades?run_id=${runId}`)

// 筛选重要交易（大额、亏损、关键点位）
const importantTrades = trades.filter(trade => 
  Math.abs(trade.realized_pnl) > 100 || // 盈亏超过100 USDT
  trade.liquidation // 爆仓
)

// 批量分析
for (const trade of importantTrades) {
  const analysis = await analyzeTrade(runId, trade.id)
  console.log(`${trade.symbol}: ${analysis.rating} - ${analysis.summary}`)
}
```

### 场景 2：实时分析（回测进行中）

在回测进行时，实时分析每笔交易：

```typescript
// 监听交易事件
socket.on('trade', async (trade) => {
  // 异步分析，不阻塞回测
  analyzeTrade(runId, trade.id).then(analysis => {
    // 显示分析结果
    showTradeAnalysis(trade, analysis)
  })
})
```

### 场景 3：策略优化

基于 AI 分析改进策略：

```typescript
// 获取所有分析
const trades = await api.get(`/backtest/trades?run_id=${runId}`)

// 统计各评级数量
const stats = {
  excellent: 0,
  good: 0,
  fair: 0,
  poor: 0
}

trades.forEach(trade => {
  if (trade.ai_analysis) {
    stats[trade.ai_analysis.rating]++
  }
})

// 汇总改进建议
const allImprovements = trades
  .filter(t => t.ai_analysis)
  .flatMap(t => t.ai_analysis.improvements)

// 找出最常见的建议
const improvementCounts = {}
allImprovements.forEach(imp => {
  improvementCounts[imp] = (improvementCounts[imp] || 0) + 1
})

console.log('最常见的改进建议:', 
  Object.entries(improvementCounts)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)
)
```

## 配置选项

### 后端配置

在回测配置中添加：

```go
config := backtest.BacktestConfig{
    // ... 其他配置
    EnableTradeAnalysis: true,  // 启用交易分析
    AnalysisMode: "selective",  // realtime | batch | selective
    AnalysisDetail: "standard", // brief | standard | detailed
}
```

### 前端配置

```typescript
const config = {
  enableTradeAnalysis: true,
  analysisMode: 'selective', // 只分析重要交易
  analysisDetail: 'standard'
}
```

## 性能优化

### 1. 异步分析

交易记录后异步调用 AI，不阻塞回测：

```go
// 在 backtest/runner.go 中
func (r *Runner) recordTrade(event TradeEvent) error {
    // 保存交易记录
    if err := r.store.AppendTradeEvent(r.runID, event); err != nil {
        return err
    }
    
    // 异步触发 AI 分析
    if r.config.EnableTradeAnalysis {
        go r.analyzeTradeAsync(event)
    }
    
    return nil
}
```

### 2. 选择性分析

只分析重要交易，节省 API 调用：

```go
func (a *TradeAnalyzer) ShouldAnalyzeTrade(trade TradeEvent) bool {
    // 只分析：
    // 1. 大额交易（>1000 USDT）
    // 2. 大盈亏（>3%）
    // 3. 爆仓
    
    if trade.LiquidationFlag {
        return true
    }
    
    if trade.OrderValue > 1000 {
        return true
    }
    
    if trade.OrderValue > 0 {
        pnlPct := (trade.RealizedPnL / trade.OrderValue) * 100
        if math.Abs(pnlPct) > 3 {
            return true
        }
    }
    
    return false
}
```

### 3. 批量模式

回测结束后批量分析，减少 API 调用频率：

```typescript
// 回测完成后
const trades = await api.get(`/backtest/trades?run_id=${runId}`)

// 批量分析（控制并发）
const batchSize = 5
for (let i = 0; i < trades.length; i += batchSize) {
  const batch = trades.slice(i, i + batchSize)
  await Promise.all(
    batch.map(trade => analyzeTrade(runId, trade.id))
  )
  // 延迟避免超限
  await new Promise(resolve => setTimeout(resolve, 1000))
}
```

## 成本估算

使用 DeepSeek 模型（推荐）：

- 每次分析约 500 tokens
- 成本：约 $0.0007 / 次
- 100 笔交易分析成本：约 $0.07

使用 GPT-4：

- 每次分析约 500 tokens
- 成本：约 $0.015 / 次
- 100 笔交易分析成本：约 $1.50

**建议**：
- 使用 DeepSeek 进行日常分析（性价比高）
- 使用 GPT-4 进行重要交易的深度分析
- 启用选择性分析模式，只分析重要交易

## 故障排除

### 问题 1：AI 分析失败

**原因**：API 密钥无效或模型不可用

**解决**：
```bash
# 检查 AI 模型配置
curl http://localhost:8080/api/ai-models

# 测试 AI 连接
curl -X POST http://localhost:8080/api/test-ai \
  -H "Content-Type: application/json" \
  -d '{"model_id": "your_model_id"}'
```

### 问题 2：分析结果为空

**原因**：交易尚未分析或分析失败

**解决**：
```typescript
// 先尝试获取已有分析
let analysis = await getTradeAnalysis(runId, tradeId)

// 如果没有，触发新分析
if (!analysis) {
  analysis = await analyzeTrade(runId, tradeId)
}
```

### 问题 3：分析速度慢

**原因**：同步分析阻塞了回测

**解决**：
- 使用异步分析模式
- 或使用批量模式（回测结束后分析）

## 下一步

- [ ] 查看完整设计文档：[AI_TRADE_ANALYSIS_FEATURE.md](./AI_TRADE_ANALYSIS_FEATURE.md)
- [ ] 集成到前端界面
- [ ] 添加分析统计面板
- [ ] 实现策略优化建议汇总

## 反馈

如有问题或建议，请提交 Issue 或 PR。

