# AI 开仓与过滤机制调整方案

本文档针对两点诉求：**(1) 恢复 AI 开仓并约束其行为，禁止模棱两可决策；(2) 过滤机制过严导致 12 小时无开仓，需整体放宽。**

---

## 一、现状与问题

### 1.1 AI 角色

- 当前「系统执行开仓」模式下：AI 仅输出 trend_view、market_summary、risk_alert，不开仓，**几乎无决策权**。
- 诉求：**重新启用 AI 开仓**，但**严格约束**——不允许模棱两可决策，保证开仓建议明确、可执行、可追溯。

### 1.2 过滤过严

- **Layer1**：16 项全部启用且 `RequiredAll=true`，**任一未过即整币不通过**；实测常见失败：量能未达标、长/短周期未对齐、量价趋势未同向。
- **Layer2**：MinFactors≥6、Reliability≥0.67、EntryConfidence≥31%，三者同时满足才过，**通过率低**。
- **Layer3**：信号年龄、OI 对齐、入场强度等进一步收缩。
- 结果：**12 小时 0 开仓**，机会被过滤殆尽。

---

## 二、调整方向概览

| 维度 | 方向 |
|------|------|
| **AI 开仓** | 保留「系统执行开仓」选项；新增/强化「AI 开仓」模式：AI 可输出 open，但**提示词与执行层双重约束**，禁止模糊决策。 |
| **过滤机制** | Layer1 支持「至少通过 N 项」；Layer2 降低默认阈值；预设提供「宽松」可选。 |

---

## 三、AI 开仓：约束与规范

### 3.1 设计原则

- **明确性**：开仓必须带**明确理由**（哪些信号共振、为何置信度≥X），禁止「可能」「或许」「观望」式开仓。
- **可执行性**：confidence、position_size_usd、stop_loss、take_profit 必须为**具体数值**，且 confidence 不低于策略 MinConfidence（建议执行侧再校验 ≥75 时更严）。
- **禁止模糊**：若理由不充分或信号矛盾，**只允许输出 wait**，并在 reasoning 中说明缺哪类信号。

### 3.2 提示词约束（系统提示词中追加）

当 **未启用**「系统执行开仓」时，在 Hard Constraints / Entry Standards 中增加：

- **中文**：  
  - 「开仓决策必须明确：仅当多周期、量能、OI/流向等至少 3 类信号共振且能写出具体依据时，才可输出 open_long/open_short；否则一律输出 wait 并说明缺少的信号。」  
  - 「禁止基于单一指标或模棱两可理由开仓。confidence 必须为具体数字且 ≥ 配置的最小信心度。」  
- **英文**：  
  - "Entry must be unambiguous: only output open_long/open_short when at least 3 categories of signals (e.g. multi-timeframe, volume, OI/flow) align and you can state concrete reasons; otherwise output wait and state which signals are missing."  
  - "Do not open on a single indicator or vague reasoning. confidence must be a number ≥ the configured minimum."

### 3.3 执行侧校验（可选）

- 对 AI 输出的 open 建议：若 `confidence < MinConfidence` 或 `confidence < 75`（可配置），**拒绝执行**并记日志。  
- 若某条决策的 reasoning 过短或无明确依据，可记录为「低质量建议」不执行（可选，实现成本较高，优先靠提示词约束）。

### 3.4 策略配置

- **系统执行开仓** = false：即恢复「AI 开仓」模式；Pipeline 仍可运行，用于**预筛候选**（或仅作展示），最终是否开仓以 **AI 输出 + 执行校验** 为准。  
- 保留「系统执行开仓」= true：行为与当前一致，开仓完全由方向池/待提交列表执行。

---

## 四、过滤机制：放宽方案

### 4.1 Layer1

- **现状**：RequiredAll=true → 16 项全部通过才进入 Layer2，任一不通过即淘汰。  
- **调整**：  
  - 新增 **MinItemsToPass**（默认 0）：  
    - **0**：保持现逻辑，即「全部通过才过」（RequiredAll 语义）。  
    - **N（如 12）**：**至少通过 N 项**即视为 Layer1 通过（未通过项仍记录，用于统计与展示）。  
  - 这样单币在「量能未达标」「长周期未对齐」等少数项不通过时，仍有机会进入 Layer2。

### 4.2 Layer2

- **现状**：MinFactors=6、ReliabilityThreshold=0.67、EntryConfidenceThresholdPct=31，三者同时满足。  
- **调整（预设/可选宽松）**：  
  - MinFactors：6 → **4**（满足 4 个因子即过）。  
  - ReliabilityThreshold：0.67 → **0.55**。  
  - EntryConfidenceThresholdPct：31 → **25**。  
  - 可在策略预设中提供「**标准**」与「**宽松**」两档，默认或实测无开仓时建议选用宽松档。

### 4.3 Layer3

- 暂不强制放宽；若仍无开仓，可再考虑：  
  - 放宽信号年龄（如 <10 分钟）、  
  - OIAlignedRequired 保持 false 或仅在「标准」预设中开启。

### 4.4 方向池

- 保持现有逻辑；过滤放宽后，进入方向池的标的会自然增多。

---

## 五、实现清单

1. **提示词**：在 `BuildSystemPrompt` 中，当 `!SystemExecutesEntry` 时追加「开仓明确性、禁止模糊、confidence 要求」的硬性说明（中英）。  
2. **Layer1**：`Layer1Config` 增加 `MinItemsToPass`；`RunPipeline` 中 Layer1 通过条件改为：`(MinItemsToPass == 0 && len(failed)==0) || (MinItemsToPass > 0 && (enabledCount - len(failed)) >= MinItemsToPass)`。  
3. **Layer2 预设**：在默认/优化策略中增加「宽松」预设或调低默认阈值（MinFactors 4、Reliability 0.55、EntryConfidence 25）。  
4. **执行校验**：对 AI 的 open 建议，若 confidence < MinConfidence 拒绝执行（若已有则强化日志）；可选增加「最低 75」的二次校验。  
5. **前端**：策略模式/风控中「系统执行开仓」关闭即恢复 AI 开仓；Layer1 配置处展示 MinItemsToPass（0=全部通过，>0=至少 N 项通过）；Layer2 展示并可调三阈值；可选「标准/宽松」预设切换。

---

## 六、使用建议

- **希望 AI 参与开仓且决策可追溯**：关闭「系统执行开仓」，启用上述提示词约束与执行侧 confidence 校验。  
- **12 小时无开仓**：优先将 Layer1 设为「至少通过 12 项」、Layer2 改为宽松预设；观察通过率与开仓频率后再微调。  
- **仍希望系统完全执行开仓**：保持「系统执行开仓」开启，仅做过滤放宽，AI 仍只做 trend_view/risk_alert。

---

*文档版本：与代码实现同步更新。*
