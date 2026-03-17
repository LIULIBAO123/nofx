# 过滤机制、方向池与 AI 实时+预测分析：漏洞与优化分析

针对「两单亏损、方向选错」现象，从**过滤机制**、**方向池**、**AI 实时+预测**三条链路分析可能漏洞，并给出可落地的优化建议。配置以当前截图为准：最低强度 40%、单侧归属、按强度排序、仅 Layer2 达标进池、强度平滑（权重 0.7，强度/可靠度加成上限 10/8）。

---

## 一、当前链路简述

1. **过滤机制**：Layer1 条件过滤 → Layer2 因子数/可靠度/入场信心 → 得到 `afterLayer2` 与 `ToSubmitSymbols`。
2. **方向池**：在 `afterLayer1`（或按配置用 Layer2 再滤）上跑 `buildDirectionPools`，得到多池/空池（含 StrengthPct、ReliabilityPct）；可选单侧归属、按强度排序、强度平滑与加成上限。
3. **AI 实时+预测**：同一周期内 pipeline 状态（含方向池）与行情等传给 AI；AI 输出 `symbol_predictions`（predicted_direction、suggest_open、confidence）。
4. **开仓**：三条件共振（① 该币在对应方向池 ② AI 预测方向与池一致 ③ suggest_open=true）+ confidence ≥ minConfidence；执行前再经 `EffectiveOpenConstraints`（regime 提高置信度、极端资金费率/多空比禁止开多/开空）。

---

## 二、漏洞与风险点

### 2.1 方向池：最低强度 40% 易放进「弱趋势」

- **逻辑**：`buildOneDirectionItem` 中，当多周期对齐与动量一般时会给 `StrengthPct=50`、`MarketCondition=weak_trend`；再经 `applyDirectionBonus` 加成（上限 10），可得 50+10=60。
- **现状**：最低强度 40% 时，弱趋势（50～60）仍能进池；单侧归属只比较多空两侧强度，取高的一侧，**两侧都弱**（如 42 vs 45）也会进某一侧池。
- **影响**：方向池中存在「强度不高、趋势不清晰」的标的，一旦 AI 也给出 suggest_open，就会参与开仓，容易在震荡或假突破中选错方向。

**建议**：  
- 将「最低强度」提高到 **50%～55%**，弱趋势+加成上限 60 仅勉强进池或无法进池；或  
- 在方向池构建时对 `MarketCondition == "weak_trend"` 的标的**禁止进池**（或仅作展示、不参与开仓列表）。

---

### 2.2 开仓时无「池内最低强度」门槛

- **逻辑**：`GetSystemEntryListFromPrediction` 仅要求 `longStrength[n] > 0` 或 `shortStrength[n] > 0`，即只要在池内即可参与三条件共振，再按强度排序决定顺序。
- **现状**：强度 40% 与 85% 的标的在「能否开仓」上等价，仅排序不同；若多标的同时共振，仍可能先开到强度偏低的。
- **影响**：在「最低强度 40%」下，弱信号仍能被实际执行，放大方向选错概率。

**建议**：  
- 增加**开仓最低强度**（如 55%）：只有 `StrengthPct >= 开仓最低强度` 的标的才进入 `GetSystemEntryListFromPrediction` 的返回列表；或  
- 在**执行开仓循环**中增加校验：从 state 中取该标的在对应方向池的 StrengthPct，低于阈值则跳过，与现有 regime/极端规则并列。

---

### 2.3 多周期「方向」判定过松（1h/4h 震荡当趋势）

- **逻辑**：`formatMarketData` 中 `dirThreshold = 0.15`（0.15%），即 1h/4h 涨跌超过 ±0.15% 即判为 up/down，否则 ranging。
- **现状**：0.2% 这类波动在震荡市很常见，会被判为「有方向」，方向池与 AI 都容易在噪音上建多/空观点。
- **影响**：震荡市中被误判为「有趋势」的标的进入方向池并得到 AI suggest_open，导致「方向选错」。

**建议**：  
- 将方向判定的幅度阈值提高到 **0.3%～0.5%**（或做成可配置），减少噪音被当成趋势；  
- 或在方向池/提示词中明确：**仅当 4h 与 1h 同向且幅度均超过某阈值时，方向信号才更可靠**，AI 在 ranging 或幅度不足时应对 suggest_open 更保守。

---

### 2.4 强度平滑导致「反转后仍显示高强度」

- **逻辑**：`applyStrengthSmoothing` 用 `smoothed = weight×上次 + (1-weight)×本次`（如 weight=0.7），缓释周期间抖动。
- **现状**：趋势反转后，强度已下降，但平滑会延迟 1～2 周期才明显反映，这段时间内该标的仍可能排在前面并被开仓。
- **影响**：在拐点附近用「滞后强度」开仓，容易开在错误方向。

**建议**：  
- 当 AI 输出 `market_regime` 为 **ranging / high_volatility / reversal** 时，可**降低平滑权重**（如 0.5），使强度更快反映当前周期；或  
- 在开仓逻辑中引入「强度变化」：若某标的**连续 2 周期 StrengthPct 下降**，可降低其开仓优先级或暂不开仓（需在 pipeline 中保留上一周期强度）。

---

