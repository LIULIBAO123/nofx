package backtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nofx/logger"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nofx/kernel"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
)

var (
	errBacktestCompleted = errors.New("backtest completed")
	errLiquidated        = errors.New("account liquidated")
)

const (
	metricsWriteInterval = 5 * time.Second
	aiDecisionMaxRetries = 3
)

// Runner encapsulates the lifecycle of a single backtest run.
type Runner struct {
	cfg            BacktestConfig
	feed           *DataFeed
	account        *BacktestAccount
	strategyEngine *kernel.StrategyEngine

	decisionLogDir string
	mcpClient      mcp.AIClient

	statusMu sync.RWMutex
	status   RunState

	stateMu sync.RWMutex
	state   *BacktestState

	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}
	doneCh   chan struct{}

	err              error
	errMu            sync.RWMutex
	lastError        string
	lastCheckpoint   time.Time
	createdAt        time.Time
	lastMetricsWrite time.Time

	aiCache   *AICache
	cachePath string

	// positionParams holds stop_loss, take_profit, atr_at_open and scaled TP levels taken per position
	positionParamsMu sync.RWMutex
	positionParams   map[string]positionParam

	// slConfirmCount: consecutive bars SL condition met per position; execute only when >= ConfirmCycles (and ATR tolerance)
	slConfirmCountMu sync.Mutex
	slConfirmCount   map[string]int

	lockInfo     *RunLockInfo
	lockStop     chan struct{}
	lockStopOnce sync.Once // Ensures lockStop is closed only once
}

// NewRunner constructs a backtest runner.
func NewRunner(cfg BacktestConfig, mcpClient mcp.AIClient) (*Runner, error) {
	if err := ensureRunDir(cfg.RunID); err != nil {
		return nil, err
	}

	// Use the provided MCP client directly
	client := mcpClient
	if client == nil {
		return nil, fmt.Errorf("mcpClient cannot be nil")
	}

	feed, err := NewDataFeed(cfg)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(decisionLogDir(cfg.RunID), 0o755); err != nil {
		return nil, err
	}

	dLogDir := decisionLogDir(cfg.RunID)
	account := NewBacktestAccount(cfg.InitialBalance, cfg.FeeBps, cfg.SlippageBps)

	createdAt := time.Now().UTC()
	state := &BacktestState{
		Positions:      make(map[string]PositionSnapshot),
		Cash:           account.Cash(),
		Equity:         cfg.InitialBalance,
		UnrealizedPnL:  0,
		RealizedPnL:    0,
		MaxEquity:      cfg.InitialBalance,
		MinEquity:      cfg.InitialBalance,
		MaxDrawdownPct: 0,
		LastUpdate:     createdAt,
	}
	positionParams := make(map[string]positionParam)

	var (
		aiCache   *AICache
		cachePath string
	)
	if cfg.CacheAI || cfg.ReplayOnly || cfg.SharedAICachePath != "" {
		cachePath = cfg.SharedAICachePath
		if cachePath == "" {
			cachePath = filepath.Join(runDir(cfg.RunID), "ai_cache.json")
		}
		cache, err := LoadAICache(cachePath)
		if err != nil {
			return nil, fmt.Errorf("load ai cache: %w", err)
		}
		aiCache = cache
	}

	// Create strategy engine from backtest config for unified prompt generation
	strategyConfig := cfg.ToStrategyConfig()
	strategyEngine := kernel.NewStrategyEngine(strategyConfig)

	r := &Runner{
		cfg:             cfg,
		feed:            feed,
		account:         account,
		strategyEngine:  strategyEngine,
		decisionLogDir:  dLogDir,
		mcpClient:       client,
		status:          RunStateCreated,
		state:           state,
		pauseCh:         make(chan struct{}, 1),
		resumeCh:        make(chan struct{}, 1),
		stopCh:          make(chan struct{}, 1),
		doneCh:          make(chan struct{}),
		createdAt:       createdAt,
		aiCache:         aiCache,
		cachePath:       cachePath,
		positionParams:   positionParams,
		slConfirmCount:   make(map[string]int),
	}

	if err := r.initLock(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Runner) initLock() error {
	if r.cfg.RunID == "" {
		return fmt.Errorf("run_id required for lock")
	}
	info, err := acquireRunLock(r.cfg.RunID)
	if err != nil {
		return err
	}
	r.lockInfo = info
	r.lockStop = make(chan struct{})
	go r.lockHeartbeatLoop()
	return nil
}

func (r *Runner) lockHeartbeatLoop() {
	ticker := time.NewTicker(lockHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := updateRunLockHeartbeat(r.lockInfo); err != nil {
				logger.Infof("failed to update lock heartbeat for %s: %v", r.cfg.RunID, err)
			}
		case <-r.lockStop:
			return
		}
	}
}

func (r *Runner) releaseLock() {
	// Use sync.Once to ensure channel is closed exactly once, preventing panic on double-close
	r.lockStopOnce.Do(func() {
		if r.lockStop != nil {
			close(r.lockStop)
		}
	})
	if err := deleteRunLock(r.cfg.RunID); err != nil {
		logger.Infof("failed to release lock for %s: %v", r.cfg.RunID, err)
	}
	r.lockInfo = nil
}

// Start launches the backtest loop.
func (r *Runner) Start(ctx context.Context) error {
	r.statusMu.Lock()
	if r.status != RunStateCreated && r.status != RunStatePaused {
		r.statusMu.Unlock()
		return fmt.Errorf("cannot start runner in state %s", r.status)
	}
	r.status = RunStateRunning
	r.statusMu.Unlock()

	go r.loop(ctx)
	return nil
}

// PersistMetadata writes the current snapshot to run.json.
func (r *Runner) PersistMetadata() {
	r.persistMetadata()
}

func (r *Runner) setLastError(err error) {
	r.errMu.Lock()
	defer r.errMu.Unlock()
	if err == nil {
		r.lastError = ""
		return
	}
	r.lastError = err.Error()
}

func (r *Runner) lastErrorString() string {
	r.errMu.RLock()
	defer r.errMu.RUnlock()
	return r.lastError
}

// CurrentMetadata returns the metadata corresponding to the current in-memory state.
func (r *Runner) CurrentMetadata() *RunMetadata {
	state := r.snapshotState()
	meta := r.buildMetadata(state, r.Status())
	meta.CreatedAt = r.createdAt
	meta.UpdatedAt = state.LastUpdate
	return meta
}

func (r *Runner) loop(ctx context.Context) {
	defer close(r.doneCh)

	for {
		select {
		case <-ctx.Done():
			r.handleStop(fmt.Errorf("context canceled: %w", ctx.Err()))
			return
		case <-r.stopCh:
			r.handleStop(nil)
			return
		case <-r.pauseCh:
			r.handlePause()
			<-r.resumeCh
			r.resumeFromPause()
		default:
		}

		err := r.stepOnce()
		if errors.Is(err, errBacktestCompleted) {
			r.handleCompletion()
			return
		}
		if errors.Is(err, errLiquidated) {
			r.handleLiquidation()
			return
		}
		if err != nil {
			r.handleFailure(err)
			return
		}
	}
}

