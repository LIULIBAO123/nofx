# 动态止盈止损功能文档

## 概述

动态止盈止损功能为 NOFX AI 交易系统提供了灵活的风险管理工具，支持多种止损和止盈策略。

## 功能特性

### 🔴 动态止损 (Dynamic Stop Loss)

#### 1. 追踪止损 (Trailing Stop)
- **追踪止损百分比**: 价格回撤多少百分比触发止损
- **激活盈利阈值**: 盈利达到多少百分比后激活追踪止损
- **使用场景**: 适合趋势行情，锁定利润同时给予价格波动空间

**示例配置**:
```json
{
  "enabled": true,
  "mode": "trailing",
  "trailing_percent": 2,
  "trailing_activation": 3,
  "initial_stop_percent": 3,
  "min_stop_percent": 1
}
```

**工作原理**:
1. 开仓时设置初始止损 3%
2. 当盈利达到 3% 时，激活追踪止损
3. 价格每上涨，止损线跟随上移，保持 2% 的距离
4. 如果价格回撤 2%，触发止损平仓

#### 2. ATR 止损 (ATR-Based Stop)
- **ATR 倍数**: 止损距离 = ATR × 倍数
- **ATR 周期**: 计算 ATR 的周期（默认 14）
- **使用场景**: 根据市场波动性动态调整止损距离

**示例配置**:
```json
{
  "enabled": true,
  "mode": "atr",
  "atr_multiplier": 2,
  "atr_period": 14,
  "min_stop_percent": 1
}
```

**工作原理**:
- 如果 ATR = 100 USDT，倍数 = 2
- 止损距离 = 200 USDT
- 动态根据市场波动调整

#### 3. 支撑阻力止损 (Support/Resistance Stop)
- **缓冲百分比**: 在支撑/阻力位基础上的缓冲
- **使用场景**: 基于技术分析的关键价位设置止损

**示例配置**:
```json
{
  "enabled": true,
  "mode": "support_resistance",
  "support_resistance_buffer": 0.5,
  "min_stop_percent": 1
}
```

#### 4. 时间止损 (Time-Based Stop)
- **最大持仓时间**: 超过此时间自动平仓
- **时间退出盈利要求**: 时间止损时的最小盈利要求（负数表示允许亏损）
- **使用场景**: 避免长时间占用资金，强制止损

**示例配置**:
```json
{
  "enabled": true,
  "mode": "time_based",
  "max_hold_hours": 48,
  "time_based_exit_percent": -1,
  "initial_stop_percent": 3
}
```

**工作原理**:
- 持仓超过 48 小时
- 如果盈利 ≥ -1%（即亏损不超过 1%），自动平仓
- 如果亏损超过 1%，等待价格回升或触发初始止损

---

### 🟢 动态止盈 (Dynamic Take Profit)

#### 1. 固定止盈 (Fixed Take Profit)
- **固定止盈百分比**: 达到此盈利百分比时全部平仓
- **使用场景**: 简单直接，适合明确目标的交易

**示例配置**:
```json
{
  "enabled": true,
  "mode": "fixed",
  "fixed_percent": 5
}
```

#### 2. 分批止盈 (Scaled Take Profit) ⭐ 推荐
- **多层级止盈**: 在不同盈利点分批平仓
- **移动止损到盈亏平衡**: 达到某层级后保护利润
- **使用场景**: 平衡风险和收益，适合大多数交易

**示例配置**:
```json
{
  "enabled": true,
  "mode": "scaled",
  "scaled_levels": [
    {
      "profit_percent": 3,
      "close_percent": 30,
      "move_stop_to_breakeven": false
    },
    {
      "profit_percent": 5,
      "close_percent": 30,
      "move_stop_to_breakeven": true
    },
    {
      "profit_percent": 8,
      "close_percent": 40
    }
  ],
  "partial_close_enabled": true,
  "lock_profit_percent": 5
}
```

**工作原理**:
1. 盈利达到 3% → 平仓 30%
2. 盈利达到 5% → 平仓 30%，并移动止损到开仓价（盈亏平衡）
3. 盈利达到 8% → 平仓剩余 40%

