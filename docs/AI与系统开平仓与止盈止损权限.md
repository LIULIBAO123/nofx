# AI 与系统：开仓、止盈、止损、平仓权限说明

## 设计原则

- **开仓、止盈、止损、平仓** 的权限由 **AI 与系统共同拥有**，二者并存、互为补充。
- **AI 的执行必须带约束**：仅在结合历史、实时与预测后，认为「此交易与预测不符」时才建议止盈/止损/平仓，且需满足置信度与退出原因等配置约束。

---

## 一、谁可以做什么

| 动作         | 系统 | AI |
|--------------|------|-----|
| **开仓**     | ✅ 系统可根据多层过滤/方向池执行开仓（`system_executes_entry=true` 时） | ✅ AI 可输出 open_long/open_short，受 MinConfidence、regime、极端资金费率等约束 |
| **止盈**     | ✅ 动态止盈规则（分层止盈、ATR 止盈、锁定利润等）每周期检查并执行 | ✅ AI 可建议 close_long/close_short 且 exit_reason=take_profit（需开启 allow_ai_close 并满足约束） |
| **止损**     | ✅ 动态止损规则（ATR、追踪、支撑/阻力、确认周期等）每周期检查并执行 | ✅ AI 可建议 close_long/close_short 且 exit_reason=stop_loss（同上） |
| **平仓**     | ✅ 通过止盈/止损逻辑触发平仓 | ✅ AI 可建议 close_long/close_short 且 exit_reason=prediction_mismatch（同上） |

同一周期内：先执行**系统**的止盈/止损检查，再处理 **AI** 的决策（含平仓建议）。若系统已对某仓位触发止盈/止损，该仓位已平，AI 的平仓建议将无目标可执行。

---

## 二、AI 平仓/止盈/止损的约束

当策略开启 **允许 AI 平仓**（`allow_ai_close=true`）且非「AI 仅开仓」时，AI 的 close_long/close_short 建议**仅在同时满足以下条件时**才会被系统执行：

1. **语义约束（提示词）**  
   AI 仅在**结合历史数据、实时数据与预测**后，认为**此交易与预测不符**（如趋势反转、目标达成、关键位跌破）时，才应建议平仓，并必须输出 **exit_reason**：
   - `take_profit`：止盈/锁定利润  
   - `stop_loss`：止损/结构破坏  
   - `prediction_mismatch`：预期改变、交易不再成立  

2. **置信度约束（可选）**  
   若配置了 `min_confidence_for_ai_close > 0`，则仅当 AI 输出的 `confidence` ≥ 该值时才执行平仓。

3. **退出原因约束（可选）**  
   若配置了 `require_exit_reason_for_ai_close=true`，则仅当 AI 输出的 `exit_reason` 为上述三者之一时才执行平仓；未填或非法值将被拒绝。

4. **延迟确认（现有逻辑）**  
   close_long/close_short 仍需连续 2 个周期均建议平仓才会真正执行，以减少单周期误判。

---

## 三、配置项摘要

| 配置项 | 含义 |
|--------|------|
| `allow_ai_close` | 是否允许执行 AI 的平仓建议；与系统动态止盈/止损并存 |
| `min_confidence_for_ai_close` | AI 建议平仓时的最低置信度；0 表示不额外要求 |
| `require_exit_reason_for_ai_close` | 是否要求 exit_reason 为 take_profit \| stop_loss \| prediction_mismatch 之一 |

前端在「风控」中可配置上述三项；默认预设中 `allow_ai_close=false`，由系统止盈/止损负责平仓。

---

## 四、小结

- **系统**：始终按配置执行开仓（方向池）、止盈、止损与由此带来的平仓。  
- **AI**：可建议开仓（受开仓约束）与平仓/止盈/止损（close + exit_reason，受置信度与退出原因约束）。  
- **二者并存**：先系统规则，后 AI 决策；AI 的平仓仅在「历史+实时+预测」下认为与预测不符、且满足约束时才会被执行。
