# 做多/做空与止盈止损 · AI 实时+预测 提示词与输出结构

本文档说明：**做多/做空**（开仓方向）与**止盈/止损**（持仓期参数调节）相关的 **AI 提示词** 和 **输出结构**，对应「AI 实时+预测」与「止盈止损周期」两套用法。

---

## 一、主周期：做多/做空 + 可选止盈止损调节

### 1.1 提示词来源与位置

- **文件**：`kernel/engine.go`
- **逻辑**：在 `BuildSystemPrompt` 中，根据 `RiskControl.AIPredictOnly` / `SystemExecutesEntry` 写入不同约束；在「Output Format」的 **Optional <analysis> / <outlook> block** 中统一说明 `<analysis>` 的 JSON 字段与示例。

### 1.2 做多/做空相关约束（AIPredictOnly 时）

**中文提示词要点：**

- **【AI 仅预测】**：禁止输出 `open_long`、`open_short`、`close_long`、`close_short`；仅可输出 `hold` 或 `wait`。开平仓完全由系统根据预测信息（`market_regime`、`scenario`、**`symbol_predictions`** 等）与规则执行。
- **必须在 `<analysis>` 中输出 `symbol_predictions` 数组**，格式见下。

**英文对应：**

- **[AI predict only]** You must NOT output open_long, open_short, close_long, or close_short. Only output hold or wait. All entry/exit is decided by the system from your prediction (market_regime, scenario, **symbol_predictions**, etc.). You **MUST** output **symbol_predictions** in &lt;analysis&gt;; see format below.

### 1.3 止盈/止损调节相关提示词（主周期可选）

在「Optional <analysis> / <outlook> block」中，中英文均有以下说明（大意一致）：

- **可选 `position_sl_tp_adjustments`**：持仓期间按规则建议止盈/止损**参数调节**（系统在边界内应用，AI 不直接平仓）。
- **五条规则**：
  1. **choppy_hold 震荡持仓**：避免震荡被洗出 → `trail_aggressiveness=low` 或 `confirm_cycles_delta=+1`
  2. **trend_ride 抓住趋势**：趋势延续 → `trail_aggressiveness=high`
  3. **lock_profit 锁住利润**：已有浮盈 → `lock_profit_pct=2～3`
  4. **trend_weakening 趋势减弱**：动能减弱/拐点风险 → 收紧或 `lock_profit_pct` 略降，可选 `confirm_cycles_delta=-1`
  5. **high_vol_hold 高波动持有**：波动率突升但方向未反 → `confirm_cycles_delta=+1` 或 `atr_mult_sl` 用上限
- **每项字段**：`symbol`、`side`、`advice`、`trail_aggressiveness`、`atr_mult_sl`、`lock_profit_pct`、`confirm_cycles_delta`

### 1.4 主周期 <analysis> 输出结构

AI 在 `<decision>` 之后可输出 **`<analysis>`**（或 `<outlook>`），内为 **一个 JSON 对象**。解析时从 `<analysis>...</analysis>` 中提取该 JSON（见 `ParseAnalysisFromRawResponse`）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `market_summary` | string | 本周期市场摘要，一两句话 |
| `market_regime` | string | 市场状态：trend_up / trend_down / ranging / high_volatility / reversal |
| `risk_alert` | boolean | true 表示建议本周期不新开仓 |
| `near_term_outlook` | string | 未来 1～2 根 K 或本 session 的整体预期 |
| `scenario` | string | continuation / reversal / range 等，用于辅助止盈止损与持仓决策 |
| `key_levels` | string[] | 1～2 个关键支撑/阻力或目标价位 |
| **`symbol_predictions`** | **array** | **（AIPredictOnly 时必填）按标的的预测，系统据此做多/做空与动态参数** |
| **`position_sl_tp_adjustments`** | **array** | **（可选）持仓止盈/止损参数调节建议，系统在边界内应用** |

**`symbol_predictions` 单项结构：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `symbol` | string | 标的，如 BTCUSDT |
| `predicted_direction` | string | up / down / neutral |
| `confidence` | int | 0–100 |
| `suggest_exit` | boolean | 该持仓是否建议平仓（系统可参考，实际平仓仍由系统 SL/TP 执行） |

**示例（来自代码内提示）：**

