# AI 与系统互补设计及补强方案

本文档说明如何**恢复 AI 在开仓、持仓、止盈、止损、平仓各场景的参与**，并与系统形成**互补**；单靠提示词无法完全约束 AI 自主性，需通过**数据补强、输出结构、执行层校验与兜底**共同保证 AI 更智能、更准确、有预测性。

---

## 一、互补原则：AI 出信号，系统做执行与兜底

| 角色 | 职责 | 说明 |
|------|------|------|
| **AI** | 输出「判断与预测」类信号 | 开仓建议、trend_view、scenario、near_term_outlook、关键位、可选平仓建议；**不直接执行**，仅作为系统与规则的输入。 |
| **系统** | 规则执行 + 校验 + 兜底 | 过滤（Layer1/2/3）、方向池、SL/TP 规则、仓位/杠杆/盈亏比硬约束；**校验** AI 输出的合理性；**缺失或异常时**按规则兜底（如不执行 AI 开仓、仅用规则 SL/TP）。 |

这样既让 AI 参与各场景，又避免「完全交给 AI」带来的不可控；提示词约束行为，**执行层约束结果**。

---

## 二、各场景下 AI 与系统的分工

### 2.1 开仓

| 维度 | AI | 系统 |
|------|-----|------|
| **输出** | open_long/open_short + confidence、reasoning；可选 scenario、near_term_outlook；risk_alert 建议本周期是否新开。 | 候选列表（可来自 pipeline 预筛）；执行前校验：confidence≥MinConfidence、盈亏比、Layer3、仓位上限等。 |
| **互补** | AI 提供「是否开、方向、依据与预期」；系统提供「谁能开」（过滤）、「开多少」（仓位上限）、「开不开得过门槛」（置信度/盈亏比）。 | 若 SystemExecutesEntry=true，系统按方向池开仓，AI 仅提供 risk_alert 等辅助；若 false，AI 建议开仓，系统校验后执行。 |
| **兜底** | AI 无开仓建议或 confidence 不足 → 系统不执行开仓；risk_alert=true → 本周期不执行系统开仓。 | 硬上限：MaxPositions、仓位价值、MinConfidence、MinRiskRewardRatio 由系统强制。 |

### 2.2 持仓

| 维度 | AI | 系统 |
|------|-----|------|
| **输出** | trend_view（trend_intact/choppy/reversing）；scenario（continuation/reversal/range）；可选 near_term_outlook、key_levels。 | 不主动「持仓」动作；仅当无 SL/TP 触发时维持持仓。 |
| **互补** | AI 给出当前与短期预期，供系统**调节** SL/TP 的敏感度（如确认周期、是否提前部分止盈）。 | 系统按规则计算 SL/TP 触发条件；用 trend_view 调 SL 确认周期；后续可用 scenario 再微调。 |
| **兜底** | 若 AI 未输出某持仓的 trend_view，系统对该仓使用**默认**确认周期（不加重、不减轻）。 | 持仓的平仓最终由系统 SL/TP 或 AI 平仓建议（若启用）经确认后执行。 |

### 2.3 止盈

| 维度 | AI | 系统 |
|------|-----|------|
| **输出** | scenario、near_term_outlook、key_levels（预期阻力/目标）；可选「建议部分止盈」类表述（可解析为建议档位）。 | 分层止盈、追踪止盈、固定/ATR/阻力止盈的**规则计算与触发**。 |
| **互补** | AI 预期「上方空间有限」或「接近关键阻力」→ 系统可**放宽**部分止盈触发（如略降第一档门槛）或优先执行部分止盈。 | 系统保证最低止盈逻辑（档位、比例）不变；AI 仅作**参数微调或优先级**，不替代规则。 |
| **兜底** | 无 AI 输出或解析失败 → 完全按策略配置的止盈规则执行。 | 止盈触发条件、比例、档位由配置与系统计算。 |

### 2.4 止损

| 维度 | AI | 系统 |
|------|-----|------|
| **输出** | trend_view（已有）；scenario（reversal 等）；可选「假跌破/真反转」类判断。 | 追踪、ATR、支撑/阻力、逆势早退等**规则计算与触发**；确认周期。 |
| **互补** | trend_view=reversing → 系统**减少** SL 确认周期；scenario=reversal → 可再减 1 或提前触发；trend_intact/choppy → **增加**确认周期。 | 系统保证最低止损逻辑（价格、ATR、逆势早退）不变；AI 仅调节**确认次数与敏感度**。 |
| **兜底** | 无 trend_view → 使用默认确认周期；异常值（如非法字符串）→ 忽略，用默认。 | 止损条件、比例、逆势早退由配置与系统计算；最终执行权在系统。 |