func (r *Runner) stepOnce() error {
	state := r.snapshotState()
	if state.BarIndex >= r.feed.DecisionBarCount() {
		return errBacktestCompleted
	}

	ts := r.feed.DecisionTimestamp(state.BarIndex)

	marketData, multiTF, err := r.feed.BuildMarketData(ts)
	if err != nil {
		return err
	}

	priceMap := make(map[string]float64, len(marketData))
	for symbol, data := range marketData {
		priceMap[symbol] = data.CurrentPrice
	}

	callCount := state.DecisionCycle + 1
	shouldDecide := r.shouldTriggerDecision(state.BarIndex)

	var (
		record          *store.DecisionRecord
		decisionActions []store.DecisionAction
		tradeEvents     = make([]TradeEvent, 0)
		execLog         []string
		hadError        bool
	)

	// Dynamic stop-loss / take-profit (same as live trading): check and execute before AI decision
	forced := r.checkAndExecuteDynamicStopTakeProfit(ts, marketData, priceMap, callCount)
	if len(forced) > 0 {
		tradeEvents = append(tradeEvents, forced...)
	}

	decisionAttempted := shouldDecide

	if shouldDecide {
		var strategyClosesThisCycle []kernel.StrategyTriggeredClose
		if len(forced) > 0 {
			strategyClosesThisCycle = make([]kernel.StrategyTriggeredClose, 0, len(forced))
			for _, evt := range forced {
				reason := evt.CloseReason
				if reason == "" {
					reason = "dynamic_sl_tp"
				}
				strategyClosesThisCycle = append(strategyClosesThisCycle, kernel.StrategyTriggeredClose{
					Symbol: evt.Symbol,
					Side:   evt.Side,
					Reason: reason,
					Price:  evt.Price,
				})
			}
		}
		ctx, rec, err := r.buildDecisionContext(ts, marketData, multiTF, priceMap, callCount, strategyClosesThisCycle)
		if err != nil {
			// Defensive nil check to prevent panic if buildDecisionContext returns error with nil record
			if rec != nil {
				rec.Success = false
				rec.ErrorMessage = fmt.Sprintf("failed to build trading context: %v", err)
				_ = r.logDecision(rec)
			}
			return err
		}
		record = rec

		var (
			fullDecision *kernel.FullDecision
			fromCache    bool
			cacheKey     string
		)
		if r.aiCache != nil {
			if key, err := computeCacheKey(ctx, r.cfg.PromptVariant, ts); err == nil {
				cacheKey = key
				if cached, ok := r.aiCache.Get(cacheKey); ok {
					fullDecision = cached
					fromCache = true
				} else if r.cfg.ReplayOnly {
					decisionErr := fmt.Errorf("replay_only enabled but cache miss at %d", ts)
					record.Success = false
					record.ErrorMessage = fmt.Sprintf("cached decision not found for ts=%d", ts)
					_ = r.logDecision(record)
					return decisionErr
				}
			} else {
				logger.Infof("failed to compute ai cache key: %v", err)
			}
		}

		if !fromCache {
			fd, err := r.invokeAIWithRetry(ctx)
			if err != nil {
				decisionAttempted = true
				hadError = true
				record.Success = false
				record.ErrorMessage = fmt.Sprintf("AI decision failed: %v", err)
				execLog = append(execLog, fmt.Sprintf("⚠️ AI decision failed: %v", err))
				r.setLastError(err)
			} else {
				fullDecision = fd
				if r.cfg.CacheAI && r.aiCache != nil && cacheKey != "" {
					if err := r.aiCache.Put(cacheKey, r.cfg.PromptVariant, ts, fullDecision); err != nil {
						logger.Infof("failed to persist ai cache for %s: %v", r.cfg.RunID, err)
					}
				}
			}
		}

		if fullDecision != nil {
			r.fillDecisionRecord(record, fullDecision)

			sorted := sortDecisionsByPriority(fullDecision.Decisions)

			prevLogs := execLog
			decisionActions = make([]store.DecisionAction, 0, len(sorted))
			execLog = make([]string, 0, len(sorted)+len(prevLogs))
			if len(prevLogs) > 0 {
				execLog = append(execLog, prevLogs...)
			}

			for _, dec := range sorted {
				actionRecord, trades, logEntry, execErr := r.executeDecision(dec, priceMap, ts, callCount, marketData)
				if execErr != nil {
					actionRecord.Success = false
					actionRecord.Error = execErr.Error()
					hadError = true
					execLog = append(execLog, fmt.Sprintf("❌ %s %s: %v", dec.Symbol, dec.Action, execErr))
				} else {
					actionRecord.Success = true
					execLog = append(execLog, fmt.Sprintf("✓ %s %s", dec.Symbol, dec.Action))
				}
				if len(trades) > 0 {
					tradeEvents = append(tradeEvents, trades...)
					for _, evt := range trades {
						key := evt.Symbol + ":" + evt.Side
						switch evt.Action {
						case "open_long", "open_short":
							r.setPositionParams(key, evt.StopLoss, evt.TakeProfit, evt.ATRAtOpen)
						case "close_long", "close_short":
							r.deletePositionParams(key)
						}
					}
				}
				if logEntry != "" {
					execLog = append(execLog, logEntry)
				}
				decisionActions = append(decisionActions, actionRecord)
			}
		}
	}

	cycleForLog := state.DecisionCycle
	if decisionAttempted {
		cycleForLog = callCount
	}

	liquidationEvents, liquidationNote, err := r.checkLiquidation(ts, priceMap, cycleForLog)
	if err != nil {
		if record != nil {
			record.Success = false
			record.ErrorMessage = err.Error()
			_ = r.logDecision(record)
		}
		return err
	}
	if len(liquidationEvents) > 0 {
		hadError = true
		tradeEvents = append(tradeEvents, liquidationEvents...)
		if record != nil {
			execLog = append(execLog, fmt.Sprintf("⚠️ Forced liquidation: %s", liquidationNote))
		}
	}

	if record != nil {
		record.Decisions = decisionActions
		// 保留 buildDecisionContext 中写入的「策略平仓」行，再追加本周期 AI 执行日志
		record.ExecutionLog = append(record.ExecutionLog, execLog...)
		record.Success = !hadError && liquidationNote == ""
		if liquidationNote != "" {
			record.ErrorMessage = liquidationNote
		}
	}

	equity, unrealized, _ := r.account.TotalEquity(priceMap)
	marginUsed := r.totalMarginUsed()

	r.updateState(ts, equity, unrealized, marginUsed, priceMap, decisionAttempted)

	snapshot := r.snapshotState()
	drawdownPct := 0.0
	if snapshot.MaxEquity > 0 {
		drawdownPct = ((snapshot.MaxEquity - snapshot.Equity) / snapshot.MaxEquity) * 100
	}

	equityPoint := EquityPoint{
		Timestamp:   ts,
		Equity:      snapshot.Equity,
		Available:   snapshot.Cash,
		PnL:         snapshot.Equity - r.account.InitialBalance(),
		PnLPct:      ((snapshot.Equity - r.account.InitialBalance()) / r.account.InitialBalance()) * 100,
		DrawdownPct: drawdownPct,
		Cycle:       snapshot.DecisionCycle,
	}

	if err := appendEquityPoint(r.cfg.RunID, equityPoint); err != nil {
		return err
	}

	// Perform AI analysis on trades and save with analysis
	accountBeforeEquity, _, _ := r.account.TotalEquity(priceMap)
	accountBefore := kernel.AccountInfo{
		TotalEquity:      accountBeforeEquity,
		AvailableBalance: r.account.Cash(),
		MarginUsed:       r.totalMarginUsed(),
	}
	
	for _, evt := range tradeEvents {
		// Perform AI analysis only for completed trades (close_long/close_short) so it runs after trade is done
		isClose := evt.Action == "close_long" || evt.Action == "close_short"
		if !evt.LiquidationFlag && r.cfg.EnableTradeAnalysis && isClose {
			accountAfter := kernel.AccountInfo{
				TotalEquity:      snapshot.Equity,
				AvailableBalance: snapshot.Cash,
				MarginUsed:       r.totalMarginUsed(),
			}

			// Build market data context for analysis
			marketDataCtx := make(map[string]interface{})
			if md, ok := marketData[evt.Symbol]; ok {
				marketDataCtx["symbol"] = evt.Symbol
				marketDataCtx["current_price"] = md.CurrentPrice
				// Add more context if available
			}

			// Get decision context if available
			decisionCtx := ""
			if record != nil && record.InputPrompt != "" {
				decisionCtx = record.InputPrompt
			}

			// Analyze trade (completed round-trip)
			analysis := r.AnalyzeTrade(evt, accountBefore, accountAfter, marketDataCtx, decisionCtx)
			if analysis != nil {
				evt.AIAnalysis = analysis
				logger.Infof("📊 Trade analysis: %s %s - Score: %.1f/10", evt.Symbol, evt.Action, analysis.OverallScore)
			}
		}

		if err := appendTradeEvent(r.cfg.RunID, evt); err != nil {
			return err
		}
	}

	if record != nil {
		if err := r.logDecision(record); err != nil {
			return err
		}
	}

	if err := saveProgress(r.cfg.RunID, &snapshot, &r.cfg); err != nil {
		return err
	}

	if err := r.maybeCheckpoint(); err != nil {
		return err
	}

	r.persistMetadata()
	r.persistMetrics(false)

	if !hadError && liquidationNote == "" {
		r.setLastError(nil)
	}

	if snapshot.Liquidated {
		return errLiquidated
	}

	return nil
}

