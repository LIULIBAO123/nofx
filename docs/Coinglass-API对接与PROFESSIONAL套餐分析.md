# Coinglass API 对接与 PROFESSIONAL 套餐分析

本文说明当前系统所需数据、是否可对接 Coinglass API，以及 **PROFESSIONAL 套餐**是否覆盖这些需求。

---

## 一、当前系统用到的数据需求

| 数据类型 | 用途 | 当前数据源 | 代码位置 |
|----------|------|------------|----------|
| **OI 排名 / OI 变动** | 候选币（oi_top/oi_low）、OI 排行、单标的 OI 与 delta | nofxos（nofxos.ai） | `provider/nofxos/oi.go`、`kernel/engine.go`（GetOITopPositions、getOITopCoins、getOILowCoins） |
| **资金费率** | 当前费率、8h 均值、历史 8 期；极端费率约束开仓 | Binance API（GetPremiumIndex）+ 可选 nofxos | `provider/binancedata`、`trader/auto_trader.go`、策略极端资金费率规则 |
| **多空比** | 全账户/大户多空比，方向池加成与 AI 参考 | Binance API（GetGlobalLongShortAccountRatio、GetTopLongShortAccountRatio） | `provider/binancedata`、`kernel/pipeline.go`（applyDirectionBonus） |
| **Taker 买卖比** | 大单方向，辅助判断 | Binance API（GetTakerLongShortRatio） | `provider/binancedata`、Context.BinanceTakerMap |
| **强平数据** | 近 1h/4h/24h 多空强平金额（美元），风险与情绪 | 币安 WS 或 CoinAnk | `LiquidationAggSnapshot`、数据补强 |
| **净流入排行** | 机构/个人合约资金流入流出榜，AI 与过滤 | nofxos（GetNetFlowRanking） | `provider/nofxos/netflow.go`、FetchNetFlowRankingData |
| **涨跌榜** | 全市场涨跌幅排行，AI 与过滤 | nofxos（GetPriceRanking） | `provider/nofxos/price.go`、FetchPriceRankingData |
| **K 线/价格** | 行情、技术指标、方向池 1h/4h 涨跌 | 交易所/行情接口（如 Binance） | `market`、`fetchMarketDataWithStrategy` |

---

## 二、Coinglass API 能提供什么

Coinglass API V4 提供**多交易所聚合**的衍生品与现货数据，与当前「单交易所（Binance）+ nofxos」形成互补或替代关系。

### 2.1 与系统需求对应的端点（摘要）

| 系统需求 | Coinglass API 能力 | 典型端点（V4） |
|----------|--------------------|----------------|
| **OI / OI 变动 / OI 排名** | 多所 OI、OHLC 历史、聚合历史、交易所列表 | `/api/futures/openInterest/ohlc-history`、`/api/futures/openInterest/aggregated-history`、`/api/futures/openInterest/exchange-list` |
| **资金费率** | 多所资金费率、OHLC、OI 加权、交易所列表 | `/api/futures/fundingRate/ohlc-history`、`/api/futures/fundingRate/oi-weight-ohlc-history`、`/api/futures/fundingRate/exchange-list` |
| **强平** | 强平历史、聚合（按合约/资产）、热力图 | `/api/futures/liquidation/history`、`/api/futures/liquidation/aggregated-history`、`/api/futures/liquidation/heatmap/model2` |
| **多空比** | 全账户 / 大户多空账户比、Taker 买卖量 | `/api/futures/global-long-short-account-ratio/history`、`/api/futures/top-long-short-account-ratio/history`、`/api/futures/taker-buy-sell-volume/history` |
| **价格 / K 线** | 现货与衍生品价格、OHLC 历史 | `/api/spot/price/history`、衍生品相关 price 端点 |
| **净流入 / 资金流** | 衍生品市场资金流、机构/大户行为 | 文档中提及 “derivatives market flows”；需在 Coinglass 文档中确认对应 “net flow” 的端点名称 |
| **涨跌榜** | 现货/衍生品市场列表、价格变动 | 可通过 `/api/spot/coins-markets`、价格历史与排序自行算；或查是否有现成 ranking 类端点 |

