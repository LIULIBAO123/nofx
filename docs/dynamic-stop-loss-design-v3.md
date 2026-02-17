# 动态止损设计文档 v3.0

## 概述

本文档描述了 nofx 交易系统的动态止损功能设计，包含分层追踪止损和动态 ATR 止损的智能化改进。

## 设计目标

1. **分层追踪止损**：根据盈利水平动态调整允许的回撤幅度
2. **动态 ATR 止损**：AI 根据市场波动在设定区间内自适应选择止损倍数
3. **币种差异化**：主流币和山寨币使用不同的 ATR 周期参数
4. **美观的用户界面**：提供直观、现代化的配置界面

## 核心功能

### 1. 初始固定止损（必需）

作为保底止损机制，无论其他止损模式是否启用，都会生效。

- **参数**：`initial_stop_percent`
- **范围**：0.5% - 10%
- **默认值**：3%
- **说明**：开仓时的固定止损距离，防止极端情况下的大额亏损

### 2. 追踪止损 - 分层模式

根据盈利水平设置不同的追踪止损百分比，盈利越高，允许的回撤越大。

#### 设计理念

- **低盈利阶段**：使用较紧的止损，快速锁定利润
- **中盈利阶段**：适度放宽止损，给予价格波动空间
- **高盈利阶段**：使用较宽的止损，让利润充分奔跑

#### 参数结构

```typescript
interface TrailingStopLevel {
  profit_threshold: number;  // 盈利阈值（%）
  trailing_percent: number;  // 该层级的追踪止损百分比（%）
}
```

#### 默认配置示例

```json
{
  "trailing_enabled": true,
  "trailing_levels": [
    { "profit_threshold": 2, "trailing_percent": 1.5 },   // 盈利2%时，允许1.5%回撤
    { "profit_threshold": 5, "trailing_percent": 2.5 },   // 盈利5%时，允许2.5%回撤
    { "profit_threshold": 10, "trailing_percent": 4 }     // 盈利10%时，允许4%回撤
  ]
}
```

#### 工作逻辑

1. 系统持续监控当前盈利百分比
2. 根据盈利水平匹配对应的层级（选择最高的已达到阈值的层级）
3. 使用该层级的 `trailing_percent` 作为追踪止损距离
4. 当价格从最高点回撤超过该百分比时触发止损

#### 用户界面特性

- 支持添加/删除层级（最少保留1个层级）
- 每个层级独立配置盈利阈值和回撤百分比
- 可视化滑块调节，实时显示数值
- 层级按盈利阈值自动排序显示

### 3. ATR 止损 - 动态区间模式

AI 根据市场波动状况，在设定的倍数区间内自适应选择合适的 ATR 倍数。

#### 设计理念

- **低波动市场**：使用较小的 ATR 倍数，避免止损距离过大
- **高波动市场**：使用较大的 ATR 倍数，避免被正常波动扫出
- **币种差异**：主流币和山寨币使用不同的 ATR 周期

#### 参数说明

| 参数 | 说明 | 默认值 | 范围 |
|------|------|--------|------|
| `atr_multiplier_min` | ATR 倍数最小值 | 1.5 | 0.5 - 5.0 |
| `atr_multiplier_max` | ATR 倍数最大值 | 3.5 | 0.5 - 5.0 |
| `atr_period_btc_eth` | BTC/ETH 的 ATR 周期 | 20 | 5 - 50 |
| `atr_period_altcoin` | 山寨币的 ATR 周期 | 14 | 5 - 50 |

#### 配置示例

```json
{
  "atr_enabled": true,
  "atr_multiplier_min": 1.5,
  "atr_multiplier_max": 3.5,
  "atr_period_btc_eth": 20,
  "atr_period_altcoin": 14
}
```

#### 工作逻辑

1. **币种识别**：
   - BTC/ETH：使用 `atr_period_btc_eth`
   - 其他币种：使用 `atr_period_altcoin`

2. **ATR 计算**：
   - 根据币种选择对应的周期计算 ATR 值

3. **倍数选择**（AI 决策）：
   - 分析当前市场波动率
   - 在 `[atr_multiplier_min, atr_multiplier_max]` 区间内选择合适倍数
   - 考虑因素：近期波动、趋势强度、市场情绪等

4. **止损距离**：
   - `stop_distance = ATR × selected_multiplier`