func (r *Runner) buildDecisionContext(ts int64, marketData map[string]*market.Data, multiTF map[string]map[string]*market.Data, priceMap map[string]float64, callCount int, strategyClosesThisCycle []kernel.StrategyTriggeredClose) (*kernel.Context, *store.DecisionRecord, error) {
	equity, unrealized, _ := r.account.TotalEquity(priceMap)
	available := r.account.Cash()
	marginUsed := r.totalMarginUsed()
	marginPct := 0.0
	if equity > 0 {
		marginPct = (marginUsed / equity) * 100
	}

	accountInfo := kernel.AccountInfo{
		TotalEquity:      equity,
		AvailableBalance: available,
		TotalPnL:         equity - r.account.InitialBalance(),
		TotalPnLPct:      ((equity - r.account.InitialBalance()) / r.account.InitialBalance()) * 100,
		MarginUsed:       marginUsed,
		MarginUsedPct:    marginPct,
		PositionCount:    len(r.account.Positions()),
	}

	positions := r.convertPositions(priceMap)

	// Get candidate coins from strategy engine (includes source info)
	candidateCoins, err := r.strategyEngine.GetCandidateCoins()
	if err != nil {
		// Fallback to simple list if strategy engine fails
		candidateCoins = make([]kernel.CandidateCoin, 0, len(r.cfg.Symbols))
		for _, sym := range r.cfg.Symbols {
			candidateCoins = append(candidateCoins, kernel.CandidateCoin{Symbol: sym, Sources: []string{"backtest"}})
		}
	}

	runtime := int((ts - int64(r.cfg.StartTS*1000)) / 60000)
	ctx := &kernel.Context{
		CurrentTime:             time.UnixMilli(ts).UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes:          runtime,
		CallCount:               callCount,
		Account:                 accountInfo,
		Positions:               positions,
		CandidateCoins:          candidateCoins,
		PromptVariant:           r.cfg.PromptVariant,
		StrategyTriggeredCloses: strategyClosesThisCycle,
		MarketDataMap:           marketData,
		MultiTFMarket:           multiTF,
		BTCETHLeverage:          r.cfg.Leverage.BTCETHLeverage,
		AltcoinLeverage:         r.cfg.Leverage.AltcoinLeverage,
		Timeframes:              r.cfg.Timeframes,
	}

	// Fetch quantitative data if enabled in strategy (uses current data as approximation)
	strategyConfig := r.strategyEngine.GetConfig()
	if strategyConfig.Indicators.EnableQuantData {
		// Collect symbols to query (candidate coins + position coins)
		symbolSet := make(map[string]bool)
		for _, sym := range r.cfg.Symbols {
			symbolSet[sym] = true
		}
		for _, pos := range positions {
			symbolSet[pos.Symbol] = true
		}
		symbols := make([]string, 0, len(symbolSet))
		for sym := range symbolSet {
			symbols = append(symbols, sym)
		}
		ctx.QuantDataMap = r.strategyEngine.FetchQuantDataBatch(symbols)
		if len(ctx.QuantDataMap) > 0 {
			logger.Infof("📊 Backtest: fetched quant data for %d symbols", len(ctx.QuantDataMap))
		}
	}

	// Fetch OI ranking data if enabled in strategy (uses current data as approximation)
	if strategyConfig.Indicators.EnableOIRanking {
		ctx.OIRankingData = r.strategyEngine.FetchOIRankingData()
		if ctx.OIRankingData != nil {
			logger.Infof("📊 Backtest: OI ranking data ready: %d top, %d low positions",
				len(ctx.OIRankingData.TopPositions), len(ctx.OIRankingData.LowPositions))
		}
	}

	// Fetch NetFlow ranking data if enabled in strategy
	if strategyConfig.Indicators.EnableNetFlowRanking {
		ctx.NetFlowRankingData = r.strategyEngine.FetchNetFlowRankingData()
		if ctx.NetFlowRankingData != nil {
			logger.Infof("💰 Backtest: NetFlow ranking data ready: inst_in=%d, inst_out=%d",
				len(ctx.NetFlowRankingData.InstitutionFutureTop), len(ctx.NetFlowRankingData.InstitutionFutureLow))
		}
	}

	// Fetch Price ranking data if enabled in strategy
	if strategyConfig.Indicators.EnablePriceRanking {
		ctx.PriceRankingData = r.strategyEngine.FetchPriceRankingData()
		if ctx.PriceRankingData != nil {
			logger.Infof("📈 Backtest: Price ranking data ready for %d durations",
				len(ctx.PriceRankingData.Durations))
		}
	}

	record := &store.DecisionRecord{
		AccountState: store.AccountSnapshot{
			TotalBalance:          accountInfo.TotalEquity,
			AvailableBalance:      accountInfo.AvailableBalance,
			TotalUnrealizedProfit: unrealized,
			PositionCount:         accountInfo.PositionCount,
			MarginUsedPct:         accountInfo.MarginUsedPct,
		},
		CandidateCoins: make([]string, 0, len(candidateCoins)),
		Positions:      r.snapshotPositions(priceMap),
		ExecutionLog:   make([]string, 0),
	}
	for _, coin := range candidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}
	record.Timestamp = time.UnixMilli(ts).UTC()

	for _, c := range strategyClosesThisCycle {
		record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("策略平仓: %s %s @ %.4f (%s)", c.Symbol, c.Side, c.Price, c.Reason))
	}

	return ctx, record, nil
}

func (r *Runner) fillDecisionRecord(record *store.DecisionRecord, full *kernel.FullDecision) {
	record.InputPrompt = full.UserPrompt
	record.CoTTrace = full.CoTTrace
	if len(full.Decisions) > 0 {
		if data, err := json.MarshalIndent(full.Decisions, "", "  "); err == nil {
			record.DecisionJSON = string(data)
		}
	}
}

func (r *Runner) invokeAIWithRetry(ctx *kernel.Context) (*kernel.FullDecision, error) {
	var lastErr error
	for attempt := 0; attempt < aiDecisionMaxRetries; attempt++ {
		// Use GetFullDecisionWithStrategy with the pre-configured strategy engine
		// This ensures backtest uses the same unified prompt generation as live trading
		fd, err := kernel.GetFullDecisionWithStrategy(
			ctx,
			r.mcpClient,
			r.strategyEngine,
			r.cfg.PromptVariant,
			r.cfg.RunID,
			"",
			nil,
		)
		if err == nil {
			return fd, nil
		}
		lastErr = err
		delay := time.Duration(attempt+1) * 500 * time.Millisecond
		time.Sleep(delay)
	}
	return nil, lastErr
}

