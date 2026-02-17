# 动态止损功能设计文档 v2

## 📋 设计概述

动态止损功能采用**多条件组合**的设计，允许用户同时启用多种止损策略，并通过触发逻辑控制如何执行止损。

---

## 🎯 核心设计理念

### 1. 初始固定止损（必需）
- **作用**：作为保底止损，确保每个仓位都有基本的风险保护
- **特点**：始终生效，不可禁用
- **默认值**：3%

### 2. 三种可选止损模式
用户可以根据需要**独立启用/禁用**以下三种止损模式：

#### 📈 追踪止损 (Trailing Stop)
- **适用场景**：趋势行情，锁定利润
- **工作原理**：
  1. 开仓时使用初始固定止损
  2. 当盈利达到激活阈值（如3%）时，启动追踪止损
  3. 价格每上涨，止损线跟随上移，保持固定距离（如2%）
  4. 价格回撤触发止损距离时平仓

#### 📊 ATR 止损
- **适用场景**：根据市场波动性动态调整止损
- **工作原理**：
  - 止损距离 = ATR × 倍数
  - 市场波动大时，止损距离自动放宽
  - 市场波动小时，止损距离自动收紧

#### 📉 支撑阻力止损
- **适用场景**：基于技术分析的关键价位
- **工作原理**：
  - 系统识别支撑/阻力位
  - 在关键价位附近设置止损（带缓冲）
  - 跌破支撑位或突破阻力位时触发

---

## ⚙️ 触发逻辑

用户可以选择两种触发逻辑：

### 🔴 任一条件触发 (ANY) - 推荐
- **逻辑**：任何一个启用的止损条件触发即平仓
- **特点**：更保守，风险控制更严格
- **适用**：大多数交易场景

**示例**：
```
启用：初始止损(3%) + 追踪止损(2%) + ATR止损(2x ATR)
结果：三个条件中任何一个触发都会平仓
```

### 🟡 所有条件触发 (ALL)
- **逻辑**：所有启用的止损条件都触发才平仓
- **特点**：更激进，给予更大的波动空间
- **适用**：高波动市场，需要更大容错空间

**示例**：
```
启用：初始止损(3%) + 追踪止损(2%)
结果：必须同时满足初始止损和追踪止损条件才平仓
```

---

## 💡 配置示例

### 保守型配置
```json
{
  "enabled": true,
  "trigger_logic": "any",
  "initial_stop_percent": 2,
  "trailing_enabled": true,
  "trailing_percent": 1.5,
  "trailing_activation": 2,
  "atr_enabled": false,
  "support_resistance_enabled": false
}
```
**说明**：使用较小的止损距离，追踪止损激活门槛低，任一条件触发即止损。

---

### 平衡型配置（推荐）
```json
{
  "enabled": true,
  "trigger_logic": "any",
  "initial_stop_percent": 3,
  "trailing_enabled": true,
  "trailing_percent": 2,
  "trailing_activation": 3,
  "atr_enabled": true,
  "atr_multiplier": 2,
  "atr_period": 14,
  "support_resistance_enabled": false
}
```
**说明**：结合追踪止损和ATR止损，适应不同市场环境。

---

### 激进型配置
```json
{
  "enabled": true,
  "trigger_logic": "all",
  "initial_stop_percent": 5,
  "trailing_enabled": true,
  "trailing_percent": 3,
  "trailing_activation": 5,
  "atr_enabled": true,
  "atr_multiplier": 3,
  "atr_period": 14,
  "support_resistance_enabled": false
}
```
**说明**：使用较大的止损距离，要求所有条件都触发才止损，给予更大波动空间。

---

## 🔧 后端实现逻辑

### 止损检查函数

```go
func (at *AutoTrader) checkDynamicStopLoss(position *Position) (shouldClose bool, reason string) {
    config := at.strategy.RiskControl.DynamicStopLoss
    if config == nil || !config.Enabled {
        return false, ""
    }

    triggeredConditions := []string{}
    totalConditions := 0

    // 1. 检查初始固定止损（始终检查）
    totalConditions++
    if at.checkInitialStop(position, config) {
        triggeredConditions = append(triggeredConditions, "初始止损")
    }

    // 2. 检查追踪止损（如果启用）
    if config.TrailingEnabled != nil && *config.TrailingEnabled {
        totalConditions++
        if at.checkTrailingStop(position, config) {
            triggeredConditions = append(triggeredConditions, "追踪止损")
        }
    }

    // 3. 检查ATR止损（如果启用）
    if config.ATREnabled != nil && *config.ATREnabled {
        totalConditions++
        if at.checkATRStop(position, config) {
            triggeredConditions = append(triggeredConditions, "ATR止损")
        }
    }

    // 4. 检查支撑阻力止损（如果启用）
    if config.SupportResistanceEnabled != nil && *config.SupportResistanceEnabled {
        totalConditions++
        if at.checkSupportResistanceStop(position, config) {
            triggeredConditions = append(triggeredConditions, "支撑阻力止损")
        }
    }

    // 根据触发逻辑判断是否平仓
    if config.TriggerLogic == "any" {
        // 任一条件触发即平仓
        if len(triggeredConditions) > 0 {
            return true, fmt.Sprintf("止损触发: %s", strings.Join(triggeredConditions, ", "))
        }
    } else if config.TriggerLogic == "all" {
        // 所有条件都触发才平仓
        if len(triggeredConditions) == totalConditions {
            return true, "所有止损条件均已触发"
        }
    }

    return false, ""
}
```

