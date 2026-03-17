# 持仓止盈止损 · AI 调节设计（不直接平仓）

## 目标

- **现状**：止盈/止损在开仓时由策略配置 + 开仓时实时数据设定，持仓期间仅由系统按固定规则检查并执行。
- **需求**：让 AI 在持仓过程中根据「规则 + 实时 + 预测」**调整**止盈/止损参数；**仍由系统根据调整后的规则执行平仓**，AI 不直接发出平仓指令。

## 原则

1. **规则驱动**：给 AI 明确规则（震荡持仓、抓住趋势、锁住利润），AI 输出「建议调整」而非自由发挥。
2. **系统执行**：触发与下单一律由现有 `checkDynamicStopLossTakeProfit` + `executeStopLoss` / `executeTakeProfit` 完成；AI 只影响**生效的 SL/TP 参数**。
3. **边界约束**：所有 AI 建议在系统设定的上下界内裁剪，避免过度放宽或过度收紧。

---

## 规则约定（给 AI 的 prompt）

### 是否满足需求

- **3 个基础场景**（震荡持仓、抓住趋势、锁住利润）能覆盖大部分持仓状态，但以下情况建议补充或细化，使规则更可执行、可回溯。

### 建议：5 场景 + 字段语义细化

| 场景 | 规则含义 | 何时选用 | AI 建议方向（在系统边界内） |
|------|----------|----------|-----------------------------|
| **choppy_hold 震荡持仓** | 避免震荡中被洗出，拿住仓位 | 价格在区间内反复、无明确趋势、波动率未明显放大 | 放宽：`trail_aggressiveness=low`；或 `confirm_cycles_delta=+1`；`atr_mult_sl` 用策略上限 |
| **trend_ride 抓住趋势** | 趋势延续时让利润奔跑 | 方向与持仓一致、动量/结构未破坏、未到目标位 | 收紧：`trail_aggressiveness=high`；可不设或降低 `lock_profit_pct` |
| **lock_profit 锁住利润** | 已有浮盈后保护本金 | 浮盈达到或超过策略档位（如 ≥2%）、趋势存疑或波动加大 | `lock_profit_pct` 建议 2～3；可配合 `trail_aggressiveness=high` |
| **trend_weakening 趋势减弱** | 趋势动能减弱或拐点风险，优先兑现 | 动量减弱、背离、或接近关键阻力/支撑 | 收紧：`trail_aggressiveness=high`；`lock_profit_pct` 略降（如 1.5～2）；可选 `confirm_cycles_delta=-1`（在边界内） |
| **high_vol_hold 高波动持有** | 波动率突升但方向未反转，避免被噪音洗出 | ATR 明显放大、波动加剧但结构仍支持持仓方向 | 放宽：`confirm_cycles_delta=+1`；`atr_mult_sl` 用策略上限或略放宽 |

- **advice** 枚举建议：`choppy_hold` | `trend_ride` | `lock_profit` | `trend_weakening` | `high_vol_hold`，便于日志与回溯。
- **字段语义**（简要）：
  - **trail_aggressiveness**：low=放宽追踪（震荡/高波拿住）、high=收紧追踪（趋势/锁利）。
  - **atr_mult_sl**：仅当建议放宽或高波时使用策略允许上限；不建议随意缩小（避免过早止损）。
  - **lock_profit_pct**：仅在有浮盈时建议；趋势减弱时可略降以提前锁利。
  - **confirm_cycles_delta**：震荡/高波 +1；趋势减弱/反转风险可 -1（系统裁剪后 ≥1）。

AI 输出为**每持仓一条**的「建议调整」，而非具体平仓指令。

---

## 数据流

```
AI 周期 / 有缓存时:
  实时持仓 + 实时行情 + 预测(market_regime, scenario, symbol_predictions)
    → AI 按规则输出 position_sl_tp_adjustments[]
    → 系统解析并存储（按 posKey），带边界裁剪
    → checkDynamicStopLossTakeProfit 使用「基础配置 + 开仓 profile + AI 调整」得到 effSL/effTP
    → 系统照常检查价格 vs 规则，触发则系统执行平仓

系统周期 / 30s 后台（无新 AI）:
  使用**上一轮 AI 的调整缓存**（若有）参与 effSL/effTP；若无则仅用基础+profile。
  止盈/止损仍由系统执行，不依赖本周期是否调 AI。
```

---

## 独立「持仓止盈止损」分析周期（可选）

### 目的

- **主 AI 周期**（如 10 分钟）：完整决策（开仓候选、symbol_predictions、trend_view、position_sl_tp_adjustments 等），token 与算力占用大。
- **持仓止盈/止损**需要更频繁的「实时 + 预测」输入，以便在震荡/趋势/锁利之间及时调节参数。
- 若仅依赖 10 分钟一次的全量分析，SL/TP 参数更新频率偏低。

