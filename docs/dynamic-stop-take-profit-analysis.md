# 动态止盈止损完整设计文档 v4.0

## 概述

本文档描述了 nofx 交易系统的动态止盈止损功能的完整设计，包括逻辑一致性分析、参数合理性验证和最佳实践建议。

## 设计原则

### 1. 对称性原则
止盈和止损应该在设计上保持对称：
- **止损**：保护资金，防止损失扩大
- **止盈**：锁定利润，防止利润回吐

### 2. 独立性原则
每种止盈/止损模式都有独立的开关，可以灵活组合使用。

### 3. 智能化原则
使用 AI 在动态区间内自适应选择参数，而不是固定值。

### 4. 币种差异化原则
主流币（BTC/ETH）和山寨币使用不同的参数，适应不同的波动特性。

---

## 动态止损设计

### 1. 初始固定止损（必需）

**作用**：作为保底止损机制，无论其他止损模式是否启用都会生效。

**参数**：
- `initial_stop_percent`: 0.5% - 10%
- **默认值**: 3%

**推荐配置**：
- 保守型：2%
- 平衡型：3%
- 激进型：5%

---

### 2. 追踪止损 - 分层模式

**设计理念**：根据盈利水平动态调整允许的回撤幅度。

**参数结构**：
```typescript
interface TrailingStopLevel {
  profit_threshold: number;  // 盈利阈值（%）
  trailing_percent: number;  // 该层级的追踪止损百分比（%）
}
```

**默认配置**：
```json
{
  "trailing_enabled": true,
  "trailing_levels": [
    { "profit_threshold": 2, "trailing_percent": 1.5 },
    { "profit_threshold": 5, "trailing_percent": 2.5 },
    { "profit_threshold": 10, "trailing_percent": 4 }
  ]
}
```

**工作逻辑**：
1. 系统持续监控当前盈利百分比
2. 根据盈利水平匹配对应的层级（选择最高的已达到阈值的层级）
3. 使用该层级的 `trailing_percent` 作为追踪止损距离
4. 当价格从最高点回撤超过该百分比时触发止损

**推荐配置**：

| 风格 | 层级 1 | 层级 2 | 层级 3 |
|------|--------|--------|--------|
| 保守型 | 1% → 1% | 3% → 2% | 5% → 3% |
| 平衡型 | 2% → 1.5% | 5% → 2.5% | 10% → 4% |
| 激进型 | 3% → 2% | 8% → 4% | 15% → 6% |

---

### 3. ATR 止损 - 动态区间模式

**设计理念**：AI 根据市场波动状况，在设定的倍数区间内自适应选择合适的 ATR 倍数。

**参数说明**：

| 参数 | 说明 | 默认值 | 范围 |
|------|------|--------|------|
| `atr_multiplier_min` | ATR 倍数最小值 | 1.5 | 0.5 - 5.0 |
| `atr_multiplier_max` | ATR 倍数最大值 | 3.5 | 0.5 - 5.0 |
| `atr_period_btc_eth` | BTC/ETH 的 ATR 周期 | 20 | 5 - 50 |
| `atr_period_altcoin` | 山寨币的 ATR 周期 | 14 | 5 - 50 |

**工作逻辑**：
1. **币种识别**：BTC/ETH 使用 `atr_period_btc_eth`，其他币种使用 `atr_period_altcoin`
2. **ATR 计算**：根据币种选择对应的周期计算 ATR 值
3. **倍数选择**（AI 决策）：
   - 分析当前市场波动率
   - 在 `[atr_multiplier_min, atr_multiplier_max]` 区间内选择合适倍数
   - 考虑因素：近期波动、趋势强度、市场情绪等
4. **止损距离**：`stop_distance = ATR × selected_multiplier`

**推荐配置**：

| 风格 | 倍数区间 | BTC/ETH 周期 | 山寨币周期 |
|------|----------|--------------|------------|
| 保守型 | 1.5 - 2.5 | 20 | 14 |
| 平衡型 | 1.5 - 3.5 | 20 | 14 |
| 激进型 | 2.0 - 4.5 | 25 | 18 |

---

### 4. 支撑阻力止损

**参数**：
- `support_resistance_buffer`: 0.1% - 2%
- **默认值**: 0.5%

**工作逻辑**：
1. AI 识别关键支撑位
2. 止损价格 = 支撑位 × (1 - buffer%)
3. 当价格跌破止损价格时触发