### 各止损条件实现

```go
// 初始固定止损
func (at *AutoTrader) checkInitialStop(position *Position, config *DynamicStopLossConfig) bool {
    currentPrice := position.MarkPrice
    entryPrice := position.EntryPrice
    
    stopPrice := entryPrice * (1 - config.InitialStopPercent/100)
    return currentPrice <= stopPrice
}

// 追踪止损
func (at *AutoTrader) checkTrailingStop(position *Position, config *DynamicStopLossConfig) bool {
    currentPrice := position.MarkPrice
    entryPrice := position.EntryPrice
    
    profitPct := (currentPrice - entryPrice) / entryPrice * 100
    
    // 未达到激活阈值，不触发
    if profitPct < *config.TrailingActivation {
        return false
    }
    
    // 更新最高价
    if position.HighestPrice == 0 || currentPrice > position.HighestPrice {
        position.HighestPrice = currentPrice
    }
    
    // 计算追踪止损价格
    trailingStopPrice := position.HighestPrice * (1 - *config.TrailingPercent/100)
    
    return currentPrice <= trailingStopPrice
}

// ATR 止损
func (at *AutoTrader) checkATRStop(position *Position, config *DynamicStopLossConfig) bool {
    // 获取ATR值
    atr := at.getATR(position.Symbol, *config.ATRPeriod)
    if atr <= 0 {
        return false
    }
    
    currentPrice := position.MarkPrice
    entryPrice := position.EntryPrice
    
    stopDistance := atr * (*config.ATRMultiplier)
    stopPrice := entryPrice - stopDistance
    
    return currentPrice <= stopPrice
}

// 支撑阻力止损
func (at *AutoTrader) checkSupportResistanceStop(position *Position, config *DynamicStopLossConfig) bool {
    // 识别支撑位
    supportLevel := at.findSupportLevel(position.Symbol)
    if supportLevel <= 0 {
        return false
    }
    
    currentPrice := position.MarkPrice
    
    // 计算带缓冲的止损价格
    stopPrice := supportLevel * (1 - *config.SupportResistanceBuffer/100)
    
    return currentPrice <= stopPrice
}
```

---

## 📊 UI 设计

### 布局结构
```
┌─────────────────────────────────────┐
│ [启用动态止损]           [●─────]   │
├─────────────────────────────────────┤
│ 触发逻辑：                           │
│ [任一条件触发] [所有条件触发]        │
├─────────────────────────────────────┤
│ 初始固定止损（必需）                 │
│ ────●──── 3%                        │
├─────────────────────────────────────┤
│ ☑ 追踪止损                          │
│   追踪止损百分比: ────●──── 2%      │
│   激活盈利阈值:   ────●──── 3%      │
├─────────────────────────────────────┤
│ ☐ ATR 止损                          │
├─────────────────────────────────────┤
│ ☐ 支撑阻力止损                       │
└─────────────────────────────────────┘
```

---

## ✅ 优势

1. **灵活性**：用户可以根据市场环境和交易风格自由组合
2. **清晰性**：每种止损模式独立开关，逻辑清晰
3. **安全性**：初始固定止损作为保底，确保基本风险控制
4. **可控性**：触发逻辑让用户明确控制止损的严格程度

---

## 🚀 使用建议

### 新手推荐
- 启用：初始止损 + 追踪止损
- 触发逻辑：任一条件触发
- 参数：保守型配置

### 进阶用户
- 启用：初始止损 + 追踪止损 + ATR止损
- 触发逻辑：任一条件触发
- 参数：根据市场调整

### 专业交易者
- 启用：全部三种止损
- 触发逻辑：根据市场波动选择
- 参数：自定义优化

---

## 📝 注意事项

1. **初始止损不可禁用**：确保每个仓位都有基本保护
2. **触发逻辑选择**：
   - 保守交易者选择 "任一条件触发"
   - 激进交易者选择 "所有条件触发"
3. **参数调整**：根据不同币种和市场环境调整参数
4. **回测验证**：新配置建议先在回测系统中验证效果



