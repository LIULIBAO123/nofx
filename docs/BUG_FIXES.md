# NOFX Bug 修复记录

本文档记录系统中发现的 bug 及其修复方案，便于后续追踪和参考。

---

## BUG-001: 模拟盘余额不包含已实现盈亏

**发现日期**: 2026-02-25

**问题描述**:
模拟盘（Paper Trader）平仓盈利后，总余额没有变化。例如：初始仓位 30 USDT，盈利 1 USDT 后平仓，总余额仍显示 30 USDT。

**影响范围**: 仅模拟盘

**根本原因**:
两处代码存在问题：

1. **Paper Trader 重启后余额重置**
   - 位置: `trader/auto_trader.go` 第 443-447 行
   - 问题: 虽然有 `RestoreRealizedPnL` 调用，但日志不够详细，难以诊断恢复失败的原因

2. **API Fallback 逻辑错误**
   - 位置: `api/server.go` 第 2304-2333 行
   - 问题: 当 trader 停止时，API 返回的 `total_equity` 直接使用 `initialBalance`，没有加上已实现盈亏

**修复方案**:

### 修复 1: 增强 PnL 恢复日志 (`trader/auto_trader.go`)

```go
// 修复前:
if stats, err := at.store.Position().GetFullStats(at.id); err == nil && stats != nil && stats.TotalPnL != 0 {
    pt.RestoreRealizedPnL(stats.TotalPnL)
    logger.Infof("📊 [%s] Restored realized PnL: %.4f USDT", at.name, stats.TotalPnL)
}

// 修复后:
stats, err := at.store.Position().GetFullStats(at.id)
if err != nil {
    logger.Infof("⚠️ [%s] Failed to get stats for PnL restore: %v", at.name, err)
} else if stats == nil {
    logger.Infof("📊 [%s] No closed positions yet, balance starts at %.2f", at.name, at.initialBalance)
} else if stats.TotalPnL == 0 {
    logger.Infof("📊 [%s] Closed positions exist (%d) but total PnL is 0", at.name, stats.TotalTrades)
} else {
    pt.RestoreRealizedPnL(stats.TotalPnL)
    logger.Infof("📊 [%s] Restored realized PnL: %.4f USDT (from %d closed trades)", at.name, stats.TotalPnL, stats.TotalTrades)
}
```

### 修复 2: 修正 API Fallback 逻辑 (`api/server.go`)

```go
// 修复前:
if isSimulation {
    eq := traderRecord.InitialBalance  // ❌ 直接使用初始余额
    // ...
    c.JSON(http.StatusOK, gin.H{
        "total_equity": eq,
        // ...
    })
}

// 修复后:
if isSimulation {
    initialBalance := traderRecord.InitialBalance
    realizedPnL := 0.0
    if s.store != nil {
        if stats, err := s.store.Position().GetFullStats(traderID); err == nil && stats != nil {
            realizedPnL = stats.TotalPnL
        }
    }
    // ✅ Total equity = initial balance + realized PnL
    eq := initialBalance + realizedPnL
    totalPnL := realizedPnL
    totalPnLPct := 0.0
    if initialBalance > 0 {
        totalPnLPct = (totalPnL / initialBalance) * 100
    }
    c.JSON(http.StatusOK, gin.H{
        "total_equity":    eq,
        "total_pnl":       totalPnL,
        "total_pnl_pct":   totalPnLPct,
        // ...
    })
}
```

**验证方法**:
1. 创建模拟盘 trader
2. 执行一笔盈利交易并平仓
3. 停止 trader
4. 刷新页面，确认总余额 = 初始余额 + 已实现盈亏
5. 重启 trader，确认日志显示 "Restored realized PnL"

**部署日期**: 2026-02-25

**状态**: ✅ 已修复并部署

---

## BUG-002: 阻力位止盈在亏损时触发

**发现日期**: 2026-02-22 (根据对话历史)

**问题描述**:
一个亏损的仓位被平仓，但平仓原因显示为"阻力位止盈"，这是不合逻辑的。

