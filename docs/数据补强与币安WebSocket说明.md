# 数据补强现状、其他途径与币安 WebSocket 说明

本文档基于「以币安为主、不用 CoinGlass」的选型，说明：**还有哪些补强数据未接入**、**有无其他获取途径**，以及**币安 WebSocket 是什么、能否应用到 nofx 系统**。

---

## 一、补强数据未接入清单（相对文档建议）

| 建议数据 | 优先级 | 币安能否获取 | 当前 nofx 状态 | 说明 |
|----------|--------|--------------|----------------|------|
| **多空比 / 大户多空比** | 高 | ✅ REST 有 | ✅ 已接入 | `provider/binancedata`，buildTradingContext 已拉取并写 Context、提示词。 |
| **Taker 买卖比** | 中高 | ✅ REST 有 | ✅ 已接入 | 同上，`takerlongshortRatio`。 |
| **资金费率（当前/下一档）** | 中 | ✅ REST 有 | ✅ 已接入 | premiumIndex，已在 BinanceFundingMap。 |
| **资金费率历史（近 8h 均值等）** | 中 | ✅ REST 有 | ⚠️ 未接入 | `binancedata` 已有 `GetFundingRateHistory`，但未在 buildTradingContext 中拉取、未写入 Context/提示词，也未在策略配置中加开关。 |
| **爆仓/清算量（历史或聚合）** | 高 | ❌ REST 无；✅ WS 有实时流 | ❌ 未接入 | 币安 REST **没有**「某时段爆仓金额/笔数」历史接口；只有 WebSocket **强平订单流** `@forceOrder`（实时单笔）。要做「近 1h/4h 爆仓汇总」需自建：WS 订阅后按时间窗口聚合，或用第三方。 |
| **BTC/ETH 占比、山寨季节** | 中 | ❌ 无 | ❌ 未接入 | 需第三方（如 CoinGecko `/api/v3/global`、Bitbo 等）。 |
| **永续-现货价差 Basis** | 中 | ✅ 可自算 | ❌ 未接入 | 永续 ticker + 现货 ticker 自算溢价率即可，无需单独 API。 |
| **稳定币流入流出** | 低 | ❌ 无 | ❌ 未接入 | 需链上/第三方（CryptoQuant、Bitquery 等）。 |

**小结（未接入）：**

- **仅用币安、且不接 WS 时**：缺「爆仓/清算量」的历史或聚合、「资金费率历史/近 8h 均值」、「BTC 占比」、「Basis」。
- **资金费率历史**：接口已有，只差在策略里拉取并写入 Context/提示词。
- **Basis**：用现有永续+现货价格即可在系统内自算。
- **爆仓**：要么用币安 WebSocket 强平流自聚合，要么用其他平台（见下）。

---

## 二、其他获取途径（不用 CoinGlass 时）

| 数据类型 | 其他途径 | 说明 |
|----------|----------|------|
| **爆仓/清算** | **币安 WebSocket** | 订阅 `@forceOrder` 或全市场 `!forceOrder@arr`，在本地按 1h/4h 等窗口聚合多空爆仓量与笔数，用于 regime/risk_alert。 |
| | **CoinAnk** | 项目内已有 `provider/coinank`：`LiquidationHistory`、`LiquidationCoinAggHistory`、`LiquidationRank` 等，若贵司有 CoinAnk 权限可直接接入，作为「历史/排行」补充。 |
| | **Bybit** | 若有 Bybit 交易或可接受多数据源，可查其是否提供清算相关 REST/WS（以官方文档为准）。 |
| **资金费率历史** | **币安 REST** | 已实现 `GetFundingRateHistory`，在策略层拉取并写入 Context 即可（含近 8h 均值等）。 |
| **BTC 占比** | **CoinGecko** | `GET https://api.coingecko.com/api/v3/global`，`market_cap_percentage.btc` 等，免费额度内可用。 |
| **Basis** | **自算** | 永续：`fapi/v1/ticker/price` 或 premiumIndex；现货：`api/v3/ticker/price`；Basis = (永续价 - 现货价) / 现货价。 |
| **稳定币流** | **CryptoQuant / Bitquery** | 多为付费或链上聚合，可作为后续扩展。 |

---

## 三、币安 WebSocket 是什么

币安 U 本位合约 **WebSocket 市场流** 提供实时推送，与 REST 互补：

- **基础地址**：`wss://fstream.binance.com`
- **常用用法**：  
  - 组合流：`wss://fstream.binance.com/stream?streams=btcusdt@markPrice/btcusdt@aggTrade`  
  - 单流：`wss://fstream.binance.com/ws/btcusdt@markPrice`
- **连接限制**：单连接 24 小时有效、需处理 ping/pong；单连接最多 1024 个流；每秒最多 10 条下行消息等（见官方文档）。

**常见流类型（与数据补强相关）：**

| 流 | 说明 | 用途 |
|----|------|------|
| `@kline_<interval>` | K 线 | 实时 K 线，可减少 REST kline 轮询。 |
| `@markPrice` 或 `@markPrice@1s` | 标记价 | 实时标记价，可用于盯盘、风控。 |
| `@aggTrade` | 成交 | 逐笔成交，可自算 Taker 量等。 |
| **`@forceOrder`** | **强平订单** | **每条为一次强平事件（多/空、数量、价格、时间），是币安侧「爆仓」数据的唯一实时来源；REST 无此数据。** |
| `!forceOrder@arr` | 全市场强平 | 所有标的的强平合并推送。 |
| `@ticker` / `@miniTicker` | 24h 汇总 / 精简 | 涨跌幅、成交量等。 |

