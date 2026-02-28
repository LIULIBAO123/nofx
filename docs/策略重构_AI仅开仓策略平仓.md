# 策略重构思路：AI 仅预测/开仓与参数，策略负责平仓（适应震荡市）

本文档基于你的 4 笔实验结果和新想法，分析「**AI 只做预测与开仓、策略负责平仓**」的可行性、利弊与实现要点。

---

## 一、实验结论回顾

| 交易 | 方向 | 盈亏 | 持仓时长 | 平仓方式 | 结论 |
|------|------|------|----------|----------|------|
| PIPPIN | LONG | **+2.78 (+9.68%)** | **6.8h** | 关闭 AI 分析 + 策略触发，后开启 AI 平仓 | 拿住仓、盈利出场 |
| ZRO | LONG | -0.12 (-0.40%) | 60m | AI 分析平仓（平仓触发） | 早平、小亏 |
| ZRO | LONG | -0.14 (-0.47%) | 50m | AI 分析平仓（平仓触发） | 早平、小亏 |
| PIPPIN | LONG | -1.03 (-3.60%) | 20m | AI 分析平仓（平仓触发） | 早平、亏损 |

- **盈利的一笔**：开仓后**关闭 AI 对持仓的平仓决策**，仅由策略（止盈/止损/追踪等）执行，持仓 6.8 小时后在盈利位置平仓。
- **亏损的三笔**：由 **AI 持续分析并触发平仓**，持仓时间短（20m～60m），在震荡/波动中被提前平掉。

你的结论：**在震荡市里，AI 根据实时数据反复判断，拿不住趋势；即使用连续确认 + 高波动宽容，AI 平仓仍容易导致盈利变少或提前止损。** 因此希望重新划分职责。

---

## 二、新策略设想（你的想法）

1. **AI 的职责**
   - **预测市场**：判断多空、趋势/震荡。
   - **决定开仓方向**：在合适时机输出 open_long / open_short。
   - **定义策略动态参数**：开仓时给出止损价、止盈价、ATR 倍数等（或由策略根据 AI 方向使用预设区间），**这些参数只交给策略执行，不用于「AI 建议平仓」**。
   - **不参与平仓**：不再根据实时数据输出 close_long / close_short；持仓期间的平仓完全交给策略。

2. **策略的职责**
   - **适应震荡市**：通过连续确认、高波动宽容、最小持仓时间、追踪止损、分层止盈、LockProfit 等，在震荡中拿住仓。
   - **在盈利位置平仓**：由动态止盈、追踪止损、分层止盈等规则在达到条件时执行平仓，不依赖 AI 的实时平仓建议。

这样分工：**AI = 何时开、开多还是开空、开仓时的风险参数；策略 = 何时平、如何平（全部由 SL/TP 规则执行）。**

---

## 三、分析

### 3.1 优点

| 点 | 说明 |
|----|------|
| **与实验结果一致** | 盈利的一笔正是「关掉 AI 平仓、只靠策略」拿住 6.8h 并在盈利平仓；新方案把这一点固化为默认行为。 |
| **震荡市更易拿住仓** | 不再每周期用 AI 重新判断「要不要平」，避免因短期波动、噪音触发提前平仓；策略的 ConfirmCycles、ATR 宽容、MinHoldMinutes、追踪/分层止盈专门为「扛震荡、盈利出」设计。 |
| **职责清晰** | 开仓 = AI（预测 + 方向 + 参数）；平仓 = 策略（规则执行），逻辑简单、可解释、可回测。 |
| **减少「AI 拿不住」** | 避免 AI 在震荡中因实时数据频繁给出平仓建议（即便有 2 周期确认，仍可能在两周期内都看到「转弱」而执行平仓）。 |

### 3.2 需要注意的点

| 点 | 说明 |
|----|------|
| **趋势反转** | 若 4h/1h 明确转势，当前可由 AI 建议「平多反空」等；关闭 AI 平仓后，只能依赖策略的 ATR 止损、初始止损、支撑/阻力止损等。若这些设计合理（如 8% 兜底、ATR 2×、结构破位），大反转仍会被止损，只是可能比「AI 提前判断反转」略慢一点。 |
| **极端行情/黑天鹅** | 目前 AI 可在极端时建议「全部平仓」；关闭 AI 平仓后，需依赖策略硬止损、风控规则或人工干预。可通过保留「紧急平仓」入口（如手动、或单独的风控模块）弥补。 |
| **Prompt 与输出** | 若启用「AI 仅开仓」，可在 Prompt 中明确：对已有持仓一律输出 hold，不输出 close_long/close_short；仅对新开仓输出 open_long/open_short。这样模型不会「浪费」在平仓判断上，也可减少误触。 |