### 2.5 Layer2 门槛可能仍放过边缘标的

- **逻辑**：仅 Layer2 达标进池时，要求 `factorsPassed >= MinFactors`、`reliability >= ReliabilityThreshold`、`entryConfPct >= EntryConfidenceThresholdPct`；默认 6 因子、0.67、31%。
- **现状**：6 因子、67% 可靠度、31% 入场信心在边界上的标的仍能进池，与「强趋势」标的混在一起，没有在开仓侧再做区分。
- **影响**：边缘标的进入方向池并参与三条件共振，在方向不明时更容易亏损。

**建议**：  
- 在「方向选错」多发的阶段，可**适当提高 Layer2 门槛**（如 MinFactors 7、Reliability 0.72、EntryConfidence 35%），并在前端/文档说明「方向错误多时可收紧 Layer2」；  
- 或增加「仅当 Layer2 可靠度/入场信心超过更高一档（如 0.75/40%）时，才允许参与开仓」的二次过滤（与方向池最低强度、开仓最低强度配合）。

---

### 2.6 AI suggest_open 在震荡/反转下仍可能偏多

- **逻辑**：Prompt 要求「仅当预测方向与系统方向池一致且置信度≥策略要求时 suggest_open=true」；执行侧用 `EffectiveOpenConstraints` 按 regime 提高置信度（如 ranging→75）。
- **现状**：AI 若在 ranging/reversal 下仍对多个标的设 suggest_open=true 且 confidence 略高于基础门槛，会先进入列表，再在执行时被 regime 提高门槛挡掉一部分，但**列表生成阶段并未显式考虑 regime**。
- **影响**：震荡/反转市中，AI 建议开仓的「数量」可能偏多，依赖执行层 regime 抬门槛做最后防线；若 regime 识别不准或未配置 RegimeMinConfidenceMap，仍可能开错方向。

**建议**：  
- 在 **Prompt 中显式增加**：当 `market_regime` 为 **ranging / high_volatility / reversal** 时，应更保守，**仅对置信度明显高于基础门槛**（如 ≥75 或 ≥80）且趋势证据充分的标的设 `suggest_open=true`，其余设为 false；  
- 确保策略已开启 **RegimeAdjustEnabled** 并配置 **RegimeMinConfidenceMap**（如 ranging: 75, high_volatility: 78, reversal: 80），与提示词一致。

---

### 2.7 单侧归属不保证「绝对强度高」

- **逻辑**：单侧归属只保证「每标的只进多或空一侧，且进强度更高的一侧」。  
- **现状**：若多空两侧强度分别为 42 与 45，该标的仍会进空池（45）；在「最低强度 40%」下合规，但 45 仍是弱信号。  
- **影响**：与 2.1、2.2 叠加，弱信号仍能进池并被开仓。

**建议**：  
与 2.1、2.2 一致：提高「进池最低强度」和「开仓最低强度」，使单侧归属只在高强度一侧中做选择，而不是在「弱 vs 弱」中选一个。

---

## 三、优化项汇总与优先级

| 优先级 | 优化项 | 说明 | 实现位置建议 |
|--------|--------|------|--------------|
| 高 | 提高方向池最低强度 | 40% → 50%～55%，减少弱趋势进池 | 前端/配置：DirectionPool.MinStrengthPct |
| 高 | 开仓最低强度 | 仅当方向池 StrengthPct ≥ 阈值（如 55%）才允许开仓 | kernel.GetSystemEntryListFromPrediction 或执行循环中读取 state 池强度做过滤 |
| 高 | 多周期方向阈值 | 0.15% → 0.3%～0.5%，减少震荡当趋势 | kernel formatMarketData 的 dirThreshold，或可配置 |
| 中 | 震荡/反转时 AI 更保守 | Prompt 中明确 ranging/reversal 时少设 suggest_open、提高置信度要求 | kernel BuildUserPrompt / 分析行为规范 |
| 中 | Regime 与 RegimeMinConfidenceMap | 确保开启并按 regime 提高置信度 | 策略配置 + EffectiveOpenConstraints（已有） |
| 中 | Layer2 门槛可调严 | MinFactors/Reliability/EntryConfidence 适当提高 | 前端/配置：Layer2 |
| 低 | 强度平滑在震荡下减弱 | regime 为 ranging 时降低平滑权重或减少滞后 | pipeline applyStrengthSmoothing，需传入 regime |
| 低 | 弱趋势禁止进池 | MarketCondition==weak_trend 时不加入方向池 | kernel buildDirectionPools |

---

## 四、小结

- **方向选错**往往来自：**弱趋势/震荡被当成趋势** + **方向池与开仓门槛偏低** + **AI 在震荡市仍给出较多 suggest_open**。  
- 优先建议：**提高方向池最低强度（50%～55%）**、**增加开仓最低强度（如 55%）**、**提高 1h/4h 方向判定阈值（0.3%～0.5%）**，并在提示词中明确**震荡/反转市下更保守的 suggest_open 规则**。  
- 现有 **EffectiveOpenConstraints**（regime 提门槛、极端资金费率/多空比禁开多/开空）已能挡掉一部分错误方向，建议与上述方向池与 AI 侧优化一起使用，形成多层过滤，减少「两单亏损、方向选错」的重复发生。
