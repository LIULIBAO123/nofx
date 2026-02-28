# AI 决策间隔变频繁问题分析

## 结论概览

- **设计上**：AI 决策周期**只**由「决策间隔」（ScanInterval）决定，按 `lastCycleStart + ScanInterval` 排下一轮，**与是否持仓、K 线周期、行情刷新无关**。
- 若出现「持仓后约 1 分钟一次」的决策，多半是**配置或部署**导致实际使用的间隔偏小，而不是按 K 线节奏执行。

## 1. 主循环逻辑（trader/auto_trader.go）

- 唯一驱动 AI 周期的是 `for` 循环里的：
  - `nextRunAt = lastCycleStart.Add(at.config.ScanInterval)`
  - 若 `now < nextRunAt`，则 `time.After(nextRunAt - now)` 睡到点再跑 `runCycle`
- **没有任何**按 K 线、按行情或按持仓状态改变 `ScanInterval` 或 `nextRunAt` 的逻辑。
- 策略止盈/止损是**独立**的 30s 检查协程，只做平仓，**不**触发 `runCycle`，也不会在决策列表里多出一条「周期」。

因此：**代码层面不存在「持仓后按 K 线节奏执行」**；若周期变密，一定是**实际使用的 ScanInterval 偏小**或**同一交易员被多次启动**。

## 2. 间隔从哪里来

- **DB**：`trader.scan_interval_minutes`（默认 3）。
- **API**：
  - 创建/更新交易员时，若 `scan_interval_minutes < 3` 会强制为 3。
  - 启动交易员前会先 RemoveTrader 再 `LoadUserTradersFromStore`，即**从 DB 重新加载**，用的就是当前 DB 里的间隔。
- **Manager 加载**：`addTraderFromStore` 里若 `ScanIntervalMinutes < 3` 会强制为 3，再转成 `ScanInterval = scanMins * time.Minute`。
- **Run() 入口**：若 `at.config.ScanInterval < 3*time.Minute` 会钳制为 3 分钟并打日志。

因此：在**当前代码 + 正常部署**下，间隔最少是 3 分钟；若出现约 1 分钟一次，可能是：
- 运行的是**未包含上述钳制/重载逻辑的旧镜像**；或
- DB 里曾写入过 1，且某次启动时用了旧逻辑未钳制；或
- 同一交易员被多次 Start，导致多个 `Run()` 同时跑（见下）。

## 3. 是否存在「按 K 线执行」的代码路径

- 已搜索：**没有任何**地方用 K 线周期、K 线时间戳或行情推送来触发 `runCycle` 或改写 `ScanInterval`。
- 拉取 K 线只在 `runCycle` 内部（如 buildTradingContext、checkDynamicStopLossTakeProfit）做**数据用**，不参与「何时跑下一周期」的调度。

所以：**不存在按 K 线节奏驱动 AI 决策的路径**；您观察到的「持仓后变频繁」更可能是间隔配置或多次启动导致的。

## 4. 建议排查与修复

1. **看日志（推荐先做）**  
   当前代码已在主循环中打日志，例如：
   - 启动时：`Scan interval: Xm0s (AI cycle runs every this duration only; not tied to K-line)`
   - 每次等待下一周期前：`Next AI cycle at HH:MM:SS (interval=Xm0s, sleep=Ys)`  
   若这里 `interval=1m0s`，说明内存里 `ScanInterval` 就是 1 分钟，需查 DB 与加载逻辑；若 `interval=10m0s` 但界面仍约 1 分钟一条，需怀疑同一交易员被多次 Start。

2. **确认 DB 中的间隔**  
   查该交易员 `scan_interval_minutes`（例如 10）。若为 1 或 0，在界面把「决策间隔」改为 10 并保存，再**停止 → 启动**该交易员，使重新从 DB 加载。

3. **确认只启动一次**  
   启动前会 RemoveTrader 再 Load；若某处逻辑导致同一 ID 被启动两次（例如重复点「开始」或并发请求），会出现两套 `Run()` 交替写决策，看起来像「约 1 分钟一次」。可通过日志里「Next AI cycle」和决策记录时间戳对比是否每 10 分钟一条、且只有一条。

4. **重新部署**  
   若当前运行的是旧镜像（没有 3 分钟钳制、没有「启动前重载配置」），请用包含以下改动的镜像重新部署：
   - 启动前移除内存中的交易员并重新从 DB 加载；
   - Manager 与 Run() 入口对 `ScanInterval` 的 <3 分钟钳制；
   - 主循环中「下一周期仅由 lastCycleStart + ScanInterval 决定」的注释与日志。

## 5. 本次代码改动摘要