### 2.5 平仓

| 维度 | AI | 系统 |
|------|-----|------|
| **输出** | close_long/close_short 建议（可选）；scenario=reversal + 高置信时可视为「建议平仓」信号。 | 实际平仓由**系统执行**：要么规则 SL/TP 触发，要么 AI 平仓建议经**确认逻辑**后执行。 |
| **互补** | AI 可输出主动平仓建议（如「预期反转，建议了结」）；系统要求**连续 N 周期**均建议平仓才执行（已有 2 周期确认），避免单周期误判。 | 若 AIOnlyEntry=true，当前不执行 AI 平仓，仅由 SL/TP 平仓；若未来开放「AI 辅助平仓」，仍保留确认次数与系统校验。 |
| **兜底** | 未满确认次数不执行；confidence 低于阈值可拒绝执行 AI 平仓。 | 平仓指令由系统发往交易所；仓位与风控由系统保证。 |

---

## 三、单靠提示词无法完全规范 AI 的应对：执行层补强

### 3.1 校验（Validation）

- **开仓**：执行前必查 confidence≥MinConfidence、盈亏比≥配置、标的在允许列表（或通过 Layer3）、仓位未超限；任一不满足则**拒绝执行**并记日志。
- **平仓**：若启用 AI 平仓，可要求 confidence≥X 且（可选）scenario 为 reversal；否则仅记录不执行。
- **数值**：stop_loss、take_profit、position_size_usd 等必须在合理区间（系统可带上下界校验），异常则用策略默认或拒绝。

### 3.2 兜底（Fallback）

- **无 AI 输出**：如请求超时、解析失败、无有效 decision → 不执行任何 AI 建议的开/平仓；SL/TP 完全按规则与默认 trend_view 行为执行。
- **缺字段**：某持仓无 trend_view → 该仓使用默认 SL 确认周期；无 scenario → 不应用「scenario 微调」逻辑。
- **risk_alert**：为 true 时本周期不执行系统开仓（已有）；可扩展为「不执行 AI 开仓建议」或「降低仓位系数」。

### 3.3 上限与硬约束（Caps）

- 仓位、杠杆、最大持仓数、最小盈亏比等**仅由系统与配置决定**，AI 建议不得突破。
- 可选：对「AI 建议的开仓次数」做周期内上限（如每周期最多执行 1 次 AI 开仓），防止过度依赖 AI。

### 3.4 一致性检查（可选）

- 若 SystemExecutesEntry=false 且启用 pipeline：可检查 AI 建议开仓的标的是否在 pipeline 的 ToSubmitSymbols 或候选列表中；若不在可拒绝或仅记录，避免与过滤结果冲突。

---

## 四、达到「更智能、更准确、有预测性」所需的补强

### 4.1 数据补强

| 补强项 | 说明 | 改动位置 |
|--------|------|----------|
| **预测相关摘要** | 在组装 User Prompt 时，由系统根据 Context 生成 1～2 句「预测相关摘要」：如 4h/1h 同向、OI 增跌、资金流方向、费率与强平倾向。 | engine.BuildUserPrompt：在候选/持仓数据块前增加一段「本周期预测参考摘要」。 |
| **波动率/Regime（可选）** | 当前 ATR、与长期 ATR 比、简单 regime（趋势/震荡/高波）写入 prompt，便于 AI 给出与波动匹配的预期与置信度。 | Context 或 market 层提供 ATR 比值与 regime 标签；BuildUserPrompt 写入。 |
| **关键位（可选）** | 若已有支撑/阻力或前高/前低，写入 prompt，便于 AI 输出 key_levels 与 near_term_outlook。 | 若 market 或 indicator 层有计算，写入 Context 与 prompt。 |

### 4.2 提示词补强

| 补强项 | 说明 | 改动位置 |
|--------|------|----------|
| **强制「先预测再决策」** | 在决策流程中明确：先写「近 1～2 根 K 或本 session 的预期（方向、关键位、依据）」，再写 action/trend_view。 | BuildSystemPrompt：Decision Process 或 Entry Standards 中增加一步。 |
| **输出结构扩展** | 要求 `<analysis>` 或新块 `<outlook>` 中必须/建议包含：near_term_outlook、scenario（continuation/reversal/range）、key_levels。 | BuildSystemPrompt：Output Format；示例 JSON。 |
| **各场景职责说明** | 在系统提示中写清：开仓需 confidence 与依据；持仓需 trend_view + scenario；止盈/止损由系统执行，AI 通过 trend_view/scenario 调节敏感度；平仓可建议但需系统确认。 | BuildSystemPrompt：新增「AI 与系统分工」简短说明。 |