#### 3. ATR 止盈 (ATR-Based Take Profit)
- **ATR 倍数**: 止盈距离 = ATR × 倍数
- **使用场景**: 根据市场波动性设置止盈目标

**示例配置**:
```json
{
  "enabled": true,
  "mode": "atr",
  "atr_multiplier": 3,
  "atr_period": 14
}
```

#### 4. 阻力位止盈 (Resistance Take Profit)
- **阻力位缓冲**: 在阻力位基础上的缓冲百分比
- **使用场景**: 基于技术分析的关键价位设置止盈

**示例配置**:
```json
{
  "enabled": true,
  "mode": "resistance",
  "resistance_buffer": 0.5
}
```

---

## 使用指南

### 前端配置

1. 打开**策略工作室** (Strategy Studio)
2. 选择或创建一个策略
3. 展开**风控参数** (Risk Control) 部分
4. 找到**动态止损**和**动态止盈**配置区域
5. 启用功能并选择合适的模式
6. 调整参数
7. 保存策略

### 推荐配置组合

#### 保守型交易者
```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "mode": "trailing",
    "trailing_percent": 1.5,
    "trailing_activation": 2,
    "initial_stop_percent": 2
  },
  "dynamic_take_profit": {
    "enabled": true,
    "mode": "scaled",
    "scaled_levels": [
      { "profit_percent": 2, "close_percent": 40, "move_stop_to_breakeven": true },
      { "profit_percent": 4, "close_percent": 30 },
      { "profit_percent": 6, "close_percent": 30 }
    ]
  }
}
```

#### 激进型交易者
```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "mode": "atr",
    "atr_multiplier": 2.5,
    "atr_period": 14
  },
  "dynamic_take_profit": {
    "enabled": true,
    "mode": "scaled",
    "scaled_levels": [
      { "profit_percent": 5, "close_percent": 30 },
      { "profit_percent": 10, "close_percent": 30, "move_stop_to_breakeven": true },
      { "profit_percent": 15, "close_percent": 40 }
    ]
  }
}
```

#### 日内交易者
```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "mode": "time_based",
    "max_hold_hours": 24,
    "time_based_exit_percent": 0,
    "initial_stop_percent": 2
  },
  "dynamic_take_profit": {
    "enabled": true,
    "mode": "fixed",
    "fixed_percent": 3
  }
}
```

---

## 后端实现要点

### 1. 止损逻辑实现位置
在 `trader/auto_trader.go` 或 `trader/position_manager.go` 中实现：

```go
func (at *AutoTrader) checkDynamicStopLoss(position *Position) (shouldClose bool, reason string) {
    config := at.strategy.RiskControl.DynamicStopLoss
    if config == nil || !config.Enabled {
        return false, ""
    }

    switch config.Mode {
    case "trailing":
        return at.checkTrailingStop(position, config)
    case "atr":
        return at.checkATRStop(position, config)
    case "support_resistance":
        return at.checkSupportResistanceStop(position, config)
    case "time_based":
        return at.checkTimeBasedStop(position, config)
    }

    return false, ""
}
```

### 2. 止盈逻辑实现

```go
func (at *AutoTrader) checkDynamicTakeProfit(position *Position) (shouldClose bool, closePercent float64, reason string) {
    config := at.strategy.RiskControl.DynamicTakeProfit
    if config == nil || !config.Enabled {
        return false, 0, ""
    }

    switch config.Mode {
    case "fixed":
        return at.checkFixedTakeProfit(position, config)
    case "scaled":
        return at.checkScaledTakeProfit(position, config)
    case "atr":
        return at.checkATRTakeProfit(position, config)
    case "resistance":
        return at.checkResistanceTakeProfit(position, config)
    }

    return false, 0, ""
}
```

### 3. 追踪止损实现示例

