# 可获取增强市场判断数据的 API 接口列举

本文档列举可用于获取《交易数据增强与市场判断》中建议数据的**交易数据 API 接口**，按数据类型分类，便于接入 nofx 时选用。接口格式、参数与限制以各平台当前文档为准，接入前请核对最新文档。

---

## 一、多空比 / 大户多空比

### 1.1 Binance 币安

| 数据 | 接口 | 说明 |
|------|------|------|
| **全账户多空比** | `GET https://fapi.binance.com/futures/data/globalLongShortAccountRatio` | 参数：`symbol`（如 BTCUSDT）、`period`（5m/15m/30m/1h/2h/4h/6h/12h/1d）、`limit`（默认 30）。返回 longShortRatio、longAccount、shortAccount、timestamp。**仅最近 30 天**。 |
| **大户账户多空比** | `GET https://fapi.binance.com/futures/data/topLongShortAccountRatio` | 参数同上。大户账户数多空比。 |
| **大户持仓多空比** | `GET https://fapi.binance.com/futures/data/topLongShortPositionRatio` | 参数同上。大户持仓量多空比。 |

- 文档：<https://developers.binance.com/docs/derivatives/coin-margined-futures/market-data/rest-api/Long-Short-Ratio>（USDT 合约类似在 fapi 下）

### 1.2 Bybit

| 数据 | 接口 | 说明 |
|------|------|------|
| **账户多空比** | `GET https://api.bybit.com/v5/market/account-ratio` | 参数：`category`（linear/inverse）、`symbol`（如 BTCUSDT）、`period`（5min/15min/30min/1h/4h/1d）、`startTime`/`endTime`、`limit`（1–500）。返回 buyRatio、sellRatio、timestamp。**自 2020-07-20 起**。 |
| **持仓量多空比** | `GET https://api.bybit.com/v5/market/open-interest/ratio` | 按持仓量统计的多空比，参数类似。 |

- 文档：<https://bybit-exchange.github.io/docs/v5/market/long-short-ratio>、<https://bybit-exchange.github.io/docs/v5/market/account-ratio>

### 1.3 CoinGlass（聚合多交易所）

| 数据 | 接口 | 说明 |
|------|------|------|
| **全账户多空比历史** | `GET /api/futures/global-long-short-account-ratio/history` | 参数：`exchange`、`symbol`、`interval`（1m/3m/5m/15m/30m/1h/4h/6h/8h/12h/1d/1w）、`limit`、`startTime`/`endTime`。需 `CG-API-KEY`。 |
| **大户账户多空比历史** | `GET /api/futures/top-long-short-account-ratio/history` | 参数同上，大户账户多空比。 |
| **大户持仓多空比历史** | `GET /api/futures/top-long-short-position-ratio/history` | 参数同上，大户持仓多空比。 |

- 文档：<https://docs.coinglass.com/>（如 v4.0 中文：交易对账户多空比历史、大户账户数多空比历史）

---

## 二、爆仓 / 清算数据

### 2.1 CoinGlass

| 数据 | 接口 | 说明 |
|------|------|------|
| **交易对爆仓历史** | `GET /api/futures/liquidation/history` | 参数：交易对、时间间隔（1m/3m/5m/15m/30m/1h/4h 等）、起止时间、条数。返回多单爆仓金额、空单爆仓金额（美元）等。不同套餐对最小间隔有限制（如 ≥30 分钟）。 |
| **清算热力图/聚合** | `GET /api/futures/liquidation/aggregated-map` 等 | 按价格档位清算规模，多用于可视化；部分为专业版/企业版。 |

- 文档：<https://docs.coinglass.com/>（如「交易对爆仓历史」）、<https://www.coinglass.com/zh/CryptoApi>

---

## 三、资金费率历史 / 预测

### 3.1 Binance

| 数据 | 接口 | 说明 |
|------|------|------|
| **历史资金费率** | `GET https://fapi.binance.com/fapi/v1/fundingRate` | 参数：`symbol`、`startTime`、`endTime`、`limit`（默认 100，最大 1000）。返回 symbol、fundingRate、fundingTime、markPrice。 |
| **当前/下一档资金费率** | `GET https://fapi.binance.com/fapi/v1/premiumIndex` | 含当前费率、下次结算时间等。 |

- 文档：<https://developers.binance.com/docs/derivatives/usds-margined-futures/market-data/rest-api/Get-Funding-Rate-History>

### 3.2 Bybit

| 数据 | 接口 | 说明 |
|------|------|------|
| **历史资金费率** | `GET https://api.bybit.com/v5/market/funding/history` | 参数：`category`（linear/inverse）、`symbol`、`startTime`、`endTime`、`limit`（1–200）。返回 fundingRate、fundingRateTimestamp。 |

- 文档：<https://bybit-exchange.github.io/docs/v5/market/history-fund-rate>

---

## 四、主动买卖 / Taker 买卖量

### 4.1 Binance

| 数据 | 接口 | 说明 |
|------|------|------|
| **Taker 买卖量（合约）** | `GET https://fapi.binance.com/futures/data/takerlongshortRatio` 或 `takerBuySellVol` | 币本位：`/futures/data/takerBuySellVol`，参数：`pair`、`contractType`（PERPETUAL 等）、`period`（5m/15m/30m/1h/2h/4h/6h/12h/1d）、`limit`。返回 takerBuyVol、takerSellVol 等。**仅最近 30 天**。 |
| **K 线中的 Taker 量（现货）** | `GET https://api.binance.com/api/v3/klines` | 每根 K 线含「Taker buy base asset volume」「Taker buy quote asset volume」，可自行算比例。 |

