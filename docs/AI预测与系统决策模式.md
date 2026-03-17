# AI 预测 + 系统决策 模式说明

## 你的想法

**由 AI 输出「预测数据」，系统根据预测数据做开平仓/止盈止损决定。**  
即：AI 不直接输出「开多/平仓」等动作，只输出结构化预测（方向、置信度、关键位、情景等），**由系统规则根据这些预测 + 行情 + 持仓 来下决定**。

---

## 可行性结论：**可行，且当前已部分存在**

- 系统里已经有「AI 输出预测类信息 → 系统用这些信息参与决策」的用法。
- 可以在此基础上，做成一种**更纯粹**的模式：**AI 只出预测、系统全权决策**。

---

## 一、当前已存在的「预测 → 系统用」部分

| AI 输出（预测/分析） | 系统如何用 |
|----------------------|------------|
| `market_regime`      | 调节开仓最低置信度（如震荡/反转时提高 MinConfidence） |
| `scenario`           | 反转时减少止损确认周期，加快止损 |
| `risk_alert`         | 本周期可不新开仓（可被策略或风控引用） |
| `trend_view`（持仓） | 调节动态止损确认：reversing → 少确认一次 |
| `near_term_outlook` / `key_levels` | 目前主要供推理与展示，尚未直接参与规则 |

也就是说：**「AI 给预测数据 → 系统用这些数据做判断」这条路已经在用**，只是目前 AI 同时还会输出具体动作（open/close），而系统在动作上再做约束和与规则结合。

---

## 二、纯「AI 预测 → 系统决策」模式长什么样

目标形态可以概括为：

1. **AI 只输出「预测/分析」**  
   不输出 `open_long` / `close_short` 等动作，只输出结构化预测，例如：
   - 整体：`market_regime`, `scenario`, `near_term_outlook`, `risk_alert`, `key_levels`
   - 按标的：`predicted_direction`（up/down/neutral）、`confidence`、`key_support`、`key_resistance`、`exit_signal`（是否建议退出）等

2. **系统有明确的「决策层」**  
   输入 = 预测结构体 + 当前行情 + 持仓 + 多层过滤/方向池结果；  
   输出 = 本周期要执行的动作列表（开多/开空/平多/平空/不动）。  
   所有「要不要开、要不要平」都写在代码规则里，可审计、可回测。

3. **优点**  
   - 逻辑全在系统侧，行为可解释、可调参、可回测。  
   - AI 只当「预测信号源」，不直接碰执行，风险更可控。  
   - 调策略只改规则或配置，不必改 prompt 里的动作设计。

4. **需要补的**  
   - 定义好「预测结构体」的字段（可复用/扩展现有 FullDecision 里的分析块）。  
   - 实现/扩展「系统决策函数」：根据预测 + 行情 + 持仓 + 现有 pipeline 结果 → 生成动作。  
   - 可选：增加策略模式或开关，在「当前混合模式」和「仅预测 + 系统决策」之间切换。

---

## 三、一种可落地的实现思路（保持与现有架构兼容）

在不推翻现有逻辑的前提下，可以这样接进去：

1. **沿用/扩展现有 AI 分析输出**  
   - 继续用或扩展 `market_regime`、`scenario`、`near_term_outlook`、`risk_alert`、`key_levels`。  
   - 如需「按标的」的预测，可增加 per-symbol 字段，例如：  
     `predicted_direction`、`confidence`、`key_support`、`key_resistance`、`suggest_exit`（bool 或等级）。

2. **新增「系统决策函数」**  
   - 例如：`DecideFromPrediction(pred *PredictionPayload, ctx *Context, pipelineResult *PipelineResult) (actions []Action)`。  
   - 规则示例（具体可再细调）：  
     - 开仓：仅当 pipeline 给出可开方向 + 该方向与 AI 的 `predicted_direction` 一致且 `confidence >= 阈值` 时才生成 open。  
     - 平仓：若 AI 给出 `suggest_exit` 且系统规则允许（如已盈利/或亏损达某条件），则生成 close；否则仍只依赖现有动态止盈/止损。

3. **策略模式或配置开关**  
   - 例如增加 `use_ai_as_prediction_only` 或策略模式 `ai_predict_system_decide`。  
   - 当开启时：  
     - 调用 AI 时只要求输出「预测/分析」部分（或忽略 decisions 里的 open/close）。  
     - 本周期实际执行的动作完全由 `DecideFromPrediction`（以及现有 TP/SL）产出。

4. **与现有逻辑的关系**  
   - 现有「系统动态止盈/止损」不变，仍每周期先跑。  
   - 现有「多层过滤 + 方向池」可继续产出「可开标的与方向」，作为 `DecideFromPrediction` 的输入之一。  
   - 这样就是：**AI 预测 → 系统根据预测 + pipeline + 持仓 下决定**，同时保留现有风控与 TP/SL。

---

## 四、小结

- **通过 AI 给出预测数据、再由系统根据预测数据下决定**，这种思路**可行**，且当前架构里已经有「预测数据被系统使用」的基础。  
- 要做成「更纯粹」的形态，只需要：  
  - 把 AI 的输出约束/设计成「仅预测、不直接给动作」；  
  - 在系统侧实现/扩展「根据预测 + 行情 + 持仓 + pipeline 结果」的决策函数；  
  - 用配置或策略模式在「混合模式」和「仅预测 + 系统决策」之间切换。  

如果你愿意，下一步可以针对「预测结构体字段」和「开仓/平仓规则示例」写一版更具体的接口与伪代码，方便直接对接到现有 nofx 代码里。

---

## 五、已实现：预测定多空 + 实时数据定强度与执行

在 **AI 仅预测（系统决策）** 模式下，已实现：

- **预测定多空**：系统开仓列表由 `GetSystemEntryListFromPrediction` 生成。对通过过滤的标的，仅当 AI 的 `symbol_predictions` 中该标的 `predicted_direction=up` 且 `confidence >= MinConfidence` 时加入 open_long；`predicted_direction=down` 时加入 open_short；neutral 或缺失则不入列表。
- **实时数据定强度**：同一侧（多或空）的排序仍用方向池的 `StrengthPct`（来自实时行情、OI/净流入/涨跌榜等），优先开强度更高的仓。
- **实时数据定是否执行**：每条开仓建议仍经 `EffectiveOpenConstraints`（regime、极端资金费率/多空比）、Layer3（OI 对齐、信号年龄）、仓位与风控等校验，不满足则本笔不执行。

即：**多空方向由 AI 预测决定，强度与是否执行由实时数据与系统规则决定。**