---

### 5. 触发逻辑

**任一条件触发（`any`）**：
- 任何一个启用的止损条件触发即平仓
- 适用场景：保守型交易者，优先保护资金

**所有条件触发（`all`）**：
- 所有启用的止损条件都触发才平仓
- 适用场景：激进型交易者，给予更多空间

---

## 动态止盈设计

### 1. 固定止盈

**参数**：
- `fixed_percent`: 1% - 50%
- **默认值**: 8%

**工作逻辑**：
- 当盈利达到设定百分比时，全部平仓

**推荐配置**：
- 短线：5% - 8%
- 中线：10% - 15%
- 长线：20% - 30%

---

### 2. 分批止盈

**设计理念**：在不同盈利水平分批平仓，既锁定部分利润，又保留继续盈利的可能。

**参数结构**：
```typescript
interface ScaledTakeProfitLevel {
  profit_percent: number;          // 盈利百分比触发点
  close_percent: number;           // 平仓百分比
  move_stop_to_breakeven?: boolean; // 是否移动止损到盈亏平衡点
}
```

**默认配置**：
```json
{
  "scaled_enabled": true,
  "scaled_levels": [
    { "profit_percent": 3, "close_percent": 30, "move_stop_to_breakeven": false },
    { "profit_percent": 6, "close_percent": 30, "move_stop_to_breakeven": true },
    { "profit_percent": 10, "close_percent": 40, "move_stop_to_breakeven": false }
  ]
}
```

**工作逻辑**：
1. 监控当前盈利百分比
2. 当达到某个层级的 `profit_percent` 时：
   - 平仓 `close_percent` 百分比的持仓
   - 如果 `move_stop_to_breakeven` 为 true，将止损移动到开仓价格
3. 继续监控剩余持仓

**推荐配置**：

| 风格 | 层级 1 | 层级 2 | 层级 3 |
|------|--------|--------|--------|
| 保守型 | 2% → 40% | 4% → 30% | 6% → 30% |
| 平衡型 | 3% → 30% | 6% → 30% + 锁定 | 10% → 40% |
| 激进型 | 5% → 25% | 10% → 25% + 锁定 | 20% → 50% |

---

### 3. ATR 止盈 - 动态区间模式

**设计理念**：AI 根据市场趋势强度，在设定的倍数区间内自适应选择合适的 ATR 倍数。

**参数说明**：

| 参数 | 说明 | 默认值 | 范围 |
|------|------|--------|------|
| `atr_multiplier_min` | ATR 倍数最小值 | 2.5 | 1.0 - 10.0 |
| `atr_multiplier_max` | ATR 倍数最大值 | 5.0 | 1.0 - 10.0 |
| `atr_period_btc_eth` | BTC/ETH 的 ATR 周期 | 20 | 5 - 50 |
| `atr_period_altcoin` | 山寨币的 ATR 周期 | 14 | 5 - 50 |

**工作逻辑**：
1. **币种识别**：与止损相同
2. **ATR 计算**：使用相同的周期参数
3. **倍数选择**（AI 决策）：
   - 分析当前趋势强度
   - 在 `[atr_multiplier_min, atr_multiplier_max]` 区间内选择合适倍数
   - 考虑因素：趋势持续性、动量强度、成交量等
4. **止盈距离**：`take_profit_distance = ATR × selected_multiplier`

**推荐配置**：

| 风格 | 倍数区间 | BTC/ETH 周期 | 山寨币周期 |
|------|----------|--------------|------------|
| 保守型 | 2.0 - 4.0 | 20 | 14 |
| 平衡型 | 2.5 - 5.0 | 20 | 14 |
| 激进型 | 3.0 - 7.0 | 25 | 18 |

---

### 4. 阻力位止盈

**参数**：
- `resistance_buffer`: 0.1% - 2%
- **默认值**: 0.5%

**工作逻辑**：
1. AI 识别关键阻力位
2. 止盈价格 = 阻力位 × (1 - buffer%)
3. 当价格接近止盈价格时触发

---

### 5. 锁定利润机制

**参数**：
- `lock_profit_percent`: 1% - 20%
- **默认值**: 5%

**工作逻辑**：
- 当盈利达到设定百分比时，自动将止损移动到开仓价格（盈亏平衡点）
- 确保至少不亏损

---

## 逻辑一致性分析

### 1. ATR 参数一致性 ✅