```go
func (at *AutoTrader) checkTrailingStop(position *Position, config *DynamicStopLossConfig) (bool, string) {
    currentPrice := position.MarkPrice
    entryPrice := position.EntryPrice
    
    // 计算当前盈利百分比
    profitPct := (currentPrice - entryPrice) / entryPrice * 100
    
    // 检查是否激活追踪止损
    if profitPct < *config.TrailingActivation {
        // 未激活，使用初始止损
        initialStopPrice := entryPrice * (1 - *config.InitialStopPercent/100)
        if currentPrice <= initialStopPrice {
            return true, fmt.Sprintf("Initial stop loss triggered at %.2f%%", *config.InitialStopPercent)
        }
        return false, ""
    }
    
    // 已激活追踪止损
    // 计算最高价（需要在 Position 结构体中记录）
    if position.HighestPrice == 0 || currentPrice > position.HighestPrice {
        position.HighestPrice = currentPrice
    }
    
    // 计算追踪止损价格
    trailingStopPrice := position.HighestPrice * (1 - *config.TrailingPercent/100)
    
    if currentPrice <= trailingStopPrice {
        return true, fmt.Sprintf("Trailing stop triggered at %.2f%% from peak", *config.TrailingPercent)
    }
    
    return false, ""
}
```

### 4. 分批止盈实现示例

```go
func (at *AutoTrader) checkScaledTakeProfit(position *Position, config *DynamicTakeProfitConfig) (bool, float64, string) {
    currentPrice := position.MarkPrice
    entryPrice := position.EntryPrice
    
    profitPct := (currentPrice - entryPrice) / entryPrice * 100
    
    // 检查每个层级
    for i, level := range config.ScaledLevels {
        // 检查是否已经执行过这个层级
        if position.ExecutedTPLevels[i] {
            continue
        }
        
        if profitPct >= level.ProfitPercent {
            // 标记该层级已执行
            position.ExecutedTPLevels[i] = true
            
            // 移动止损到盈亏平衡
            if level.MoveStopToBreakeven != nil && *level.MoveStopToBreakeven {
                position.StopLossPrice = entryPrice
            }
            
            return true, level.ClosePercent, fmt.Sprintf("Scaled TP level %d: %.2f%% profit, closing %.0f%%", 
                i+1, level.ProfitPercent, level.ClosePercent)
        }
    }
    
    return false, 0, ""
}
```

---

## 数据结构扩展

需要在 `Position` 结构体中添加以下字段：

```go
type Position struct {
    // ... 现有字段 ...
    
    // 动态止盈止损相关
    HighestPrice      float64           `json:"highest_price"`       // 持仓期间最高价
    LowestPrice       float64           `json:"lowest_price"`        // 持仓期间最低价
    ExecutedTPLevels  map[int]bool      `json:"executed_tp_levels"`  // 已执行的止盈层级
    TrailingStopPrice float64           `json:"trailing_stop_price"` // 当前追踪止损价格
    EntryTime         time.Time         `json:"entry_time"`          // 开仓时间
}
```

---

## 测试建议

1. **回测验证**: 在回测系统中测试不同配置的效果
2. **小仓位测试**: 先用小仓位在实盘测试
3. **监控日志**: 观察止损止盈触发的频率和时机
4. **参数优化**: 根据不同币种和市场环境调整参数

---

## 注意事项

⚠️ **重要提示**:

1. **滑点影响**: 实际成交价可能与触发价有偏差
2. **网络延迟**: 确保系统能及时检测价格变化
3. **交易所限制**: 某些交易所可能不支持部分平仓
4. **费用考虑**: 频繁止盈可能增加手续费
5. **市场波动**: 极端行情下可能无法按预期价格成交

---

## 未来扩展

- [ ] 支持基于成交量的止损
- [ ] 支持基于资金费率的止损
- [ ] 支持多目标止盈（同时设置多个止盈价）
- [ ] 支持条件止损（例如：跌破 EMA 时止损）
- [ ] 支持智能止损（AI 动态调整止损位）

---

## 相关文档

- [风控参数配置](./risk-control.md)
- [策略工作室使用指南](./guides/strategy-studio.md)
- [回测系统文档](./backtest-guide.md)