- **认证**：请求头 `CG-API-KEY`。
- **Base URL**：`https://open-api-v4.coinglass.com`（以官方文档为准）。

---

## 三、PROFESSIONAL 套餐是否包含“我们需要的”数据

### 3.1 套餐概览（来自 Coinglass 定价页）

| 项目 | HOBBYIST | STARTUP | STANDARD | **PROFESSIONAL** | ENTERPRISE |
|------|----------|----------|----------|-------------------|------------|
| 月费 | $29 | $79 | $299 | **$699** | 定制 |
| 数据端点数量 | 80+ | 130+ | 150+ | **160+** | 160+ |
| 限速（次/分钟） | 30 | 80 | 300 | **1,200** | 6,000 |
| 数据更新 | ≤1 分钟 | ≤1 分钟 | ≤1 分钟 | ≤1 分钟 | ≤1 分钟 |
| 历史数据（1h 间隔） | 180 天 | 360 天 | 720 天 | **720 天** | 720 天 |
| 日 K 历史 | 全历史 | 全历史 | 全历史 | **全历史** | 全历史 |

- PROFESSIONAL 与 ENTERPRISE 在**端点数量**上一致（160+），差异主要在限速与商务支持。
- 文档与介绍中明确列出的**衍生品核心能力**包括：Open Interest、Funding Rate、Liquidation、Long/Short Ratios、Taker 买卖量等，这些都在「160+ 端点」范围内，因此 **PROFESSIONAL 套餐可以访问我们需要的 OI、资金费率、强平、多空比、Taker 等接口**。

### 3.2 按“我们需要的”逐项看 PROFESSIONAL

| 我们需要的数据 | 是否在 Coinglass 中存在 | PROFESSIONAL 是否可用 | 说明 |
|----------------|-------------------------|------------------------|------|
| OI 与 OI 变动 / 排名 | ✅ 有 | ✅ 是 | 多所 OI、聚合、历史；可替代/补强 nofxos 的 OI 排行与单标的 OI。 |
| 资金费率（当前 + 历史） | ✅ 有 | ✅ 是 | 多所资金费率、OHLC、OI 加权；可替代/补强 Binance 单所。 |
| 强平（1h/4h/24h 聚合） | ✅ 有 | ✅ 是 | 强平历史与聚合；可替代/补强当前 WS/CoinAnk 强平数据。 |
| 全账户/大户多空比 | ✅ 有 | ✅ 是 | global / top long-short account ratio；可替代 Binance 多空比。 |
| Taker 买卖量 | ✅ 有 | ✅ 是 | taker-buy-sell-volume；可替代 Binance Taker。 |
| 净流入/资金流排行 | ⚠️ 需确认 | ⚠️ 需确认 | 文档有 “derivatives market flows”；需在 API 文档中查是否有“净流入排行”或等价端点及是否在 160+ 内。 |
| 涨跌榜（全市场） | ⚠️ 可自算或查端点 | ✅ 是 | 有现货/市场与价格数据；若无现成 ranking 端点，可用价格历史在应用层算排行。 |

**结论**：  
- **OI、资金费率、强平、多空比、Taker** 等核心衍生品数据，PROFESSIONAL 套餐**都具备**，且限速 1200/分钟对系统周期（如每分钟或数分钟拉一次）足够。  
- **净流入排行**需在 Coinglass 文档中确认对应端点与套餐权限；**涨跌榜**可用现有价格类接口在应用层实现，或确认是否有现成 ranking 接口。

---

## 四、通过 KeyStore 中转站对接（推荐）

若使用 **KeyStore** 作为 Coinglass 代理，请求先经 KeyStore 网关再转发至 Coinglass 官方 API，**请求/响应格式与官方完全一致**，业务逻辑无需改动，只需改 Base URL 与认证方式。

### 4.1 运行模式与流程

