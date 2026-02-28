# 减少 AI 调用 Token 用量的方法

本文档汇总当前 **Token 消耗来源**，以及可落地的 **缩减手段**（配置、策略、代码级），便于在控制成本的前提下保留必要信息。

---

## 一、Token 主要消耗在哪里

| 来源 | 说明 | 大致占比（视配置而定） |
|------|------|------------------------|
| **System Prompt** | 数据字典 + 市场状态识别 + 多周期指南 + 交易场景示例（Schema）+ 角色/入场/决策流程 | 固定、较长；若 API 支持 **system prompt 缓存**可显著省成本 |
| **User Prompt - 账户与持仓** | 账户信息、当前持仓列表及浮盈/持仓时长等 | 固定、较小 |
| **User Prompt - 候选币行情** | 每个币种 × 每个周期：K 线表（OHLCV 逐行）+ EMA/MACD/RSI/ATR 序列 | **最大**：币数 × 周期数 × (K 线行数 + 指标数组) |
| **User Prompt - 量化/排行** | 量化数据、OI 排行、资金流排行、涨跌幅排行（表格） | 中等 |
| **AI 输出** | 思维链 + JSON 决策 | 受 max_tokens 限制，通常为输出侧成本 |

因此，**缩减重点**在：候选币数量、每币每周期的 K 线/指标篇幅、以及排行/量化数据条数。

---

## 二、可用的缩减手段（按实施难度）

### 2.1 仅改配置即可（无需改代码）

| 手段 | 操作 | 效果 |
|------|------|------|
| **K 线数量** | 策略 → 市场数据 → **K线数量** 设为 **20 或更小**（如 15、10） | 每周期 K 线行数及指标数组长度同步减少，Token 明显下降；你已用 20，可酌情再降到 15。 |
| **时间周期数** | 策略 → 时间周期 → 只勾选 **3 个**（如 15m、1h、4h），去掉 5m/30m 等 | 少一个周期就少一整块「K 线表 + 指标序列」，效果明显。 |
| **OI / 资金流 / 涨跌幅排行** | 策略 → 量化数据与排行 → 将 **OI 排行**、**资金流排行**、**涨跌幅排行** 的 **条数** 从 10 改为 **5** | 表格行数减半，总 Token 减少。 |
| **关闭不必要的指标** | 策略 → 技术指标 → 关闭 BOLL、或 MACD/RSI 中不常用的 | 每个周期少 1～2 组指标序列，略减 Token。 |

### 2.2 需要改代码或配置项（已实现或可加）

| 手段 | 说明 | 效果 |
|------|------|------|
| **限制候选币数量** | 当前实盘/模拟最多 **8** 个候选币写入 Prompt；可改为 **5 或 4**（见下「配置项 max_coins_in_prompt」） | 候选币数 × 每币数据量，Token 线性下降。 |
| **非主周期紧凑输出** | 仅**主周期**输出完整 K 线表 + 指标序列；**非主周期**只输出一行摘要（最新价、EMA20/50、ATR14）（见下「紧凑模式」） | 每币少 2～3 个周期的完整表格与序列，Token 大幅下降。 |

### 2.3 依赖 API / 产品能力

| 手段 | 说明 |
|------|------|
| **System Prompt 缓存** | 若调用方支持「系统提示缓存」（如 OpenAI / 部分兼容 API），System Prompt 只计一次或按缓存价计费，可显著降低重复调用的 Token 成本。 |
| **使用英文 Prompt** | 部分模型下英文描述比中文略短；若策略/系统支持英文，可尝试切换语言以略减 Token。 |

---

## 三、配置项说明（代码中已支持或可加）

### 3.1 候选币数量上限（max_coins_in_prompt）✅ 已实现

- **含义**：写入 User Prompt 的**候选币**（不含当前持仓）最多多少个。
- **配置**：策略 → K 线/市场数据中的 **`max_coins_in_prompt`**（JSON 键 `max_coins_in_prompt`）。为 0 或不填时默认 8；设为 **5 或 4** 可减 Token。
- **代码**：`store/strategy.go` 的 `KlineConfig.MaxCoinsInPrompt`；`kernel/engine.go` 的 `fetchMarketDataWithStrategy` 中按该值截断候选币。

### 3.2 非主周期紧凑模式（compact_non_primary_timeframe）✅ 已实现

- **含义**：仅**主周期**输出完整 K 线表 + 完整指标序列；**非主周期**只输出一行摘要（Close、EMA20、EMA50、ATR14），不输出整张 K 线表和长数组。
- **配置**：策略 → 技术指标中的 **`compact_non_primary_timeframe`**（JSON 键 `compact_non_primary_timeframe`）。设为 true 即开启。
- **代码**：`store/strategy.go` 的 `IndicatorConfig.CompactNonPrimaryTimeframe`；`kernel/engine.go` 的 `formatMarketData` 中非主周期调用 `formatTimeframeSeriesDataCompact` 输出单行摘要。

---

## 四、推荐组合（在兼顾分析质量下尽量省 Token）

1. **K 线数量**：保持 **20**（或 15），不再增加。
2. **时间周期**：只选 **3 个**（如 15m、1h、4h），主周期 15m。
3. **候选币数量**：在策略 `klines` 中设置 **`max_coins_in_prompt`: 5**（或 4），默认 8。
4. **排行条数**：OI / 资金流 / 涨跌幅排行各 **5** 条（若策略支持）。
5. **非主周期**：在策略 `indicators` 中设置 **`compact_non_primary_timeframe`: true**，开启紧凑输出。
6. **System Prompt**：若 API 支持，开启 **缓存**。

若需进一步压缩，可再：把 K 线数量降到 15、候选币降到 4、或关闭 BOLL / 部分量化数据。

---

## 五、小结

- **不改代码**：调小 K 线数量、减少时间周期数、减少排行条数、关闭不必要指标，即可明显减少 Token。
- **已实现配置**：**max_coins_in_prompt**（klines）、**compact_non_primary_timeframe**（indicators）已在代码中支持，在策略 JSON 中填写即可生效。
- **依赖 API**：使用 System Prompt 缓存、必要时用英文，可再降成本。

以上手段可组合使用；优先建议：K 线数量 20、时间周期 3 个、max_coins_in_prompt=5、compact_non_primary_timeframe=true。