#### 推荐配置

**保守型**：
- 倍数区间：1.5 - 2.5
- BTC/ETH 周期：20
- 山寨币周期：14

**平衡型**（默认）：
- 倍数区间：1.5 - 3.5
- BTC/ETH 周期：20
- 山寨币周期：14

**激进型**：
- 倍数区间：2.0 - 4.5
- BTC/ETH 周期：25
- 山寨币周期：18

### 4. 支撑阻力止损

在识别的支撑位下方设置止损，带有缓冲距离。

#### 参数说明

- **参数**：`support_resistance_buffer`
- **范围**：0.1% - 2%
- **默认值**：0.5%
- **说明**：在支撑位基础上额外留出的缓冲空间

#### 工作逻辑

1. AI 识别关键支撑位
2. 止损价格 = 支撑位 × (1 - buffer%)
3. 当价格跌破止损价格时触发

### 5. 触发逻辑

用户可选择多个止损模式同时启用时的触发方式：

#### 任一条件触发（`any`）

- 任何一个启用的止损条件触发即平仓
- **适用场景**：保守型交易者，优先保护资金
- **优点**：更快止损，减少损失
- **缺点**：可能过早退出

#### 所有条件触发（`all`）

- 所有启用的止损条件都触发才平仓
- **适用场景**：激进型交易者，给予更多空间
- **优点**：避免被单一指标误导
- **缺点**：可能承受更大回撤

## 数据结构

### TypeScript 类型定义

```typescript
// 动态止损配置
export interface DynamicStopLossConfig {
  enabled: boolean;
  trigger_logic: 'any' | 'all';
  
  // 初始固定止损（必需）
  initial_stop_percent: number;
  
  // 追踪止损 - 分层模式
  trailing_enabled?: boolean;
  trailing_levels?: TrailingStopLevel[];
  
  // ATR 止损 - 动态区间模式
  atr_enabled?: boolean;
  atr_multiplier_min?: number;
  atr_multiplier_max?: number;
  atr_period_btc_eth?: number;
  atr_period_altcoin?: number;
  
  // 支撑阻力止损
  support_resistance_enabled?: boolean;
  support_resistance_buffer?: number;
}

// 追踪止损层级
export interface TrailingStopLevel {
  profit_threshold: number;   // 盈利阈值
  trailing_percent: number;   // 允许回撤百分比
}
```

### Go 结构体定义

```go
// DynamicStopLossConfig dynamic stop loss configuration
type DynamicStopLossConfig struct {
	Enabled      bool   `json:"enabled"`
	TriggerLogic string `json:"trigger_logic"`

	// Initial fixed stop loss (required)
	InitialStopPercent float64 `json:"initial_stop_percent"`

	// Trailing Stop - Tiered Mode
	TrailingEnabled *bool                `json:"trailing_enabled,omitempty"`
	TrailingLevels  []TrailingStopLevel  `json:"trailing_levels,omitempty"`

	// ATR Stop - Dynamic Range Mode
	ATREnabled        *bool    `json:"atr_enabled,omitempty"`
	ATRMultiplierMin  *float64 `json:"atr_multiplier_min,omitempty"`
	ATRMultiplierMax  *float64 `json:"atr_multiplier_max,omitempty"`
	ATRPeriodBTCETH   *int     `json:"atr_period_btc_eth,omitempty"`
	ATRPeriodAltcoin  *int     `json:"atr_period_altcoin,omitempty"`

	// Support/Resistance Stop
	SupportResistanceEnabled *bool    `json:"support_resistance_enabled,omitempty"`
	SupportResistanceBuffer  *float64 `json:"support_resistance_buffer,omitempty"`
}

// TrailingStopLevel trailing stop level configuration
type TrailingStopLevel struct {
	ProfitThreshold float64 `json:"profit_threshold"`
	TrailingPercent float64 `json:"trailing_percent"`
}
```

## UI 设计特性

### 视觉设计

1. **渐变背景**：使用深色渐变背景增强层次感
2. **图标系统**：每个功能模块配有专属图标
3. **颜色编码**：
   - 红色：止损相关（#F6465D）
   - 绿色：盈利相关（#0ECB81）
   - 黄色：警告/配置（#F0B90B）
   - 灰色：禁用状态（#848E9C）