func (r *Runner) executeDecision(dec kernel.Decision, priceMap map[string]float64, ts int64, cycle int, marketData map[string]*market.Data) (store.DecisionAction, []TradeEvent, string, error) {
	symbol := dec.Symbol
	// hold/wait do not need a valid symbol or price (e.g. Claude may return symbol "ALL" for "wait").
	// Treat as success so the run continues; next cycle may return real symbols and 候选币种分析.
	if dec.Action == "hold" || dec.Action == "wait" {
		actionRecord := store.DecisionAction{
			Action:    dec.Action,
			Symbol:    symbol,
			Success:   true,
			Timestamp: time.UnixMilli(ts).UTC(),
		}
		return actionRecord, nil, "观望，无操作", nil
	}

	if symbol == "" {
		return store.DecisionAction{}, nil, "", fmt.Errorf("empty symbol in decision")
	}

	usedLeverage := r.resolveLeverage(dec.Leverage, symbol)
	actionRecord := store.DecisionAction{
		Action:    dec.Action,
		Symbol:    symbol,
		Leverage:  usedLeverage,
		Timestamp: time.UnixMilli(ts).UTC(),
	}

	if priceMap == nil {
		return actionRecord, nil, "", fmt.Errorf("priceMap is nil")
	}

	basePrice, ok := priceMap[symbol]
	if !ok || basePrice <= 0 {
		return actionRecord, nil, "", fmt.Errorf("price unavailable for %s (found=%v, price=%.4f)", symbol, ok, basePrice)
	}
	fillPrice := r.executionPrice(symbol, basePrice, ts)

	atrAtOpen := 0.0
	if marketData != nil {
		if md, ok := marketData[symbol]; ok && md != nil {
			if md.LongerTermContext != nil && md.LongerTermContext.ATR14 > 0 {
				atrAtOpen = md.LongerTermContext.ATR14
			} else if md.IntradaySeries != nil && md.IntradaySeries.ATR14 > 0 {
				atrAtOpen = md.IntradaySeries.ATR14
			}
		}
	}

	switch dec.Action {
	case "open_long":
		qty := r.determineQuantity(dec, basePrice)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid qty")
		}
		pos, fee, execPrice, err := r.account.Open(symbol, "long", qty, usedLeverage, fillPrice, ts)
		if err != nil {
			return actionRecord, nil, "", err
		}
		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = pos.Leverage
		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "long",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      execPrice - basePrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   0,
			Leverage:      pos.Leverage,
			Cycle:         cycle,
			PositionAfter: pos.Quantity,
			StopLoss:      dec.StopLoss,
			TakeProfit:   dec.TakeProfit,
			ATRAtOpen:    atrAtOpen,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "open_short":
		qty := r.determineQuantity(dec, basePrice)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid qty")
		}
		pos, fee, execPrice, err := r.account.Open(symbol, "short", qty, usedLeverage, fillPrice, ts)
		if err != nil {
			return actionRecord, nil, "", err
		}
		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = pos.Leverage
		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "short",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      basePrice - execPrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   0,
			Leverage:      pos.Leverage,
			Cycle:         cycle,
			PositionAfter: pos.Quantity,
			StopLoss:      dec.StopLoss,
			TakeProfit:   dec.TakeProfit,
			ATRAtOpen:    atrAtOpen,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "close_long":
		qty := r.determineCloseQuantity(symbol, "long", dec)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid close qty")
		}
		openTime := r.getPositionOpenTime(symbol, "long")
		posLev := r.account.positionLeverage(symbol, "long")
		realized, fee, execPrice, err := r.account.Close(symbol, "long", qty, fillPrice)
		if err != nil {
			return actionRecord, nil, "", err
		}
		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = posLev
		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "long",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      basePrice - execPrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   realized - fee,
			Leverage:      posLev,
			Cycle:         cycle,
			PositionAfter: r.remainingPosition(symbol, "long"),
			OpenTime:      openTime,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "close_short":
		qty := r.determineCloseQuantity(symbol, "short", dec)
		if qty <= 0 {
			return actionRecord, nil, "", fmt.Errorf("invalid close qty")
		}
		openTime := r.getPositionOpenTime(symbol, "short")
		posLev := r.account.positionLeverage(symbol, "short")
		realized, fee, execPrice, err := r.account.Close(symbol, "short", qty, fillPrice)
		if err != nil {
			return actionRecord, nil, "", err
		}
		actionRecord.Quantity = qty
		actionRecord.Price = execPrice
		actionRecord.Leverage = posLev
		trade := TradeEvent{
			Timestamp:     ts,
			Symbol:        symbol,
			Action:        dec.Action,
			Side:          "short",
			Quantity:      qty,
			Price:         execPrice,
			Fee:           fee,
			Slippage:      execPrice - basePrice,
			OrderValue:    execPrice * qty,
			RealizedPnL:   realized - fee,
			Leverage:      posLev,
			Cycle:         cycle,
			PositionAfter: r.remainingPosition(symbol, "short"),
			OpenTime:      openTime,
		}
		return actionRecord, []TradeEvent{trade}, "", nil

	case "hold", "wait":
		return actionRecord, nil, fmt.Sprintf("hold position: %s", dec.Action), nil
	default:
		return actionRecord, nil, "", fmt.Errorf("unsupported action %s", dec.Action)
	}
}

// MinPositionSizeUSD is the minimum position size in USD to avoid dust positions
const MinPositionSizeUSD = 10.0

func (r *Runner) determineQuantity(dec kernel.Decision, price float64) float64 {
	snapshot := r.snapshotState()
	equity := snapshot.Equity
	if equity <= 0 {
		equity = r.account.InitialBalance()
	}

	// Get leverage for this symbol
	leverage := r.resolveLeverage(dec.Leverage, dec.Symbol)
	if leverage <= 0 {
		leverage = 5
	}

	// Calculate available margin (leave some buffer for fees)
	availableCash := r.account.Cash()
	maxMarginToUse := availableCash * 0.9 // Use max 90% of available cash
	maxPositionValue := maxMarginToUse * float64(leverage)

	sizeUSD := dec.PositionSizeUSD
	if sizeUSD <= 0 {
		// Default to 5% of equity, but cap to available margin
		sizeUSD = 0.05 * equity
	}

	// Cap position size to what we can actually afford
	if sizeUSD > maxPositionValue {
		logger.Infof("📊 Backtest: capping position from %.2f to %.2f (available margin: %.2f, leverage: %dx)",
			sizeUSD, maxPositionValue, maxMarginToUse, leverage)
		sizeUSD = maxPositionValue
	}

	// Reject positions below minimum size to avoid dust positions
	if sizeUSD < MinPositionSizeUSD {
		logger.Infof("📊 Backtest: rejecting position size %.2f USD (below minimum %.2f USD)",
			sizeUSD, MinPositionSizeUSD)
		return 0
	}

	qty := sizeUSD / price
	if qty < 0 {
		qty = 0
	}
	return qty
}

func (r *Runner) determineCloseQuantity(symbol, side string, dec kernel.Decision) float64 {
	for _, pos := range r.account.Positions() {
		if pos.Symbol == strings.ToUpper(symbol) && pos.Side == side {
			return pos.Quantity
		}
	}
	return 0
}

func (r *Runner) resolveLeverage(requested int, symbol string) int {
	sym := strings.ToUpper(symbol)
	isBTCETH := sym == "BTCUSDT" || sym == "ETHUSDT"

	// Determine configured max leverage for this symbol type
	var maxLeverage int
	if isBTCETH {
		maxLeverage = r.cfg.Leverage.BTCETHLeverage
		if maxLeverage <= 0 {
			maxLeverage = 10 // Default max for BTC/ETH
		}
	} else {
		maxLeverage = r.cfg.Leverage.AltcoinLeverage
		if maxLeverage <= 0 {
			maxLeverage = 5 // Default max for altcoins
		}
	}

	// Use requested leverage if provided, otherwise use max as default
	leverage := requested
	if leverage <= 0 {
		leverage = maxLeverage
	}

	// Enforce max leverage limit
	if leverage > maxLeverage {
		logger.Infof("📊 Backtest: capping leverage from %dx to %dx for %s",
			leverage, maxLeverage, symbol)
		leverage = maxLeverage
	}

	return leverage
}

func (r *Runner) remainingPosition(symbol, side string) float64 {
	for _, pos := range r.account.Positions() {
		if pos.Symbol == strings.ToUpper(symbol) && pos.Side == side {
			return pos.Quantity
		}
	}
	return 0
}

func (r *Runner) getPositionOpenTime(symbol, side string) int64 {
	for _, pos := range r.account.Positions() {
		if pos.Symbol == strings.ToUpper(symbol) && pos.Side == side {
			return pos.OpenTime
		}
	}
	return 0
}

func (r *Runner) snapshotPositions(priceMap map[string]float64) []store.PositionSnapshot {
	positions := r.account.Positions()
	list := make([]store.PositionSnapshot, 0, len(positions))
	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		list = append(list, store.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        price,
			UnrealizedProfit: unrealizedPnL(pos, price),
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}
	return list
}

func (r *Runner) convertPositions(priceMap map[string]float64) []kernel.PositionInfo {
	positions := r.account.Positions()
	list := make([]kernel.PositionInfo, 0, len(positions))
	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		pnl := unrealizedPnL(pos, price)
		// Calculate P&L percentage based on entry notional (position cost)
		pnlPct := 0.0
		if pos.Notional > 0 {
			pnlPct = (pnl / pos.Notional) * 100
		}
		list = append(list, kernel.PositionInfo{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        price,
			Quantity:         pos.Quantity,
			Leverage:         pos.Leverage,
			UnrealizedPnL:    pnl,
			UnrealizedPnLPct: pnlPct,
			LiquidationPrice: pos.LiquidationPrice,
			MarginUsed:       pos.Margin,
			UpdateTime:       time.Now().UnixMilli(),
		})
	}
	return list
}

func (r *Runner) executionPrice(symbol string, markPrice float64, ts int64) float64 {
	curr, next := r.feed.decisionBarSnapshot(symbol, ts)
	switch r.cfg.FillPolicy {
	case FillPolicyNextOpen:
		if next != nil && next.Open > 0 {
			return next.Open
		}
	case FillPolicyBarVWAP:
		if curr != nil {
			if vwap := barVWAP(*curr); vwap > 0 {
				return vwap
			}
		}
	case FillPolicyMidPrice:
		if curr != nil && curr.High > 0 && curr.Low > 0 {
			return (curr.High + curr.Low) / 2
		}
	}
	return markPrice
}

// positionParam holds per-position SL/TP and layered TP state.
type positionParam struct {
	StopLoss          float64
	TakeProfit        float64
	ATRAtOpen         float64
	ScaledLevelsTaken []float64 // profit percents already taken (e.g. 3, 5 for 3%, 5%)
}