**影响范围**: 实盘和模拟盘

**根本原因**:
`kernel/take_profit.go` 中的 `checkResistanceTakeProfit` 函数只检查价格是否达到目标，没有验证：
1. 目标价位是否有效（阻力位必须高于开仓价）
2. 当前是否实际盈利

**修复方案**:

```go
// 修复后的 checkResistanceTakeProfit:
func (c *TakeProfitChecker) checkResistanceTakeProfit(position *PositionInfo, currentPrice float64, srLevel float64) *TakeProfitSignal {
    if position.Side == "long" {
        // ✅ 阻力位必须高于开仓价，否则不是有效的止盈目标
        if srLevel <= position.EntryPrice {
            return &TakeProfitSignal{Triggered: false}
        }
        targetPrice = srLevel * (1 - buffer/100)
        // ✅ 同时验证实际盈利
        if currentPrice >= targetPrice && currentPrice > position.EntryPrice {
            // 触发止盈...
        }
    } else { // short
        // ✅ 支撑位必须低于开仓价
        if srLevel >= position.EntryPrice {
            return &TakeProfitSignal{Triggered: false}
        }
        targetPrice = srLevel * (1 + buffer/100)
        // ✅ 同时验证实际盈利
        if currentPrice <= targetPrice && currentPrice < position.EntryPrice {
            // 触发止盈...
        }
    }
}
```

**部署日期**: 2026-02-22

**状态**: ✅ 已修复并部署

---

## BUG-003: 空头止损/多头止盈的支撑阻力位传递错误

**发现日期**: 2026-02-22 (根据对话历史)

**问题描述**:
- 空头仓位的支撑阻力位止损无法触发
- 多头仓位的阻力位止盈无法触发

**影响范围**: 实盘和模拟盘

**根本原因**:
`trader/auto_trader.go` 中 `checkDynamicStopLossTakeProfit` 只根据仓位方向计算一个价位：
- 多头只计算 `supportLevel`
- 空头只计算 `resistanceLevel`

导致传递给止损/止盈检查器的价位为 0。

**修复方案**:

```go
// 修复前:
if side == "long" {
    supportLevel = kernel.FindSupportLevel(klines, markPrice, 30)
} else {
    resistanceLevel = kernel.FindResistanceLevel(klines, markPrice, 30)
}

// 修复后:
// ✅ 始终计算两个价位
supportLevel := kernel.FindSupportLevel(klines, markPrice, 30)
resistanceLevel := kernel.FindResistanceLevel(klines, markPrice, 30)

// 止损检查：多头用支撑位，空头用阻力位
slLevel := supportLevel
if side == "short" {
    slLevel = resistanceLevel
}

// 止盈检查：多头用阻力位，空头用支撑位
tpLevel := resistanceLevel
if side == "short" {
    tpLevel = supportLevel
}
```

**部署日期**: 2026-02-22

**状态**: ✅ 已修复并部署

---

## BUG-004: 杠杆显示不一致（模拟盘显示 2x 而非 5x）

**发现日期**: 2026-02-22 (根据对话历史)

**问题描述**:
策略设置 5x 杠杆，AI 开仓也显示 5x，但"当前持仓"显示 2x。

**影响范围**: 仅模拟盘

**根本原因**:
`trader/auto_trader.go` 中的 `getPosFloat` 函数只检查 `float64` 类型，但 Paper Trader 存储杠杆为 `int` 类型，导致转换失败返回 0，最终默认显示 2。

**修复方案**:

```go
// 新增 toFloat64 辅助函数:
func toFloat64(v interface{}) float64 {
    switch val := v.(type) {
    case float64:
        return val
    case float32:
        return float64(val)
    case int:
        return float64(val)
    case int64:
        return float64(val)
    case int32:
        return float64(val)
    default:
        return 0
    }
}

// 修改 getPosFloat:
func getPosFloat(pos map[string]interface{}, primary, fallback string) float64 {
    if v := toFloat64(pos[primary]); v != 0 {
        return v
    }
    if fallback != "" {
        if v := toFloat64(pos[fallback]); v != 0 {
            return v
        }
    }
    return 0
}
```