- **透明代理**：应用 → KeyStore 网关（校验 KeyStore API Key + 注入上游 Coinglass 凭证）→ Coinglass 官方 API → 响应原样回传应用。
- **对接要点**：仅需把 Coinglass 的 **Base URL** 换成 KeyStore 的代理地址；路径、Query、Body 与官方文档一致；**不再在应用内携带 Coinglass 的 CG-API-KEY**，由 KeyStore 注入。

### 4.2 Base URL 与版本路由

| 使用方式 | Base URL（REST） | 说明 |
|----------|------------------|------|
| **显式 V4（推荐）** | `https://www.keystore.com.cn/api/v1/proxy/coinglass/v4` | 对应官方 `https://open-api-v4.coinglass.com` |
| **显式 V3** | `https://www.keystore.com.cn/api/v1/proxy/coinglass/v3` | 对应官方 `https://open-api-v3.coinglass.com` |
| **自动路由** | `https://www.keystore.com.cn/api/v1/proxy/coinglass` | 不加 `/v3`/`/v4` 时，按 apis.json 匹配；匹配到则走 V4，否则 V3 |

**路径规则**：官方文档中的 path 直接接在 Base URL 后，**路径与 Query 参数与官方一致**，仅域名替换并加上版本前缀。  
示例：官方 `https://open-api-v4.coinglass.com/api/futures/open-interest/exchange-list?symbol=BTC`  
→ KeyStore `https://www.keystore.com.cn/api/v1/proxy/coinglass/v4/api/futures/open-interest/exchange-list?symbol=BTC`

**旧版兼容**：V3 的 camelCase 路径（如 `openInterest`、`fundingRate`）会被自动转为 V4 的 kebab-case（`open-interest`、`funding-rate`）并转发到 V4。

### 4.3 认证方式

- **请求头**：`X-Api-Key: <你的 KeyStore API Key>`  
- **说明**：`X-Api-Key` 由 **KeyStore 网关消费，不会转发到 Coinglass**；上游 Coinglass 凭证由 KeyStore 侧配置并注入，应用只需持有 KeyStore 的 Key。

### 4.4 参数转发规则

| 部分 | 行为 |
|------|------|
| Query 参数 | 原样转发到上游 |
| Path | 与官方文档一致，直接拼接 |
| Request Body（POST/PUT 的 JSON） | 原样转发 |
| 自定义 Header | 原样转发（**X-Api-Key 仅网关消费，不转发**） |

### 4.5 请求示例（cURL）

```bash
curl -X GET "https://www.keystore.com.cn/api/v1/proxy/coinglass/v4/api/futures/open-interest/exchange-list?symbol=BTC" \
  -H "X-Api-Key: YOUR_KEYSTORE_API_KEY"
```

### 4.6 WebSocket 推送（可选）

- **Endpoint**：`wss://www.keystore.com.cn/api/v1/ws/coinglass?api_key=YOUR_KEYSTORE_API_KEY`  
  或使用 Header：`X-Api-Key: YOUR_KEYSTORE_API_KEY`
- **订阅命令**（连接后发送 JSON）：
  ```json
  { "action": "subscribe", "channels": ["funding-rate", "liquidation", "open-interest", "price"] }
  ```
- **频道**：`funding-rate`（资金费率）、`liquidation`（大额清算）、`open-interest`（未平仓量）、`price`（综合价格），可用于实时补强行情与强平/费率。

### 4.7 代码侧对接要点

- 在 `provider/coinglass`（或统一 HTTP 客户端）中配置 **Base URL** 为可切换：
  - 直连：`https://open-api-v4.coinglass.com`，请求头带 `CG-API-KEY`
  - 中转：`https://www.keystore.com.cn/api/v1/proxy/coinglass/v4`，请求头带 `X-Api-Key`（KeyStore Key）
- 同一套 path、Query、Body 解析逻辑不变，仅根据配置选择 Base URL 和认证 Header。

---

## 五、对接思路（实现层面）