**止损 ATR 倍数区间**：1.5 - 3.5（默认）
**止盈 ATR 倍数区间**：2.5 - 5.0（默认）

**分析**：
- ✅ **合理**：止盈倍数大于止损倍数，符合风险回报比原则
- ✅ **盈亏比**：最小盈亏比 = 2.5 / 3.5 ≈ 0.71，最大盈亏比 = 5.0 / 1.5 ≈ 3.33
- ✅ **建议**：保持止盈倍数 > 止损倍数，确保盈亏比 > 1

**ATR 周期一致性**：
- ✅ **统一**：止损和止盈使用相同的 ATR 周期参数
- ✅ **优点**：确保止损和止盈基于相同的波动率计算，逻辑一致

---

### 2. 追踪止损与分批止盈的协调 ✅

**潜在冲突**：
- 追踪止损可能在分批止盈之前触发
- 分批止盈后，剩余持仓的追踪止损需要重新计算

**解决方案**：
1. **优先级**：分批止盈优先于追踪止损
2. **重新计算**：每次分批平仓后，重新计算剩余持仓的追踪止损基准
3. **锁定机制**：分批止盈时可选择移动止损到盈亏平衡点

**示例**：
```
开仓价格：100 USDT
当前价格：106 USDT（盈利 6%）

分批止盈配置：
- 3% 盈利时平仓 30%（已触发，剩余 70%）
- 6% 盈利时平仓 30% + 锁定（即将触发）

追踪止损配置：
- 5% 盈利时允许 2.5% 回撤

执行顺序：
1. 价格达到 103 USDT（3% 盈利）→ 平仓 30%
2. 价格达到 106 USDT（6% 盈利）→ 平仓 30% + 移动止损到 100 USDT
3. 剩余 40% 持仓，止损在 100 USDT（盈亏平衡点）
4. 追踪止损继续监控剩余持仓
```

---

### 3. 固定止盈与其他模式的冲突 ⚠️

**问题**：
- 如果同时启用固定止盈和分批止盈，可能产生冲突
- 固定止盈会在达到目标时全部平仓，导致分批止盈无法执行

**解决方案**：
1. **互斥模式**：固定止盈和分批止盈建议只启用一个
2. **优先级**：如果同时启用，分批止盈优先级更高
3. **UI 提示**：在前端提示用户这两种模式的互斥关系

**建议**：
- 短线交易：使用固定止盈
- 中长线交易：使用分批止盈
- 趋势交易：使用 ATR 止盈或阻力位止盈

---

### 4. 止损止盈距离的合理性 ✅

**风险回报比验证**：

| 配置 | 止损距离 | 止盈距离 | 盈亏比 | 评价 |
|------|----------|----------|--------|------|
| 保守型 | 2% 固定 | 5% 固定 | 2.5:1 | ✅ 优秀 |
| 平衡型 | 3% 固定 | 8% 固定 | 2.67:1 | ✅ 优秀 |
| 激进型 | 5% 固定 | 15% 固定 | 3:1 | ✅ 优秀 |
| ATR 最小 | 1.5 ATR | 2.5 ATR | 1.67:1 | ✅ 良好 |
| ATR 最大 | 3.5 ATR | 5.0 ATR | 1.43:1 | ⚠️ 偏低 |

**建议调整**：
- ATR 止盈最大倍数可以提高到 6.0 - 7.0，以提高盈亏比
- 或者降低 ATR 止损最大倍数到 3.0

---

### 5. 锁定利润机制的时机 ✅

**默认配置**：
- `lock_profit_percent`: 5%

**与其他机制的协调**：
- ✅ **与分批止盈**：可以在第二层级（6% 盈利）时锁定
- ✅ **与追踪止损**：锁定后，追踪止损从盈亏平衡点开始计算
- ✅ **与固定止盈**：锁定阈值应低于固定止盈目标

**推荐配置**：
- 保守型：3% 锁定
- 平衡型：5% 锁定
- 激进型：8% 锁定

---

## 参数优化建议

### 1. ATR 止盈倍数调整

**当前**：2.5 - 5.0
**建议**：2.5 - 6.0

**理由**：
- 提高最大盈亏比
- 给予趋势更多发展空间
- 在强趋势市场中获取更大利润

---

### 2. 分批止盈比例优化

**当前**：30% + 30% + 40%
**建议**：25% + 25% + 50%

