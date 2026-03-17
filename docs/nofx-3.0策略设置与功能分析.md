# nofx 系统 3.0 策略设置与各功能分析

本文档基于代码与官方文档，梳理 **nofx 3.0 策略** 的配置项、功能模块及设计思路。

---

## 一、3.0 策略定位与版本

- **版本标识**：`StrategyConfig.Version = "3.0"`（见 `store/strategy.go` 中 `GetDefaultStrategyConfig`）
- **策略类型**：`strategy_type: "ai_trading"`（另有 `grid_trading` 网格策略，此处仅讨论 AI 交易）
- **核心理念**：多周期共振开仓（4h 定方向 → 1h 确认 → 15m 入场）+ **AI 仅开仓** + **策略动态 SL/TP 执行平仓**，兼顾趋势与震荡市（区间边缘开仓、拿住仓、盈利后平仓）

---

## 二、策略配置结构总览

### 2.1 顶层结构（StrategyConfig）

| 字段 | 说明 |
|------|------|
| `version` | 预设版本，如 "3.0" |
| `language` | "zh" / "en"，影响提示词与数据展示语言 |
| `coin_source` | 币种来源（见下） |
| `indicators` | K 线、技术指标、量化数据（见下） |
| `risk_control` | 风控与动态止损/止盈（见下） |
| `prompt_sections` | 可编辑的系统提示词片段（角色、频率、入场、决策流程） |
| `custom_prompt` | 追加在系统提示词末尾的自定义说明与场景 |
| `grid_config` | 仅当 `strategy_type == "grid_trading"` 时使用 |

---

## 三、币种来源（coin_source）

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `source_type` | `"ai500"` | 可选：`static` / `ai500` / `oi_top` / `oi_low` / `mixed` |
| `use_ai500` | true | 使用 AI500 评分池 |
| `ai500_limit` | 10 | AI500 取前 N 个币 |
| `use_oi_top` | false | 持仓增加榜（偏多） |
| `oi_top_limit` | 10 | OI Top 数量 |
| `use_oi_low` | false | 持仓减少榜（偏空） |
| `oi_low_limit` | 10 | OI Low 数量 |
| `static_coins` | [] | source_type=static 时的手动列表 |
| `excluded_coins` | [] | 全局排除的币种 |

**数据流**：候选币用于拉取 K 线、指标、OI、资金流等，并写入 AI 的 User Prompt；数量受 `indicators.klines.max_coins_in_prompt`（默认 8）限制以控制 Token。

---

## 四、指标与 K 线（indicators）

### 4.1 K 线（klines）

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `primary_timeframe` | **"15m"** | 主周期，与「15m 找入场」一致 |
| `primary_count` | 30 | 主周期 K 线根数 |
| `longer_timeframe` | "4h" | 大周期 |
| `longer_count` | 10 | 大周期根数 |
| `enable_multi_timeframe` | true | 多周期分析 |
| `selected_timeframes` | `["15m","1h","4h"]` | 参与分析的时间框架 |
| `max_coins_in_prompt` | 8 | 候选币写入 prompt 上限，0 则内核用 8 |

设计目的：**先 4h/1h 定方向，再 15m 找入场**；提示词中会写入 4h/1h 方向及「是否同向」等字段。

### 4.2 技术指标开关

| 参数 | 3.0 默认 | 用途 |
|------|----------|------|
| `enable_raw_klines` | true | 原始 OHLCV，AI 分析必需 |
| `enable_ema` | true | 趋势/结构（如 EMA20） |
| `enable_macd` | true | 动能 |
| `enable_rsi` | true | 超买超卖 |
| `enable_atr` | true | 波动率，用于动态 SL/TP 倍数 |
| `enable_boll` | false（默认）/ true（优化版） | 布林带 |
| `enable_volume` | true | 成交量 |
| `enable_oi` | true | 持仓量 |
| `enable_funding_rate` | true | 资金费率 |
| `ema_periods` | [20, 50] | EMA 周期 |
| `rsi_periods` | [7, 14] | RSI 周期 |
| `atr_periods` | [14] 或 [3, 14] | ATR 周期 |