- 文档：<https://developers.binance.com/docs/derivatives/coin-margined-futures/market-data/rest-api/Taker-Buy-Sell-Volume>、K 线见 Binance Spot API

---

## 五、BTC 占比 / 山寨季节指数

### 5.1 CoinGecko

| 数据 | 接口 | 说明 |
|------|------|------|
| **全球市值与占比** | `GET https://api.coingecko.com/api/v3/global` | 返回 `market_cap_percentage`（含 btc、eth 等占比）。约 10 分钟更新。部分计划需 API Key。 |

- 文档：<https://docs.coingecko.com/reference/crypto-global>

### 5.2 其他

- **山寨季节指数**：多由第三方根据 BTC 与山寨市值比计算，无统一标准 API；若有 NofxOS 或其它数据源提供可直接接入。
- **Bitbo**：<https://bitbo.io/api/docs/endpoints/btc-dominance> 提供 BTC 占比接口，可作备选。

---

## 六、永续-现货价差（Basis / Premium）

### 6.1 自算（推荐）

- **方式**：同一标的「永续最新价」减去「现货最新价」，再除以现货价得到溢价率。
- **永续价**：各交易所合约 ticker（如 Binance `fapi/v1/ticker/price`、Bybit `v5/market/tickers`）。
- **现货价**：各交易所现货 ticker（如 Binance `api/v3/ticker/price`）。
- 无需额外「Basis 专用」接口，只需已有行情或 ticker 即可。

### 6.2 CoinGlass

- 提供衍生品与价差类数据，具体 endpoint 见：<https://www.coinglass.com/zh/CryptoApi>、<https://docs.coinglass.com/>。

### 6.3 CoinEx

| 数据 | 接口 | 说明 |
|------|------|------|
| **Basis 历史** | `GET /api/v2/futures/market/list-market-basis-history` | 可查历史 basis 率，参数见 CoinEx 文档。 |

- 文档：<https://docs.coinex.com/>

---

## 七、稳定币流入流出（交易所）

### 7.1 CryptoQuant

- 提供稳定币交易所流入/流出、净流入、储备等指标，多通过其**平台与付费 API** 获取，非公开 REST 免费接口。
- 文档/产品页：<https://cryptoquant.com/>、<https://dataguide.cryptoquant.com/stablecoin-exchange-flows-indicators/stablecoin-exchange-in-outflow-and-netflow>

### 7.2 Bitquery

- **Stablecoin Payments API**：链上稳定币转账等，偏链上；若需「交易所净流入」通常需在其上做聚合或使用 CryptoQuant 类数据。
- 文档：<https://docs.bitquery.io/docs/stablecoin-APIs/stablecoin-payments-api>

---

## 八、汇总表（按建议数据类型）

| 建议数据类型       | 可用的 API 来源 | 接口类型 / 说明 |
|--------------------|------------------|-----------------|
| 多空比 / 大户多空比 | Binance          | REST：globalLongShortAccountRatio、topLongShortAccountRatio、topLongShortPositionRatio |
|                    | Bybit            | REST：/v5/market/account-ratio、open-interest/ratio |
|                    | CoinGlass        | REST：global-long-short-account-ratio、top-long-short-account-ratio、top-long-short-position-ratio（需 API Key） |
| 爆仓/清算量        | CoinGlass        | REST：/api/futures/liquidation/history（及 heatmap/aggregated 等，部分付费） |
| 资金费率历史/预测  | Binance          | REST：fapi/v1/fundingRate、premiumIndex |
|                    | Bybit            | REST：/v5/market/funding/history |
| Taker 买卖量       | Binance          | REST：futures/data/takerBuySellVol；或 klines 内 taker 字段 |
| BTC 占比           | CoinGecko        | REST：/api/v3/global（market_cap_percentage） |
| 永续-现货价差      | 各交易所         | 永续 ticker + 现货 ticker 自算；或 CoinGlass/CoinEx 等 |
| 稳定币流入流出     | CryptoQuant 等   | 多为付费/平台 API；链上可用 Bitquery 等 |

---

## 九、接入 nofx 时的建议

1. **多空比 / 大户多空比**：若交易在 Bybit，优先用 Bybit `/v5/market/account-ratio`；若需多所或历史更长，用 CoinGlass。
2. **爆仓/清算**：目前公开且成体系的以 **CoinGlass** 为主，需注册 API Key，注意套餐对时间粒度的限制。
3. **资金费率**：当前 nofx 若已用 Binance/Bybit 行情，可直接在同所调 fundingRate 历史与 premiumIndex。
4. **Taker 买卖**：Binance 有现成 takerBuySellVol；若只用 K 线，可从 klines 的 taker 字段聚合。
5. **BTC 占比**：CoinGecko `/global` 即可，免费额度内可用。
6. **Basis**：建议用现有现货+永续 ticker 自算，无需单独「Basis API」。
7. **稳定币流**：属增强项，可后续对接 CryptoQuant 或合作数据方。

以上接口的**完整 URL、参数与限频**请以各平台**最新官方文档**为准，本文仅作列举与选型参考。

---

*文档版本：1.0，与《交易数据增强与市场判断》配套。*