**理由**：
- 前两次平仓减少比例，保留更多持仓
- 最后一次平仓增加比例，确保大部分利润锁定
- 更好地平衡风险和收益

---

### 3. 追踪止损层级细化

**当前**：3 个层级
**建议**：4-5 个层级

**示例**：
```json
[
  { "profit_threshold": 2, "trailing_percent": 1.5 },
  { "profit_threshold": 5, "trailing_percent": 2.5 },
  { "profit_threshold": 10, "trailing_percent": 4 },
  { "profit_threshold": 15, "trailing_percent": 5.5 },
  { "profit_threshold": 25, "trailing_percent": 8 }
]
```

---

## 最佳实践

### 1. 短线交易（日内）

```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "trigger_logic": "any",
    "initial_stop_percent": 2,
    "trailing_enabled": true,
    "trailing_levels": [
      { "profit_threshold": 1, "trailing_percent": 0.8 },
      { "profit_threshold": 2, "trailing_percent": 1.2 },
      { "profit_threshold": 3, "trailing_percent": 1.8 }
    ],
    "atr_enabled": true,
    "atr_multiplier_min": 1.5,
    "atr_multiplier_max": 2.5,
    "atr_period_btc_eth": 14,
    "atr_period_altcoin": 10
  },
  "dynamic_take_profit": {
    "enabled": true,
    "fixed_enabled": true,
    "fixed_percent": 5,
    "lock_profit_percent": 3
  }
}
```

---

### 2. 中线交易（数天）

```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "trigger_logic": "any",
    "initial_stop_percent": 3,
    "trailing_enabled": true,
    "trailing_levels": [
      { "profit_threshold": 2, "trailing_percent": 1.5 },
      { "profit_threshold": 5, "trailing_percent": 2.5 },
      { "profit_threshold": 10, "trailing_percent": 4 }
    ],
    "atr_enabled": true,
    "atr_multiplier_min": 1.5,
    "atr_multiplier_max": 3.5,
    "atr_period_btc_eth": 20,
    "atr_period_altcoin": 14
  },
  "dynamic_take_profit": {
    "enabled": true,
    "scaled_enabled": true,
    "scaled_levels": [
      { "profit_percent": 3, "close_percent": 30, "move_stop_to_breakeven": false },
      { "profit_percent": 6, "close_percent": 30, "move_stop_to_breakeven": true },
      { "profit_percent": 10, "close_percent": 40, "move_stop_to_breakeven": false }
    ],
    "lock_profit_percent": 5
  }
}
```

---

### 3. 长线交易（数周）

```json
{
  "dynamic_stop_loss": {
    "enabled": true,
    "trigger_logic": "all",
    "initial_stop_percent": 5,
    "trailing_enabled": true,
    "trailing_levels": [
      { "profit_threshold": 5, "trailing_percent": 3 },
      { "profit_threshold": 10, "trailing_percent": 5 },
      { "profit_threshold": 20, "trailing_percent": 8 }
    ],
    "atr_enabled": true,
    "atr_multiplier_min": 2.0,
    "atr_multiplier_max": 4.5,
    "atr_period_btc_eth": 25,
    "atr_period_altcoin": 18
  },
  "dynamic_take_profit": {
    "enabled": true,
    "atr_enabled": true,
    "atr_multiplier_min": 3.0,
    "atr_multiplier_max": 7.0,
    "atr_period_btc_eth": 25,
    "atr_period_altcoin": 18,
    "lock_profit_percent": 8
  }
}
```

---

## 总结

### 优化要点

1. ✅ **ATR 参数统一**：止损和止盈使用相同的 ATR 周期
2. ✅ **盈亏比合理**：确保止盈距离 > 止损距离
3. ⚠️ **模式互斥**：固定止盈和分批止盈建议只启用一个
4. ✅ **分层设计**：追踪止损和分批止盈都采用分层模式
5. ✅ **锁定机制**：在合适的盈利水平锁定利润

### 建议调整

1. **ATR 止盈最大倍数**：从 5.0 提高到 6.0
2. **分批止盈比例**：调整为 25% + 25% + 50%
3. **UI 提示**：提示用户固定止盈和分批止盈的互斥关系

### 实现优先级

1. **高优先级**：ATR 参数统一、盈亏比验证
2. **中优先级**：分批止盈与追踪止损的协调
3. **低优先级**：UI 优化、参数微调

这套设计在逻辑上是一致的，参数设置是合理的，能够适应不同的交易风格和市场环境。