### 4.3 量化与排行数据（NofxOS API）

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `nofxos_api_key` | 预设 key | 统一访问 NofxOS 数据源 |
| `enable_quant_data` | true | 量化数据总开关 |
| `enable_quant_oi` | true | OI 数据 |
| `enable_quant_netflow` | true | 资金流 |
| `enable_oi_ranking` | true | OI 排行 |
| `oi_ranking_duration` | "1h" | 1h/4h/24h |
| `oi_ranking_limit` | 10 | 条数 |
| `enable_netflow_ranking` | true | 资金流排行 |
| `netflow_ranking_duration` | "1h" | 同上 |
| `enable_price_ranking` | true | 涨跌幅排行 |
| `price_ranking_duration` | "1h,4h,24h" | 多周期 |
| `price_ranking_limit` | 10 | 条数 |

---

## 五、风控（risk_control）

### 5.1 仓位与杠杆（代码强制 / AI 引导）

| 参数 | 3.0 默认 | 类型 | 说明 |
|------|----------|------|------|
| `max_positions` | 3 | **CODE ENFORCED** | 同时最多持仓数 |
| `btc_eth_max_position_value_ratio` | 5.0 | **CODE ENFORCED** | BTC/ETH 单仓最大 = 权益 × 该比例 |
| `altcoin_max_position_value_ratio` | 1.0 | **CODE ENFORCED** | 山寨币单仓最大 = 权益 × 该比例 |
| `max_margin_usage` | 0.9 | **CODE ENFORCED** | 最大保证金使用率 90% |
| `min_position_size` | 12 | **CODE ENFORCED** | 最小仓位（USDT） |
| `btc_eth_max_leverage` | 5 | AI 引导 | BTC/ETH 建议最大杠杆 |
| `altcoin_max_leverage` | 5 | AI 引导 | 山寨币建议最大杠杆 |
| `min_risk_reward_ratio` | 3.0 | 执行校验 | 开仓时盈亏比 ≥ 3:1 才通过 |
| `min_confidence` | 70 | AI 引导 | 建议最低置信度，平衡机会与质量 |

### 5.2 AI 仅开仓（ai_only_entry）

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `ai_only_entry` | **true** | 为 true 时：**不执行** AI 的 `close_long`/`close_short`；平仓完全由策略动态止损/止盈执行；AI 对已有仓输出 `hold` 并附带 `trend_view`（reversing/trend_intact/choppy），用于调节止损确认周期 |

这是 3.0 的重要特性：**开仓由 AI 决策，平仓由策略规则执行**，便于在震荡市拿住仓、盈利后按分层/追踪平仓。

---

## 六、动态止损（dynamic_stop_loss）

### 6.1 总开关与触发逻辑

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `enabled` | true | 启用动态止损 |
| `trigger_logic` | `"any"` | 多个条件：`any`=任一触发即平仓，`all`=全部触发才平仓 |
| `min_hold_minutes` | 10 | 最小持仓分钟数，未满不参与止损，避免开仓即触发 |
| `initial_stop_percent` | 0 | 3.0 预设**无固定初始止损**（0），仅用追踪/ATR/支撑阻力等 |
| `confirm_cycles` | 2 | 连续 N 个周期满足条件才执行，减少单 K 假跌破 |
| `confirm_minutes` | 0 | >0 时按真实分钟数确认；0 则用 confirm_cycles |
| `klines_timeframe` | "15m" | 止损计算所用 K 线周期，可选 "1h" 减毛刺 |

### 6.2 追踪止损（分层）

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `trailing_enabled` | true | 启用追踪止损 |
| `trailing_levels` | [2.5%/1.5%, 6%/2.5%] | 盈利达 2.5% 时允许回撤 1.5% 触发；达 6% 时允许回撤 2.5% |

即：盈利越高，允许回撤越大，既锁利又给趋势空间。