func (r *Runner) getPositionParams(key string) (p positionParam, ok bool) {
	r.positionParamsMu.RLock()
	defer r.positionParamsMu.RUnlock()
	p, ok = r.positionParams[key]
	if ok {
		p.ScaledLevelsTaken = append([]float64(nil), p.ScaledLevelsTaken...)
	}
	return p, ok
}

func (r *Runner) setPositionParams(key string, stopLoss, takeProfit, atrAtOpen float64) {
	r.positionParamsMu.Lock()
	defer r.positionParamsMu.Unlock()
	if r.positionParams == nil {
		r.positionParams = make(map[string]positionParam)
	}
	r.positionParams[key] = positionParam{StopLoss: stopLoss, TakeProfit: takeProfit, ATRAtOpen: atrAtOpen}
}

// addScaledLevelTaken records that a scaled TP level (profitPercent) was taken for the position.
func (r *Runner) addScaledLevelTaken(key string, profitPercent float64) {
	r.positionParamsMu.Lock()
	defer r.positionParamsMu.Unlock()
	if r.positionParams == nil {
		return
	}
	p := r.positionParams[key]
	p.ScaledLevelsTaken = append(p.ScaledLevelsTaken, profitPercent)
	r.positionParams[key] = p
}

func (r *Runner) deletePositionParams(key string) {
	r.positionParamsMu.Lock()
	defer r.positionParamsMu.Unlock()
	delete(r.positionParams, key)
}

func (r *Runner) totalMarginUsed() float64 {
	sum := 0.0
	for _, pos := range r.account.Positions() {
		sum += pos.Margin
	}
	return sum
}

func (r *Runner) updateState(ts int64, equity, unrealized, marginUsed float64, priceMap map[string]float64, advancedDecision bool) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	if r.state.MaxEquity == 0 || equity > r.state.MaxEquity {
		r.state.MaxEquity = equity
	}
	if r.state.MinEquity == 0 || equity < r.state.MinEquity {
		r.state.MinEquity = equity
	}
	if r.state.MaxEquity > 0 {
		drawdown := ((r.state.MaxEquity - equity) / r.state.MaxEquity) * 100
		if drawdown > r.state.MaxDrawdownPct {
			r.state.MaxDrawdownPct = drawdown
		}
	}

	positions := make(map[string]PositionSnapshot)
	for _, pos := range r.account.Positions() {
		key := fmt.Sprintf("%s:%s", pos.Symbol, pos.Side)
		snap := PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			Quantity:         pos.Quantity,
			AvgPrice:         pos.EntryPrice,
			Leverage:         pos.Leverage,
			LiquidationPrice: pos.LiquidationPrice,
			MarginUsed:       pos.Margin,
			OpenTime:         pos.OpenTime,
			AccumulatedFee:   pos.AccumulatedFee,
		}
		if p, ok := r.getPositionParams(key); ok {
			snap.StopLoss, snap.TakeProfit, snap.ATRAtOpen = p.StopLoss, p.TakeProfit, p.ATRAtOpen
		}
		positions[key] = snap
	}

	r.state.BarTimestamp = ts
	r.state.BarIndex++
	if advancedDecision {
		r.state.DecisionCycle++
	}
	r.state.Cash = r.account.Cash()
	r.state.Equity = equity
	r.state.UnrealizedPnL = unrealized
	r.state.RealizedPnL = r.account.RealizedPnL()
	r.state.Positions = positions
	r.state.LastUpdate = time.Now().UTC()
}

func (r *Runner) maybeCheckpoint() error {
	state := r.snapshotState()
	shouldCheckpoint := false

	if r.cfg.CheckpointIntervalBars > 0 && state.BarIndex > 0 && state.BarIndex%r.cfg.CheckpointIntervalBars == 0 {
		shouldCheckpoint = true
	}

	interval := time.Duration(r.cfg.CheckpointIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if time.Since(r.lastCheckpoint) >= interval {
		shouldCheckpoint = true
	}

	if !shouldCheckpoint {
		return nil
	}

	if err := r.saveCheckpoint(state); err != nil {
		return err
	}

	return nil
}

func (r *Runner) snapshotForCheckpoint(state BacktestState) []PositionSnapshot {
	res := make([]PositionSnapshot, 0, len(state.Positions))
	for _, pos := range state.Positions {
		res = append(res, pos)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Symbol == res[j].Symbol {
			return res[i].Side < res[j].Side
		}
		return res[i].Symbol < res[j].Symbol
	})
	return res
}

