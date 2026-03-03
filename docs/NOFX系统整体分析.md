# NOFX 系统整体分析：代码结构、策略、参数与逻辑一致性

本文对 nofx 系统的**架构与调用链**、**策略配置来源**、**参数预设一致性**、**Prompt 与执行对齐**及**已发现矛盾与修正**做整体梳理，便于确认逻辑合理、消除矛盾。

---

## 一、架构与入口流程

### 1.1 启动与交易员加载

- **main.go**：`config.Init()` → `store.NewWithConfig()` → `traderManager.LoadTradersFromStore(st)` → `api.NewServer(..., traderManager).Start()`。交易循环由各交易员的 **AutoTrader** 在后台 runCycle 驱动，不集中在 main。
- **策略绑定**：交易员表 `strategy_id` → `store.Strategy.Get(userID, strategyID)` → `ParseConfig()` 得到 `*StrategyConfig`，**不做与 GetDefaultStrategyConfig 的字段级合并**，直接作为 `StrategyConfig` 传给 AutoTrader；缺失字段在运行时由 kernel 的 `getFloat64Value(ptr, default)` 等补默认值。

### 1.2 交易周期与 SL/TP 执行顺序

- **runCycle**（auto_trader.go）：先 **checkDynamicStopLossTakeProfit()**，再 buildTradingContext、GetFullDecisionWithStrategy（AI 决策）、执行开平仓。
- **checkDynamicStopLossTakeProfit**：
  - 从 `at.strategyEngine.GetConfig().RiskControl` 取 `DynamicStopLoss` / `DynamicTakeProfit`。
  - 对每个持仓：算 min_hold（取 SL 与 TP 的 MinHoldMinutes **较大值**）、peak、klines、ATR、支撑/阻力；若已锁利则判 breakeven 止损；再 `StopLossChecker.CheckStopLoss(...)`；再 `TakeProfitChecker.CheckTakeProfit(...)`；触发则 executeStopLoss / executeTakeProfit。
- **止损检查顺序**（kernel/stop_loss.go）：initial → trailing → ATR → support_resistance → adverse_exit；`trigger_logic == "any"` 时任一触发即返回。
- **止盈检查顺序**（kernel/take_profit.go）：scaled → trailing_tp → fixed → atr → resistance；任一触发即返回。

---

## 二、策略配置来源与默认值

- **来源**：DB 中该策略的 JSON（API 保存/编辑策略时写入）；「创建策略」时前端可请求 `GET /strategies/default-config?lang=` 得到 `GetDefaultStrategyConfig(lang)` 作为模板，保存后即写入 DB，**服务端不再做“与默认策略合并”**。
- **缺失字段**：DB 中未保存的字段解析后为 nil 或零值；kernel 内通过 `getFloat64Value(ptr, default)`、`getIntValue` 等回退，**不会因 nil 崩溃**，但行为会偏向 kernel 内建默认（与“完整预设”可能略有差异）。
- **前端**：`DynamicStopLossEditor` / `DynamicTakeProfitEditor` 的 `defaultConfig` 与 `currentConfig = config || defaultConfig` 仅影响 UI 占位与新建时的初始值；保存到 DB 的是用户当前编辑的 config，**与后端 GetDefaultStrategyConfig 无自动同步**，故前后端预设需人工对齐并文档化。

---

## 三、参数预设一致性（已统一与修正）

### 3.1 统一后的预设（截至本次分析）

| 参数 | 后端 GetDefaultStrategyConfig | 前端 SL defaultConfig | 前端 TP defaultConfig | kernel 回退默认 |
|------|-------------------------------|------------------------|------------------------|------------------|
| **min_hold_minutes (SL)** | 10 | **10**（已由 5 改为 10） | — | (用 config) |
| **min_hold_minutes (TP)** | 10 | — | 10 | (用 config) |
| **klines_timeframe** | "15m" | "15m" | — | auto_trader 空时 "15m" |
| **confirm_cycles / confirm_minutes** | 2 / 0 | 2 / 0 | — | (用 config) |
| **ATR SL (min/max)** | 1.5 / 2.5 | 1.5 / 3.5 | — | 1.5 / 3.5 |
| **ATR TP (min/max)** | 2.5 / 4.0 | — | 2.5 / **4.0**（已由 6.0 改为 4.0） | 2.0 / 4.0 |
| **LockProfitPercent** | 2.5 | — | 2.5 | (用 config) |
| **高波动阈值 (SL/TP)** | 1.2 | 1.2 | 1.2 | 1.2 |
| **回撤止盈 (activate/retrace/atrMult/close%)** | 未设 | — | 2/1.5/0.5/100 | 2.0/1.5/0.5/100 |

