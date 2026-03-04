# 持仓期间 AI 趋势判断与止损止盈调节

## 一、目标

- **约束 AI**：通过结构化输出（如对每个持仓必须给出趋势看法）减少自主性带来的误判。
- **持仓期间参与决策**：AI 不仅用于开仓，还在每个周期对**当前持仓**给出「市场趋势/状态」判断，供策略使用。
- **缓解矛盾**：用 AI 的趋势判断调节止损/止盈的「确认」强度，从而在**更早砍**（真反转时快速止损）与**震荡被洗**（震荡中多确认再动手）之间取得平衡。

## 二、思路

- 每个周期 AI 对每个**已有持仓**在输出 `action: hold` 时，同时输出 **`trend_view`**：当前对该标的/仓位的趋势看法。
- 策略层根据 `trend_view` **调节**本周期对该仓位的止损确认逻辑（不改变触发条件，只改变「确认周期数」或「确认时长」）：
  - **reversing（反转中）**：减少确认（如少 1 个周期或缩短确认时长）→ 更快执行止损，实现「更早砍」。
  - **trend_intact（趋势仍在）** / **choppy（震荡）**：增加确认（如多 1 个周期或延长确认时长）→ 避免震荡中一次假跌破就止损，减少「震荡被洗」。

这样：
- 真反转时 AI 倾向给出 `reversing`，策略少确认、早止损。
- 震荡或趋势仍在时 AI 倾向给出 `choppy` / `trend_intact`，策略多确认、少被洗。

## 三、设计要点

### 3.1 结构化输出：`trend_view`

- **位置**：与 `action` 同级，仅当 `action === "hold"` 且该条决策对应**当前某持仓**时有意义。
- **取值**（建议三选一）：
  - `trend_intact`：趋势仍在，结构未破坏，可继续持有。
  - `choppy`：震荡/无明确方向，不宜过度反应。
  - `reversing`：认为趋势反转或结构破坏，建议尽快止损或减仓。
- **缺省**：若 AI 未输出或输出不在上述集合，本周期对该仓位**不调节**，按原配置的确认逻辑执行。

### 3.2 策略层如何使用 `trend_view`

- **只调节「确认」**：不改变止损/止盈的**触发条件**（如 ATR 倍数、追踪回撤%、分层档位），只调节**需要满足条件的次数或时长**。
- **止损确认**（当前为「连续 N 周期满足才执行」或「持续 M 分钟才执行」）：
  - `reversing`：`requiredCycles = max(1, requiredCycles - 1)`，或 `confirmMinutes *= 0.5`（若用时长确认），使止损更快执行。
  - `trend_intact` / `choppy`：`requiredCycles = requiredCycles + 1`，或 `confirmMinutes *= 1.5`，使止损更「耐震」。
- **止盈**：暂不调节（或后续可对「回撤止盈」做类似调节），先只做止损侧。

### 3.3 执行顺序

- 当前顺序：先 `checkDynamicStopLossTakeProfit()`，再拉 context、再调 AI、再执行开仓。
- 新顺序（**有候选且会调 AI 时**）：先拉 context → 调 AI → 解析出每个持仓的 `trend_view` → 再执行 `checkDynamicStopLossTakeProfit(trendViewByPosition)`，最后执行开仓。
- **无候选时**：不调 AI，仍在本周期**开头**执行一次 `checkDynamicStopLossTakeProfit(nil)`，不做调节，保持与现有一致（避免有仓无候选时漏检 SL/TP）。

### 3.4 约束与提示词

- 在 schema 与 user prompt 中明确：
  - 对每个**已有持仓**若输出 `hold`，必须同时输出 **`trend_view`**，且只能为 `trend_intact` | `choppy` | `reversing` 之一。
  - 说明：该字段会用于策略对止损确认次数的调节（reversing 更快止损，trend_intact/choppy 多确认一次以减少震荡被洗）。
- 这样既**约束**了 AI（必须对持仓给出趋势看法），又**不剥夺**其判断权（仍由 AI 决定是趋势仍在、震荡还是反转）。

## 四、实现清单

| 项 | 说明 |
|----|------|
| Decision 结构体 | 增加 `TrendView string`（如 `json:"trend_view,omitempty"`）。 |
| Schema / 示例 | 在 decision 示例中增加 `trend_view` 的说明与 hold 示例（含三种取值）。 |
| User Prompt | 在「当前持仓」或「AI 仅开仓」相关段落中，要求对 hold 输出 trend_view 及取值含义。 |
| runCycle 顺序 | 有候选时：buildContext → AI → 从 decisions 中按持仓解析 trend_view → checkDynamicStopLossTakeProfit(trendViewMap) → 执行开仓。无候选时：先 checkDynamicStopLossTakeProfit(nil) 再 return。 |
| checkDynamicStopLossTakeProfit | 增加参数 `trendViewByPosKey map[string]string`；在计算 requiredCycles（及可选 confirmMinutes）时按 trend_view 调节。 |

## 五、AI 的定位：仅做实时市场趋势区分

- **AI 分析**的职责是**对实时市场趋势做区分**（趋势仍在 / 震荡 / 反转），而不是代替策略做最终执行。
- **市场多为波动**，随时可能出现**结构反转、趋势破坏**等情况，因此需要**每周期**对持仓做一次趋势/震荡/反转判断，并把结果通过 `trend_view` 交给策略。
- **当 AI 识别到此类情况时**（如结构反转、多周期转势、支撑/阻力有效跌破等）：
  - **应输出** `trend_view=reversing`。
  - **策略的应对**：减少该仓位的止损确认周期（少 1 个周期），在本周期或下一周期更快满足止损条件并执行平仓，避免反转后亏损扩大。
- 平仓仍由**策略的 SL/TP 逻辑**执行（如 ATR 止损、追踪止损、支撑跌破等），AI 不直接触发下单；AI 只通过 `reversing` 让策略「更早、更果断」地执行已有的止损条件。

## 六、小结

- 通过「持仓期间 AI 趋势判断」+「仅调节确认次数/时长」，在不改动现有 SL/TP 触发条件的前提下，用 AI 的输出缓解「更早砍」与「震荡被洗」的矛盾。
- 通过强制对持仓输出 `trend_view`，对 AI 形成约束并统一成可被策略使用的结构化信息。
- **AI 仅做实时趋势区分**；市场波动大、随时可能反转，识别到反转时输出 `reversing`，策略即减少确认、更快止损。