// checkAndExecuteDynamicStopTakeProfit runs strategy dynamic SL/TP (same logic as live trading). Returns trade events for any forced closes.
func (r *Runner) checkAndExecuteDynamicStopTakeProfit(ts int64, marketData map[string]*market.Data, priceMap map[string]float64, cycle int) []TradeEvent {
	sc := r.cfg.ToStrategyConfig()
	if sc == nil {
		return nil
	}
	slConfig := sc.RiskControl.DynamicStopLoss
	tpConfig := sc.RiskControl.DynamicTakeProfit
	if (slConfig == nil || !slConfig.Enabled) && (tpConfig == nil || !tpConfig.Enabled) {
		return nil
	}
	stopLossChecker := kernel.NewStopLossChecker(slConfig)
	takeProfitChecker := kernel.NewTakeProfitChecker(tpConfig)
	if stopLossChecker == nil && takeProfitChecker == nil {
		return nil
	}

	positions := r.account.Positions()
	if len(positions) == 0 {
		return nil
	}

	minHoldMs := 0.0
	if slConfig != nil && slConfig.MinHoldMinutes > 0 {
		minHoldMs = slConfig.MinHoldMinutes * 60 * 1000
	}
	if tpConfig != nil && tpConfig.MinHoldMinutes > 0 {
		if tpConfig.MinHoldMinutes*60*1000 > minHoldMs {
			minHoldMs = tpConfig.MinHoldMinutes * 60 * 1000
		}
	}

	var events []TradeEvent
	for _, pos := range positions {
		key := pos.Symbol + ":" + pos.Side
		holdDurationMs := ts - pos.OpenTime
		if minHoldMs > 0 && float64(holdDurationMs) < minHoldMs {
			continue // 未满最小持仓时间，不触发动态止损/止盈，避免开仓即平仓
		}

		bar := r.feed.GetBarAt(pos.Symbol, ts)
		if bar == nil {
			continue
		}
		var priceForSL, priceForTP, highestForTrailing float64
		if pos.Side == "long" {
			priceForSL = bar.Low
			priceForTP = bar.High
			highestForTrailing = bar.High
		} else {
			priceForSL = bar.High
			priceForTP = bar.Low
			highestForTrailing = bar.Low
		}

		markPrice := priceMap[pos.Symbol]
		if markPrice <= 0 {
			continue
		}
		unrealized := 0.0
		if pos.Side == "long" {
			unrealized = (markPrice - pos.EntryPrice) * pos.Quantity
		} else {
			unrealized = (pos.EntryPrice - markPrice) * pos.Quantity
		}
		marginUsed := pos.Margin
		pnlPct := 0.0
		if marginUsed > 0 {
			pnlPct = (unrealized / marginUsed) * 100
		}
		var posParams positionParam
		posParamsOk := false
		scaledTaken := []float64(nil)
		if p, ok := r.getPositionParams(key); ok {
			scaledTaken = p.ScaledLevelsTaken
			posParams = p
			posParamsOk = true
		}
		posInfo := &kernel.PositionInfo{
			Symbol:            pos.Symbol,
			Side:              pos.Side,
			EntryPrice:        pos.EntryPrice,
			MarkPrice:         markPrice,
			Quantity:          pos.Quantity,
			Leverage:          pos.Leverage,
			UnrealizedPnL:     unrealized,
			UnrealizedPnLPct:  pnlPct,
			PeakPnLPct:        pnlPct,
			LiquidationPrice:  pos.LiquidationPrice,
			MarginUsed:        marginUsed,
			UpdateTime:        pos.OpenTime,
			ScaledLevelsTaken: scaledTaken,
		}

		atr := 0.0
		atrLong := 0.0
		supportLevel := 0.0
		resistanceLevel := 0.0
		if md, ok := marketData[pos.Symbol]; ok && md != nil {
			if md.LongerTermContext != nil && md.LongerTermContext.ATR14 > 0 {
				atr = md.LongerTermContext.ATR14
			} else if md.IntradaySeries != nil && md.IntradaySeries.ATR14 > 0 {
				atr = md.IntradaySeries.ATR14
			}
		}
		klines := r.feed.KlinesUpTo(pos.Symbol, ts)
		if len(klines) >= 3 {
			if pos.Side == "long" {
				supportLevel = kernel.FindSupportLevel(klines, markPrice, 30)
			} else {
				resistanceLevel = kernel.FindResistanceLevel(klines, markPrice, 30)
			}
		}
		if len(klines) >= 29 {
			atrLong = kernel.CalculateATR(klines, 28)
			if atr == 0 {
				atr = kernel.CalculateATR(klines, 14)
			}
		}
		// Running extreme since entry (for trailing TP/SL, align with live)
		if len(klines) > 0 {
			if pos.Side == "long" {
				runHigh := pos.EntryPrice
				for _, k := range klines {
					if k.OpenTime >= pos.OpenTime && k.High > runHigh {
						runHigh = k.High
					}
				}
				highestForTrailing = runHigh
			} else {
				runLow := pos.EntryPrice
				for _, k := range klines {
					if k.OpenTime >= pos.OpenTime && k.Low < runLow {
						runLow = k.Low
					}
				}
				highestForTrailing = runLow
			}
		}
		// Long: SL uses support; short: SL uses resistance (same as live)
		slLevel := supportLevel
		if pos.Side == "short" {
			slLevel = resistanceLevel
		}

		var closeReason string
		var tpPartialPct float64
		triggered := false
		var tpSig *kernel.TakeProfitSignal
		if stopLossChecker != nil {
			sig := stopLossChecker.CheckStopLoss(posInfo, priceForSL, highestForTrailing, atr, atrLong, slLevel)
			if sig != nil && sig.Triggered {
				requiredCycles := 1
				if slConfig.ConfirmCycles > 0 {
					requiredCycles = slConfig.ConfirmCycles
				}
				if slConfig.ATRToleranceEnabled != nil && *slConfig.ATRToleranceEnabled && atrLong > 0 {
					highMult := 1.2
					if slConfig.ATRHighMultiplier != nil {
						highMult = *slConfig.ATRHighMultiplier
					}
					if atr > atrLong*highMult {
						requiredCycles++
					}
				}
				r.slConfirmCountMu.Lock()
				r.slConfirmCount[key]++
				count := r.slConfirmCount[key]
				r.slConfirmCountMu.Unlock()
				if count >= requiredCycles {
					triggered = true
					closeReason = sig.Type
					logger.Infof("📉 Backtest dynamic SL: %s %s %s @ %.4f", pos.Symbol, pos.Side, sig.Type, sig.Price)
				}
			} else {
				r.slConfirmCountMu.Lock()
				delete(r.slConfirmCount, key)
				r.slConfirmCountMu.Unlock()
			}
		}
		if !triggered && takeProfitChecker != nil {
			sig := takeProfitChecker.CheckTakeProfit(posInfo, priceForTP, highestForTrailing, atr, atrLong, resistanceLevel)
			if sig != nil && sig.Triggered {
				triggered = true
				closeReason = sig.Type
				tpSig = sig
				tpPartialPct = sig.PartialPercent
				logger.Infof("📈 Backtest dynamic TP: %s %s %s @ %.4f (close %.1f%%)", pos.Symbol, pos.Side, sig.Type, sig.Price, tpPartialPct)
			}
		}
		if !triggered {
			continue
		}

		execPrice := priceMap[pos.Symbol]
		fullQty := pos.Quantity
		closeQty := fullQty
		if tpSig != nil && tpPartialPct > 0 && tpPartialPct < 100 {
			closeQty = fullQty * (tpPartialPct / 100)
			if closeQty <= 0 || closeQty > fullQty {
				closeQty = fullQty
			}
		}
		realized, fee, finalPrice, err := r.account.Close(pos.Symbol, pos.Side, closeQty, execPrice)
		if err != nil {
			logger.Infof("⚠️ Backtest dynamic SL/TP close failed %s %s: %v", pos.Symbol, pos.Side, err)
			continue
		}
		positionAfter := fullQty - closeQty
		if positionAfter <= 0 {
			r.deletePositionParams(key)
			r.slConfirmCountMu.Lock()
			delete(r.slConfirmCount, key)
			r.slConfirmCountMu.Unlock()
		} else if tpSig != nil && tpSig.Type == "scaled" {
			// Keep scaled dedup units consistent with checkScaledTakeProfit:
			// store the same profit% threshold unit used in the scaled checker.
			// (ROE mode => ROE%; price mode => price%)
			profitPct := tpSig.ScaledProfitPercentUsed
			if profitPct <= 0 {
				// Fallback (shouldn't happen for scaled signals).
				if pos.Side == "long" {
					profitPct = (tpSig.Price - pos.EntryPrice) / pos.EntryPrice * 100
				} else {
					profitPct = (pos.EntryPrice - tpSig.Price) / pos.EntryPrice * 100
				}
			}
			r.addScaledLevelTaken(key, profitPct)
		}

		action := "close_long"
		if pos.Side == "short" {
			action = "close_short"
		}
		evt := TradeEvent{
			Timestamp:     ts,
			Symbol:        pos.Symbol,
			Action:        action,
			Side:          pos.Side,
			Quantity:      closeQty,
			Price:         finalPrice,
			Fee:           fee,
			Slippage:      0,
			OrderValue:    finalPrice * closeQty,
			RealizedPnL:   realized - fee,
			Leverage:      pos.Leverage,
			Cycle:         cycle,
			PositionAfter: positionAfter,
			Note:          "dynamic_stop_take_profit",
			OpenTime:      pos.OpenTime,
			CloseReason:   closeReason,
		}
		// Attach same params as position (final fixed values at close)
		if posParamsOk {
			evt.StopLoss = posParams.StopLoss
			evt.TakeProfit = posParams.TakeProfit
			evt.ATRAtOpen = posParams.ATRAtOpen
			if posParams.ATRAtOpen > 0 {
				if pos.Side == "long" {
					if posParams.StopLoss > 0 {
						evt.ATRMultipleSL = (pos.EntryPrice - posParams.StopLoss) / posParams.ATRAtOpen
					}
					if posParams.TakeProfit > 0 {
						evt.ATRMultipleTP = (posParams.TakeProfit - pos.EntryPrice) / posParams.ATRAtOpen
					}
				} else {
					if posParams.StopLoss > 0 {
						evt.ATRMultipleSL = (posParams.StopLoss - pos.EntryPrice) / posParams.ATRAtOpen
					}
					if posParams.TakeProfit > 0 {
						evt.ATRMultipleTP = (pos.EntryPrice - posParams.TakeProfit) / posParams.ATRAtOpen
					}
				}
			}
			evt.ScaledTPLevel = len(posParams.ScaledLevelsTaken)
		}
		if sc := r.cfg.ToStrategyConfig(); sc != nil {
			if sl := sc.RiskControl.DynamicStopLoss; sl != nil && sl.ATRPeriodAltcoin != nil {
				evt.ATRPeriod = *sl.ATRPeriodAltcoin
			}
			if tp := sc.RiskControl.DynamicTakeProfit; tp != nil && tp.ScaledEnabled != nil && *tp.ScaledEnabled && len(tp.ScaledLevels) > 0 {
				for i := 0; i < evt.ScaledTPLevel && i < len(tp.ScaledLevels); i++ {
					evt.ScaledTPClosedPct += tp.ScaledLevels[i].ClosePercent
				}
				if evt.ScaledTPClosedPct > 100 {
					evt.ScaledTPClosedPct = 100
				}
			}
			if sl := sc.RiskControl.DynamicStopLoss; sl != nil && sl.TrailingEnabled != nil && *sl.TrailingEnabled && len(sl.TrailingLevels) > 0 {
				for i, lv := range sl.TrailingLevels {
					if pnlPct >= lv.ProfitThreshold {
						evt.TrailingTierActivated = i + 1
						evt.TrailingAllowedDrawdown = lv.TrailingPercent
					}
				}
			}
		}
		events = append(events, evt)
	}
	return events
}