1. **新增 Coinglass 数据源**  
   - 新建 `provider/coinglass` 包；Base URL 支持直连 Coinglass 或 KeyStore 代理（见第四节）。  
   - 直连时用 `CG-API-KEY`，走 KeyStore 时用 `X-Api-Key`（KeyStore API Key）。  
   - 实现与现有结构兼容的 DTO 与转换（如 OI 排行 → `OIRankingData`/`OIPosition`，资金费率 → `FundingSnapshot`，多空比 → `BinanceLongShortSnapshot`，强平 → `LiquidationAggSnapshot`）。

2. **策略配置切换/互补**  
   - 在策略或配置中增加“数据源”选项，例如：  
     - OI/排行：nofxos **或** Coinglass；  
     - 资金费率/多空/Taker：Binance **或** Coinglass（或“优先 Coinglass，失败回退 Binance”）；  
     - 强平：当前 WS/CoinAnk **或** Coinglass 强平聚合。  
   - 这样可以在保留现有行为的前提下，逐步切换到或叠加 Coinglass。

3. **限速与缓存**  
   - PROFESSIONAL 1200 次/分钟；若多标的、多周期拉取，建议按“周期内合并请求”或“按标的/指标缓存”减少重复调用，避免触及限速。

4. **净流入与涨跌榜**  
   - 确认 Coinglass 文档中 “net flow” / “fund flow” 对应端点及套餐；涨跌榜若无现成接口，可用 Coinglass 价格/市场接口在应用层算排名并写入现有 `PriceRankingData` 结构。

---

## 六、Coinglass 是否存在本系统需要补强的参数（市场趋势与 AI 预测）

系统里 **市场趋势** 与 **AI 预测** 依赖：`market_regime`（trend_up/trend_down/ranging/high_volatility/reversal）、`scenario`（continuation/reversal/range）、`near_term_outlook`、以及方向与 `symbol_predictions`。Prompt 明确要求 AI 依据「多周期趋势、量能、OI/资金流、资金费率与强平」形成上述结论。下面按「当前已有」与「Coinglass 可补强」对照说明。

### 6.1 当前已用于趋势与预测的输入

| 输入 | 作用 | 数据源 |
|------|------|--------|
| OI 排行（增/减） | 资金聚集方向、趋势强度、候选币 | nofxos |
| 净流入排行（机构/个人、合约） | 资金流向、多空倾向 | nofxos |
| 涨跌榜 | 全市场强弱、情绪 | nofxos |
| 多空比（全账户/大户） | 持仓结构、情绪 | Binance |
| 资金费率（当前、8h 均值、近 8 期） | 持仓成本、极端时约束开仓 | Binance |
| Taker 买卖比 | 大单方向 | Binance |
| Basis（永续-现货价差） | 期现结构 | 计算自 Binance |
| BTC 市值占比 | 风险偏好、资金轮动 | 外部 |
| 强平聚合（1h/4h/24h 多空 USD） | 爆仓压力、情绪与风险 | 币安 WS / CoinAnk |
| K 线/技术指标（1h/4h 等同周期） | 趋势、方向、同向性 | 行情接口 |

### 6.2 Coinglass 可补强的参数（对市场趋势与 AI 预测）

以下均为**当前系统未用或仅单所/单一维度**，Coinglass 能提供且对 **market_regime、scenario、near_term_outlook、方向与置信度** 有直接补强作用。