### 3.3 小结

- 新思路与你的实验和「震荡市拿住仓、盈利后平仓」目标**高度一致**，在逻辑和实现上**可行**。
- 代价是放弃「AI 主动判断反转/提前平仓」；用「策略规则化平仓」换取在震荡市中的稳定拿仓与盈利出场，对当前 4 笔样本来说是更优选择。

---

## 四、实现要点（代码与配置）

### 4.1 配置项（已实现）

在策略的**风控配置**中已增加：

- **`ai_only_entry`**（前端「AI 仅开仓」开关）：  
  - `true`：**AI 仅负责开仓与开仓参数**；执行层**不执行** AI 输出的 `close_long` / `close_short`，仅由策略动态 SL/TP 平仓。  
  - `false`（默认）：保持现状，AI 可建议平仓，并经连续确认后执行。

在策略编辑 → 风险控制 中可勾选「AI 仅开仓」；保存后该策略下的交易员将按此配置运行。

### 4.2 执行层逻辑（auto_trader）

- 在 `runCycle` 中，处理每条 AI 决策时：
  - 若 `ai_only_entry == true` 且 `d.Action == "close_long"` 或 `"close_short"`：
    - **不执行**该平仓，不调用 `executeCloseLongWithRecord` / `executeCloseShortWithRecord`。
    - 可记录日志：如「AI 建议平仓已忽略（策略为仅开仓模式），由策略 SL/TP 负责平仓」。
  - 其他动作（open_long、open_short、hold、wait）照常执行。

- 策略侧已有逻辑**无需改**：  
  动态止损/止盈、追踪、分层止盈、LockProfit、ConfirmCycles、ATR 宽容等继续在 `checkDynamicStopLossTakeProfit` 中运行，与是否执行 AI 平仓无关。

### 4.3 Prompt / Schema（可选但推荐）

- 当 `ai_only_entry == true` 时，在系统提示或策略说明中增加一句（中英各一），例如：  
  「**当前为「AI 仅开仓」模式：你只负责预测市场、决定开仓方向与开仓时的止损/止盈参数；持仓的平仓完全由策略（动态止损、追踪止损、分层止盈）执行。对已有持仓请一律输出 hold，不要输出 close_long/close_short。**」
- 这样 AI 不会对已有仓位输出平仓建议，行为与配置一致，也便于回测和解释。

### 4.4 与现有功能的关系

| 功能 | 是否保留 | 说明 |
|------|----------|------|
| AI 开仓（open_long / open_short） | ✅ | 不变，仍由 AI 决定方向与时机。 |
| AI 设定 SL/TP/ATR 等（开仓时） | ✅ | 不变，开仓时写入策略使用的参数。 |
| 策略动态 SL/TP（追踪、ATR、分层止盈、LockProfit、ConfirmCycles） | ✅ | 不变，**唯一**的平仓执行来源（在仅开仓模式下）。 |
| AI 平仓（close_long / close_short） | ⚠️ 可配置关闭 | 当 `ai_only_entry == true` 时不执行，仅记录日志。 |
| 手动/紧急平仓、OrderSync 同步平仓 | ✅ | 不变，与 AI 是否参与平仓无关。 |

---

## 五、总结

- **实验**：关掉 AI 平仓、只靠策略的 PIPPIN 单笔盈利 +9.68%、持仓 6.8h；由 AI 触发平仓的三笔均为短持仓、亏损或小亏。
- **新思路**：AI 只做**预测 + 开仓方向 + 定义策略动态参数**，**不参与平仓**；策略负责**适应震荡市、在盈利位置平仓**。
- **评价**：该分工与实验结论和「震荡市拿住仓、盈利后平仓」目标一致，实现上可通过配置项 + 执行层跳过 AI 平仓即可，其余逻辑复用现有策略与风控。

---

## 六、已实现状态与使用方式

以下已全部实现并生效：

| 项目 | 状态 | 位置 |
|------|------|------|
| 配置项 `ai_only_entry` | ✅ | `store/strategy.go` RiskControlConfig；策略编辑 → 风险控制 → 「AI 仅开仓」 |
| 执行层跳过 AI 平仓 | ✅ | `trader/auto_trader.go` runCycle：若 `ai_only_entry==true` 且动作为 close_long/close_short，不执行并记录日志 |
| Prompt 注入「对已有仓一律 hold」 | ✅ | `kernel/engine.go`：当 AIOnlyEntry 为 true 时注入中英说明，要求对已有持仓只输出 hold |
| 前端开关与说明 | ✅ | `web/.../RiskControlEditor.tsx`：勾选「AI 仅开仓」及说明文案 |