func (r *Runner) checkLiquidation(ts int64, priceMap map[string]float64, cycle int) ([]TradeEvent, string, error) {
	positions := append([]*position(nil), r.account.Positions()...)
	events := make([]TradeEvent, 0)
	var noteBuilder strings.Builder

	for _, pos := range positions {
		price := priceMap[pos.Symbol]
		liqPrice := pos.LiquidationPrice
		trigger := false
		execPrice := price
		if pos.Side == "long" {
			if price <= liqPrice && liqPrice > 0 {
				trigger = true
				execPrice = liqPrice
			}
		} else {
			if price >= liqPrice && liqPrice > 0 {
				trigger = true
				execPrice = liqPrice
			}
		}
		if !trigger {
			continue
		}

		closeQty := pos.Quantity
		realized, fee, finalPrice, err := r.account.Close(pos.Symbol, pos.Side, closeQty, execPrice)
		if err != nil {
			return nil, "", err
		}

		noteBuilder.WriteString(fmt.Sprintf("%s %s @ %.4f; ", pos.Symbol, pos.Side, finalPrice))

		evt := TradeEvent{
			Timestamp:       ts,
			Symbol:          pos.Symbol,
			Action:          "liquidated",
			Side:            pos.Side,
			Quantity:        closeQty,
			Price:           finalPrice,
			Fee:             fee,
			Slippage:        0,
			OrderValue:      finalPrice * closeQty,
			RealizedPnL:     realized - fee,
			Leverage:        pos.Leverage,
			Cycle:           cycle,
			PositionAfter:   0,
			LiquidationFlag: true,
			Note:            fmt.Sprintf("forced liquidation at %.4f", finalPrice),
			OpenTime:        pos.OpenTime,
		}
		events = append(events, evt)
	}

	if len(events) == 0 {
		return events, "", nil
	}

	note := strings.TrimSuffix(noteBuilder.String(), "; ")

	r.stateMu.Lock()
	r.state.Liquidated = true
	r.state.LiquidationNote = note
	r.stateMu.Unlock()

	return events, note, nil
}

func (r *Runner) shouldTriggerDecision(barIndex int) bool {
	if r.cfg.DecisionCadenceNBars <= 1 {
		return true
	}
	if barIndex < 0 {
		return true
	}
	return barIndex%r.cfg.DecisionCadenceNBars == 0
}