### 6.3 ATR 止损（动态区间）

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `atr_enabled` | true | 启用 ATR 止损 |
| `atr_multiplier_min` | 1.5 | AI 在区间内选倍数，低波动偏小 |
| `atr_multiplier_max` | 2.5 | 高波动偏大，避免被震出 |
| `atr_period_btc_eth` | 20 | BTC/ETH 的 ATR 周期 |
| `atr_period_altcoin` | 14 | 山寨币 ATR 周期 |
| `atr_tolerance_enabled` | true | 高波动时更宽容（多 1 确认等） |
| `atr_high_multiplier` | 1.2 | 当前 ATR > 长期 ATR×1.2 视为高波动 |

### 6.4 支撑/阻力止损（可选）

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `support_resistance_enabled` | false | 默认关，按需开启 |
| `support_resistance_buffer` | 0.5~0.8% | 支撑/阻力下方缓冲 |

### 6.5 逆势早退（从未浮盈 + 反向波动）

| 参数 | 说明 |
|------|------|
| `adverse_exit_when_never_profit_atr` | 从未浮盈且价格反向 ≥ N×ATR 时触发止损，0 表示关闭 |
| `adverse_exit_when_never_profit_atr_altcoin` | 山寨币单独倍数（可选） |
| `adverse_exit_require_atr_spike` | 是否要求当前 ATR 相对长期放大才触发，减少震荡误杀 |

### 6.6 trend_view 调节（与 AI 联动）

- AI 对持仓输出 `trend_view`：`reversing` / `trend_intact` / `choppy`
- **reversing**：减少确认周期（如 requiredCycles-1，最少 1），加快止损
- **trend_intact / choppy**：增加确认周期，避免噪音止损

---

## 七、动态止盈（dynamic_take_profit）

### 7.1 总开关与过滤

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `enabled` | true | 启用动态止盈 |
| `min_hold_minutes` | 10 | 与止损一致，未满不参与止盈 |
| `min_profit_percent_to_allow_tp` | nil | 当前浮盈（价格%）低于此值不触发任何止盈；0/nil 表示不限制 |

### 7.2 分层止盈（scaled_levels）— 3.0 核心

| 档位 | 3.0 默认 | 说明 |
|------|----------|------|
| 第一档 | 2.5% 盈利 → 平 25% 仓 | 早锁利，不移动止损（move_stop_to_breakeven: false） |
| 第二档 | 6% 盈利 → 再平 25%，并移动止损至成本 | 锁本 |
| 第三档 | 10% 盈利 → 平 100% | 全部落袋 |

即 **2.5% / 6% / 10%** 三档分批止盈，与文档中「震荡市拿住仓、盈利后平仓」一致。

### 7.3 固定止盈 / ATR 止盈 / 阻力止盈

| 参数 | 3.0 默认 | 说明 |
|------|----------|------|
| `fixed_enabled` | 可选 | 固定百分比止盈 |
| `scaled_enabled` | true | 上述分层止盈 |
| `atr_enabled` | true | ATR 止盈倍数区间 |
| `atr_multiplier_min` / `atr_multiplier_max` | 2.5 / 4.0 | 强趋势用高倍 |
| `atr_use_max_in_high_volatility` | true | 高波动时用 Max 倍数 |
| `atr_high_volatility_threshold` | 1.2 | 判定高波动的 ATR 比值 |
| `resistance_enabled` | 可选 | 阻力位止盈 |
| `lock_profit_percent` | 2.5 | 盈利达 2.5% 后，回撤到成本时按 breakeven 止损 |

### 7.4 回撤止盈（Trailing TP）

| 参数 | 说明 |
|------|------|
| `trailing_tp_enabled` | 从峰值回撤一定比例或 ATR 时止盈 |
| `trailing_tp_activate_profit_pct` | 达到该盈利%才激活回撤止盈 |
| `trailing_tp_retrace_pct` | 从峰值回撤%触发 |
| `trailing_tp_retrace_atr_mult` | 或按 ATR 倍数回撤触发 |
| `trailing_tp_confirm_minutes` | 条件需持续分钟数再执行 |
| `trailing_tp_close_percent` | 触发时平仓比例 |

---

## 八、提示词（prompt_sections + custom_prompt）

### 8.1 可编辑片段（prompt_sections）