| 补强项 | 说明 | 对趋势/预测的作用 | Coinglass 能力 |
|--------|------|-------------------|----------------|
| **多交易所聚合 OI / 资金费率 / 多空比** | 当前仅 Binance（及 nofxos 部分数据）；单所易有偏差 | 趋势与 regime 更稳、减少单所噪音；AI 判断「全市场」多空与资金成本更准 | 多所聚合 OI、funding、global/top long-short、taker；PROFESSIONAL 可用 |
| **OI 加权资金费率** | 当前只有单所费率或简单均值 | 更真实反映全市场持仓成本；对「极端资金费率」下的 regime 与开仓约束更准 | `fundingRate/oi-weight-ohlc-history` 等 |
| **强平热力图 / 关键价位** | 当前只有多空强平金额汇总，无价位信息 | 潜在支撑/阻力、集中爆仓区 → 补强 **scenario**（reversal）、**key_levels** 与 near_term_outlook | `liquidation/heatmap/model2`、liquidation 相关 history/aggregated |
| **ETF 资金流（BTC/ETH）** | 当前无 | 宏观资金进出 → 补强**趋势方向**与 **market_regime**（资金持续流入偏 trend_up，流出偏谨慎/reversal） | Bitcoin/ETH ETF flow history（文档有对应端点） |
| **恐惧贪婪指数** | 当前无 | 宏观情绪 → 补强 **market_regime**（如 high_volatility、reversal）与 **risk_alert**；极端恐惧/贪婪可提高置信度门槛 | Crypto Fear & Greed Index（文档有） |
| **更长历史区间** | 当前 nofxos/部分接口可能窗口较短 | 更长 1h/4h 序列 → regime 与趋势延续性更好、减少短期噪音 | PROFESSIONAL 支持 1h 约 720 天、日 K 全历史 |
| **大户/鲸鱼与大规模头寸** | 当前仅有「大户多空比」等 | 若 Coinglass 提供更细大户/鲸鱼活动 → 可补强趋势确认与反转信号 | 文档提及 whale and large-position activity；需查具体端点与套餐 |

### 6.3 小结：是否存在本系统需要补强的参数

**存在。** 对「市场趋势」和「AI 预测」而言，Coinglass 能补强的参数主要包括：

1. **多所聚合**：OI、资金费率、多空比、Taker → 趋势与 regime 更稳、预测依据更全。  
2. **OI 加权资金费率** → 极端费率下的 regime 与开仓逻辑更准。  
3. **强平热力图/价位** → scenario（reversal）、key_levels、near_term_outlook 更具体。  
4. **ETF 资金流** → 宏观趋势与 regime。  
5. **恐惧贪婪指数** → regime（high_volatility/reversal）与 risk_alert。  
6. **更长历史** → 趋势与 regime 的延续性判断。  
7. **大户/鲸鱼活动**（若有对应端点）→ 趋势与反转的辅助信号。

对接时建议优先：多所聚合（OI、资金费率、多空比）、OI 加权资金费率、强平聚合/热力图；其次补 ETF 流与恐惧贪婪；再视需要增加历史长度与大户/鲸鱼类数据。

---

## 七、151 个 REST 端点 + WSS 与「新增/补强」对照（市场分析 + AI 实时+预测）

以下按中转站可获取的 **151 个 GET 端点** 与 **WSS 四频道**（融资率、清算、未平仓合约、价格），对照当前交易系统已有数据，标出**可新增**与**可补强**，便于后续接入优先级排序。

### 7.1 当前系统已有数据（简要）

| 类别 | 当前来源 | 用途 |
|------|----------|------|
| OI 排行 / 单标的 OI | nofxos + 已接入 Coinglass OI exchange-list | 候选、趋势强度、数据补强 |
| 净流入排行 | nofxos | 资金流向、AI 与过滤 |
| 涨跌榜 | nofxos | 全市场强弱、AI |
| 多空比、资金费率、Taker | Binance | 方向池加成、数据补强、极端费率约束 |
| Basis、BTC 占比 | 自算 / 外部 | 期现结构、风险偏好 |
| 强平聚合（1h/4h/24h USD） | 币安 WS / CoinAnk | 情绪与风险、数据补强 |
| K 线/技术指标 | 行情接口 | 趋势、方向、同向性 |

### 7.2 可补强（已有类似数据，用 Coinglass 多所/多维度替代或叠加）

