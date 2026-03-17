# 系统根据 AI 分析动态调整参数

## 结论：可以，且已在用

系统**已经**会根据 AI 的分析结果动态调整部分参数；可以在此基础上继续扩展。

---

## 一、当前已实现的动态调整

| AI 输出 | 系统如何动态调整 |
|--------|------------------|
| **market_regime** | 开仓最低置信度：震荡/高波/反转时按 `RegimeMinConfidenceMap` 提高 MinConfidence（见 `EffectiveOpenConstraints`）。 |
| **scenario** | 止损确认：当 `scenario=reversal` 且开启 `ScenarioAdjustEnabled` 时，减少 SL 确认周期，加快止损。 |
| **trend_view**（持仓） | 止损确认：`reversing` 时少确认一次，`trend_intact`/`choppy` 时多确认一次，减少震荡洗盘。 |
| **risk_alert** | 本周期不执行系统开仓（系统开仓/仅预测模式下若 `risk_alert=true` 则整周期不新开仓）。 |

以上均在代码中实现，无需改配置即可生效（部分依赖策略里是否开启 RegimeAdjustEnabled、ScenarioAdjustEnabled 等）。

---

## 二、可扩展的调整方向

在现有基础上，可以继续用 AI 分析做更多「动态参数」而不改策略预设本身，例如：

| AI 输出 | 可做的动态调整 | 说明 |
|--------|----------------|------|
| **risk_alert** | 本周期开仓仓位比例 × 0.5 | 已有「不开仓」；可改为「仍可开但仓位减半」。 |
| **market_regime** | 按 regime 选用不同仓位比例或 SL 档位 | 如 ranging → 用更保守的仓位比例或 sl_tight。 |
| **scenario** | 高波/反转时临时加大 ATR 倍数或确认周期 | 减少噪音触发。 |
| **near_term_outlook** / **key_levels** | 用于动态调整止盈/止损关键位 | 若 AI 给出关键位，可作 TP/SL 的参考边界（需定义规则）。 |

实现方式可以是：在「系统开仓」或「系统 TP/SL」逻辑里，根据本周期 AI 的 `FullDecision`（MarketRegime、Scenario、RiskAlert 等）计算本周期用的**有效参数**（如 effectivePositionRatio、effectiveConfirmCycles），再交给现有开仓/止盈止损逻辑使用。

---

## 三、小结

- **可以**：系统完全可以根据 AI 的分析动态调整参数。
- **已经在做**：regime → MinConfidence；scenario → SL 确认；trend_view → SL 确认；risk_alert → 是否系统开仓。
- **可继续做**：用 risk_alert / regime / scenario 等调节仓位比例、SL 松紧、确认周期等，在现有约束与风控框架内扩展即可。