**部署日期**: 2026-02-22

**状态**: ✅ 已修复并部署

---

## BUG-005: 盈亏比显示格式不一致

**发现日期**: 2026-02-22 (根据对话历史)

**问题描述**:
AI 分析链显示盈亏比 2.7:1（奖励:风险），但前端开仓截图显示 1:3.8（风险:奖励），造成混淆。

**影响范围**: 前端显示

**根本原因**:
前端组件使用了不同的格式：
- AI 链输出: `奖励:风险` (2.7:1)
- 前端显示: `风险:奖励` (1:3.8)

**修复方案**:

统一使用 `奖励:风险` (X:1) 格式：

```tsx
// web/src/components/DecisionCard.tsx
// 修复前:
<span>1</span>:<span>{ratio.toFixed(1)}</span>

// 修复后:
<span>{ratio.toFixed(1)}</span>:<span>1</span>

// web/src/components/strategy/RiskControlEditor.tsx
// 修复前:
<span>1:</span><input ... />

// 修复后:
<input ... /><span>:1</span>
```

**部署日期**: 2026-02-22

**状态**: ✅ 已修复并部署

---

## BUG-006: 止盈止损未根据实际入场价调整

**发现日期**: 2026-02-22 (根据对话历史)

**问题描述**:
AI 分析时的止盈止损价格是基于分析时的价格计算的，但实际入场价可能因市场波动而不同，导致实际盈亏比偏离预期。

**影响范围**: 实盘和模拟盘

**根本原因**:
系统直接使用 AI 输出的绝对价格设置止盈止损，没有根据实际入场价重新计算。

**修复方案**:

新增 `adjustStopLossTakeProfitForActualEntry` 函数，保持百分比不变，根据实际入场价重新计算止盈止损：

```go
func adjustStopLossTakeProfitForActualEntry(
    aiAnalysisPrice float64,
    actualEntryPrice float64,
    aiStopLoss float64,
    aiTakeProfit float64,
    side string,
) (adjustedSL, adjustedTP, priceDeviation float64) {
    // 计算 AI 分析时的止损/止盈百分比
    if side == "long" {
        slPct = (aiAnalysisPrice - aiStopLoss) / aiAnalysisPrice * 100
        tpPct = (aiTakeProfit - aiAnalysisPrice) / aiAnalysisPrice * 100
    } else {
        slPct = (aiStopLoss - aiAnalysisPrice) / aiAnalysisPrice * 100
        tpPct = (aiAnalysisPrice - aiTakeProfit) / aiAnalysisPrice * 100
    }
    
    // 基于实际入场价应用相同百分比
    if side == "long" {
        adjustedSL = actualEntryPrice * (1 - slPct/100)
        adjustedTP = actualEntryPrice * (1 + tpPct/100)
    } else {
        adjustedSL = actualEntryPrice * (1 + slPct/100)
        adjustedTP = actualEntryPrice * (1 - tpPct/100)
    }
    
    priceDeviation = ((actualEntryPrice - aiAnalysisPrice) / aiAnalysisPrice) * 100
    return
}
```

在 `executeOpenLongWithRecord` 和 `executeOpenShortWithRecord` 中调用此函数。

**部署日期**: 2026-02-22

**状态**: ✅ 已修复并部署

---

## BUG-007: Token 用量在四处场景下混在一起显示

**发现日期**: 2026-02-25

**问题描述**:
Token 用量板块出现在 4 处：回测实验室、实盘模拟、实盘、策略 AI 测试。只要有一次 API 调用，四处的 Token 用量都会显示同一条“最近一次调用”的数据；多个交易员、多种模型调用时也无法区分，无法按场景/交易员独立显示。

**影响范围**: 所有使用 AI 的场景（回测、实盘、模拟、策略工作室）