### 4.3 输出解析与结构补强

| 补强项 | 说明 | 改动位置 |
|--------|------|----------|
| **解析 outlook/scenario** | 从 AI 回复中解析 `<outlook>` 或扩展 `<analysis>` 的 near_term_outlook、scenario、key_levels。 | kernel 解析函数（如 parseFullDecisionResponse 或新函数）；写入 FullDecision。 |
| **FullDecision 新字段** | FullDecision 增加 NearTermOutlook、Scenario、KeyLevels（可全局或 per-symbol，视设计）。 | kernel 中 FullDecision 结构体；调用方传入执行层。 |
| **按持仓传递 scenario** | 若 scenario 按标的给出，与 trend_view 一样按 posKey 传入 checkDynamicStopLossTakeProfit，用于 SL 确认周期等。 | auto_trader：从 aiDecision 构建 scenarioByPosKey；stop_loss 逻辑中读取。 |

### 4.4 执行层补强

| 补强项 | 说明 | 改动位置 |
|--------|------|----------|
| **开仓校验** | 已有 MinConfidence、盈亏比；可加：标的在候选/ToSubmit 列表（当 pipeline 启用时）。 | auto_trader.executeOpenLong/Short 或调用前。 |
| **SL 确认周期使用 scenario** | 在 trend_view 基础上，若 scenario=reversal 再减 1 个确认周期（或配置化）。 | auto_trader.checkDynamicStopLossTakeProfit 或 kernel/stop_loss 入参。 |
| **TP 与 scenario（可选）** | 若 scenario=reversal 或 near_term_outlook 含「空间有限」，可略降第一档部分止盈门槛或优先触发部分止盈；需配置开关与参数。 | take_profit 或 auto_trader 中 TP 检查处。 |
| **AI 平仓可选开放** | 配置项「允许 AI 建议平仓」：当 true 时执行 AI 的 close_long/close_short（仍保留连续 2 周期确认）；confidence 与 scenario 可作额外条件。 | RiskControlConfig 新字段；auto_trader 中分支。 |
| **兜底与日志** | 无 AI、解析失败、缺 trend_view 时明确使用默认行为并打日志。 | 已有部分；在传入 trendViewByPosKey、scenarioByPosKey 处统一兜底。 |

### 4.5 配置与策略补强

| 补强项 | 说明 | 改动位置 |
|--------|------|----------|
| **AI 参与模式** | 明确三种：仅辅助（不执行 AI 开/平）、AI 开仓+系统平仓、AI 开仓+AI 平仓（经确认）。当前用 SystemExecutesEntry + AIOnlyEntry 组合；可增加「AllowAIClose」等。 | store.RiskControlConfig；前端策略页。 |
| **scenario 对 SL 的影响** | 配置：如「scenario=reversal 时 SL 确认周期再减 1」开关与默认值。 | store.DynamicStopLossConfig；kernel 使用处。 |

---

## 五、改动清单（按优先级）

### P0：必须（平衡 AI 与系统、保证安全）

1. **提示词**：增加「先预测再决策」一步；扩展 `<analysis>` 或增加 `<outlook>`，要求 near_term_outlook、scenario。
2. **解析**：解析 scenario、near_term_outlook、key_levels 写入 FullDecision。
3. **执行校验**：开仓前 confidence≥MinConfidence、盈亏比、仓位上限；缺 AI 或解析失败时不执行 AI 开/平，仅用规则。

### P1：强烈建议（互补与智能）

4. **数据**：BuildUserPrompt 中增加「预测相关摘要」段落（由 Context 生成 1～2 句）。
5. **SL 使用 scenario**：trendViewByPosKey 旁增加 scenarioByPosKey；在 SL 确认周期逻辑中，scenario=reversal 时再减 1（可配置）。
6. **兜底**：无 trend_view 时该仓用默认确认周期；无 scenario 时不应用 scenario 微调。

### P2：可选（增强预测与平仓）

7. **波动率/regime 入 prompt**：若已有 ATR 比值或 regime，写入 prompt。
8. **TP 与 scenario**：scenario=reversal 或 outlook 含「空间有限」时，第一档部分止盈略放宽（需配置）。
9. **AllowAIClose**：配置项允许执行 AI 平仓建议（保留 2 周期确认 + confidence 门槛）。