**与 REST 的对比：**

- **爆仓/清算**：REST 无；只有 WebSocket 的 `@forceOrder` / `!forceOrder@arr`。
- **多空比、Taker 比、资金费率（当前/历史）**：仅 REST 提供，WS 无对应流。
- **标记价、K 线、成交**：WS 有实时流，可减少 REST 轮询、降低延迟。

---

## 四、WebSocket 如何应用到 nofx 系统

**适用场景：**

1. **爆仓数据补强（推荐）**  
   - 订阅 `@forceOrder`（单标的）或 `!forceOrder@arr`（全市场）。  
   - 在 nofx 内维护一个「强平事件缓存」（如按 1h/4h 时间窗），按 symbol、side 聚合笔数/金额。  
   - 每个策略周期从缓存读「近 1h/4h 多空爆仓量」，写入 `Context`，供 pipeline 或 AI（market_summary、market_regime、risk_alert）使用。  
   - 这样无需 CoinGlass 也能得到「近期爆仓规模」的近似。

2. **实时标记价 / 最新价**  
   - 订阅 `@markPrice`（或 `@markPrice@1s`），写入内存缓存。  
   - 止盈止损、风控或展示逻辑优先读 WS 缓存，缺失时再回退 REST。可降低延迟、减少 REST 调用。

3. **实时 K 线（可选）**  
   - 订阅 `@kline_1m` 等，在内存中维护最新一根或几根 K 线。  
   - 用于需要「当前周期实时 K 线」的模块，减少对 REST kline 的轮询。

**实现要点：**

- **常驻连接**：在 trader 或独立 data 服务中起一个 goroutine 维护 WS 连接，断线重连（并注意 24h 重连策略）。
- **与现有周期解耦**：WS 只负责「写缓存」；`buildTradingContext` 仍按现有周期执行，从缓存读 WS 产出的数据（强平聚合、标记价等）。
- **配置**：可在策略或全局配置中增加「是否启用 Binance WS」「订阅流列表」（如 `forceOrder`、`markPrice`），便于开关与扩展。

**结论：**  
币安 WebSocket 可以且适合应用到 nofx：**爆仓数据**用 WS 是当前唯一不依赖 CoinGlass 的可行途径；**标记价/K 线**用 WS 可减轻 REST 压力、提升实时性。

---

## 五、建议落地顺序（仅用币安 + 不买 CoinGlass）

1. **资金费率历史**：在 buildTradingContext 中调用已有 `GetFundingRateHistory`，算近 8h 均值（或最近 N 档），写入 Context 与提示词；策略加开关。  
2. **Basis**：用现有或新拉取的永续价与现货价在系统内算 Basis，写入 Context/提示词。  
3. **爆仓**：接入币安 WS `@forceOrder` / `!forceOrder@arr`，在本地做时间窗口聚合，结果写入 Context，供 AI 与风控使用。  
4. **BTC 占比**：接 CoinGecko `/global`（或其它免费 API），写入 Context，供宏观判断。  
5. **可选**：WS 标记价/K 线缓存，用于止盈止损与展示，降低 REST 依赖。

若后续引入 CoinAnk 且有权限制，可将其清算接口作为「历史/排行」的补充，与币安 WS 实时强平一起使用。

---

## 六、CoinAnk 套餐1 能否满足缺失数据

**结论：套餐1 能部分满足「爆仓/清算」类数据，不能覆盖全部补强项；其余补强（资金费率历史、Basis、BTC 占比）已用币安 + CoinGecko 在 nofx 内实现。**

| 补强数据           | 套餐1 是否提供 | 说明 |
|--------------------|----------------|------|
| 交易所清算统计     | ✅ 是           | `/api/liquidation/allExchange/intervals`（按 baseCoin 返回 1h/24h 多空清算金额）属套餐1，nofx 已接入为「CoinAnk 清算」可选数据源。 |
| 清算热图支持交易对 | ✅ 是           | 套餐1 支持清算热图相关接口。 |
| 累计资金费率       | ✅ 是           | 套餐1 提供累计资金费率，可与币安「资金费率历史」互补。 |
| 爆仓排行榜         | ❌ 否           | 爆仓排行榜（LiquidationRank）为**套餐2**，套餐1 不包含。 |
| 资金费率近 8h 均值 | —               | 由**币安 REST** `GetFundingRateHistory` 在 nofx 内计算，不依赖 CoinAnk。 |
| Basis / BTC 占比   | —               | 由**币安现货+永续**自算、**CoinGecko** 在 nofx 内实现，不依赖 CoinAnk。 |
| 强平实时流 1h/4h   | —               | 由**币安 WebSocket** `@forceOrder` 在 nofx 内聚合，不依赖 CoinAnk。 |

**使用建议：**

- 若仅有 **CoinAnk 套餐1**：可开启「CoinAnk 清算」并配置 `coinank_api_key`（及可选 `coinank_url`），使用交易所清算统计（1h/24h）作为强平维度的补充；爆仓排行榜需升级套餐2 或改用币安 WS 强平聚合。
- nofx 已实现的补强（资金费率历史、Basis、BTC 占比、币安 WS 强平）不依赖 CoinAnk，**套餐1 主要用于补充「清算统计」这一项**；其余缺失数据已通过币安 + CoinGecko + 自算在系统内补齐。

---

*文档版本：1.1，与《交易数据增强与市场判断》《交易数据API接口列举》配套。*