| 端点/频道 | 说明 | 补强点 |
|-----------|------|--------|
| **Futures > Funding Rate**（6）：`/api/futures/funding-rate/history`、`oi-weight-history`、`exchange-list`、`accumulated-exchange-list`、`vol-weight-history`、`arbitrage` | 多所资金费率、OI 加权、交易所列表 | 替代/补强单所 Binance 费率；**oi-weight-history** 更贴近全市场持仓成本，利于 regime 与开仓约束 |
| **Futures > Long/Short Ratio**（6）：`global-long-short-account-ratio/history`、`top-long-short-account-ratio/history`、`taker-buy-sell-volume/exchange-list`、`net-position/history` 等 | 多所全账户/大户多空比、Taker、净持仓 | 补强单所 Binance 多空比/Taker；多所聚合趋势更稳 |
| **Futures > Open Interest**（6）：`history`、`aggregated-history`、`exchange-list`（已用）、`exchange-history-chart` 等 | OI 历史、全市场聚合、交易所维度 | 已用 exchange-list；可补 **aggregated-history** / **history** 做趋势与延续性 |
| **Futures > Liquidation**（14）：`history`、`aggregated-history`、`heatmap/model1-3`、`aggregated-heatmap`、`map`、`aggregated-map`、`max-pain` 等 | 强平历史、热力图、关键价位 | 补强当前仅「多空金额汇总」；**heatmap/map** 可出支撑/阻力与 scenario（reversal）、key_levels |
| **Futures > Taker Buy/Sell（6）**：`taker-buy-sell-volume/history`、`aggregated-taker-buy-sell-volume/history`、`cvd/history`、`aggregated-cvd/history`、`netflow-list` 等 | Taker、CVD、净流入列表 | 补强 Binance Taker；**netflow-list** 可补强/对照 nofxos 净流入 |
| **Indicator > Market（7）**：`/api/index/fear-greed-history`、`bitcoin-dominance`、`altcoin-season`、`option-vs-futures-oi-ratio` 等 | 恐惧贪婪、BTC 占比、山寨季、期权/合约 OI 比 | **fear-greed-history** 补强 regime/risk_alert；**bitcoin-dominance** 可替代现有来源；**altcoin-season** 利于轮动与 regime |
| **WSS：融资率、清算、未平仓合约、价格** | 实时推送 | 补强当前轮询 REST；实时资金费率/强平/OI/价格，利于「AI 实时+预测」与风控 |

### 7.3 可新增（当前系统未用，对市场分析与 AI 预测有价值）

| 端点/类别 | 说明 | 对市场分析 + AI 的作用 |
|-----------|------|------------------------|
| **ETF（16）**：`/api/etf/bitcoin/flow-history`、`net-assets/history`、`premium-discount/history`、`/api/etf/ethereum/flow-history`、`/api/hk-etf/bitcoin/flow-history`、Grayscale、Solana、XRP 等 | BTC/ETH/HK-ETF 资金流、净值、溢价折价、Grayscale 持仓 | **新增**宏观资金进出；补强 trend 与 **market_regime**（流入偏 trend_up，流出偏谨慎/reversal） |
| **Indicator > Bitcoin（26）**：AHR999、Puell、stock-to-flow、rainbow、SOPR、realized price、LTH/STH supply、reserve-risk、NUPL、correlation、macro-oscillator 等 | 链上/宏观指标 | **新增** regime 与情绪维度；SOPR/NUPL/reserve-risk 等利于 reversal/high_volatility 判断 |
| **Indicator > Futures（12）**：`basis/history`、`whale-index/history`、`cgdi-index/history`、`cdri-index/history`、RSI/MA/EMA/BOLL/MACD/ATR 等 | 合约 Basis、鲸鱼指数、多空/多空扩散指数、技术指标 | **basis/history** 可补强自算 Basis；**whale-index/cgdi/cdri** 可新增大户与多空扩散，利于趋势与反转 |
| **Indicator > Spot（3）**：Coinbase premium、Bitfinex margin long/short、borrow rate | 现货溢价、杠杆多空、借币利率 | 新增现货/杠杆情绪，辅助 regime 与资金成本 |
| **Options（4）**：max-pain、exchange-oi-history、exchange-vol-history | 期权痛点、OI、成交量 | 新增期权维度，利于关键位与波动预期 |
| **On-Chain（7）**：exchange assets/balance、coin unlock/vesting、whale transfer 等 | 交易所资产、解锁、鲸鱼转账 | 新增链上与大户行为，利于趋势与反转信号 |
| **Futures > Hyperliquid（6）**：whale-alert、whale-position、position、wallet distribution、pnl-distribution | HL 鲸鱼与仓位分布 | 若交易 HL，可新增大户与仓位结构 |
| **Other（2）**：economic-data、article/list | 宏观日历、资讯 | 可选新增，辅助重大事件与 risk_alert |