**使用方式**：在对应策略的 **风险控制** 中勾选 **「AI 仅开仓」**，保存后该策略下的交易员将只执行 AI 的开仓与参数设定，平仓完全由策略动态 SL/TP（追踪、分层止盈、LockProfit、连续确认等）执行，适合震荡市拿住仓、盈利后平仓。

---

## 七、整体代码逻辑与提示词对应关系（是怎样变的）

下面按「配置 → 提示词 → 执行」顺序说明**已实现的完整链路**，以及相对原来的变化。

### 7.1 配置层（策略风控）

| 位置 | 变化 |
|------|------|
| **store/strategy.go** | 在 `RiskControlConfig` 中新增字段 `AIOnlyEntry bool`，JSON 键 `ai_only_entry`。未配置时为零值 `false`，行为与之前一致。 |
| **web** | 策略编辑 → 风险控制 中新增「AI 仅开仓」勾选框，绑定 `config.ai_only_entry`；保存后写入策略 JSON。 |

**效果**：用户勾选并保存后，该策略的 `risk_control.ai_only_entry == true`，该策略下的交易员在后续逻辑中都会按「仅开仓」模式运行。

### 7.2 提示词层（BuildUserPrompt）

| 位置 | 变化 |
|------|------|
| **kernel/engine.go** | 在 `BuildUserPrompt` 中，**仅当** `e.config.RiskControl.AIOnlyEntry == true` 时，向 User Prompt 追加一段约束（中英按语言二选一）：<br>• 中文：**当前为「AI 仅开仓」模式：你只负责预测市场、决定开仓方向与开仓时的止损/止盈参数；持仓的平仓完全由策略（动态止损、追踪止损、分层止盈）执行。对已有持仓请一律输出 hold，不要输出 close_long 或 close_short。**<br>• 英文：**AI-only-entry mode: You only predict market and decide entry direction/params; strategy handles all exits (dynamic SL/TP, trailing, scaled TP). For existing positions always output hold, do NOT output close_long or close_short.** |

**效果**：在「AI 仅开仓」模式下，模型被明确告知：对已有持仓只输出 `hold`，不输出 `close_long`/`close_short`，与执行层行为一致。

### 7.3 执行层（runCycle 决策执行）

| 位置 | 变化 |
|------|------|
| **trader/auto_trader.go** | 在 `runCycle` 中，遍历 AI 决策、在执行**任一条**前：<br>1. 读取当前策略的 `RiskControl.AIOnlyEntry`（来自 `strategyEngine.GetConfig()`）。<br>2. 若 `aiOnlyEntry == true` 且当前决策动作为 `close_long` 或 `close_short`：<br>   - **不执行**该条（不调用 `executeCloseLongWithRecord` / `executeCloseShortWithRecord`）。<br>   - 记录日志与 ExecutionLog：`"AI close xxx skipped (ai_only_entry=true, strategy handles exit)"`，并把该条决策记入 record 后 `continue`。<br>3. 其余动作（open_long、open_short、hold、wait）**不变**，照常执行。 |

**效果**：即使模型仍输出 `close_long`/`close_short`（例如未严格遵循提示），执行层也会直接跳过，不发起平仓；平仓只由策略的 `checkDynamicStopLossTakeProfit`（动态止损/追踪/分层止盈等）触发。

### 7.4 与「原逻辑」的对比

| 环节 | 原来（ai_only_entry=false 或未配置） | 现在（ai_only_entry=true） |
|------|--------------------------------------|-----------------------------|
| **提示词** | 无「仅开仓」约束，AI 可对持仓输出 close_long/close_short。 | 追加「仅开仓」说明，要求对已有持仓一律 hold。 |
| **执行** | AI 的 close_long/close_short 经「连续 2 周期确认」后执行。 | AI 的 close_long/close_short **一律不执行**，只记日志。 |
| **平仓来源** | AI 建议平仓 + 策略动态 SL/TP 均可触发平仓。 | **仅**策略动态 SL/TP（及手动/同步等）触发平仓。 |

Schema、系统提示中的「平仓建议」「结构破坏即减仓」「反手」等描述**未删除**，仍会出现在上下文中；但在「AI 仅开仓」模式下，执行层不执行 AI 的平仓类动作，因此实际平仓行为由策略与风控完全接管。

### 7.5 小结

- **已实现**：配置（ai_only_entry）→ 提示词（按配置注入「仅开仓」说明）→ 执行（按配置跳过 AI 的 close）三者已打通并对应。
- **怎样变的**：增加一个布尔配置；在 BuildUserPrompt 里按该配置追加一段约束；在 runCycle 里按该配置在执行前过滤掉 close_long/close_short。策略动态 SL/TP 逻辑未改，始终在后台运行，在仅开仓模式下成为唯一的自动平仓来源。
