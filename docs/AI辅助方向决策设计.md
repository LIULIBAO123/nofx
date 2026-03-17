# AI 辅助方向决策设计（结合历史+实时+预测，并约束 AI 行为）

## 一、目标

- **现状**：过滤机制只对「币种来源」给出的候选币做筛选；做多/做空由**系统方向池**完全基于**实时数据**规则（如 1h 涨跌、多周期对齐）决定，震荡或单根 K 线易误判。
- **目标**：引入 **AI**，让其结合**历史数据与实时数据**，对当下情况分析和未来预测，参与**做多/做空**决策；同时通过**结构化输出与系统校验**严格约束 AI 行为，避免随意或过度决策。

## 二、原则

1. **系统仍为主**：过滤（Layer1/2/3）与「谁有资格进池」由系统规则决定；AI 只参与**方向**（多/空/中性）与**置信度**，不新增/删除候选币。
2. **AI 可被约束**：必须输出规定格式；置信度低于阈值视为中性或忽略；超时/解析失败时**回退到仅用系统方向池**。
3. **历史+实时+预测**：给 AI 的上下文中包含近期历史摘要（如过去几根 K 的走势、资金费变化）、当前实时数据（与现有 prompt 一致）、以及已有预测字段（near_term_outlook、scenario）的生成要求，使 AI 的「做多/做空」建议基于更完整信息。

## 三、接入点与流程

```
候选币(币种来源) → Layer1 → Layer2 → 系统方向池(多/空) → [新增] AI 方向建议 → 合并/过滤 → 待提交
```

- **系统方向池**：沿用现有 `buildDirectionPools`，得到 `DirectionLong`、`DirectionShort`（规则基于实时数据）。
- **AI 方向建议**（可选，由配置开关）：
  - **输入**：本周期通过 Layer2 的标的列表 + 各标的行情与补强数据 + 简要历史摘要（如 24h/4h 涨跌、资金费变化、强平摘要）。
  - **输出**：每个标的的 `direction`（long / short / neutral）与 `confidence`（0–100）；可选整体 `market_bias`（long / short / neutral）与 `bias_confidence`。
- **合并策略**（约束核心）：
  - 仅当**系统方向池**认为某标的可做多（在 DirectionLong）且 **AI 建议为 long 且 confidence ≥ 阈值** 时，该标的才保留在「待提交」做多列表；做空同理。
  - 若 AI 返回 neutral 或 confidence 不足，该标的在本周期不以此方向开仓（或按配置：neutral 可视为「允许系统方向」）。
  - 若 AI 调用超时、解析失败或未启用，**完全回退**到当前逻辑：仅用系统方向池决定待提交。

这样：**过滤仍只针对币种来源给出的候选币；做多/做空由「系统方向池 + AI 方向建议」共同决定，且 AI 行为被格式与置信度严格约束。**

## 四、AI 行为约束（实现要点）

| 约束项 | 说明 |
|--------|------|
| **结构化输出** | 仅接受固定 JSON 结构，如 `{ "symbol_directions": [ { "symbol": "BTCUSDT", "direction": "long", "confidence": 75 } ], "market_bias": "long", "bias_confidence": 70 }`；其余内容忽略或仅作 reasoning 记录。 |
| **置信度阈值** | 配置项 `ai_direction_min_confidence`（如 60）；低于此值的建议视为 neutral，不用于通过/拒绝方向。 |
| **方向取值** | 仅接受 `long` / `short` / `neutral`；非法值视为 neutral。 |
| **标的范围** | AI 只允许对「本周期进入系统方向池」的标的输出建议；对未在池中的标的忽略或丢弃。 |
| **超时与失败** | 调用 AI 超时（如 15s）或解析失败时，不修改方向池，完全使用系统方向池结果。 |
| **可选上限** | 可配置「AI 最多建议做多 N 个、做空 M 个」，避免单边过度集中。 |

## 五、策略配置建议

在 `MultilayerFilterConfig` 或 `DirectionPoolConfig` 下增加（或复用 `DirectionPoolMode` 并扩展）：

```json
{
  "ai_direction_enabled": false,
  "ai_direction_min_confidence": 65,
  "ai_direction_timeout_sec": 15,
  "ai_direction_neutral_allow_system": true,
  "ai_direction_max_long": 0,
  "ai_direction_max_short": 0
}
```

- `ai_direction_enabled`：是否启用 AI 辅助方向；关闭则与现有一致，仅系统方向池。
- `ai_direction_min_confidence`：AI 方向置信度下限，低于视为 neutral。
- `ai_direction_timeout_sec`：超时后回退系统方向池。
- `ai_direction_neutral_allow_system`：为 true 时，AI 返回 neutral 的标的仍按系统方向池结果参与待提交；为 false 时，neutral 不参与本周期该方向开仓。
- `ai_direction_max_long` / `ai_direction_max_short`：0 表示不限制；>0 时对 AI 建议做多/做空的标的按置信度排序后只保留前 N/M 个。

## 六、提示词要点（AI 方向专用或整合进现有决策 prompt）

- 明确角色：**仅输出做多/做空/中性及置信度**，不新增标的、不输出开平仓指令。
- 输入信息：当前周期候选标的列表、各标的实时行情与补强数据、近期历史摘要（如 24h/4h 涨跌、资金费、强平概况）。
- 要求：结合**历史走势 + 当前状态 + 短期预期**（1–2 根 K 或本 session）给出方向与 0–100 置信度；不确定时务必用 neutral 且降低 confidence。
- 输出格式：严格按约定 JSON，便于解析与约束校验。

## 七、小结

- **过滤机制**：仍然只过滤「币种来源」给出的那批候选币，由系统规则决定谁通过 Layer1/2/3 和谁进方向池。
- **做多/做空**：在保留系统方向池的基础上，增加**可选的 AI 方向建议**，AI 结合历史+实时+预测给出 per-symbol 的 long/short/neutral 与 confidence；系统用**阈值与格式**约束 AI，只采纳高置信度同向建议，失败或超时则回退到仅用系统方向池，从而在利用 AI 分析能力的同时避免行为不可控。