**根本原因**:
1. 后端仅按 `run_id`（回测）和全局 `lastUsage` 记录；实盘/模拟的 `trader_id` 虽有记录逻辑，但前端未传 `trader_id` 参数。
2. 策略工作室的 AI 测试走 `CallWithMessages`，未带 runID/traderID，只写入全局，且 API 未支持按场景（如 `context=strategy_studio`）查询。

**修复方案**:

1. **前端 API** (`web/src/lib/api.ts`): `getAIUsage(runId?, traderId?, context?)`，请求时携带 `trader_id`、`context` 查询参数。
2. **前端 AIUsageCard** (`web/src/components/AIUsageCard.tsx`): 增加可选参数 `traderId`、`context`，请求时传入，使 SWR key 与 fetcher 按场景区分。
3. **实盘/模拟看板** (`web/src/pages/TraderDashboardPage.tsx`): 传入 `traderId={selectedTraderId}`，仅显示当前交易员的 Token 用量。
4. **策略工作室** (`web/src/pages/StrategyStudioPage.tsx`): 传入 `context="strategy_studio"`，仅显示策略 AI 测试的用量。
5. **后端 mcp** (`mcp/usage.go`): 新增 `lastUsageByScope`、`RecordTokenUsageForScope`、`GetTokenUsageForScope`；回调增加 `scope` 参数，`config` 中按 scope > traderID > runID 记录。
6. **后端 mcp client** (`mcp/client.go`): 回调改为 `(usage, runID, traderID, scope)`；Client 增加 `UsageScope` 与 `SetUsageScope(scope string)`；`call()` 与 `Invoke()` 调用回调时传入 scope。
7. **策略测试** (`api/strategy.go`): 在 `runRealAITest` 中调用 `aiClient.SetUsageScope("strategy_studio")`，使策略 AI 测试的用量单独记录。
8. **API** (`api/server.go`): `handleGetAIUsage` 支持查询参数 `context`，优先返回 `GetTokenUsageForScope(context)`。

**验证方法**:
- 回测：选择某次 run，Token 用量仅显示该次回测的用量。
- 实盘/模拟：切换交易员 A/B，各自看板仅显示该交易员的用量。
- 策略工作室：在策略页点击“运行测试”，Token 用量仅显示策略 AI 测试的用量，与回测/交易员互不干扰。

**部署日期**: 2026-02-25

**状态**: ✅ 已修复

---

## BUG-008: Claude 等模型偶发 504 Gateway Timeout

**发现日期**: 2026-02-26

**问题描述**:
使用 Claude 模型时偶发报错：`Failed to get AI decision: AI API call failed: API returned error (status 504): ...`。504 一般为网关/上游超时，推理链较长时更容易出现。

**影响范围**: 所有走 MCP 的 AI 调用（Claude、DeepSeek 等），尤其 Claude/推理链长的模型

**根本原因**:
1. 504/502/503 未视为可重试错误，一次失败即返回，没有自动重试。
2. 超时仅使用默认 300 秒，无法通过配置延长（部分网关或推理链需更长时间）。

**修复方案**:

1. **可重试错误** (`mcp/client.go`): 在 `retryableErrors` 中增加 `"status 502"`、`"status 503"`、`"status 504"`，收到这些 HTTP 状态时自动按现有重试逻辑重试（最多 3 次，间隔递增）。
2. **超时可配置** (`mcp/config.go`): `DefaultConfig` 支持环境变量 `AI_TIMEOUT_SECONDS`（默认 300）。例如设置 `AI_TIMEOUT_SECONDS=600` 可将单次请求超时改为 10 分钟，降低因超时导致的 504。

**验证方法**:
- 出现 504 时日志中应看到重试：`AI API call failed, retrying (1/3)...`，若重试成功会有 `AI retry succeeded`。
- 若仍频繁 504，可在部署环境设置 `AI_TIMEOUT_SECONDS=600`（或更大）后重启服务。

**部署日期**: 2026-02-26

**状态**: ✅ 已修复

---

## 实盘 vs 模拟盘问题对比