### 7.4 建议接入优先级（市场分析 + AI 实时+预测）

1. **补强优先（与现有逻辑直接对齐）**  
   - **资金费率**：`/api/futures/funding-rate/exchange-list`、`/api/futures/funding-rate/oi-weight-history`（多所 + OI 加权，补强 Binance）  
   - **多空比**：`/api/futures/global-long-short-account-ratio/history`、`top-long-short-account-ratio/history`（多所，补强 Binance）  
   - **强平**：`/api/futures/liquidation/aggregated-history`、`/api/futures/liquidation/heatmap/model2`（补强金额汇总，热力图出关键价位）  
   - **恐惧贪婪**：`/api/index/fear-greed-history`（补强 regime 与 risk_alert）

2. **新增优先（对 trend/regime 提升大）**  
   - **ETF 资金流**：`/api/etf/bitcoin/flow-history`、`/api/etf/ethereum/flow-history`（宏观资金，补强 market_regime）  
   - **BTC 占比 / 山寨季**：`/api/index/bitcoin-dominance`、`/api/index/altcoin-season`（可替代/补强现有占比，轮动判断）  
   - **WSS**：订阅 `融资率、清算、未平仓合约、价格`，实现实时推送（应用层需处理 ping/pong 保活与订阅命令 `{"操作":"订阅","频道":["融资率","清算","未平仓合约","价格"]}` 或英文 `action: subscribe, channels: ["funding-rate","liquidation","open-interest","price"]`），用于实时补强与风控。

3. **后续可扩展**  
   - OI：`aggregated-history`、`exchange-history-chart`；净流入：`/api/futures/netflow-list` 与 nofxos 对照  
   - 链上/期权：Bitcoin 指标、Options max-pain、On-Chain 交易所/鲸鱼（按需选部分端点）  
   - Hyperliquid：若交易 HL，再接入 whale/position 等

### 7.5 WSS 对接要点（已实现）

- **Endpoint**：`wss://www.keystore.com.cn/api/v1/ws/coinglass?api_key=YOUR_KEYSTORE_API_KEY`（或 Header `X-Api-Key`）。  
- **订阅**：连接后发送 JSON：`{"action":"subscribe","channels":["funding-rate","liquidation","open-interest","price"]}`。  
- **保活**：若服务端发应用层 `{"type":"ping"}`，需回复 `{"type":"pong","message":<原 message>}`。  
- **实现**：`provider/coinglass/wss.go` 已实现 `WSClient`：`Connect()` 建立连接并订阅，`Run()` 阻塞读消息并缓存各频道最新一条，`GetLatest(channel)` 供业务读取。应用层可在启用 Coinglass 时启动 goroutine 运行 `Run()`，后续可把 WSS 缓存并入 Context 做实时补强。

---

## 八、总结

- **可以对接 Coinglass API**：在现有架构下新增 `provider/coinglass`，通过配置选择或补强 nofxos/Binance/强平数据源即可。  
- **PROFESSIONAL 套餐**具备我们需要的：  
  - **OI、资金费率、强平、多空比、Taker** 等衍生品数据，且 160+ 端点与 1200 次/分钟对当前用法充足。  
- **建议**：  
  - 在 Coinglass 文档中再确认“净流入/资金流”对应端点是否在 PROFESSIONAL 内；  
  - 对接时优先实现 OI、资金费率、多空比、强平，再视需要补净流入与涨跌榜（或自算）。
