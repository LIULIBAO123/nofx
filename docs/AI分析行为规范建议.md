# AI 分析行为规范建议

## 一、当前状态

### 已有约束（较严格）

- **输出结构**：必须用 `<reasoning>` + `<decision>` 分离思维链与决策 JSON；`<decision>` 内必须先输出 JSON 数组。
- **AIPredictOnly**：禁止输出 open/close 动作，**必须**在 `<analysis>` 中输出 **`symbol_predictions`**。
- **持仓 hold**：若 action=hold，**必须**带 **`trend_view`**（trend_intact | choppy | reversing）。
- **平仓**：若 action=close_long/close_short，**必须**带 **`exit_reason`**（take_profit | stop_loss | prediction_mismatch）。

### 已有建议（非强制）

- **Decision Process** 五步：1) 检查持仓 2) 先形成 short-term outlook 3) 4h/1h 定方向再找入场 4) 扫描候选 5) 先输出 JSON 再写 reasoning。未强制「必须按此顺序分析」。
- **`<analysis>` 块**：建议包含 market_summary、market_regime、scenario、near_term_outlook、key_levels；**仅 AIPredictOnly 时强制 symbol_predictions**，其余为可选。
- **思维链**：只要求「Step 2: chain of thought」在 `<reasoning>` 里，未规定必须包含「账户→持仓→候选→决策」等固定段落。

因此：**输出格式和少量字段已严格约束；分析步骤与 `<analysis>` 内多数字段仍是建议性质，未严格规范「必须分析哪几部分」。**

---

## 二、是否需要严格规范「分析哪几部分」

### 建议：**要规范，但只规范「输出结构」与「必填分析结论」，不强制思维链的段落顺序**

理由简述：

| 维度 | 不严格规范 | 适度严格规范（推荐） |
|------|------------|----------------------|
| 一致性 | 不同周期/模型输出差异大，前端、日志、排查难以统一 | 每周期都有相同的「分析结论」字段，便于展示与回溯 |
| 可解析性 | `<analysis>` 缺字段时需大量兼容与默认值 | 明确必填项后解析简单、失败可检测 |
| 行为可预期 | 有时缺 market_regime/scenario，下游逻辑要处处防御 | 系统可依赖「至少会有 regime + scenario + symbol_predictions」 |
| 灵活性 | 模型可自由组织推理 | 仅约束「必须输出哪些结论」，不约束推理顺序与篇幅 |

结论：**建议在「输出结构」上做适度严格规范**：明确 **`<analysis>` 里必须包含哪几项**，以及（可选）**建议的分析顺序**，而不强制模型在 `<reasoning>` 里写固定的小标题。

---

## 三、推荐的最小规范（主周期）

### 3.1 强制：`<analysis>` 必须包含的字段

在现有基础上，把以下字段从「建议」改为 **必填**（与 AIPredictOnly 已有要求一致，便于系统与前端统一使用）：

| 字段 | 说明 | 当前 | 建议 |
|------|------|------|------|
| `market_regime` | 市场状态（trend_up/trend_down/ranging/…） | 可选 | **必填** |
| `scenario` | 情景（continuation/reversal/range） | 可选 | **必填** |
| `symbol_predictions` | 按标的预测（AIPredictOnly 时已必填） | 已必填 | 保持 |
| `risk_alert` | 是否建议本周期不新开仓 | 可选 | 建议必填（可默认 false） |

仍为可选：market_summary、near_term_outlook、key_levels、position_sl_tp_adjustments（有持仓时建议输出，但不强制）。

### 3.2 建议：分析顺序（提示词中写清，不强制解析）

在提示词中明确写出一份「建议分析顺序」，让模型形成习惯，**不**在解析端强制顺序：

1. **市场环境**：先给出 market_regime、scenario、risk_alert（一句话或短句即可）。
2. **短期预期**：near_term_outlook、key_levels（可选）。
3. **标的结论**：symbol_predictions（每标的方向与置信度）；若有持仓，再给 position_sl_tp_adjustments（可选）。

这样既规范「输出里必须有哪些分析结论」，又保留推理顺序和篇幅的弹性。

### 3.3 止盈止损专用周期

当前已规范为「只输出 `position_sl_tp_adjustments`」，无需再增加分析段落；若希望更一致，可要求每笔持仓在数组中**至少有一条**（advice 可为 no_change 或 hold），便于前端/日志统一展示。

---

## 四、落地方式建议

1. **改提示词**（`kernel/engine.go`）  
   - 在「Optional <analysis> / <outlook> block」中，将 `market_regime`、`scenario` 改为 **必填**，并写清「建议按：① 市场环境 ② 短期预期 ③ 标的结论 的顺序组织分析」。  
   - 明确 `risk_alert` 必填，若无特别风险则填 false。

2. **解析与兼容**  
   - 若历史响应缺 `market_regime`/`scenario`，解析时给默认值（如 "" 或 "unknown"），避免前端报错。  
   - 新请求一律按新提示词要求；旧数据仍可展示。

3. **文档**  
   - 在《AI实时预测与止盈止损提示词与输出结构》中增一节「必填分析结论与建议顺序」，与本文档一致。

4. **（可选）校验**  
   - 若需严格校验：在解析 `<analysis>` 后检查必填字段是否存在；缺则打日志或标记为「不完整」，便于排查模型未按规范输出的问题。

---

## 五、总结

- **需要适度严格规范**：建议明确「AI 必须输出哪几部分分析结论」（尤其是 market_regime、scenario、symbol_predictions、risk_alert），并固定为 `<analysis>` 的必填字段。
- **不必过度约束**：不强制 `<reasoning>` 的段落标题和顺序，只建议「先环境再预期再标的」的分析顺序，保证行为可预期且便于排查，同时保留模型表达空间。

若你确认采用上述最小规范，可在提示词中按第三节逐项改为必填，并在文档中同步说明。