| 片段 | 3.0 内容要点 |
|------|----------------|
| **role_definition** | 专业加密货币交易 AI（优化版 v3.0），多周期、OI、资金流、动态风控 |
| **trading_frequency** | 每天约 2–4 笔、单笔持仓 ≥30–60 分钟、系统 2.5%/6%/10% 分批止盈与追踪止损 |
| **entry_standards** | 多周期共振、OI/资金流确认、禁止 4h 逆势、禁止 OI 减少当突破做多；推理顺序：4h 趋势 → 1h 趋势 → 同向才开仓 → 15m 入场（多支撑/回调，空阻力/反弹）；震荡市可在区间边缘开仓、拿住仓、盈利后平仓 |
| **decision_process** | 检查持仓 → 扫描候选 → 多时间框架 4 步（4h→1h→同向→15m 入场）→ OI/资金流确认 → 输出思维链 + JSON |

### 8.2 自定义提示词（custom_prompt）

- 市场状态识别：趋势判断、OI 解读、资金费率、资金流
- 动态止损/止盈 ATR 倍数选择原则（止损 1.5–2.5×、止盈 2.5–4.0×，高波动用大倍数）
- 交易场景示例：强势突破、假突破、趋势反转、震荡市等
- 系统自动功能说明：分批止盈 2.5%/6%/10%、追踪止损、ATR 动态止损、连续确认、高波动宽容、锁利 2.5% 等

---

## 九、执行流程简述（与策略 3.0 对应）

1. **每周期**：拉取持仓与候选、构建行情（含 4h/1h 方向）、OI/资金流等。
2. **有候选**：调用 AI 决策；解析 `trend_view` 按持仓传给动态 SL/TP。
3. **无候选**：仍对持仓执行动态 SL/TP（不依赖 AI）。
4. **动态 SL/TP**：先最小持仓过滤 → 锁利 breakeven → 止损（初始→追踪→ATR→支撑阻力→逆势早退）→ 止盈（分层→回撤止盈→固定→ATR→阻力）；`trend_view` 仅调节止损确认周期。
5. **决策执行**：排序（先平后开）；若 `ai_only_entry==true`，不执行 AI 的 close，只执行开仓与策略触发的平仓。

---

## 十、各功能小结表

| 功能模块 | 主要作用 |
|----------|----------|
| **币种来源** | 决定候选池（AI500/静态/OI 榜/混合），影响 AI 看到的标的 |
| **K 线与多周期** | 15m 主周期 + 1h/4h，先定方向再入场，控制 Token（max_coins_in_prompt） |
| **技术指标** | EMA/MACD/RSI/ATR 等供趋势与结构判断；ATR 用于 SL/TP 倍数 |
| **量化与排行** | OI、资金流、涨跌排行，强化多维度确认 |
| **风控** | 仓位上限、单仓比例、保证金、最小仓位、盈亏比/置信度约束 |
| **AI 仅开仓** | 平仓权交给策略，AI 只开仓 + 提供 trend_view |
| **动态止损** | 追踪分层 + ATR 区间 + 可选支撑阻力 + 逆势早退 + 连续确认 + 高波动宽容 |
| **动态止盈** | 2.5%/6%/10% 分层 + ATR 止盈 + 锁利 2.5% + 可选回撤止盈 |
| **提示词** | 角色、频率、入场、决策流程与场景，与 3.0 参数一致（2.5%/6%/10%、MinConfidence 70 等） |

---

## 十一、参考文件

| 内容 | 路径 |
|------|------|
| 策略配置结构、默认 3.0 预设 | `store/strategy.go`（StrategyConfig、GetDefaultStrategyConfig） |
| 策略模块数据流与执行 | `docs/architecture/STRATEGY_MODULE.zh-CN.md` |
| 动态止损设计 v3 | `docs/dynamic-stop-loss-design-v3.md` |
| 代码与参数、提示词一致性与预设建议 | `docs/代码结构与策略参数梳理.md` |
| 最新策略与盈利性分析 | `docs/最新策略与系统盈利性分析.md` |

---

**文档版本**：1.0  
**对应代码**：nofx 默认策略 3.0（GetDefaultStrategyConfig / GetOptimizedStrategyConfig）