- 在 `Run()` 中明确注释：**AI 决策周期仅按 ScanInterval 触发，与 K 线无关**。
- 启动时打印当前使用的 `Scan interval`。
- 主循环中在每次等待下一周期前打印：`Next AI cycle at ... (interval=..., sleep=...)`，便于核对实际间隔与睡眠时间。

若你愿意，我可以再根据你当前的部署方式（Docker/二进制）写一版「部署后如何抓一条日志确认间隔」的操作步骤。

---

## 6. 实盘与模拟是否一致

**结论：实盘与模拟共用同一套逻辑，决策间隔与 drawdown 修复在实盘上同样生效。**

### 6.1 决策间隔

- **配置来源**：`manager/trader_manager.go` 的 `addTraderFromStore` 中，`ScanInterval` 来自 `traderCfg.ScanIntervalMinutes`，**不区分** `traderCfg.IsSimulation`。同一张表、同一字段，实盘与模拟的「决策间隔」读取与钳制（<3 分钟强制为 3）完全一致。
- **主循环**：`trader/auto_trader.go` 的 `Run()` 仅使用 `at.config.ScanInterval` 计算 `nextRunAt` 与 sleep，**没有任何** `if at.config.IsSimulation` 的分支。实盘与模拟都是「每 ScanInterval 跑一次 runCycle」。
- 因此：**决策间隔问题在实盘上同样已解决**；只要 DB 里该交易员的 `scan_interval_minutes` 正确（并已重启/重载），实盘也会按设定间隔执行。

### 6.2 回撤监控与 panic 修复

- **回撤监控**：`startDrawdownMonitor()` 在 `Run()` 中**无条件**调用（约第 446 行），不区分实盘/模拟。实盘与模拟都会每分钟执行 `checkPositionDrawdown()`。
- **GetPositions**：回撤检查里调用的是 `at.trader.GetPositions()`。模拟时 `at.trader` 为 PaperTrader；实盘时为交易所适配器（Binance/Bybit/OKX 等）。若某交易所返回的持仓里 `side`/`symbol` 为 nil 或使用不同 key（如 `position_side`），原先的直接类型断言会 panic。
- **修复**：已改为使用 `getPosStr`/`getPosFloat` 并跳过 symbol/side 为空或 quantity 为 0 的持仓。该逻辑**不区分实盘/模拟**，实盘若交易所返回结构不一致也会被安全处理，**实盘同样受益于此次 panic 修复**。

---

## 7. 为何持仓历史里「平仓方式」都是 AI 分析、且持仓时长常为约 5 分钟

### 7.1 平仓方式都显示「AI 分析平仓」

- 系统里**只有**当平仓是由 **AI 决策**（本周期内 AI 返回了 close_long/close_short）并走 `executeCloseLong`/`executeCloseShort` 时，才会写入平仓原因为 `"ai"`，前端解析后显示为「平仓方式: AI / 平仓类型: AI分析判断」。
- 若平仓是由**策略**触发（动态止损/止盈、支撑阻力等），会写入 `system:sl:xxx` 或 `system:tp:xxx`，前端会显示为「系统」及具体类型。
- 因此：**只要持仓历史上显示「AI分析平仓」，就说明该笔平仓确实来自某次 AI 周期的平仓决策**，而不是策略或手动。

### 7.2 为何很多订单持仓时长约 5 分钟

- 若**决策间隔为 5 分钟**：周期 T 开仓 → 周期 T+1（5 分钟后）AI 决定平仓（或先平后开），则这笔仓位的持仓时长就会接近 **5 分钟**。所以「很多订单都是约 5 分钟」与「当时生效的决策间隔为 5 分钟」是一致的。
- 若**决策间隔已改为 10 分钟**：新产生的单子会更常见「约 10 分钟」的持仓时长；之前用 5 分钟间隔跑出的单子仍会保持约 5 分钟，属历史数据。
- 另一种情况：**同一周期内 AI 先平后开同一币种**。例如周期 #N 执行了「平 ESP → 再开 ESP」，则「平」的是上一周期开的仓位（约 10 分钟持仓），「开」的是本周期新仓。决策列表里该周期应显示「ESP CLOSE · ESP LONG (含先平后开)」；若只看到「ESP LONG」，可**展开该周期**查看是否还有 close 操作，或查看上一周期。

### 7.3 策略平仓被误标为「AI 分析平仓」的 bug（已修复）