### 设计：单独 SL/TP 分析周期

- **独立间隔**：例如 2～5 分钟（可配置 `sltp_analysis_interval_minutes`；0 表示不启用，沿用主周期）。
- **分析内容仅面向止盈/止损**：
  - **输入**：当前持仓列表（标的、方向、入场价、市价、浮盈%、持仓时长）、各标的短周期 ATR/波动率、上一周期 regime/scenario（或本周期轻量预测）。
  - **不做**：不开仓候选列表、不输出 symbol_predictions 用于开仓、不输出开平仓动作。
- **输出**：仅 `position_sl_tp_adjustments`（及可选每仓 `trend_view`），供系统在边界内写入并参与下一次 SL/TP 检查。
- **执行**：独立调度（与主 AI 周期、系统周期并行）；到点只跑「SL/TP 轻量分析」一次调用，解析 `<analysis>` 中的调节建议并更新缓存；不写完整决策记录、不触发开仓逻辑。

### 与主周期的关系

- 主周期（10 分钟）：照常输出 position_sl_tp_adjustments，若存在则覆盖缓存。
- SL/TP 周期（如 3 分钟）：仅输出 position_sl_tp_adjustments（+ 可选 trend_view），仅更新同一缓存；系统 SL/TP 检查始终使用「最新一次写入的缓存」。
- 这样在 10 分钟主周期之间，仍有 2～3 次专门针对持仓的「实时 + 预测」分析，只消耗止盈/止损所需的数据与 token。

---

## AI 输出结构（建议）

在现有 `<analysis>` 中增加可选字段，例如：

```json
"position_sl_tp_adjustments": [
  {
    "symbol": "BTCUSDT",
    "side": "long",
    "advice": "choppy_hold",
    "trail_aggressiveness": "low",
    "atr_mult_sl": 2.0,
    "lock_profit_pct": 2.5,
    "confirm_cycles_delta": 1
  }
]
```

- `advice`: 枚举 `choppy_hold` | `trend_ride` | `lock_profit`，便于规则回溯与日志。
- `trail_aggressiveness`: 已有逻辑，可被 AI 覆盖（low/medium/high）。
- `atr_mult_sl`: 建议 ATR 止损乘数，系统裁剪到策略允许的 [min, max]。
- `lock_profit_pct`: 建议达到该盈利%后锁利（移动止损到保本），裁剪到策略允许范围。
- `confirm_cycles_delta`: 在基础 ConfirmCycles 上加减（震荡+1、反转-1），系统裁剪后生效。

所有字段可选；缺失则保持当前配置不变。

---

## 系统边界（示例）

- ATR 止损乘数：`[策略 ATRMultiplierMin, 策略 ATRMultiplierMax]`，不突破。
- 锁利阈值：`[1.0, 5.0]%` 或由策略配置。
- 确认周期 delta：最终 `ConfirmCycles + delta` 至少为 1，且不超过策略允许上限（如 3）。

---

## 实现要点

1. **Prompt**：在 system/user 中增加「持仓止盈止损调节规则」说明，要求 AI 在 `<analysis>` 中输出 `position_sl_tp_adjustments`，且仅给出上表范围内的建议。
2. **解析**：在 `ParseFullDecision` / 解析 `<analysis>` 时解析 `position_sl_tp_adjustments`，写入 AutoTrader 的 `positionSLTPAdjustment map[string]PositionSLTPAdjustment`（key = posKey）。
3. **应用**：在 `checkDynamicStopLossTakeProfit` 中，在现有「base + profile + trailAgg」之后，若存在该 posKey 的 adjustment，则对 effSL/effTP 的副本做覆盖（并做边界裁剪），再创建 StopLossChecker/TakeProfitChecker；无 adjustment 时行为与现有一致。
4. **缓存**：AI 周期更新 adjustment；系统周期与 30s 后台检查使用上次缓存的 adjustment（或按周期号/时间戳决定是否过期）。
5. **不新增 AI 平仓**：不启用或不经由 AI 的 close_long/close_short 执行平仓；平仓仅由「价格触及调整后的 SL/TP 规则」触发，由系统执行。

---

## 小结

- **可以实现**：AI 按规则输出「参数调整建议」，系统在边界内应用并继续用现有逻辑判断触发与执行，不让 AI 直接平仓。
- **效果**：震荡时更拿得住、趋势时更易锁利/抓趋势，同时风险仍由系统边界与既有 SL/TP 逻辑控制。