### 3.2 已修正的不一致

- **SL min_hold_minutes**：前端 defaultConfig 由 5 改为 **10**，与后端、TP 一致；输入框占位由 `?? 5` 改为 **?? 10**。
- **TP min_hold_minutes 占位**：输入框由 `?? 5` 改为 **?? 10**，与 defaultConfig 一致。
- **TP atr_multiplier_max**：前端 defaultConfig 由 6.0 改为 **4.0**，与后端、kernel 一致；展示用 `?? 4.0`。

---

## 四、Prompt 与执行对齐

- **注入**：`kernel/engine.go` 的 `BuildSystemPrompt()` 调用 `formatStrategyDynamicSLTP()`，将当前策略的 SL/TP 配置写入系统提示，供 AI 知晓「最小持仓、追踪止损、分层止盈、回撤止盈、ATR 倍数、高波动用 Max、锁定利润」等。
- **一致性**：formatStrategyDynamicSLTP 输出的「有无/档位/倍数/阈值」与 kernel 的 CheckStopLoss / CheckTakeProfit 使用方式一致；**执行顺序**以代码为准（SL：initial→trailing→ATR→support_resistance→adverse_exit；TP：scaled→trailing_tp→fixed→atr→resistance），prompt 未写顺序，但各模块描述与实现一致。
- **建议**：若需 AI 更明确“先看止损再看止盈、止盈内先分层再回撤再固定”，可在 prompt 中补一句简短说明。

---

## 五、口径与潜在混淆点（已文档化）

### 5.1 价格 % 与 保证金 %

- **止盈档位、回撤止盈、MinProfitPercentToAllowTP、固定止盈、阻力止盈**：均为 **价格相对入场价的变动 %**（(price - entry)/entry×100）。
- **LockProfitPercent（锁利）**：在 **auto_trader** 中与 **pnlPct（保证金收益率 = unrealizedPnl/marginUsed×100）** 比较；达此保证金 % 后，回撤到入场价时触发 breakeven 止损。
- **页面「当前盈亏%」**：一般为保证金 %（含杠杆）；故与「价格 %」的止盈档位不同，需在 UI 或文档中说明（已在前端止盈编辑处增加「所有止盈档位均为价格变动%」的说明）。
- **建议**：在策略说明或帮助中明确写「LockProfitPercent 为保证金收益率 %；其余止盈档位/回撤止盈为价格 %」。

### 5.2 回测与实盘

- **Trailing 峰值**：回测已改为自开仓以来的 **running high/low**（backtest/runner.go 中按 klines 与 pos.OpenTime 计算），与实盘一致。

---

## 六、逻辑合理性小结

| 项目 | 结论 |
|------|------|
| **配置来源** | 单一来源（DB 策略 JSON），无服务端合并默认，缺失由 kernel 回退；合理。 |
| **SL/TP 共用 min_hold** | 取两者较大值，避免任一侧过短即参与；前后端已统一为 10，合理。 |
| **检查顺序** | SL 先于 TP；SL 内 initial→trailing→ATR→S/R→adverse；TP 内 scaled→trailing_tp→fixed→atr→resistance；与设计一致。 |
| **Prompt 与执行** | formatStrategyDynamicSLTP 与 kernel 行为一致；顺序以代码为准，无矛盾。 |
| **参数预设** | 后端预设、前端 defaultConfig、kernel 回退已对齐关键项（min_hold、ATR TP max、高波动阈值等）；其余见 `止盈策略参数预设与说明.md`、`止损策略参数默认值与整体分析.md`。 |
| **价格% vs 保证金%** | 执行与文档已区分；LockProfitPercent 为保证金 %，其余止盈为价格 %，需在说明中写清。 |

---

## 七、已修正与建议

- **已修正**：SL/TP 前端 min_hold 默认与占位统一为 10；TP atr_multiplier_max 统一为 4.0；SL min_hold 前端默认 10 与后端一致。
- **建议**：  
  - 在策略/帮助文案中明确「LockProfitPercent = 保证金 %；止盈档位 = 价格 %」。  
  - 若需，在 prompt 中补一句「先执行止损检查再止盈；止盈内先分层再回撤再固定再 ATR 再阻力」。  
  - 新建策略时若前端希望与后端预设完全一致，可优先使用「从默认模板创建」并保存全部字段，避免仅保存部分字段导致更多依赖 kernel 默认。

---

本文档与 `止盈策略参数预设与说明.md`、`止损策略参数默认值与整体分析.md` 一起，构成 nofx 策略与参数一致性的整体依据；后续改动预设或执行顺序时，建议同步更新本文档与两篇参数文档。