- **原因**：`executeCloseLongWithRecord` / `executeCloseShortWithRecord` 在写库前**无条件**调用 `setPendingCloseReason("ai")`。当平仓由**策略**触发（如追踪止损 `executeStopLoss`）时，流程是：先 `setPendingCloseReason("system:sl:Trailing")`，再调用 `executeDecisionWithRecord` → `executeCloseLongWithRecord`，后者又执行 `setPendingCloseReason("ai")`，把已设置的 `system:sl:xxx` **覆盖**掉了，导致 DB 里 `close_reason` 全是 `ai`。
- **修复**：新增 `setPendingCloseReasonIfEmpty(reason)`，仅在当前未设置时写入。在 `executeCloseLongWithRecord` / `executeCloseShortWithRecord` 中改为调用 `setPendingCloseReasonIfEmpty("ai")`，这样由策略触发的平仓会保留 `executeStopLoss`/`executeTakeProfit` 已设置的 `system:sl:xxx` / `system:tp:xxx`，前端会正确显示「平仓方式: 系统」及具体类型（如追踪止损）。
- **「持仓约 5 分钟」的实际情况**：日志显示决策间隔确为 10 分钟（如 runCycle #8 20:27:26，Next at 20:37:26）。不少订单在约 5 分钟后被平仓，是因为**策略的追踪止损**在盈利回撤达到阈值时触发（例如 1.5% 回撤），与 10 分钟周期无关。修复后，这类平仓会显示为「系统 / 追踪止损」而非「AI 分析」，便于区分。

### 7.4 实际平仓原因有哪些？为何盈利/亏损都约 5 分钟、逻辑是否合理？

**实际平仓原因（修复误标为 AI 之后）**

- **系统（策略）平仓**：由策略在每 30 秒一次的检查中触发，写入 `close_reason` 为 `system:sl:xxx` 或 `system:tp:xxx`，前端解析后显示为「平仓方式: 系统」及具体类型：
  - **追踪止损**（trailing）：先有浮盈、再回撤达到设定比例时触发，**只会平盈利单**，不会平纯亏损单。
  - **初始止损**（initial）：价格触及开仓时设定的固定止损比例时触发，**多用于亏损单**。
  - **ATR 止损**（atr）：价格相对入场价不利波动超过 ATR 倍数时触发，**盈亏单都可能**。
  - **支撑/阻力止损**（support_resistance）：价格跌破支撑/涨破阻力时触发。
  - **止盈类**（system:tp:xxx）：固定/分层/ATR/阻力位止盈等。
- **AI 平仓**：仅当本周期 AI 返回了 close_long/close_short 并执行平仓时，才记为 `ai`，前端显示「平仓方式: AI / 平仓类型: AI分析判断」。

**为何「不管盈利亏损都约 5 分钟」？逻辑上有没有问题？**

- 代码里**没有任何**「持仓满 5 分钟就强制平仓」的逻辑，也没有按 5 分钟周期触发的平仓。
- 能解释「几乎所有订单都在约 5 分钟平仓」的**唯一合理原因**是：**当时生效的决策间隔是 5 分钟**。
  - AI 每 5 分钟跑一次；下一周期（T+5min）AI 经常做出「平仓」或「先平后开」。
  - 于是**盈利单**可能被 AI 止盈/调仓平掉，**亏损单**可能被 AI 砍仓或换仓平掉，时间点都在「下一轮 AI 周期」≈ 开仓后约 5 分钟。这样盈利、亏损都会集中在约 5 分钟，**逻辑一致**。
- 若当前已改为 **10 分钟**决策间隔且运行正常，**新产生的单子**应表现为：
  - 由 **AI** 平仓的：多为约 **10 分钟**（下一周期）平仓；
  - 由 **策略**（追踪/初始/ATR 等）平仓的：时间不固定，取决于价格何时触及条件。
- 若新单仍大量在约 **5 分钟**被平、且平仓方式显示为「系统」（如追踪止损），则说明是**策略过紧**（例如追踪回撤比例很小、或 ATR/初始止损很紧），导致很快触发，而不是「系统按 5 分钟强制平仓」。建议检查策略里动态止损/止盈的阈值与档位。

**小结**：实际平仓原因 = 系统（策略）或 AI，修复后界面会正确区分；「都约 5 分钟」在历史上与「决策间隔曾为 5 分钟」一致，逻辑合理；当前若已改为 10 分钟，新单应出现约 10 分钟的 AI 平仓与不固定时长的策略平仓。

### 7.5 与「决策链只看到开仓」的关系

- 每个周期保存的 `record.Decisions` 会按**执行顺序**包含本周期所有操作（先平后开时会有 close_long 与 open_long 两条）。
- 前端 collapsed 摘要会把这些操作拼成一行；若该周期同时有平仓和开仓，会追加「(含先平后开)」提示，并建议展开该周期查看完整序列，与持仓历史里的「AI分析平仓」或「系统」平仓对应起来。