```json
{
  "market_summary": "...",
  "market_regime": "trend_down",
  "risk_alert": false,
  "scenario": "continuation",
  "key_levels": ["88000"],
  "symbol_predictions": [
    { "symbol": "BTCUSDT", "predicted_direction": "down", "confidence": 75, "suggest_exit": false },
    { "symbol": "ETHUSDT", "predicted_direction": "up", "confidence": 70, "suggest_exit": false }
  ]
}
```

**`position_sl_tp_adjustments` 单项结构（与 kernel 结构体一致）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `symbol` | string | 标的 |
| `side` | string | long / short |
| `advice` | string | choppy_hold / trend_ride / lock_profit / trend_weakening / high_vol_hold |
| `trail_aggressiveness` | string | low / medium / high |
| `atr_mult_sl` | number | 建议 ATR 止损乘数，系统裁剪到策略 [min,max] |
| `lock_profit_pct` | number | 建议达到该盈利%后锁利（移动止损到保本），系统限制 1–5 |
| `confirm_cycles_delta` | int | 在基础 ConfirmCycles 上加减，系统限制最终 1–3 |

---

## 二、止盈止损专用周期（仅 SL/TP 调节）

### 2.1 用途

- **不做开平仓决策**，不输出 `symbol_predictions` 或开仓动作。
- 仅对**当前持仓**做「实时+预测」的轻量分析，输出 **`position_sl_tp_adjustments`**，供系统在边界内应用（见 `RunSLTPOnlyAnalysis`、`BuildSLTPOnlyPrompts`）。

### 2.2 提示词（BuildSLTPOnlyPrompts）

**System prompt（英文，固定）：**

```
You are a risk assistant. Output ONLY a JSON object inside <analysis></analysis>. No opening/closing decisions.
Rules for position_sl_tp_adjustments (one per position):
- choppy_hold: price choppy → loosen (trail_aggressiveness=low or confirm_cycles_delta=+1)
- trend_ride: trend intact → tighten (trail_aggressiveness=high)
- lock_profit: in profit → suggest lock_profit_pct 2~3
- trend_weakening: momentum fading → tighten or lower lock_profit_pct
- high_vol_hold: volatility spike but direction ok → confirm_cycles_delta=+1 or atr_mult_sl at upper bound
Output format: <analysis>{"position_sl_tp_adjustments":[{"symbol":"X","side":"long","advice":"choppy_hold","trail_aggressiveness":"low",...}]}</analysis>
```

**User prompt（动态）：**

- 当前持仓列表（仅用于 SL/TP 调节）：每行 `- {symbol} {side} entry=... mark=... pnl_pct=... hold_mins=... atr=...`
- 末尾：`Last regime=... scenario=... Output only <analysis> with position_sl_tp_adjustments.`

### 2.3 输出结构（止盈止损专用周期）

- 只解析 **`<analysis>...</analysis>`** 中的 JSON。
- JSON 中只需包含 **`position_sl_tp_adjustments`** 数组，结构与上表一致；无 `symbol_predictions`、`market_regime` 等（regime/scenario 仅作为上下文传入 user prompt）。

解析函数：`ParsePositionSLTPAdjustmentsFromRawResponse`，仅取 `position_sl_tp_adjustments` 数组。

---

## 三、小结

| 场景 | 提示词位置 | 做多/做空 | 止盈止损调节 | 输出结构 |
|------|------------|-----------|--------------|----------|
| **主 AI 周期** | `engine.BuildSystemPrompt` + Output Format 的 <analysis> 说明 | 通过 **symbol_predictions**（AIPredictOnly 时必填）由系统决定开仓方向与是否开仓 | 可选 **position_sl_tp_adjustments**，五条规则（choppy_hold / trend_ride / lock_profit / trend_weakening / high_vol_hold） | `<analysis>` 内 JSON：market_regime, scenario, symbol_predictions, position_sl_tp_adjustments 等 |
| **止盈止损专用周期** | `kernel.BuildSLTPOnlyPrompts` | 不涉及 | 仅输出 **position_sl_tp_adjustments**（每仓一条） | `<analysis>` 内 JSON：仅 **position_sl_tp_adjustments** 数组 |

做多/做空由 **symbol_predictions** 的 `predicted_direction`（up/down/neutral）与 `confidence` 驱动系统开仓逻辑；止盈/止损的**执行**仍由系统按策略与 30 秒检查完成，AI 只通过 **position_sl_tp_adjustments** 建议参数（追踪强度、ATR 倍数、锁利%、确认周期增减），系统在边界内应用后按调节后的条件执行。