---

## 六、额外补强（已实现）

在 P0～P2 之外，以下补强已接入执行层与配置，与「AI + 系统互补」一致使用。

### 6.1 按 market_regime 调节开仓门槛

- **配置**：`RiskControlConfig.RegimeAdjustEnabled`、`RegimeMinConfidenceMap`（如 `ranging→75`、`high_volatility→78`、`reversal→80`）。
- **逻辑**：AI 输出 `market_regime` 后，若启用且 map 中有该 regime，则开仓时使用的**有效最低置信度** = max(基础 MinConfidence, map 中对应值)；未列出 regime 仍用基础 MinConfidence。
- **作用**：震荡/高波/反转时自动提高开仓门槛，减少逆势或噪音开仓。

### 6.2 极端资金费率与多空比约束

- **配置**：`RiskControlConfig.ExtremeFundingRule`（`Enabled`、`FundingThresholdPct`、`LongShortRatioHigh/Low`、`BlockOpenLongWhenExcessiveLongs`、`BlockOpenShortWhenExcessiveShorts`、`RaiseConfidenceBy`、`MinConfidenceWhenExtreme`）。
- **逻辑**（见 `kernel.EffectiveOpenConstraints`）：
  - 资金费率绝对值 ≥ 阈值 → 正为多头过热、负为空头过热；
  - 多空比 > High → 多头过热；< Low → 空头过热；
  - 多头过热：若 `BlockOpenLongWhenExcessiveLongs` 则本周期**禁止开多**，否则将有效最低置信度提高（RaiseConfidenceBy 或 MinConfidenceWhenExtreme 取更严）；
  - 空头过热：同理禁止开空或提高置信度。
- **数据来源**：`Context.BinanceFundingMap`、`BinanceFundingRateAvg8h`、`BinanceLongShortMap`（按标的取）。
- **作用**：在明显过热一侧不开仓或仅高置信度开仓，降低逼空/逼多风险。

### 6.3 执行层接入方式

- **AI 开仓**：每笔 open_long/open_short 执行前调用 `EffectiveOpenConstraints(ctx, symbol, aiDecision.MarketRegime, ...)`，得到 `effectiveMinConf`、`blockLong`、`blockShort`；若被禁止则跳过并记日志，否则用 `effectiveMinConf` 做置信度校验（传入 `executeOpenLongWithRecord`/`executeOpenShortWithRecord`）。
- **系统开仓**：同样在每笔系统开仓前调用 `EffectiveOpenConstraints`，被禁止则跳过该笔，否则用得到的 `effectiveMinConf` 作为系统开仓的置信度并传入 execute。

### 6.4 预设与前端

- **默认预设**：Regime 与 Extreme 规则**默认关闭**（`RegimeAdjustEnabled=false`、`ExtremeFundingRule.Enabled=false`），避免改变现有行为。
- **优化预设**：Regime 调节**开启**，Extreme 规则**开启**，不禁止方向仅提高置信度（Block 为 false，RaiseConfidenceBy=10、MinConfidenceWhenExtreme=80）。
- **前端**：`web/src/types.ts` 已增加 `regime_adjust_enabled`、`regime_min_confidence_map`、`extreme_funding_rule`、`scenario_adjust_enabled`（SL）等类型；策略页可按需增加「额外补强」卡片绑定上述配置。

---

## 七、小结

- **恢复 AI 功能**：让 AI 参与开、持、止盈、止损、平仓，通过**输出信号（开仓建议、trend_view、scenario、outlook）**参与，**执行与硬约束始终在系统**。
- **平衡 AI 与系统**：AI 提供判断与预测；系统提供规则、校验、兜底与上限；单靠提示词不够，需**执行层校验与 fallback**。
- **更智能、更准确、有预测性**：通过**数据补强**（预测摘要、波动率/regime）、**提示词补强**（先预测再决策、结构化 outlook）、**解析与执行补强**（scenario 调 SL、校验与兜底）共同实现。
- **改动**：按 P0→P1→P2 分步落地；先提示词与解析 + 执行校验，再 scenario 调 SL 与数据摘要，最后可选 TP 与 AI 平仓开放。
- **额外补强**（第六节）：按 **market_regime** 提高开仓置信度；**极端资金费率/多空比**时禁止开多/开空或提高置信度；AI 开仓与系统开仓共用 `EffectiveOpenConstraints`，预设中默认关闭、优化预设中开启。