4. **交互反馈**：
   - 滑块实时显示数值
   - 启用/禁用状态清晰可见
   - 悬停效果和过渡动画

### 布局结构

1. **总开关**：顶部显著位置，控制整个动态止损功能
2. **触发逻辑**：两个选项卡式按钮，清晰展示选择
3. **初始止损**：独立区块，强调其必需性
4. **各止损模式**：可折叠区块，启用时展开详细配置
5. **分层配置**：追踪止损支持动态添加/删除层级

### 响应式设计

- 适配不同屏幕尺寸
- 移动端优化触摸交互
- 保持配置项的可读性和可操作性

## 实现要点

### 前端实现

1. **状态管理**：
   - 使用受控组件管理配置状态
   - 实时同步到父组件

2. **表单验证**：
   - ATR 倍数最小值不能大于最大值
   - 追踪止损层级按盈利阈值排序
   - 数值范围限制

3. **用户体验**：
   - 滑块和数字输入双向绑定
   - 添加层级时自动计算合理的默认值
   - 删除层级时确认操作

### 后端实现

1. **配置验证**：
   - 检查必需字段
   - 验证数值范围
   - 确保逻辑一致性

2. **止损检查逻辑**：
   ```go
   func CheckStopLoss(config *DynamicStopLossConfig, position *Position) bool {
       if !config.Enabled {
           return false
       }
       
       triggers := []bool{}
       
       // 检查初始固定止损
       if position.Loss >= config.InitialStopPercent {
           triggers = append(triggers, true)
       }
       
       // 检查追踪止损
       if config.TrailingEnabled != nil && *config.TrailingEnabled {
           if shouldTriggerTrailing(config, position) {
               triggers = append(triggers, true)
           }
       }
       
       // 检查 ATR 止损
       if config.ATREnabled != nil && *config.ATREnabled {
           if shouldTriggerATR(config, position) {
               triggers = append(triggers, true)
           }
       }
       
       // 检查支撑阻力止损
       if config.SupportResistanceEnabled != nil && *config.SupportResistanceEnabled {
           if shouldTriggerSR(config, position) {
               triggers = append(triggers, true)
           }
       }
       
       // 根据触发逻辑判断
       if config.TriggerLogic == "any" {
           return containsTrue(triggers)
       } else {
           return allTrue(triggers)
       }
   }
   ```

3. **AI 集成**：
   - 在 AI 决策时传递 ATR 倍数区间
   - AI 根据市场分析选择合适的倍数
   - 记录 AI 的选择以供后续分析

## 使用示例

### 保守型配置

```json
{
  "enabled": true,
  "trigger_logic": "any",
  "initial_stop_percent": 2,
  "trailing_enabled": true,
  "trailing_levels": [
    { "profit_threshold": 1, "trailing_percent": 1 },
    { "profit_threshold": 3, "trailing_percent": 2 },
    { "profit_threshold": 5, "trailing_percent": 3 }
  ],
  "atr_enabled": true,
  "atr_multiplier_min": 1.5,
  "atr_multiplier_max": 2.5,
  "atr_period_btc_eth": 20,
  "atr_period_altcoin": 14,
  "support_resistance_enabled": true,
  "support_resistance_buffer": 0.5
}
```

### 激进型配置

```json
{
  "enabled": true,
  "trigger_logic": "all",
  "initial_stop_percent": 5,
  "trailing_enabled": true,
  "trailing_levels": [
    { "profit_threshold": 3, "trailing_percent": 2 },
    { "profit_threshold": 8, "trailing_percent": 4 },
    { "profit_threshold": 15, "trailing_percent": 6 }
  ],
  "atr_enabled": true,
  "atr_multiplier_min": 2.0,
  "atr_multiplier_max": 4.5,
  "atr_period_btc_eth": 25,
  "atr_period_altcoin": 18,
  "support_resistance_enabled": false
}
```

## 总结

本次升级将动态止损功能提升到了新的智能化水平：

1. **分层追踪止损**：根据盈利动态调整，让利润充分奔跑的同时保护已有收益
2. **动态 ATR 区间**：AI 自适应选择止损倍数，更好地应对市场变化
3. **币种差异化**：针对主流币和山寨币的不同特性优化参数
4. **美观的界面**：现代化的设计提升用户体验

这些改进使得止损策略更加灵活、智能，能够更好地适应不同的市场环境和交易风格。