| Bug ID | 问题描述 | 实盘影响 | 模拟盘影响 |
|--------|----------|----------|------------|
| BUG-001 | 余额不含已实现盈亏 | ❌ 不影响（从交易所获取实时余额） | ✅ 已修复 |
| BUG-002 | 阻力位止盈亏损触发 | ✅ 已修复 | ✅ 已修复 |
| BUG-003 | 支撑阻力位传递错误 | ✅ 已修复 | ✅ 已修复 |
| BUG-004 | 杠杆显示不一致 | ❌ 不影响（从交易所获取） | ✅ 已修复 |
| BUG-005 | 盈亏比格式不一致 | ✅ 已修复 | ✅ 已修复 |
| BUG-006 | 止盈止损未调整 | ✅ 已修复 | ✅ 已修复 |
| BUG-007 | Token 用量未按场景/交易员隔离 | 全场景（回测/实盘/模拟/策略工作室）已按 run_id、trader_id、context 隔离 | ✅ 已修复 |
| BUG-008 | Claude 等 504 超时未重试 | 502/503/504 自动重试；可配置 AI_TIMEOUT_SECONDS 延长超时 | ✅ 已修复 |

---

## 功能改进：止损延迟确认 + 高波动宽容

**日期**: 2026-02-22

**说明**: 为减少单 K 线假跌破导致的过早止损，并高波动时更宽容，增加两项策略配置（与实盘/回测一致）：

1. **连续确认再止损 (ConfirmCycles)**  
   - 配置项：`confirm_cycles`（默认 2）  
   - 逻辑：只有当止损条件**连续 N 个检查周期**都满足时，才执行止损；否则本周期仅累加计数，不执行。  
   - 实现：`trader/auto_trader.go` 使用 `slConfirmCount` 按 `posKey` 记录连续满足次数；`backtest/runner.go` 使用 `slConfirmCount` 按 `key`（Symbol:Side）记录。  
   - 配置为 1 时行为与原来一致（立即执行）。

2. **高波动宽容 (ATR Tolerance)**  
   - 配置项：`atr_tolerance_enabled`（默认 true）、`atr_high_multiplier`（默认 1.2）  
   - 逻辑：  
     - **Checker 内**：当 `atr > atrLong * atr_high_multiplier` 时视为高波动，ATR 止损使用 `ATRMultiplierMax`（更宽），减少噪音触发。  
     - **确认周期**：高波动时 `requiredCycles = ConfirmCycles + 1`，多要求 1 个确认周期再执行。  
   - 实现：`kernel/stop_loss.go` 的 `CheckStopLoss` 增加参数 `atrLong`，`checkATRStop` 内根据配置与 atr/atrLong 判断是否使用更宽倍数；实盘/回测在调用前计算 `atrLong = ATR(klines, 28)` 并传入。

**涉及文件**:
- `store/strategy.go`: `DynamicStopLossConfig` 增加 `ConfirmCycles`、`ATRToleranceEnabled`、`ATRHighMultiplier` 及默认值  
- `kernel/stop_loss.go`: `CheckStopLoss(..., atrLong)`、`checkATRStop(..., atrLong)` 高波动时用 Max 倍数  
- `trader/auto_trader.go`: `slConfirmCount` 与 `requiredCycles` 逻辑，以及 `atrLong` 计算与传递  
- `backtest/runner.go`: 同上确认计数与 `atrLong`，且 SL 使用 `slLevel`（多=支撑、空=阻力）  
- 前端 `DynamicStopLossEditor` / `types` 已有 `confirm_cycles`、`atr_tolerance_enabled`、`atr_high_multiplier` 配置项

**状态**: ✅ 已实现

---

## 文档维护说明

- 每次发现并修复 bug 后，在此文档添加新记录
- 使用递增的 BUG-XXX 编号
- 记录内容包括：发现日期、问题描述、影响范围、根本原因、修复方案、验证方法、部署日期、状态
- 在对比表格中更新实盘/模拟盘影响情况