func (r *Runner) handleStop(reason error) {
	r.forceCheckpoint()
	if reason != nil {
		r.setLastError(reason)
	} else {
		r.setLastError(nil)
	}
	r.statusMu.Lock()
	r.err = reason
	r.status = RunStateStopped
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handlePause() {
	r.forceCheckpoint()
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStatePaused
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
}

func (r *Runner) resumeFromPause() {
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStateRunning
	r.statusMu.Unlock()
	r.persistMetadata()
}

func (r *Runner) handleCompletion() {
	r.setLastError(nil)
	r.statusMu.Lock()
	r.status = RunStateCompleted
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handleFailure(err error) {
	r.forceCheckpoint()
	if err != nil {
		r.setLastError(err)
	}
	r.statusMu.Lock()
	r.err = err
	r.status = RunStateFailed
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) handleLiquidation() {
	r.forceCheckpoint()
	r.setLastError(errLiquidated)
	r.statusMu.Lock()
	r.err = errLiquidated
	r.status = RunStateLiquidated
	r.statusMu.Unlock()
	r.persistMetadata()
	r.persistMetrics(true)
	r.releaseLock()
}

func (r *Runner) Pause() {
	select {
	case r.pauseCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Resume() {
	select {
	case r.resumeCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Stop() {
	select {
	case r.stopCh <- struct{}{}:
	default:
	}
}

func (r *Runner) Wait() error {
	<-r.doneCh
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return r.err
}

// Status returns the current run state.
func (r *Runner) Status() RunState {
	r.statusMu.RLock()
	defer r.statusMu.RUnlock()
	return r.status
}

// StatusPayload builds the status response for the API.
func (r *Runner) StatusPayload() StatusPayload {
	snapshot := r.snapshotState()
	progress := progressPercent(snapshot, r.cfg)

	// Build position statuses with unrealized P&L
	positions := make([]PositionStatus, 0, len(snapshot.Positions))
	for _, pos := range snapshot.Positions {
		if pos.Quantity <= 0 {
			continue
		}
		// Get mark price from feed if available
		markPrice := pos.AvgPrice // fallback to entry price
		if r.feed != nil && snapshot.BarTimestamp > 0 {
			if md, _, err := r.feed.BuildMarketData(snapshot.BarTimestamp); err == nil {
				if data, ok := md[pos.Symbol]; ok {
					markPrice = data.CurrentPrice
				}
			}
		}

		// Calculate unrealized P&L
		var unrealizedPnL float64
		if pos.Side == "long" {
			unrealizedPnL = (markPrice - pos.AvgPrice) * pos.Quantity
		} else {
			unrealizedPnL = (pos.AvgPrice - markPrice) * pos.Quantity
		}

		// Calculate P&L percentage based on margin
		pnlPct := 0.0
		if pos.MarginUsed > 0 {
			pnlPct = (unrealizedPnL / pos.MarginUsed) * 100
		}

		// ATR multiples for display (instead of raw ATR value)
		atrMultSL, atrMultTP := 0.0, 0.0
		if pos.ATRAtOpen > 0 {
			if pos.Side == "long" {
				if pos.StopLoss > 0 {
					atrMultSL = (pos.AvgPrice - pos.StopLoss) / pos.ATRAtOpen
				}
				if pos.TakeProfit > 0 {
					atrMultTP = (pos.TakeProfit - pos.AvgPrice) / pos.ATRAtOpen
				}
			} else {
				if pos.StopLoss > 0 {
					atrMultSL = (pos.StopLoss - pos.AvgPrice) / pos.ATRAtOpen
				}
				if pos.TakeProfit > 0 {
					atrMultTP = (pos.AvgPrice - pos.TakeProfit) / pos.ATRAtOpen
				}
			}
		}
		// Distance to SL/TP as % of mark price (positive = room before hit)
		distSLPct, distTPPct := 0.0, 0.0
		if markPrice > 0 {
			if pos.Side == "long" {
				if pos.StopLoss > 0 {
					distSLPct = ((markPrice - pos.StopLoss) / markPrice) * 100
				}
				if pos.TakeProfit > 0 {
					distTPPct = ((pos.TakeProfit - markPrice) / markPrice) * 100
				}
			} else {
				if pos.StopLoss > 0 {
					distSLPct = ((pos.StopLoss - markPrice) / markPrice) * 100
				}
				if pos.TakeProfit > 0 {
					distTPPct = ((markPrice - pos.TakeProfit) / markPrice) * 100
				}
			}
		}
		trailingEnabled := false
		scaledTPEnabled := false
		scaledTPLevel := 0
		scaledTPClosedPct := 0.0
		trailingTier := 0
		trailingDrawdown := 0.0
		atrPeriod := 0
		key := pos.Symbol + ":" + pos.Side
		if p, ok := r.getPositionParams(key); ok {
			scaledTPLevel = len(p.ScaledLevelsTaken)
		}
		if sc := r.cfg.ToStrategyConfig(); sc != nil {
			if sc.RiskControl.DynamicStopLoss != nil && sc.RiskControl.DynamicStopLoss.TrailingEnabled != nil {
				trailingEnabled = *sc.RiskControl.DynamicStopLoss.TrailingEnabled
			}
			if sl := sc.RiskControl.DynamicStopLoss; trailingEnabled && sl != nil && len(sl.TrailingLevels) > 0 {
				for i, lv := range sl.TrailingLevels {
					if pnlPct >= lv.ProfitThreshold {
						trailingTier = i + 1
						trailingDrawdown = lv.TrailingPercent
					}
				}
			}
			if sc.RiskControl.DynamicTakeProfit != nil && sc.RiskControl.DynamicTakeProfit.ScaledEnabled != nil {
				scaledTPEnabled = *sc.RiskControl.DynamicTakeProfit.ScaledEnabled
			}
			if tp := sc.RiskControl.DynamicTakeProfit; scaledTPEnabled && tp != nil && len(tp.ScaledLevels) > 0 {
				for i := 0; i < scaledTPLevel && i < len(tp.ScaledLevels); i++ {
					scaledTPClosedPct += tp.ScaledLevels[i].ClosePercent
				}
				if scaledTPClosedPct > 100 {
					scaledTPClosedPct = 100
				}
			}
			if sl := sc.RiskControl.DynamicStopLoss; sl != nil && sl.ATRPeriodAltcoin != nil {
				atrPeriod = *sl.ATRPeriodAltcoin
			}
		}

		positions = append(positions, PositionStatus{
			Symbol:                  pos.Symbol,
			Side:                    pos.Side,
			Quantity:                pos.Quantity,
			EntryPrice:              pos.AvgPrice,
			MarkPrice:               markPrice,
			Leverage:                pos.Leverage,
			UnrealizedPnL:           unrealizedPnL,
			UnrealizedPnLPct:        pnlPct,
			MarginUsed:              pos.MarginUsed,
			StopLoss:                pos.StopLoss,
			TakeProfit:              pos.TakeProfit,
			ATRAtOpen:               pos.ATRAtOpen,
			ATRMultipleSL:           atrMultSL,
			ATRMultipleTP:           atrMultTP,
			DistanceToSLPct:         distSLPct,
			DistanceToTPPct:         distTPPct,
			TrailingEnabled:         trailingEnabled,
			ScaledTPEnabled:         scaledTPEnabled,
			ScaledTPLevel:           scaledTPLevel,
			ScaledTPClosedPct:       scaledTPClosedPct,
			TrailingTierActivated:   trailingTier,
			TrailingAllowedDrawdown: trailingDrawdown,
			ATRPeriod:               atrPeriod,
		})
	}

	payload := StatusPayload{
		RunID:          r.cfg.RunID,
		State:          r.Status(),
		ProgressPct:    progress,
		ProcessedBars:  snapshot.BarIndex,
		CurrentTime:    snapshot.BarTimestamp,
		DecisionCycle:  snapshot.DecisionCycle,
		Equity:         snapshot.Equity,
		UnrealizedPnL:  snapshot.UnrealizedPnL,
		RealizedPnL:    snapshot.RealizedPnL,
		Positions:      positions,
		Note:           snapshot.LiquidationNote,
		LastError:      r.lastErrorString(),
		LastUpdatedIso: snapshot.LastUpdate.UTC().Format(time.RFC3339),
	}
	return payload
}

func (r *Runner) snapshotState() BacktestState {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()

	copyState := *r.state
	copyState.Positions = make(map[string]PositionSnapshot, len(r.state.Positions))
	for k, v := range r.state.Positions {
		copyState.Positions[k] = v
	}
	return copyState
}

func (r *Runner) persistMetadata() {
	state := r.snapshotState()
	meta := r.buildMetadata(state, r.Status())
	meta.CreatedAt = r.createdAt
	if err := SaveRunMetadata(meta); err != nil {
		logger.Infof("failed to save run metadata for %s: %v", r.cfg.RunID, err)
	} else {
		if err := updateRunIndex(meta, &r.cfg); err != nil {
			logger.Infof("failed to update index for %s: %v", r.cfg.RunID, err)
		}
	}
}

func (r *Runner) logDecision(record *store.DecisionRecord) error {
	if record == nil {
		return nil
	}
	persistDecisionRecord(r.cfg.RunID, record)
	return nil
}

func (r *Runner) persistMetrics(force bool) {
	if r.cfg.RunID == "" {
		return
	}

	if !force && !r.lastMetricsWrite.IsZero() {
		if time.Since(r.lastMetricsWrite) < metricsWriteInterval {
			return
		}
	}

	state := r.snapshotState()
	metrics, err := CalculateMetrics(r.cfg.RunID, &r.cfg, &state)
	if err != nil {
		logger.Infof("failed to compute metrics for %s: %v", r.cfg.RunID, err)
		return
	}
	if metrics == nil {
		return
	}
	if err := PersistMetrics(r.cfg.RunID, metrics); err != nil {
		logger.Infof("failed to persist metrics for %s: %v", r.cfg.RunID, err)
		return
	}
	r.lastMetricsWrite = time.Now()
}

func (r *Runner) buildMetadata(state BacktestState, runState RunState) *RunMetadata {
	if state.Liquidated && runState != RunStateLiquidated {
		runState = RunStateLiquidated
	}

	progress := progressPercent(state, r.cfg)

	summary := RunSummary{
		SymbolCount:     len(r.cfg.Symbols),
		DecisionTF:      r.cfg.DecisionTimeframe,
		ProcessedBars:   state.BarIndex,
		ProgressPct:     progress,
		EquityLast:      state.Equity,
		MaxDrawdownPct:  state.MaxDrawdownPct,
		Liquidated:      state.Liquidated,
		LiquidationNote: state.LiquidationNote,
	}

	meta := &RunMetadata{
		RunID:     r.cfg.RunID,
		UserID:    r.cfg.UserID,
		State:     runState,
		LastError: r.lastErrorString(),
		Summary:   summary,
	}

	return meta
}

func progressPercent(state BacktestState, cfg BacktestConfig) float64 {
	duration := cfg.Duration()
	if duration <= 0 {
		return 0
	}
	if state.BarTimestamp == 0 {
		return 0
	}

	start := time.Unix(cfg.StartTS, 0)
	end := time.Unix(cfg.EndTS, 0)
	current := time.UnixMilli(state.BarTimestamp)

	if !current.After(start) {
		return 0
	}
	if current.After(end) {
		return 100
	}

	elapsed := current.Sub(start)
	pct := float64(elapsed) / float64(duration) * 100
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	return pct
}

func (r *Runner) buildCheckpointFromState(state BacktestState) *Checkpoint {
	return &Checkpoint{
		BarIndex:        state.BarIndex,
		BarTimestamp:    state.BarTimestamp,
		Cash:            state.Cash,
		Equity:          state.Equity,
		UnrealizedPnL:   state.UnrealizedPnL,
		RealizedPnL:     state.RealizedPnL,
		Positions:       r.snapshotForCheckpoint(state),
		DecisionCycle:   state.DecisionCycle,
		Liquidated:      state.Liquidated,
		LiquidationNote: state.LiquidationNote,
		MaxEquity:       state.MaxEquity,
		MinEquity:       state.MinEquity,
		MaxDrawdownPct:  state.MaxDrawdownPct,
		AICacheRef:      r.cachePath,
	}
}

func (r *Runner) saveCheckpoint(state BacktestState) error {
	ckpt := r.buildCheckpointFromState(state)
	if ckpt == nil {
		return nil
	}
	if err := SaveCheckpoint(r.cfg.RunID, ckpt); err != nil {
		return err
	}
	r.lastCheckpoint = time.Now()
	return nil
}

func (r *Runner) forceCheckpoint() {
	state := r.snapshotState()
	if err := r.saveCheckpoint(state); err != nil {
		logger.Infof("failed to save checkpoint for %s: %v", r.cfg.RunID, err)
	}
}

func (r *Runner) RestoreFromCheckpoint() error {
	ckpt, err := LoadCheckpoint(r.cfg.RunID)
	if err != nil {
		return err
	}
	return r.applyCheckpoint(ckpt)
}

func (r *Runner) applyCheckpoint(ckpt *Checkpoint) error {
	if ckpt == nil {
		return fmt.Errorf("checkpoint is nil")
	}
	r.account.RestoreFromSnapshots(ckpt.Cash, ckpt.RealizedPnL, ckpt.Positions)
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.state.BarIndex = ckpt.BarIndex
	r.state.BarTimestamp = ckpt.BarTimestamp
	r.state.Cash = ckpt.Cash
	r.state.Equity = ckpt.Equity
	r.state.UnrealizedPnL = ckpt.UnrealizedPnL
	r.state.RealizedPnL = ckpt.RealizedPnL
	r.state.DecisionCycle = ckpt.DecisionCycle
	r.state.Liquidated = ckpt.Liquidated
	r.state.LiquidationNote = ckpt.LiquidationNote
	r.state.MaxEquity = ckpt.MaxEquity
	r.state.MinEquity = ckpt.MinEquity
	r.state.MaxDrawdownPct = ckpt.MaxDrawdownPct
	r.state.Positions = snapshotsToMap(ckpt.Positions)
	r.state.LastUpdate = time.Now().UTC()
	r.lastCheckpoint = time.Now()
	return nil
}

func snapshotsToMap(snaps []PositionSnapshot) map[string]PositionSnapshot {
	positions := make(map[string]PositionSnapshot, len(snaps))
	for _, snap := range snaps {
		key := fmt.Sprintf("%s:%s", snap.Symbol, snap.Side)
		positions[key] = snap
	}
	return positions
}

func sortDecisionsByPriority(decisions []kernel.Decision) []kernel.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	priority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1
		case "open_long", "open_short":
			return 2
		case "hold", "wait":
			return 3
		default:
			return 99
		}
	}

	result := make([]kernel.Decision, len(decisions))
	copy(result, decisions)

	sort.Slice(result, func(i, j int) bool {
		pi := priority(result[i].Action)
		pj := priority(result[j].Action)
		if pi != pj {
			return pi < pj
		}
		return i < j
	})

	return result
}

func barVWAP(k market.Kline) float64 {
	values := []float64{k.Open, k.High, k.Low, k.Close}
	sum := 0.0
	count := 0.0
	for _, v := range values {
		if v > 0 {
			sum += v
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / count
}
