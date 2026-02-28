package trader

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/experience"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/store"
	"nofx/trader/aster"
	"nofx/trader/binance"
	"nofx/trader/bitget"
	"nofx/trader/bybit"
	"nofx/trader/gate"
	"nofx/trader/hyperliquid"
	"nofx/trader/kucoin"
	"nofx/trader/lighter"
	"nofx/trader/okx"
	"nofx/trader/paper"
	"strings"
	"sync"
	"time"
)

// AutoTraderConfig auto trading configuration (simplified version - AI makes all decisions)
type AutoTraderConfig struct {
	// Trader identification
	ID      string // Trader unique identifier (for log directory, etc.)
	Name    string // Trader display name
	AIModel string // AI model: "qwen" or "deepseek"

	// Trading platform selection
	Exchange   string // Exchange type: "binance", "bybit", "okx", "bitget", "gate", "hyperliquid", "aster" or "lighter"
	ExchangeID string // Exchange account UUID (for multi-account support)

	// Binance API configuration
	BinanceAPIKey    string
	BinanceSecretKey string

	// Bybit API configuration
	BybitAPIKey    string
	BybitSecretKey string

	// OKX API configuration
	OKXAPIKey    string
	OKXSecretKey string
	OKXPassphrase string

	// Bitget API configuration
	BitgetAPIKey    string
	BitgetSecretKey string
	BitgetPassphrase string

	// Gate API configuration
	GateAPIKey    string
	GateSecretKey string

	// KuCoin API configuration
	KuCoinAPIKey    string
	KuCoinSecretKey string
	KuCoinPassphrase string

	// Hyperliquid configuration
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster configuration
	AsterUser       string // Aster main wallet address
	AsterSigner     string // Aster API wallet address
	AsterPrivateKey string // Aster API wallet private key

	// LIGHTER configuration
	LighterWalletAddr       string // LIGHTER wallet address (L1 wallet)
	LighterPrivateKey       string // LIGHTER L1 private key (for account identification)
	LighterAPIKeyPrivateKey string // LIGHTER API Key private key (40 bytes, for transaction signing)
	LighterAPIKeyIndex      int    // LIGHTER API Key index (0-255)
	LighterTestnet          bool   // Whether to use testnet

	// AI configuration
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string

	// Custom AI API configuration
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// Scan configuration
	ScanInterval time.Duration // Scan interval (recommended 3 minutes)

	// Account configuration
	InitialBalance float64 // Initial balance (for P&L calculation, must be set manually)

	// Risk control (only as hints, AI can make autonomous decisions)
	MaxDailyLoss    float64       // Maximum daily loss percentage (hint)
	MaxDrawdown     float64       // Maximum drawdown percentage (hint)
	StopTradingTime time.Duration // Pause duration after risk control triggers

	// Position mode
	IsCrossMargin bool // true=cross margin mode, false=isolated margin mode

	// Competition visibility
	ShowInCompetition bool // Whether to show in competition page

	// Simulation/paper trading: virtual balance, no real orders
	IsSimulation bool // When true, use paper trader adapter instead of real exchange

	// Strategy configuration (use complete strategy config)
	StrategyConfig *store.StrategyConfig // Strategy configuration (includes coin sources, indicators, risk control, prompts, etc.)
}

// positionParams fixed params stored at open for API/UI (same as backtest current-position display)
type positionParams struct {
	StopLoss       float64
	TakeProfit     float64
	ATRAtOpen      float64
	ATRMultipleSL  float64
	ATRMultipleTP  float64
	ATRPeriod      int
}

// AutoTrader automatic trader
type AutoTrader struct {
	id                    string // Trader unique identifier
	name                  string // Trader display name
	aiModel               string // AI model name
	exchange              string // Trading platform type (binance/bybit/etc)
	exchangeID            string // Exchange account UUID
	showInCompetition     bool   // Whether to show in competition page
	config                AutoTraderConfig
	trader                Trader // Use Trader interface (supports multiple platforms)
	mcpClient             mcp.AIClient
	marketClient          *market.APIClient        // Market data client for K-line data
	store                 *store.Store             // Data storage (decision records, etc.)
	strategyEngine        *kernel.StrategyEngine // Strategy engine (uses strategy configuration)
	cycleNumber           int                      // Current cycle number
	initialBalance        float64
	dailyPnL              float64
	customPrompt          string // Custom trading strategy prompt
	overrideBasePrompt    bool   // Whether to override base prompt
	lastResetTime         time.Time
	stopUntil             time.Time
	isRunning             bool
	isRunningMutex        sync.RWMutex       // Mutex to protect isRunning flag
	startTime             time.Time          // System start time
	callCount             int                // AI call count
	positionFirstSeenTime map[string]int64   // Position first seen time (symbol_side -> timestamp in milliseconds)
	stopMonitorCh         chan struct{}      // Used to stop monitoring goroutine
	monitorWg             sync.WaitGroup     // Used to wait for monitoring goroutine to finish
	peakPnLCache          map[string]float64   // Peak profit cache (symbol_side -> peak P&L %)
	peakPnLCacheMutex     sync.RWMutex        // Cache read-write lock
	slConfirmCount        map[string]int      // Consecutive cycles SL condition met (posKey -> count); execute only when >= ConfirmCycles
	slConfirmCountMu      sync.Mutex          // Protects slConfirmCount
	aiCloseConfirmCount   map[string]int      // Consecutive cycles AI requested close for a position (symbol_side -> count)
	aiCloseConfirmMu      sync.Mutex          // Protects aiCloseConfirmCount
	scaledLevelsTaken     map[string][]float64 // Scaled TP levels already taken (posKey -> profit percents)
	scaledLevelsTakenMu   sync.RWMutex        // For layered TP parity with backtest
	positionParamsMap     map[string]*positionParams // Fixed params per position (symbol_side -> params for API/UI)
	positionParamsMu      sync.RWMutex
	sltpCheckMu               sync.Mutex   // 串行化策略止盈/止损检查，避免后台协程与 runCycle 并发重复平仓
	strategyTriggeredCloses   []kernel.StrategyTriggeredClose // 本周期或后台触发的策略平仓，供 AI 链展示
	strategyTriggeredClosesMu sync.Mutex
	pendingCloseReason        string       // 本次平仓原因，供 recordAndConfirmOrder 写入 DB（system:sl:xxx / system:tp:xxx / ai / manual）
	pendingCloseReasonMu      sync.Mutex
	lastBalanceSyncTime       time.Time    // Last balance sync time
	userID                string             // User ID
	gridState             *GridState         // Grid trading state (only used when StrategyType == "grid_trading")
}

// NewAutoTrader creates an automatic trader
// st parameter is used to store decision records to database
func NewAutoTrader(config AutoTraderConfig, st *store.Store, userID string) (*AutoTrader, error) {
	// Set default values
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	// Initialize AI client based on provider
	var mcpClient mcp.AIClient
	aiModel := config.AIModel
	if config.UseQwen && aiModel == "" {
		aiModel = "qwen"
	}

	switch aiModel {
	case "claude":
		mcpClient = mcp.NewClaudeClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Claude AI", config.Name)

	case "kimi":
		mcpClient = mcp.NewKimiClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Kimi (Moonshot) AI", config.Name)

	case "gemini":
		mcpClient = mcp.NewGeminiClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Google Gemini AI", config.Name)

	case "grok":
		mcpClient = mcp.NewGrokClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using xAI Grok AI", config.Name)

	case "openai":
		mcpClient = mcp.NewOpenAIClient()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using OpenAI", config.Name)

	case "qwen":
		mcpClient = mcp.NewQwenClient()
		apiKey := config.QwenKey
		if apiKey == "" {
			apiKey = config.CustomAPIKey
		}
		mcpClient.SetAPIKey(apiKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using Alibaba Cloud Qwen AI", config.Name)

	case "custom":
		mcpClient = mcp.New()
		mcpClient.SetAPIKey(config.CustomAPIKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using custom AI API: %s (model: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)

	default: // deepseek or empty
		mcpClient = mcp.NewDeepSeekClient()
		apiKey := config.DeepSeekKey
		if apiKey == "" {
			apiKey = config.CustomAPIKey
		}
		mcpClient.SetAPIKey(apiKey, config.CustomAPIURL, config.CustomModelName)
		logger.Infof("🤖 [%s] Using DeepSeek AI", config.Name)
	}

	if config.CustomAPIURL != "" || config.CustomModelName != "" {
		logger.Infof("🔧 [%s] Custom config - URL: %s, Model: %s", config.Name, config.CustomAPIURL, config.CustomModelName)
	}

	// Set default trading platform
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// Create corresponding trader based on configuration
	var trader Trader
	var err error

	// Simulation: same behaviour as live except virtual funds; do not simplify (see PaperTrader doc).
	if config.IsSimulation {
		logger.Infof("📊 [%s] Paper/Simulation mode: virtual balance %.2f USDT, exchange type %s for market data", config.Name, config.InitialBalance, config.Exchange)
		if config.InitialBalance <= 0 {
			config.InitialBalance = 10000
			logger.Infof("📊 [%s] Simulation initial balance not set, using default 10000 USDT", config.Name)
		}
		trader = paper.NewPaperTrader(config.InitialBalance, config.Exchange)
	} else {
		// Record position mode (general)
		marginModeStr := "Cross Margin"
		if !config.IsCrossMargin {
			marginModeStr = "Isolated Margin"
		}
		logger.Infof("📊 [%s] Position mode: %s", config.Name, marginModeStr)

		switch config.Exchange {
	case "binance":
		logger.Infof("🏦 [%s] Using Binance Futures trading", config.Name)
		trader = binance.NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey, userID)
	case "bybit":
		logger.Infof("🏦 [%s] Using Bybit Futures trading", config.Name)
		trader = bybit.NewBybitTrader(config.BybitAPIKey, config.BybitSecretKey)
	case "okx":
		logger.Infof("🏦 [%s] Using OKX Futures trading", config.Name)
		trader = okx.NewOKXTrader(config.OKXAPIKey, config.OKXSecretKey, config.OKXPassphrase)
	case "bitget":
		logger.Infof("🏦 [%s] Using Bitget Futures trading", config.Name)
		trader = bitget.NewBitgetTrader(config.BitgetAPIKey, config.BitgetSecretKey, config.BitgetPassphrase)
	case "gate":
		logger.Infof("🏦 [%s] Using Gate.io Futures trading", config.Name)
		trader = gate.NewGateTrader(config.GateAPIKey, config.GateSecretKey)
	case "kucoin":
		logger.Infof("🏦 [%s] Using KuCoin Futures trading", config.Name)
		trader = kucoin.NewKuCoinTrader(config.KuCoinAPIKey, config.KuCoinSecretKey, config.KuCoinPassphrase)
	case "hyperliquid":
		logger.Infof("🏦 [%s] Using Hyperliquid trading", config.Name)
		trader, err = hyperliquid.NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Hyperliquid trader: %w", err)
		}
	case "aster":
		logger.Infof("🏦 [%s] Using Aster trading", config.Name)
		trader, err = aster.NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Aster trader: %w", err)
		}
	case "lighter":
		logger.Infof("🏦 [%s] Using LIGHTER trading", config.Name)

		if config.LighterWalletAddr == "" || config.LighterAPIKeyPrivateKey == "" {
			return nil, fmt.Errorf("Lighter requires wallet address and API Key private key")
		}

		// Lighter only supports mainnet (testnet disabled)
		trader, err = lighter.NewLighterTraderV2(
			config.LighterWalletAddr,
			config.LighterAPIKeyPrivateKey,
			config.LighterAPIKeyIndex,
			false, // Always use mainnet for Lighter
		)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize LIGHTER trader: %w", err)
		}
		logger.Infof("✓ LIGHTER trader initialized successfully")
	default:
		return nil, fmt.Errorf("unsupported trading platform: %s", config.Exchange)
	}
	}

	// Validate initial balance configuration, auto-fetch from exchange if 0 (skip for simulation)
	if !config.IsSimulation && config.InitialBalance <= 0 {
		logger.Infof("📊 [%s] Initial balance not set, attempting to fetch current balance from exchange...", config.Name)
		account, err := trader.GetBalance()
		if err != nil {
			return nil, fmt.Errorf("initial balance not set and unable to fetch balance from exchange: %w", err)
		}
		// Try multiple balance field names (different exchanges return different formats)
		balanceKeys := []string{"total_equity", "totalWalletBalance", "wallet_balance", "totalEq", "balance"}
		var foundBalance float64
		for _, key := range balanceKeys {
			if balance, ok := account[key].(float64); ok && balance > 0 {
				foundBalance = balance
				break
			}
		}
		if foundBalance > 0 {
			config.InitialBalance = foundBalance
			logger.Infof("✓ [%s] Auto-fetched initial balance: %.2f USDT", config.Name, foundBalance)
			// Save to database so it persists across restarts
			if st != nil {
				if err := st.Trader().UpdateInitialBalance(userID, config.ID, foundBalance); err != nil {
					logger.Infof("⚠️  [%s] Failed to save initial balance to database: %v", config.Name, err)
				} else {
					logger.Infof("✓ [%s] Initial balance saved to database", config.Name)
				}
			}
		} else {
			return nil, fmt.Errorf("initial balance must be greater than 0, please set InitialBalance in config or ensure exchange account has balance")
		}
	}

	// Get last cycle number (for recovery)
	var cycleNumber int
	if st != nil {
		cycleNumber, _ = st.Decision().GetLastCycleNumber(config.ID)
		logger.Infof("📊 [%s] Decision records will be stored to database", config.Name)
	}

	// Create strategy engine (must have strategy config)
	if config.StrategyConfig == nil {
		return nil, fmt.Errorf("[%s] strategy not configured", config.Name)
	}
	strategyEngine := kernel.NewStrategyEngine(config.StrategyConfig)
	logger.Infof("✓ [%s] Using strategy engine (strategy configuration loaded)", config.Name)

	// Create market data client for K-line data
	marketClient := market.NewAPIClient()

	return &AutoTrader{
		id:                    config.ID,
		name:                  config.Name,
		aiModel:               config.AIModel,
		exchange:              config.Exchange,
		exchangeID:            config.ExchangeID,
		showInCompetition:     config.ShowInCompetition,
		config:                config,
		trader:                trader,
		mcpClient:             mcpClient,
		marketClient:          marketClient,
		store:                 st,
		strategyEngine:        strategyEngine,
		cycleNumber:           cycleNumber,
		initialBalance:        config.InitialBalance,
		lastResetTime:         time.Now(),
		startTime:             time.Now(),
		callCount:             0,
		isRunning:             false,
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		monitorWg:             sync.WaitGroup{},
		peakPnLCache:          make(map[string]float64),
		peakPnLCacheMutex:     sync.RWMutex{},
		slConfirmCount:        make(map[string]int),
		aiCloseConfirmCount:   make(map[string]int),
		scaledLevelsTaken:     make(map[string][]float64),
		scaledLevelsTakenMu:   sync.RWMutex{},
		positionParamsMap:     make(map[string]*positionParams),
		lastBalanceSyncTime:   time.Now(),
		userID:                userID,
	}, nil
}

// Run runs the automatic trading main loop
func (at *AutoTrader) Run() error {
	at.isRunningMutex.Lock()
	at.isRunning = true
	at.isRunningMutex.Unlock()

	at.stopMonitorCh = make(chan struct{})
	at.startTime = time.Now()

	// 强制决策间隔至少 3 分钟，避免 DB/配置为 0 或过小导致实际约 1 分钟跑一次
	if at.config.ScanInterval < 3*time.Minute {
		logger.Infof("⚠️ Scan interval %v < 3m, clamping to 3m (frontend setting may not have been applied)", at.config.ScanInterval)
		at.config.ScanInterval = 3 * time.Minute
	}
	// AI 决策周期仅按 ScanInterval 时间间隔触发，与 K 线周期、行情刷新无关
	logger.Info("🚀 AI-driven automatic trading system started")
	logger.Infof("💰 Initial balance: %.2f USDT", at.initialBalance)
	logger.Infof("⚙️  Scan interval: %v (AI cycle runs every this duration only; not tied to K-line)", at.config.ScanInterval)
	logger.Info("🤖 AI will make full decisions on leverage, position size, stop loss/take profit, etc.")
	at.monitorWg.Add(1)
	defer at.monitorWg.Done()

	// 实盘模拟：与实盘一致，从 DB 恢复未平仓位和已实现盈亏（进程重启后持仓和余额不丢失）
	if at.config.IsSimulation && at.store != nil {
		if pt, ok := at.trader.(*paper.PaperTrader); ok {
			// Restore open positions
			openPositions, err := at.store.Position().GetOpenPositions(at.id)
			if err == nil && len(openPositions) > 0 {
				for _, pos := range openPositions {
					pt.RestoreOpenPosition(pos.Symbol, pos.Side, pos.Quantity, pos.EntryPrice, pos.Leverage, pos.EntryTime)
				}
				logger.Infof("📊 [%s] Restored %d paper positions from store (same logic as live)", at.name, len(openPositions))
			}
			// Restore realized PnL from closed positions
			stats, err := at.store.Position().GetFullStats(at.id)
			if err != nil {
				logger.Infof("⚠️ [%s] Failed to get stats for PnL restore: %v", at.name, err)
			} else if stats == nil {
				logger.Infof("📊 [%s] No closed positions yet, balance starts at %.2f", at.name, at.initialBalance)
			} else if stats.TotalPnL == 0 {
				logger.Infof("📊 [%s] Closed positions exist (%d) but total PnL is 0", at.name, stats.TotalTrades)
			} else {
				pt.RestoreRealizedPnL(stats.TotalPnL)
				logger.Infof("📊 [%s] Restored realized PnL: %.4f USDT (from %d closed trades)", at.name, stats.TotalPnL, stats.TotalTrades)
			}
		}
	}

	// Start drawdown monitoring
	at.startDrawdownMonitor()

	// Start Lighter order sync if using Lighter exchange
	if at.exchange == "lighter" {
		if lighterTrader, ok := at.trader.(*lighter.LighterTraderV2); ok && at.store != nil {
			lighterTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] Lighter order+position sync enabled (every 30s)", at.name)
		}
	}

	// Start Hyperliquid order sync if using Hyperliquid exchange
	if at.exchange == "hyperliquid" {
		if hyperliquidTrader, ok := at.trader.(*hyperliquid.HyperliquidTrader); ok && at.store != nil {
			hyperliquidTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] Hyperliquid order+position sync enabled (every 30s)", at.name)
		}
	}

	// Start Bybit order sync if using Bybit exchange
	if at.exchange == "bybit" {
		if bybitTrader, ok := at.trader.(*bybit.BybitTrader); ok && at.store != nil {
			bybitTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] Bybit order+position sync enabled (every 30s)", at.name)
		}
	}

	// Start OKX order sync if using OKX exchange
	if at.exchange == "okx" {
		if okxTrader, ok := at.trader.(*okx.OKXTrader); ok && at.store != nil {
			okxTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] OKX order+position sync enabled (every 30s)", at.name)
		}
	}

	// Start Bitget order sync if using Bitget exchange
	if at.exchange == "bitget" {
		if bitgetTrader, ok := at.trader.(*bitget.BitgetTrader); ok && at.store != nil {
			bitgetTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] Bitget order+position sync enabled (every 30s)", at.name)
		}
	}

	// Start Aster order sync if using Aster exchange
	if at.exchange == "aster" {
		if asterTrader, ok := at.trader.(*aster.AsterTrader); ok && at.store != nil {
			asterTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] Aster order+position sync enabled (every 30s)", at.name)
		}
	}

	// Start Binance order sync if using Binance exchange
	if at.exchange == "binance" {
		if binanceTrader, ok := at.trader.(*binance.FuturesTrader); ok && at.store != nil {
			binanceTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] Binance order+position sync enabled (every 30s)", at.name)
		}
	}

	// Start Gate order sync if using Gate exchange
	if at.exchange == "gate" {
		if gateTrader, ok := at.trader.(*gate.GateTrader); ok && at.store != nil {
			gateTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] Gate order+position sync enabled (every 30s)", at.name)
		}
	}

	// Start KuCoin order sync if using KuCoin exchange
	if at.exchange == "kucoin" {
		if kucoinTrader, ok := at.trader.(*kucoin.KuCoinTrader); ok && at.store != nil {
			kucoinTrader.StartOrderSync(at.id, at.exchangeID, at.exchange, at.store, 30*time.Second)
			logger.Infof("🔄 [%s] KuCoin order+position sync enabled (every 30s)", at.name)
		}
	}

	// Check if this is a grid trading strategy
	isGridStrategy := at.IsGridStrategy()
	if isGridStrategy {
		logger.Infof("🔲 [%s] Grid trading strategy detected, initializing grid...", at.name)
		if err := at.InitializeGrid(); err != nil {
			logger.Errorf("❌ [%s] Failed to initialize grid: %v", at.name, err)
			return fmt.Errorf("grid initialization failed: %w", err)
		}
	}

	// 策略止盈/止损独立于 AI 周期：每 1 分钟检查一次，触发即平仓，不等到下一轮 AI 分析
	if !isGridStrategy {
		at.startDynamicSLTPMonitor()
	}

	// 固定「周期开始」间隔：下一周期开始时间 = 上一周期开始 + ScanInterval（有持仓时间隔稳定）
	lastCycleStart := time.Now()

	// Execute immediately on first run（传入周期开始时间，决策记录用该时间戳，界面展示间隔即固定为 ScanInterval）
	if isGridStrategy {
		if err := at.RunGridCycle(); err != nil {
			logger.Infof("❌ Grid execution failed: %v", err)
		}
	} else {
		if err := at.runCycle(lastCycleStart); err != nil {
			logger.Infof("❌ Execution failed: %v", err)
		}
	}

	for {
		at.isRunningMutex.RLock()
		running := at.isRunning
		at.isRunningMutex.RUnlock()

		if !running {
			break
		}

		// 下一周期仅由 lastCycleStart + ScanInterval 决定，与持仓、K 线无关
		interval := at.config.ScanInterval
		nextRunAt := lastCycleStart.Add(interval)
		now := time.Now()
		if now.Before(nextRunAt) {
			sleepDur := nextRunAt.Sub(now)
			logger.Infof("[%s] ⏳ Next AI cycle at %s (interval=%v, sleep=%.1fs)", at.name, nextRunAt.UTC().Format("15:04:05"), interval, sleepDur.Seconds())
			select {
			case <-time.After(sleepDur):
			case <-at.stopMonitorCh:
				logger.Infof("[%s] ⏹ Stop signal received, exiting automatic trading main loop", at.name)
				return nil
			}
			lastCycleStart = time.Now()
		} else {
			// 本周期执行超时（runCycle 耗时超过 ScanInterval），仍按预定间隔排下一周期，避免持仓期间连续紧挨执行
			lastCycleStart = nextRunAt
		}

		cycleStart := lastCycleStart
		if isGridStrategy {
			if err := at.RunGridCycle(); err != nil {
				logger.Infof("❌ Grid execution failed: %v", err)
			}
		} else {
			if err := at.runCycle(cycleStart); err != nil {
				logger.Infof("❌ Execution failed: %v", err)
			}
		}
	}

	return nil
}

// Stop stops the automatic trading
func (at *AutoTrader) Stop() {
	at.isRunningMutex.Lock()
	if !at.isRunning {
		at.isRunningMutex.Unlock()
		return
	}
	at.isRunning = false
	at.isRunningMutex.Unlock()

	close(at.stopMonitorCh) // Notify monitoring goroutine to stop
	at.monitorWg.Wait()     // Wait for monitoring goroutine to finish
	logger.Info("⏹ Automatic trading system stopped")
}

// runCycle runs one trading cycle (using AI full decision-making).
// cycleStart 为本周期调度开始时间，用于决策记录时间戳，使界面展示的周期间隔固定为 ScanInterval。
func (at *AutoTrader) runCycle(cycleStart time.Time) error {
	at.callCount++

	logger.Info("\n" + strings.Repeat("=", 70) + "\n")
	logger.Infof("⏰ %s - AI decision cycle #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	logger.Info(strings.Repeat("=", 70))

	// 0. Check if trader is stopped (early exit to prevent trades after Stop() is called)
	at.isRunningMutex.RLock()
	running := at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("⏹ Trader is stopped, aborting cycle #%d", at.callCount)
		return nil
	}

	// 排查「决策变频繁」：每轮开始时打出当前配置的间隔，便于确认是否按设定执行
	logger.Infof("[%s] 🔄 runCycle #%d start (cycleStart=%s, config_interval=%v)", at.name, at.callCount, cycleStart.UTC().Format("15:04:05"), at.config.ScanInterval)

	// Create decision record（时间戳用周期开始时间，保证列表展示间隔 = ScanInterval）
	record := &store.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
		Timestamp:    cycleStart.UTC(),
	}

	// 1. Check if trading needs to be stopped
	if time.Now().Before(at.stopUntil) {
		remaining := at.stopUntil.Sub(time.Now())
		logger.Infof("⏸ Risk control: Trading paused, remaining %.0f minutes", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Risk control paused, remaining %.0f minutes", remaining.Minutes())
		at.saveDecision(record)
		return nil
	}

	// 2. Reset daily P&L (reset every day)
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		logger.Info("📅 Daily P&L reset")
	}

	// 3. Check dynamic stop loss and take profit for existing positions
	if err := at.checkDynamicStopLossTakeProfit(); err != nil {
		logger.Infof("⚠️  Failed to check dynamic stop loss/take profit: %v", err)
	}

	// 4. Collect trading context
	ctx, err := at.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to build trading context: %v", err)
		at.saveDecision(record)
		return fmt.Errorf("failed to build trading context: %w", err)
	}

	// Save equity snapshot independently (decoupled from AI decision, used for drawing profit curve)
	// NOTE: Must be called BEFORE candidate coins check to ensure equity is always recorded
	at.saveEquitySnapshot(ctx)

	// 如果没有候选币种，记录但不报错
	if len(ctx.CandidateCoins) == 0 {
		logger.Infof("ℹ️  No candidate coins available, skipping this cycle")
		record.Success = true // 不是错误，只是没有候选币
		record.ExecutionLog = append(record.ExecutionLog, "No candidate coins available, cycle skipped")
		record.AccountState = store.AccountSnapshot{
			TotalBalance:          ctx.Account.TotalEquity,
			AvailableBalance:      ctx.Account.AvailableBalance,
			TotalUnrealizedProfit: ctx.Account.UnrealizedPnL,
			PositionCount:         ctx.Account.PositionCount,
			InitialBalance:        at.initialBalance,
		}
		at.saveDecision(record)
		return nil
	}

	logger.Info(strings.Repeat("=", 70))
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	logger.Infof("📊 Account equity: %.2f USDT | Available: %.2f USDT | Positions: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. Use strategy engine to call AI for decision
	logger.Infof("🤖 Requesting AI analysis and decision... [Strategy Engine]")
	aiDecision, err := kernel.GetFullDecisionWithStrategy(ctx, at.mcpClient, at.strategyEngine, "balanced", "")

	if aiDecision != nil && aiDecision.AIRequestDurationMs > 0 {
		record.AIRequestDurationMs = aiDecision.AIRequestDurationMs
		logger.Infof("⏱️ AI call duration: %.2f seconds", float64(record.AIRequestDurationMs)/1000)
		record.ExecutionLog = append(record.ExecutionLog,
			fmt.Sprintf("AI call duration: %d ms", record.AIRequestDurationMs))
	}

	// Save chain of thought, decisions, and input prompt even if there's an error (for debugging)
	if aiDecision != nil {
		record.SystemPrompt = aiDecision.SystemPrompt // Save system prompt
		record.InputPrompt = aiDecision.UserPrompt
		record.CoTTrace = aiDecision.CoTTrace
		record.RawResponse = aiDecision.RawResponse // Save raw AI response for debugging
		if len(aiDecision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(aiDecision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to get AI decision: %v", err)

		// Print system prompt and AI chain of thought (output even with errors for debugging)
		if aiDecision != nil {
			logger.Info("\n" + strings.Repeat("=", 70) + "\n")
			logger.Infof("📋 System prompt (error case)")
			logger.Info(strings.Repeat("=", 70))
			logger.Info(aiDecision.SystemPrompt)
			logger.Info(strings.Repeat("=", 70))

			if aiDecision.CoTTrace != "" {
				logger.Info("\n" + strings.Repeat("-", 70) + "\n")
				logger.Info("💭 AI chain of thought analysis (error case):")
				logger.Info(strings.Repeat("-", 70))
				logger.Info(aiDecision.CoTTrace)
				logger.Info(strings.Repeat("-", 70))
			}
		}

		at.saveDecision(record)
		return fmt.Errorf("failed to get AI decision: %w", err)
	}

	// // 5. Print system prompt
	// logger.Infof("\n" + strings.Repeat("=", 70))
	// logger.Infof("📋 System prompt [template: %s]", at.systemPromptTemplate)
	// logger.Info(strings.Repeat("=", 70))
	// logger.Info(decision.SystemPrompt)
	// logger.Infof(strings.Repeat("=", 70) + "\n")

	// 6. Print AI chain of thought
	// logger.Infof("\n" + strings.Repeat("-", 70))
	// logger.Info("💭 AI chain of thought analysis:")
	// logger.Info(strings.Repeat("-", 70))
	// logger.Info(decision.CoTTrace)
	// logger.Infof(strings.Repeat("-", 70) + "\n")

	// 7. Print AI decisions
	// logger.Infof("📋 AI decision list (%d items):\n", len(kernel.Decisions))
	// for i, d := range kernel.Decisions {
	//     logger.Infof("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	//     if d.Action == "open_long" || d.Action == "open_short" {
	//        logger.Infof("      Leverage: %dx | Position: %.2f USDT | Stop loss: %.4f | Take profit: %.4f",
	//           d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
	//     }
	// }
	logger.Info()
	logger.Info(strings.Repeat("-", 70))
	// 8. Sort decisions: ensure close positions first, then open positions (prevent position stacking overflow)
	logger.Info(strings.Repeat("-", 70))

	// 8. Sort decisions: ensure close positions first, then open positions (prevent position stacking overflow)
	sortedDecisions := sortDecisionsByPriority(aiDecision.Decisions)

	logger.Info("🔄 Execution order (optimized): Close positions first → Open positions later")
	for i, d := range sortedDecisions {
		logger.Infof("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	logger.Info()

	// Check if trader is stopped before executing any decisions (prevent trades after Stop())
	at.isRunningMutex.RLock()
	running = at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		logger.Infof("⏹ Trader stopped before decision execution, aborting cycle #%d", at.callCount)
		return nil
	}

	// Execute decisions and record results
	newAiCloseCounts := make(map[string]int) // this cycle's AI close confirm counts (for consecutive-cycle logic)
	for _, d := range sortedDecisions {
		// Check if trader is stopped before each decision (allow immediate stop during execution)
		at.isRunningMutex.RLock()
		running = at.isRunning
		at.isRunningMutex.RUnlock()
		if !running {
			logger.Infof("⏹ Trader stopped during decision execution, aborting remaining decisions")
			break
		}

		actionRecord := store.DecisionAction{
			Action:     d.Action,
			Symbol:     d.Symbol,
			Quantity:   0,
			Leverage:   d.Leverage,
			Price:      0,
			StopLoss:   d.StopLoss,
			TakeProfit: d.TakeProfit,
			Confidence: d.Confidence,
			Reasoning:  d.Reasoning,
			Timestamp:  time.Now().UTC(),
			Success:    false,
		}

		// AI 仅开仓模式：不执行 AI 的平仓建议，由策略动态 SL/TP 负责平仓
		aiOnlyEntry := false
		if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
			aiOnlyEntry = at.strategyEngine.GetConfig().RiskControl.AIOnlyEntry
		}
		if aiOnlyEntry && (d.Action == "close_long" || d.Action == "close_short") {
			msg := fmt.Sprintf("AI close %s %s skipped (ai_only_entry=true, strategy handles exit)", d.Symbol, d.Action)
			logger.Infof("⏭ %s", msg)
			record.ExecutionLog = append(record.ExecutionLog, msg)
			record.Decisions = append(record.Decisions, actionRecord)
			continue
		}

		// AI 平仓延迟确认：close_long/close_short 需要连续 2 个周期都建议平仓才真正执行
		shouldExecute := true
		confirmCount := 0
		if d.Action == "close_long" || d.Action == "close_short" {
			side := "long"
			if d.Action == "close_short" {
				side = "short"
			}
			posKey := d.Symbol + "_" + side

			at.aiCloseConfirmMu.Lock()
			prev := at.aiCloseConfirmCount[posKey]
			confirmCount = prev + 1
			newAiCloseCounts[posKey] = confirmCount
			at.aiCloseConfirmMu.Unlock()

			if confirmCount < 2 {
				shouldExecute = false
			} else {
				// 已达到确认次数，本次执行后需要重新累计
				newAiCloseCounts[posKey] = 0
			}
		}

		if !shouldExecute {
			msg := fmt.Sprintf("AI close %s %s requested (confirm %d/2) → delayed by rule", d.Symbol, d.Action, confirmCount)
			logger.Infof("⏳ %s", msg)
			record.ExecutionLog = append(record.ExecutionLog, msg)
			record.Decisions = append(record.Decisions, actionRecord)
			continue
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			logger.Infof("❌ Failed to execute decision (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s failed: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s succeeded", d.Symbol, d.Action))
			// Brief delay after successful execution
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 更新 AI 平仓确认计数，仅保留连续周期仍在请求平仓的仓位
	at.aiCloseConfirmMu.Lock()
	at.aiCloseConfirmCount = newAiCloseCounts
	at.aiCloseConfirmMu.Unlock()

	// 9. Save decision record
	if err := at.saveDecision(record); err != nil {
		logger.Infof("⚠ Failed to save decision record: %v", err)
	}

	return nil
}

// getAvailableAndEquityFromBalance returns available balance and total equity from balance map (paper or exchange format).
func getAvailableAndEquityFromBalance(balance map[string]interface{}) (availableBalance, totalEquity float64) {
	if avail, ok := balance["available"].(float64); ok {
		availableBalance = avail
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}
	if eq, ok := balance["total_equity"].(float64); ok && eq > 0 {
		totalEquity = eq
		return availableBalance, totalEquity
	}
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		totalEquity = eq
		return availableBalance, totalEquity
	}
	if eq, ok := balance["totalWalletBalance"].(float64); ok && eq > 0 {
		totalEquity = eq
		return availableBalance, totalEquity
	}
	totalEquity = availableBalance
	return availableBalance, totalEquity
}

// getPosStr gets a string from position map, trying primary then fallback key (for paper vs exchange format).
func getPosStr(pos map[string]interface{}, primary, fallback string) string {
	if v, ok := pos[primary].(string); ok && v != "" {
		return v
	}
	if fallback != "" {
		if v, ok := pos[fallback].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// recordStrategyTriggeredClose 记录由策略触发的平仓，供下一轮 AI 分析链展示（含 30s 后台检查触发的平仓）
func (at *AutoTrader) recordStrategyTriggeredClose(c kernel.StrategyTriggeredClose) {
	at.strategyTriggeredClosesMu.Lock()
	defer at.strategyTriggeredClosesMu.Unlock()
	at.strategyTriggeredCloses = append(at.strategyTriggeredCloses, c)
}

// setPendingCloseReason 设置本次平仓原因，供 recordPositionChange 写入 DB（仅对 close_long/close_short 有效）
func (at *AutoTrader) setPendingCloseReason(reason string) {
	at.pendingCloseReasonMu.Lock()
	defer at.pendingCloseReasonMu.Unlock()
	at.pendingCloseReason = reason
}

// setPendingCloseReasonIfEmpty 仅当当前未设置平仓原因时写入（避免 AI 路径覆盖策略已设置的 system:sl/tp）
func (at *AutoTrader) setPendingCloseReasonIfEmpty(reason string) {
	at.pendingCloseReasonMu.Lock()
	defer at.pendingCloseReasonMu.Unlock()
	if at.pendingCloseReason == "" {
		at.pendingCloseReason = reason
	}
}

// getAndClearPendingCloseReason 取出并清空待写入的平仓原因
func (at *AutoTrader) getAndClearPendingCloseReason() string {
	at.pendingCloseReasonMu.Lock()
	defer at.pendingCloseReasonMu.Unlock()
	s := at.pendingCloseReason
	at.pendingCloseReason = ""
	return s
}

// getPendingCloseReason 获取当前待写入的平仓原因（不清空）
func (at *AutoTrader) getPendingCloseReason() string {
	at.pendingCloseReasonMu.Lock()
	defer at.pendingCloseReasonMu.Unlock()
	return at.pendingCloseReason
}

// getAndClearStrategyTriggeredCloses 取出并清空“本周期/近期由策略触发的平仓”列表，用于填入 AI 上下文
func (at *AutoTrader) getAndClearStrategyTriggeredCloses() []kernel.StrategyTriggeredClose {
	at.strategyTriggeredClosesMu.Lock()
	defer at.strategyTriggeredClosesMu.Unlock()
	out := at.strategyTriggeredCloses
	at.strategyTriggeredCloses = nil
	return out
}

// buildTradingContext builds trading context
func (at *AutoTrader) buildTradingContext() (*kernel.Context, error) {
	// 1. Get account information
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get account balance: %w", err)
	}

	// Get account fields (support both exchange format and paper trader format)
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0
	totalEquity := 0.0

	if eq, ok := balance["total_equity"].(float64); ok {
		totalEquity = eq
	}
	if bal, ok := balance["balance"].(float64); ok {
		totalWalletBalance = bal
	}
	if avail, ok := balance["available"].(float64); ok {
		availableBalance = avail
	}
	if totalEquity > 0 && totalWalletBalance >= 0 {
		totalUnrealizedProfit = totalEquity - totalWalletBalance
	}
	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		totalEquity = eq
	}
	if totalEquity <= 0 {
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}

	// 2. Get position information
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positionInfos []kernel.PositionInfo
	totalMarginUsed := 0.0

	// Current position key set (for cleaning up closed position records)
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := getPosStr(pos, "symbol", "")
		if symbol == "" {
			continue
		}
		side := getPosStr(pos, "side", "position_side")
		entryPrice := getPosFloat(pos, "entryPrice", "entry_price")
		markPrice := getPosFloat(pos, "markPrice", "mark_price")
		quantity := getPosFloat(pos, "positionAmt", "position_amt")
		if quantity < 0 {
			quantity = -quantity
		}
		if quantity == 0 {
			continue
		}

		unrealizedPnl := getPosFloat(pos, "unRealizedProfit", "unrealized_pnl")
		liquidationPrice := getPosFloat(pos, "liquidationPrice", "liquidation_price")

		leverage := 10
		if lev := getPosFloat(pos, "leverage", ""); lev > 0 {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// Calculate P&L percentage (based on margin, considering leverage)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// Get position open time from exchange (preferred) or fallback to local tracking
		// posKey must match setPositionParams/GetPositions (symbol + "_" + lowercase side) so cleanup doesn't delete params
		posKey := symbol + "_" + strings.ToLower(side)
		currentPositionKeys[posKey] = true

		var updateTime int64
		// Priority 1: Get from database (trader_positions table) - most accurate
		if at.store != nil {
			if dbPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, side); err == nil && dbPos != nil {
				if dbPos.EntryTime > 0 {
					updateTime = dbPos.EntryTime
				}
			}
		}
		// Priority 2: Get from exchange API (Bybit: createdTime, OKX: createdTime)
		if updateTime == 0 {
			if createdTime, ok := pos["createdTime"].(int64); ok && createdTime > 0 {
				updateTime = createdTime
			}
		}
		// Priority 3: Fallback to local tracking
		if updateTime == 0 {
			if _, exists := at.positionFirstSeenTime[posKey]; !exists {
				at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
			}
			updateTime = at.positionFirstSeenTime[posKey]
		}

		// Get peak profit rate for this position
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		positionInfos = append(positionInfos, kernel.PositionInfo{
			Symbol:           symbol,
			Side:             side,
			EntryPrice:       entryPrice,
			MarkPrice:        markPrice,
			Quantity:         quantity,
			Leverage:         leverage,
			UnrealizedPnL:    unrealizedPnl,
			UnrealizedPnLPct: pnlPct,
			PeakPnLPct:       peakPnlPct,
			LiquidationPrice: liquidationPrice,
			MarginUsed:       marginUsed,
			UpdateTime:       updateTime,
		})
	}

	// Clean up closed position records
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
		}
	}
	at.positionParamsMu.Lock()
	for key := range at.positionParamsMap {
		if !currentPositionKeys[key] {
			delete(at.positionParamsMap, key)
		}
	}
	at.positionParamsMu.Unlock()

	// 3. Use strategy engine to get candidate coins (must have strategy engine)
	var candidateCoins []kernel.CandidateCoin
	if at.strategyEngine == nil {
		logger.Infof("⚠️ [%s] No strategy engine configured, skipping candidate coins", at.name)
	} else {
		coins, err := at.strategyEngine.GetCandidateCoins()
		if err != nil {
			// Log warning but don't fail - equity snapshot should still be saved
			logger.Infof("⚠️ [%s] Failed to get candidate coins: %v (will use empty list)", at.name, err)
		} else {
			candidateCoins = coins
			logger.Infof("📋 [%s] Strategy engine fetched candidate coins: %d", at.name, len(candidateCoins))
		}
	}

	// 4. Calculate total P&L
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. Get leverage from strategy config
	strategyConfig := at.strategyEngine.GetConfig()
	btcEthLeverage := strategyConfig.RiskControl.BTCETHMaxLeverage
	altcoinLeverage := strategyConfig.RiskControl.AltcoinMaxLeverage
	logger.Infof("📋 [%s] Strategy leverage config: BTC/ETH=%dx, Altcoin=%dx", at.name, btcEthLeverage, altcoinLeverage)

	// 6. Build context
	ctx := &kernel.Context{
		CurrentTime:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		RuntimeMinutes:  int(time.Since(at.startTime).Minutes()),
		CallCount:       at.callCount,
		BTCETHLeverage:  btcEthLeverage,
		AltcoinLeverage: altcoinLeverage,
		Account: kernel.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			UnrealizedPnL:    totalUnrealizedProfit,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:      positionInfos,
		CandidateCoins: candidateCoins,
	}

	// 7. Add recent closed trades (if store is available)
	if at.store != nil {
		// Get recent closed trades for AI context (cap at 5 to keep input token closer to backtest; backtest does not send RecentOrders)
		recentTradesLimit := 5
		recentTrades, err := at.store.Position().GetRecentTrades(at.id, recentTradesLimit)
		if err != nil {
			logger.Infof("⚠️ [%s] Failed to get recent trades: %v", at.name, err)
		} else {
			logger.Infof("📊 [%s] Found %d recent closed trades for AI context", at.name, len(recentTrades))
			for _, trade := range recentTrades {
				// Convert Unix timestamps to formatted strings for AI readability
				entryTimeStr := ""
				if trade.EntryTime > 0 {
					entryTimeStr = time.Unix(trade.EntryTime, 0).UTC().Format("01-02 15:04 UTC")
				}
				exitTimeStr := ""
				if trade.ExitTime > 0 {
					exitTimeStr = time.Unix(trade.ExitTime, 0).UTC().Format("01-02 15:04 UTC")
				}

				ctx.RecentOrders = append(ctx.RecentOrders, kernel.RecentOrder{
					Symbol:       trade.Symbol,
					Side:         trade.Side,
					EntryPrice:   trade.EntryPrice,
					ExitPrice:    trade.ExitPrice,
					RealizedPnL:  trade.RealizedPnL,
					PnLPct:       trade.PnLPct,
					EntryTime:    entryTimeStr,
					ExitTime:     exitTimeStr,
					HoldDuration: trade.HoldDuration,
				})
			}
		}
		// Get trading statistics for AI context
		stats, err := at.store.Position().GetFullStats(at.id)
		if err != nil {
			logger.Infof("⚠️ [%s] Failed to get trading stats: %v", at.name, err)
		} else if stats == nil {
			logger.Infof("⚠️ [%s] GetFullStats returned nil", at.name)
		} else if stats.TotalTrades == 0 {
			logger.Infof("⚠️ [%s] GetFullStats returned 0 trades (traderID=%s)", at.name, at.id)
		} else {
			ctx.TradingStats = &kernel.TradingStats{
				TotalTrades:    stats.TotalTrades,
				WinRate:        stats.WinRate,
				ProfitFactor:   stats.ProfitFactor,
				SharpeRatio:    stats.SharpeRatio,
				TotalPnL:       stats.TotalPnL,
				AvgWin:         stats.AvgWin,
				AvgLoss:        stats.AvgLoss,
				MaxDrawdownPct: stats.MaxDrawdownPct,
			}
			logger.Infof("📈 [%s] Trading stats: %d trades, %.1f%% win rate, PF=%.2f, Sharpe=%.2f, DD=%.1f%%",
				at.name, stats.TotalTrades, stats.WinRate, stats.ProfitFactor, stats.SharpeRatio, stats.MaxDrawdownPct)
		}
	} else {
		logger.Infof("⚠️ [%s] Store is nil, cannot get recent trades", at.name)
	}

	// 8. Get quantitative data (if enabled in strategy config)
	if strategyConfig.Indicators.EnableQuantData {
		// Collect symbols to query (candidate coins + position coins)
		symbolsToQuery := make(map[string]bool)
		for _, coin := range candidateCoins {
			symbolsToQuery[coin.Symbol] = true
		}
		for _, pos := range positionInfos {
			symbolsToQuery[pos.Symbol] = true
		}

		symbols := make([]string, 0, len(symbolsToQuery))
		for sym := range symbolsToQuery {
			symbols = append(symbols, sym)
		}

		logger.Infof("📊 [%s] Fetching quantitative data for %d symbols...", at.name, len(symbols))
		ctx.QuantDataMap = at.strategyEngine.FetchQuantDataBatch(symbols)
		logger.Infof("📊 [%s] Successfully fetched quantitative data for %d symbols", at.name, len(ctx.QuantDataMap))
	}

	// 9. Get OI ranking data (market-wide position changes)
	if strategyConfig.Indicators.EnableOIRanking {
		logger.Infof("📊 [%s] Fetching OI ranking data...", at.name)
		ctx.OIRankingData = at.strategyEngine.FetchOIRankingData()
		if ctx.OIRankingData != nil {
			logger.Infof("📊 [%s] OI ranking data ready: %d top, %d low positions",
				at.name, len(ctx.OIRankingData.TopPositions), len(ctx.OIRankingData.LowPositions))
		}
	}

	// 10. Get NetFlow ranking data (market-wide fund flow)
	if strategyConfig.Indicators.EnableNetFlowRanking {
		logger.Infof("💰 [%s] Fetching NetFlow ranking data...", at.name)
		ctx.NetFlowRankingData = at.strategyEngine.FetchNetFlowRankingData()
		if ctx.NetFlowRankingData != nil {
			logger.Infof("💰 [%s] NetFlow ranking data ready: inst_in=%d, inst_out=%d",
				at.name, len(ctx.NetFlowRankingData.InstitutionFutureTop), len(ctx.NetFlowRankingData.InstitutionFutureLow))
		}
	}

	// 11. Get Price ranking data (market-wide gainers/losers)
	if strategyConfig.Indicators.EnablePriceRanking {
		logger.Infof("📈 [%s] Fetching Price ranking data...", at.name)
		ctx.PriceRankingData = at.strategyEngine.FetchPriceRankingData()
		if ctx.PriceRankingData != nil {
			logger.Infof("📈 [%s] Price ranking data ready for %d durations",
				at.name, len(ctx.PriceRankingData.Durations))
		}
	}

	// 本周期或 30s 后台触发的策略平仓，传给 AI 以便分析链中有平仓记录
	ctx.StrategyTriggeredCloses = at.getAndClearStrategyTriggeredCloses()
	if len(ctx.StrategyTriggeredCloses) > 0 {
		logger.Infof("📤 [%s] Passing %d strategy-triggered close(s) to AI context", at.name, len(ctx.StrategyTriggeredCloses))
	}

	return ctx, nil
}

// executeDecisionWithRecord executes AI decision and records detailed information
func (at *AutoTrader) executeDecisionWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "hold", "wait":
		// No execution needed, just record
		return nil
	default:
		return fmt.Errorf("unknown action: %s", decision.Action)
	}
}

// ExecuteDecision executes a trading decision from external sources (e.g., debate consensus)
// This is a public method that can be called by other modules
func (at *AutoTrader) ExecuteDecision(d *kernel.Decision) error {
	logger.Infof("[%s] Executing external decision: %s %s", at.name, d.Action, d.Symbol)

	// Create a minimal action record for tracking
	actionRecord := &store.DecisionAction{
		Symbol:     d.Symbol,
		Action:     d.Action,
		Leverage:   d.Leverage,
		StopLoss:   d.StopLoss,
		TakeProfit: d.TakeProfit,
		Confidence: d.Confidence,
		Reasoning:  d.Reasoning,
	}

	// Execute the decision
	err := at.executeDecisionWithRecord(d, actionRecord)
	if err != nil {
		logger.Errorf("[%s] External decision execution failed: %v", at.name, err)
		return err
	}

	logger.Infof("[%s] External decision executed successfully: %s %s", at.name, d.Action, d.Symbol)
	return nil
}

// getEffectiveLeverage returns leverage to use for open: decision.Leverage if > 0, else strategy default (avoid 0 → display 10x bug).
func (at *AutoTrader) getEffectiveLeverage(decision *kernel.Decision) int {
	if decision.Leverage > 0 {
		logger.Infof("  📊 Using AI decision leverage: %dx for %s", decision.Leverage, decision.Symbol)
		return decision.Leverage
	}
	// AI didn't specify leverage, use strategy config
	defaultLev := 2
	if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
		rc := at.strategyEngine.GetConfig().RiskControl
		sym := market.Normalize(decision.Symbol)
		if sym == "BTCUSDT" || sym == "ETHUSDT" {
			if rc.BTCETHMaxLeverage > 0 {
				defaultLev = rc.BTCETHMaxLeverage
			}
		} else {
			if rc.AltcoinMaxLeverage > 0 {
				defaultLev = rc.AltcoinMaxLeverage
			}
		}
	}
	logger.Infof("  ⚠️ AI leverage was %d (not set), using strategy default %dx for %s", decision.Leverage, defaultLev, decision.Symbol)
	return defaultLev
}

// executeOpenLongWithRecord executes open long position and records detailed information
func (at *AutoTrader) executeOpenLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  📈 Open long: %s", decision.Symbol)

	leverage := at.getEffectiveLeverage(decision)
	if decision.Leverage <= 0 {
		decision.Leverage = leverage
	}

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions)); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		sym := getPosStr(pos, "symbol", "")
		side := getPosStr(pos, "side", "position_side")
		if sym == decision.Symbol && (side == "long" || side == "LONG") {
			return fmt.Errorf("❌ %s already has long position, close it first", decision.Symbol)
		}
	}

	// Get current price
	marketData, err := market.GetWithExchange(decision.Symbol, at.exchange)
	if err != nil {
		return err
	}

	// Get balance (support paper and exchange format)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance, equity := getAvailableAndEquityFromBalance(balance)

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// ⚠️ Auto-adjust position size if insufficient margin
	// Formula: totalRequired = positionSize/leverage + positionSize*0.001 + positionSize/leverage*0.01
	//        = positionSize * (1.01/leverage + 0.001)
	marginFactor := 1.01/float64(leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		// Use 98% of max to leave buffer for price fluctuation
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
		// Continue execution, doesn't affect trading
	}

	// Open position (use effective leverage so display matches AI/strategy)
	order, err := at.trader.OpenLong(decision.Symbol, quantity, leverage)
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f, leverage: %dx", order["orderId"], quantity, leverage)

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "open_long", quantity, marketData.CurrentPrice, leverage, 0)

	// Record position opening time
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// Adjust SL/TP based on actual entry price (preserve risk/reward ratio)
	actualEntryPrice := marketData.CurrentPrice
	aiAnalysisPrice := decision.Price // Price at time of AI analysis
	adjustedSL, adjustedTP, priceDeviation := adjustStopLossTakeProfitForActualEntry(
		aiAnalysisPrice, actualEntryPrice, decision.StopLoss, decision.TakeProfit, "long",
	)

	// Log adjustment if significant deviation
	if math.Abs(priceDeviation) > 0.1 {
		logger.Infof("  📊 SL/TP adjusted for actual entry: AI price %.6f → actual %.6f (%.2f%% deviation)",
			aiAnalysisPrice, actualEntryPrice, priceDeviation)
		logger.Infof("     SL: %.6f → %.6f, TP: %.6f → %.6f",
			decision.StopLoss, adjustedSL, decision.TakeProfit, adjustedTP)
	}

	// Warn if price deviation is large (>2%)
	if math.Abs(priceDeviation) > 2.0 {
		logger.Infof("  ⚠️ Large price deviation (%.2f%%) - market moved significantly since AI analysis", priceDeviation)
	}

	// Set stop loss and take profit with adjusted values
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, adjustedSL); err != nil {
		logger.Infof("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, adjustedTP); err != nil {
		logger.Infof("  ⚠ Failed to set take profit: %v", err)
	}

	// Store fixed params for API/UI (same as backtest current-position display)
	at.setPositionParams(posKey, adjustedSL, adjustedTP)

	// Persist params to DB open position so 交易记录 shows them when closed (by us or by sync)
	if at.store != nil {
		normalizedSymbol := market.Normalize(decision.Symbol)
		at.positionParamsMu.RLock()
		p := at.positionParamsMap[posKey]
		at.positionParamsMu.RUnlock()
		if p != nil {
			for attempt := 0; attempt < 5; attempt++ {
				openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "LONG")
				if err == nil && openPos != nil {
					_ = at.store.Position().UpdatePositionParams(openPos.ID, p.StopLoss, p.TakeProfit, p.ATRAtOpen, p.ATRMultipleSL, p.ATRMultipleTP, p.ATRPeriod)
					break
				}
				if !at.config.IsSimulation {
					time.Sleep(500 * time.Millisecond) // OrderSync may create position shortly
				} else {
					break
				}
			}
		}
	}

	return nil
}

// executeOpenShortWithRecord executes open short position and records detailed information
func (at *AutoTrader) executeOpenShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  📉 Open short: %s", decision.Symbol)

	leverage := at.getEffectiveLeverage(decision)
	if decision.Leverage <= 0 {
		decision.Leverage = leverage
	}

	// ⚠️ Get current positions for multiple checks
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// [CODE ENFORCED] Check max positions limit
	if err := at.enforceMaxPositions(len(positions)); err != nil {
		return err
	}

	// Check if there's already a position in the same symbol and direction
	for _, pos := range positions {
		sym := getPosStr(pos, "symbol", "")
		side := getPosStr(pos, "side", "position_side")
		if sym == decision.Symbol && (side == "short" || side == "SHORT") {
			return fmt.Errorf("❌ %s already has short position, close it first", decision.Symbol)
		}
	}

	// Get current price
	marketData, err := market.GetWithExchange(decision.Symbol, at.exchange)
	if err != nil {
		return err
	}

	// Get balance (support paper and exchange format)
	balance, err := at.trader.GetBalance()
	if err != nil {
		return fmt.Errorf("failed to get account balance: %w", err)
	}
	availableBalance, equity := getAvailableAndEquityFromBalance(balance)

	// [CODE ENFORCED] Position Value Ratio Check: position_value <= equity × ratio
	adjustedPositionSize, wasCapped := at.enforcePositionValueRatio(decision.PositionSizeUSD, equity, decision.Symbol)
	if wasCapped {
		decision.PositionSizeUSD = adjustedPositionSize
	}

	// ⚠️ Auto-adjust position size if insufficient margin
	marginFactor := 1.01/float64(leverage) + 0.001
	maxAffordablePositionSize := availableBalance / marginFactor

	actualPositionSize := decision.PositionSizeUSD
	if actualPositionSize > maxAffordablePositionSize {
		adjustedSize := maxAffordablePositionSize * 0.98
		logger.Infof("  ⚠️ Position size %.2f exceeds max affordable %.2f, auto-reducing to %.2f",
			actualPositionSize, maxAffordablePositionSize, adjustedSize)
		actualPositionSize = adjustedSize
		decision.PositionSizeUSD = actualPositionSize
	}

	// [CODE ENFORCED] Minimum position size check
	if err := at.enforceMinPositionSize(decision.PositionSizeUSD); err != nil {
		return err
	}

	// Calculate quantity with adjusted position size
	quantity := actualPositionSize / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
	}

	// Open position (use effective leverage so display matches AI/strategy)
	order, err := at.trader.OpenShort(decision.Symbol, quantity, leverage)
	if err != nil {
		return err
	}

	// Record order ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f, leverage: %dx", order["orderId"], quantity, leverage)

	// Record order to database and poll for confirmation
	at.recordAndConfirmOrder(order, decision.Symbol, "open_short", quantity, marketData.CurrentPrice, leverage, 0)

	// Record position opening time
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// Adjust SL/TP based on actual entry price (preserve risk/reward ratio)
	actualEntryPrice := marketData.CurrentPrice
	aiAnalysisPrice := decision.Price // Price at time of AI analysis
	adjustedSL, adjustedTP, priceDeviation := adjustStopLossTakeProfitForActualEntry(
		aiAnalysisPrice, actualEntryPrice, decision.StopLoss, decision.TakeProfit, "short",
	)

	// Log adjustment if significant deviation
	if math.Abs(priceDeviation) > 0.1 {
		logger.Infof("  📊 SL/TP adjusted for actual entry: AI price %.6f → actual %.6f (%.2f%% deviation)",
			aiAnalysisPrice, actualEntryPrice, priceDeviation)
		logger.Infof("     SL: %.6f → %.6f, TP: %.6f → %.6f",
			decision.StopLoss, adjustedSL, decision.TakeProfit, adjustedTP)
	}

	// Warn if price deviation is large (>2%)
	if math.Abs(priceDeviation) > 2.0 {
		logger.Infof("  ⚠️ Large price deviation (%.2f%%) - market moved significantly since AI analysis", priceDeviation)
	}

	// Set stop loss and take profit with adjusted values
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, adjustedSL); err != nil {
		logger.Infof("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, adjustedTP); err != nil {
		logger.Infof("  ⚠ Failed to set take profit: %v", err)
	}

	// Store fixed params for API/UI (same as backtest current-position display)
	at.setPositionParams(posKey, adjustedSL, adjustedTP)

	// Persist params to DB open position so 交易记录 shows them when closed (by us or by sync)
	if at.store != nil {
		normalizedSymbol := market.Normalize(decision.Symbol)
		at.positionParamsMu.RLock()
		p := at.positionParamsMap[posKey]
		at.positionParamsMu.RUnlock()
		if p != nil {
			for attempt := 0; attempt < 5; attempt++ {
				openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "SHORT")
				if err == nil && openPos != nil {
					_ = at.store.Position().UpdatePositionParams(openPos.ID, p.StopLoss, p.TakeProfit, p.ATRAtOpen, p.ATRMultipleSL, p.ATRMultipleTP, p.ATRPeriod)
					break
				}
				if !at.config.IsSimulation {
					time.Sleep(500 * time.Millisecond)
				} else {
					break
				}
			}
		}
	}

	return nil
}

// executeCloseLongWithRecord executes close long position and records detailed information
func (at *AutoTrader) executeCloseLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close long: %s", decision.Symbol)

	// Get current price
	marketData, err := market.GetWithExchange(decision.Symbol, at.exchange)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity - prioritize local database for accurate quantity
	var entryPrice float64
	var quantity float64
	var openPos *store.TraderPosition

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if pos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "LONG"); err == nil && pos != nil {
			openPos = pos
			quantity = pos.Quantity
			entryPrice = pos.EntryPrice
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Fallback to exchange API if local data not found
	if quantity == 0 {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				sym := getPosStr(pos, "symbol", "")
				side := strings.ToLower(getPosStr(pos, "side", "position_side"))
				if sym == decision.Symbol && side == "long" {
					entryPrice = getPosFloat(pos, "entryPrice", "entry_price")
					quantity = getPosFloat(pos, "positionAmt", "position_amt")
					if quantity < 0 {
						quantity = -quantity
					}
					break
				}
			}
		}
		logger.Infof("  📊 Using exchange position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
	}

	closeQty := 0.0
	if decision.CloseQuantity > 0 && decision.CloseQuantity < quantity {
		closeQty = decision.CloseQuantity
		logger.Infof("  📊 Partial close: %.8f of %.8f", closeQty, quantity)
	}

	// Pre-set closeReason on OPEN position for OrderSync (only if not already set by strategy)
	// This must happen BEFORE the order is submitted
	at.setPendingCloseReasonIfEmpty("ai")
	if at.store != nil && closeQty == 0 { // full close
		reason := at.getPendingCloseReason()
		if reason != "" {
			normalizedSymbol := market.Normalize(decision.Symbol)
			if err := at.store.Position().SetPendingCloseReasonBySymbol(at.id, normalizedSymbol, "LONG", reason); err != nil {
				logger.Infof("  ⚠️ Failed to pre-set close reason: %v", err)
			}
		}
	}

	order, err := at.trader.CloseLong(decision.Symbol, closeQty)
	if err != nil {
		return err
	}
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}
	closedAmt := quantity
	if closeQty > 0 {
		closedAmt = closeQty
	}
	posKey := decision.Symbol + "_long"
	var savedParams *positionParams
	at.positionParamsMu.RLock()
	savedParams = at.positionParamsMap[posKey]
	if savedParams != nil {
		savedParams = &positionParams{
			StopLoss: savedParams.StopLoss, TakeProfit: savedParams.TakeProfit,
			ATRAtOpen: savedParams.ATRAtOpen, ATRMultipleSL: savedParams.ATRMultipleSL,
			ATRMultipleTP: savedParams.ATRMultipleTP, ATRPeriod: savedParams.ATRPeriod,
		}
	}
	at.positionParamsMu.RUnlock()

	at.recordAndConfirmOrder(order, decision.Symbol, "close_long", closedAmt, marketData.CurrentPrice, 0, entryPrice)

	if openPos != nil && savedParams != nil && at.store != nil {
		_ = at.store.Position().UpdatePositionParams(openPos.ID,
			savedParams.StopLoss, savedParams.TakeProfit, savedParams.ATRAtOpen,
			savedParams.ATRMultipleSL, savedParams.ATRMultipleTP, savedParams.ATRPeriod)
	}
	if closeQty == 0 || closedAmt >= quantity {
		at.clearScaledLevelsTakenForPosition(posKey)
		at.ClearPeakPnLCache(decision.Symbol, "long")
		at.clearPositionParams(posKey)
	}
	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// executeCloseShortWithRecord executes close short position and records detailed information
func (at *AutoTrader) executeCloseShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction) error {
	logger.Infof("  🔄 Close short: %s", decision.Symbol)

	// Get current price
	marketData, err := market.GetWithExchange(decision.Symbol, at.exchange)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// Normalize symbol for database lookup
	normalizedSymbol := market.Normalize(decision.Symbol)

	// Get entry price and quantity - prioritize local database for accurate quantity
	var entryPrice float64
	var quantity float64
	var openPos *store.TraderPosition

	// First try to get from local database (more accurate for quantity)
	if at.store != nil {
		if pos, err := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, "SHORT"); err == nil && pos != nil {
			openPos = pos
			quantity = pos.Quantity
			entryPrice = pos.EntryPrice
			logger.Infof("  📊 Using local position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
		}
	}

	// Fallback to exchange API if local data not found
	if quantity == 0 {
		positions, err := at.trader.GetPositions()
		if err == nil {
			for _, pos := range positions {
				sym := getPosStr(pos, "symbol", "")
				side := strings.ToLower(getPosStr(pos, "side", "position_side"))
				if sym == decision.Symbol && side == "short" {
					entryPrice = getPosFloat(pos, "entryPrice", "entry_price")
					quantity = getPosFloat(pos, "positionAmt", "position_amt")
					break
				}
			}
		}
		logger.Infof("  📊 Using exchange position data: qty=%.8f, entry=%.2f", quantity, entryPrice)
	}
	if quantity < 0 {
		quantity = -quantity
	}

	closeQty := 0.0
	if decision.CloseQuantity > 0 && decision.CloseQuantity < quantity {
		closeQty = decision.CloseQuantity
		logger.Infof("  📊 Partial close: %.8f of %.8f", closeQty, quantity)
	}

	// Pre-set closeReason on OPEN position for OrderSync (only if not already set by strategy)
	// This must happen BEFORE the order is submitted
	at.setPendingCloseReasonIfEmpty("ai")
	if at.store != nil && closeQty == 0 { // full close
		reason := at.getPendingCloseReason()
		if reason != "" {
			normalizedSymbol := market.Normalize(decision.Symbol)
			if err := at.store.Position().SetPendingCloseReasonBySymbol(at.id, normalizedSymbol, "SHORT", reason); err != nil {
				logger.Infof("  ⚠️ Failed to pre-set close reason: %v", err)
			}
		}
	}

	order, err := at.trader.CloseShort(decision.Symbol, closeQty)
	if err != nil {
		return err
	}
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}
	closedAmt := quantity
	if closeQty > 0 {
		closedAmt = closeQty
	}
	posKey := decision.Symbol + "_short"
	var savedParams *positionParams
	at.positionParamsMu.RLock()
	savedParams = at.positionParamsMap[posKey]
	if savedParams != nil {
		savedParams = &positionParams{
			StopLoss: savedParams.StopLoss, TakeProfit: savedParams.TakeProfit,
			ATRAtOpen: savedParams.ATRAtOpen, ATRMultipleSL: savedParams.ATRMultipleSL,
			ATRMultipleTP: savedParams.ATRMultipleTP, ATRPeriod: savedParams.ATRPeriod,
		}
	}
	at.positionParamsMu.RUnlock()

	at.recordAndConfirmOrder(order, decision.Symbol, "close_short", closedAmt, marketData.CurrentPrice, 0, entryPrice)

	if openPos != nil && savedParams != nil && at.store != nil {
		_ = at.store.Position().UpdatePositionParams(openPos.ID,
			savedParams.StopLoss, savedParams.TakeProfit, savedParams.ATRAtOpen,
			savedParams.ATRMultipleSL, savedParams.ATRMultipleTP, savedParams.ATRPeriod)
	}
	if closeQty == 0 || closedAmt >= quantity {
		at.clearScaledLevelsTakenForPosition(posKey)
		at.ClearPeakPnLCache(decision.Symbol, "short")
		at.clearPositionParams(posKey)
	}
	logger.Infof("  ✓ Position closed successfully")
	return nil
}

// GetID gets trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetUnderlyingTrader returns the underlying Trader interface implementation
// This is used by grid trading and other components that need direct exchange access
func (at *AutoTrader) GetUnderlyingTrader() Trader {
	return at.trader
}

// GetName gets trader name
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel gets AI model
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetExchange gets exchange
func (at *AutoTrader) GetExchange() string {
	return at.exchange
}

// GetShowInCompetition returns whether trader should be shown in competition
func (at *AutoTrader) GetShowInCompetition() bool {
	return at.showInCompetition
}

// SetShowInCompetition sets whether trader should be shown in competition
func (at *AutoTrader) SetShowInCompetition(show bool) {
	at.showInCompetition = show
}

// SetCustomPrompt sets custom trading strategy prompt
func (at *AutoTrader) SetCustomPrompt(prompt string) {
	at.customPrompt = prompt
}

// SetOverrideBasePrompt sets whether to override base prompt
func (at *AutoTrader) SetOverrideBasePrompt(override bool) {
	at.overrideBasePrompt = override
}

// GetSystemPromptTemplate gets current system prompt template name (from strategy config)
func (at *AutoTrader) GetSystemPromptTemplate() string {
	if at.strategyEngine != nil {
		config := at.strategyEngine.GetConfig()
		if config.CustomPrompt != "" {
			return "custom"
		}
	}
	return "strategy"
}

// saveEquitySnapshot saves equity snapshot independently (for drawing profit curve, decoupled from AI decision)
func (at *AutoTrader) saveEquitySnapshot(ctx *kernel.Context) {
	if at.store == nil || ctx == nil {
		return
	}

	snapshot := &store.EquitySnapshot{
		TraderID:      at.id,
		Timestamp:     time.Now().UTC(),
		TotalEquity:   ctx.Account.TotalEquity,
		Balance:       ctx.Account.TotalEquity - ctx.Account.UnrealizedPnL,
		UnrealizedPnL: ctx.Account.UnrealizedPnL,
		PositionCount: ctx.Account.PositionCount,
		MarginUsedPct: ctx.Account.MarginUsedPct,
	}

	if err := at.store.Equity().Save(snapshot); err != nil {
		logger.Infof("⚠️ Failed to save equity snapshot: %v", err)
	}
}

// saveDecision saves AI decision log to database (only records AI input/output, for debugging)
func (at *AutoTrader) saveDecision(record *store.DecisionRecord) error {
	if at.store == nil {
		return nil
	}

	at.cycleNumber++
	record.CycleNumber = at.cycleNumber
	record.TraderID = at.id

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}

	if err := at.store.Decision().LogDecision(record); err != nil {
		logger.Infof("⚠️ Failed to save decision record: %v", err)
		return err
	}

	logger.Infof("📝 Decision record saved: trader=%s, cycle=%d", at.id, at.cycleNumber)
	return nil
}

// GetStore gets data store (for external access to decision records, etc.)
func (at *AutoTrader) GetStore() *store.Store {
	return at.store
}

// GetStatus gets system status (for API)
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	at.isRunningMutex.RLock()
	isRunning := at.isRunning
	at.isRunningMutex.RUnlock()

	result := map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      isRunning,
		"start_time":      at.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(at.startTime).Minutes()),
		"call_count":      at.callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"stop_until":      at.stopUntil.Format(time.RFC3339),
		"last_reset_time": at.lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
	}

	// Add strategy info
	if at.config.StrategyConfig != nil {
		result["strategy_type"] = at.config.StrategyConfig.StrategyType
		if at.config.StrategyConfig.GridConfig != nil {
			result["grid_symbol"] = at.config.StrategyConfig.GridConfig.Symbol
		}
	}

	return result
}

// getPosFloat gets a float64 from position map, trying primary then fallback key (for paper vs exchange format).
func getPosFloat(pos map[string]interface{}, primary, fallback string) float64 {
	// Try primary key
	if v := toFloat64(pos[primary]); v != 0 {
		return v
	}
	// Try fallback key
	if fallback != "" {
		if v := toFloat64(pos[fallback]); v != 0 {
			return v
		}
	}
	return 0
}

// getPosInt64 gets int64 from position map (e.g. update_time for entry time in ms). Tries keys in order.
func getPosInt64(pos map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		if v := pos[k]; v != nil {
			switch val := v.(type) {
			case int64:
				if val > 0 {
					return val
				}
			case int:
				if val > 0 {
					return int64(val)
				}
			case float64:
				if val > 0 {
					return int64(val)
				}
			}
		}
	}
	return 0
}

// toFloat64 converts various numeric types to float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		return 0
	}
}

// GetAccountInfo gets account information (for API)
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Get account fields (support both exchange format and paper trader format)
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0
	totalEquity := 0.0

	if eq, ok := balance["total_equity"].(float64); ok {
		totalEquity = eq
	}
	if bal, ok := balance["balance"].(float64); ok {
		totalWalletBalance = bal
	}
	if avail, ok := balance["available"].(float64); ok {
		availableBalance = avail
	}
	if totalEquity > 0 && totalWalletBalance >= 0 {
		totalUnrealizedProfit = totalEquity - totalWalletBalance
	}
	// Override with exchange-style keys if present
	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}
	if eq, ok := balance["totalEquity"].(float64); ok && eq > 0 {
		totalEquity = eq
	}
	if totalEquity <= 0 {
		totalEquity = totalWalletBalance + totalUnrealizedProfit
	}

	// Get positions to calculate total margin
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnLCalculated := 0.0
	for _, pos := range positions {
		markPrice := getPosFloat(pos, "markPrice", "mark_price")
		quantity := getPosFloat(pos, "positionAmt", "position_amt")
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := getPosFloat(pos, "unRealizedProfit", "unrealized_pnl")
		totalUnrealizedPnLCalculated += unrealizedPnl

		leverage := 10
		if lev := getPosFloat(pos, "leverage", ""); lev > 0 {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	// Verify unrealized P&L consistency (API value vs calculated from positions)
	diff := math.Abs(totalUnrealizedProfit - totalUnrealizedPnLCalculated)
	if diff > 5.0 {
		logger.Infof("⚠️ Unrealized P&L inconsistency: API=%.4f, Calculated=%.4f, Diff=%.4f",
			totalUnrealizedProfit, totalUnrealizedPnLCalculated, diff)
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	} else {
		logger.Infof("⚠️ Initial Balance abnormal: %.2f, cannot calculate P&L percentage", at.initialBalance)
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// Realized P&L: only from closed positions (so UI can show "总盈亏(含持仓)" vs "已实现盈亏(仅平仓)")
	realizedPnL := 0.0
	if at.store != nil {
		if stats, err := at.store.Position().GetFullStats(at.id); err == nil && stats != nil {
			realizedPnL = stats.TotalPnL
		}
	}

	result := map[string]interface{}{
		// Core fields
		"total_equity":       totalEquity,           // Account equity = wallet + unrealized
		"wallet_balance":     totalWalletBalance,    // Wallet balance (excluding unrealized P&L)
		"unrealized_profit": totalUnrealizedProfit, // Unrealized P&L (official value from exchange API)
		"available_balance":  availableBalance,      // Available balance

		// P&L statistics
		"total_pnl":       totalPnL,          // Total P&L = equity - initial (includes open positions)
		"total_pnl_pct":   totalPnLPct,       // Total P&L percentage
		"realized_pnl":    realizedPnL,       // Realized P&L from closed positions only
		"initial_balance": at.initialBalance, // Initial balance
		"daily_pnl":       at.dailyPnL,       // Daily P&L

		// Position information
		"position_count":   len(positions),   // Position count
		"margin_used":     totalMarginUsed,  // Margin used
		"margin_used_pct": marginUsedPct,    // Margin usage rate
	}
	return result, nil
}

// GetPositions gets position list (for API). Supports both exchange format and paper format (position_side, position_amt, entry_price, unrealized_pnl).
// 实盘与实盘模拟共用此逻辑：同一套字段与计算（含 price_change_pct、trailing、scaled_tp 等），确保展示与策略判断一致。
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		symbol := getPosStr(pos, "symbol", "")
		if symbol == "" {
			continue
		}
		side := getPosStr(pos, "side", "position_side")
		entryPrice := getPosFloat(pos, "entryPrice", "entry_price")
		markPrice := getPosFloat(pos, "markPrice", "mark_price")
		quantity := getPosFloat(pos, "positionAmt", "position_amt")
		if quantity < 0 {
			quantity = -quantity
		}
		if quantity == 0 {
			continue
		}
		unrealizedPnl := getPosFloat(pos, "unRealizedProfit", "unrealized_pnl")
		liquidationPrice := getPosFloat(pos, "liquidationPrice", "liquidation_price")

		// Use position's leverage; default 2 when 0/missing (was 10 and caused "AI 2x but display 10x" when AI sent 0)
		leverage := 2
		if lev := getPosFloat(pos, "leverage", ""); lev > 0 {
			leverage = int(lev)
		}

		// Calculate margin used
		marginUsed := (quantity * markPrice) / float64(leverage)

		// Calculate P&L percentage (based on margin)
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// Normalize side to lowercase for frontend (e.g. LONG -> long)
		sideNorm := strings.ToLower(side)
		posKey := symbol + "_" + sideNorm

		// 价格变动百分比（相对入场价，与分层止盈判断一致；当前盈亏%为保证金收益率）
		priceChangePct := 0.0
		if entryPrice > 0 && markPrice > 0 {
			if sideNorm == "long" {
				priceChangePct = (markPrice - entryPrice) / entryPrice * 100
			} else {
				priceChangePct = (entryPrice - markPrice) / entryPrice * 100
			}
		}
		out := map[string]interface{}{
			"symbol":             symbol,
			"side":               sideNorm,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"price_change_pct":   priceChangePct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		}
		// Merge fixed params: 1) in-memory (本轮 AI 开仓) 2) 缺省时从 DB 补全，实现重启后仍显示、随轮询实时更新
		at.positionParamsMu.RLock()
		if p := at.positionParamsMap[posKey]; p != nil {
			if p.StopLoss > 0 {
				out["stop_loss"] = p.StopLoss
			}
			if p.TakeProfit > 0 {
				out["take_profit"] = p.TakeProfit
			}
			if p.ATRAtOpen > 0 {
				out["atr_at_open"] = p.ATRAtOpen
			}
			if p.ATRMultipleSL > 0 {
				out["atr_multiple_sl"] = p.ATRMultipleSL
			}
			if p.ATRMultipleTP > 0 {
				out["atr_multiple_tp"] = p.ATRMultipleTP
			}
			if p.ATRPeriod > 0 {
				out["atr_period"] = p.ATRPeriod
			}
		}
		at.positionParamsMu.RUnlock()
		// 内存无参数时从 DB 补全，保证参数一直显示（含重启后）
		if _, hasSL := out["stop_loss"]; !hasSL && at.store != nil {
			dbSide := strings.ToUpper(sideNorm) // DB 存 LONG/SHORT
			if dbPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, dbSide); err == nil && dbPos != nil {
				if dbPos.StopLoss > 0 {
					out["stop_loss"] = dbPos.StopLoss
				}
				if dbPos.TakeProfit > 0 {
					out["take_profit"] = dbPos.TakeProfit
				}
				if dbPos.ATRAtOpen > 0 {
					out["atr_at_open"] = dbPos.ATRAtOpen
				}
				if dbPos.ATRMultipleSL > 0 {
					out["atr_multiple_sl"] = dbPos.ATRMultipleSL
				}
				if dbPos.ATRMultipleTP > 0 {
					out["atr_multiple_tp"] = dbPos.ATRMultipleTP
				}
				if dbPos.ATRPeriod > 0 {
					out["atr_period"] = dbPos.ATRPeriod
				}
			}
		}
		// 有止损/止盈和标记价时计算距止损、距止盈百分比，随轮询实时更新
		if markPrice > 0 {
			if sl, ok := out["stop_loss"].(float64); ok && sl > 0 {
				if sideNorm == "long" {
					out["distance_to_sl_pct"] = (markPrice - sl) / markPrice * 100
				} else {
					out["distance_to_sl_pct"] = (sl - markPrice) / markPrice * 100
				}
			}
			if tp, ok := out["take_profit"].(float64); ok && tp > 0 {
				if sideNorm == "long" {
					out["distance_to_tp_pct"] = (tp - markPrice) / markPrice * 100
				} else {
					out["distance_to_tp_pct"] = (markPrice - tp) / markPrice * 100
				}
			}
		}
		// 追踪止损 / 分层止盈：从策略配置与运行状态注入，与回测当前持仓一致
		if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
			risk := at.strategyEngine.GetConfig().RiskControl
			slCfg := risk.DynamicStopLoss
			tpCfg := risk.DynamicTakeProfit
			if slCfg != nil && slCfg.Enabled {
				if slCfg.TrailingEnabled != nil && *slCfg.TrailingEnabled {
					out["trailing_enabled"] = true
					at.peakPnLCacheMutex.RLock()
					peakPct := at.peakPnLCache[posKey]
					at.peakPnLCacheMutex.RUnlock()
					tier := 0
					var allowedDD float64
					for i, lv := range slCfg.TrailingLevels {
						if peakPct >= lv.ProfitThreshold {
							tier = i + 1
							allowedDD = lv.TrailingPercent
						}
					}
					if tier > 0 {
						out["trailing_tier_activated"] = tier
						out["trailing_allowed_drawdown"] = allowedDD
					}
				}
				if slCfg.SupportResistanceEnabled != nil && *slCfg.SupportResistanceEnabled {
					out["support_resistance_enabled"] = true
					if slCfg.SupportResistanceBuffer != nil && *slCfg.SupportResistanceBuffer > 0 {
						out["support_resistance_buffer"] = *slCfg.SupportResistanceBuffer
					}
				}
			}
			if tpCfg != nil && tpCfg.Enabled {
				if tpCfg.ScaledEnabled != nil && *tpCfg.ScaledEnabled && len(tpCfg.ScaledLevels) > 0 {
					out["scaled_tp_enabled"] = true
					taken := at.getScaledLevelsTaken(posKey)
					level := len(taken)
					if level > 0 {
						out["scaled_tp_level"] = level
						var closedPct float64
						for i, lv := range tpCfg.ScaledLevels {
							if i < len(taken) {
								closedPct += lv.ClosePercent
							}
						}
						if closedPct > 0 {
							out["scaled_tp_closed_pct"] = closedPct
						}
					}
				}
				if tpCfg.ResistanceEnabled != nil && *tpCfg.ResistanceEnabled {
					out["resistance_enabled"] = true
					if tpCfg.ResistanceBuffer != nil && *tpCfg.ResistanceBuffer > 0 {
						out["resistance_buffer"] = *tpCfg.ResistanceBuffer
					}
				}
			}
		}
		result = append(result, out)
	}

	return result, nil
}

// calculatePnLPercentage calculates P&L percentage (based on margin, automatically considers leverage)
// Return rate = Unrealized P&L / Margin × 100%
func calculatePnLPercentage(unrealizedPnl, marginUsed float64) float64 {
	if marginUsed > 0 {
		return (unrealizedPnl / marginUsed) * 100
	}
	return 0.0
}

// sortDecisionsByPriority sorts decisions: close positions first, then open positions, finally hold/wait
// This avoids position stacking overflow when changing positions
func sortDecisionsByPriority(decisions []kernel.Decision) []kernel.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// Define priority
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1 // Highest priority: close positions first
		case "open_long", "open_short":
			return 2 // Second priority: open positions later
		case "hold", "wait":
			return 3 // Lowest priority: wait
		default:
			return 999 // Unknown actions at the end
		}
	}

	// Copy decision list
	sorted := make([]kernel.Decision, len(decisions))
	copy(sorted, decisions)

	// Sort by priority
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// sltpCheckInterval 策略止盈/止损检查间隔，独立于 AI 决策间隔，触发即平仓；30s 在实时性与接口/负载间折中
const sltpCheckInterval = 30 * time.Second

// startDynamicSLTPMonitor 启动后台协程，按固定间隔检查动态止盈/止损，不依赖 AI 周期
func (at *AutoTrader) startDynamicSLTPMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()
		ticker := time.NewTicker(sltpCheckInterval)
		defer ticker.Stop()
		logger.Infof("🛡️ [%s] Strategy SL/TP check started (every %v, independent of AI interval)", at.name, sltpCheckInterval)
		for {
			select {
			case <-ticker.C:
				if err := at.checkDynamicStopLossTakeProfit(); err != nil {
					logger.Infof("⚠️ [%s] Strategy SL/TP check failed: %v", at.name, err)
				}
			case <-at.stopMonitorCh:
				logger.Infof("⏹ [%s] Strategy SL/TP check stopped", at.name)
				return
			}
		}
	}()
}

// startDrawdownMonitor starts drawdown monitoring
func (at *AutoTrader) startDrawdownMonitor() {
	at.monitorWg.Add(1)
	go func() {
		defer at.monitorWg.Done()

		ticker := time.NewTicker(1 * time.Minute) // Check every minute
		defer ticker.Stop()

		logger.Info("📊 Started position drawdown monitoring (check every minute)")

		for {
			select {
			case <-ticker.C:
				at.checkPositionDrawdown()
			case <-at.stopMonitorCh:
				logger.Info("⏹ Stopped position drawdown monitoring")
				return
			}
		}
	}()
}

// checkPositionDrawdown checks position drawdown situation
func (at *AutoTrader) checkPositionDrawdown() {
	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		logger.Infof("❌ Drawdown monitoring: failed to get positions: %v", err)
		return
	}

	for _, pos := range positions {
		symbol := getPosStr(pos, "symbol", "")
		side := getPosStr(pos, "side", "position_side")
		if symbol == "" || side == "" {
			continue
		}
		side = strings.ToLower(side)
		entryPrice := getPosFloat(pos, "entryPrice", "entry_price")
		markPrice := getPosFloat(pos, "markPrice", "mark_price")
		quantity := getPosFloat(pos, "positionAmt", "position_amt")
		if quantity < 0 {
			quantity = -quantity
		}
		if quantity == 0 {
			continue
		}

		// Calculate current P&L percentage
		leverage := 10
		if lev := getPosFloat(pos, "leverage", ""); lev > 0 {
			leverage = int(lev)
		}

		var currentPnLPct float64
		if side == "long" {
			currentPnLPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			currentPnLPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// Construct unique position identifier (distinguish long/short)
		posKey := symbol + "_" + side

		// Get historical peak profit for this position
		at.peakPnLCacheMutex.RLock()
		peakPnLPct, exists := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		if !exists {
			// If no historical peak record, use current P&L as initial value
			peakPnLPct = currentPnLPct
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		} else {
			// Update peak cache
			at.UpdatePeakPnL(symbol, side, currentPnLPct)
		}

		// Calculate drawdown (magnitude of decline from peak)
		var drawdownPct float64
		if peakPnLPct > 0 && currentPnLPct < peakPnLPct {
			drawdownPct = ((peakPnLPct - currentPnLPct) / peakPnLPct) * 100
		}

		// Check close position condition: profit > 5% and drawdown >= 40%
		if currentPnLPct > 5.0 && drawdownPct >= 40.0 {
			logger.Infof("🚨 Drawdown close position condition triggered: %s %s | Current profit: %.2f%% | Peak profit: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)

			// Execute close position
			if err := at.emergencyClosePosition(symbol, side); err != nil {
				logger.Infof("❌ Drawdown close position failed (%s %s): %v", symbol, side, err)
			} else {
				logger.Infof("✅ Drawdown close position succeeded: %s %s", symbol, side)
				// Clear cache for this position after closing
				at.ClearPeakPnLCache(symbol, side)
			}
		} else if currentPnLPct > 5.0 {
			// Record situations close to close position condition (for debugging)
			logger.Infof("📊 Drawdown monitoring: %s %s | Profit: %.2f%% | Peak: %.2f%% | Drawdown: %.2f%%",
				symbol, side, currentPnLPct, peakPnLPct, drawdownPct)
		}
	}
}

// emergencyClosePosition emergency close position function
func (at *AutoTrader) emergencyClosePosition(symbol, side string) error {
	switch side {
	case "long":
		order, err := at.trader.CloseLong(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close long position succeeded, order ID: %v", order["orderId"])
	case "short":
		order, err := at.trader.CloseShort(symbol, 0) // 0 = close all
		if err != nil {
			return err
		}
		logger.Infof("✅ Emergency close short position succeeded, order ID: %v", order["orderId"])
	default:
		return fmt.Errorf("unknown position direction: %s", side)
	}

	return nil
}

// GetPeakPnLCache gets peak profit cache
func (at *AutoTrader) GetPeakPnLCache() map[string]float64 {
	at.peakPnLCacheMutex.RLock()
	defer at.peakPnLCacheMutex.RUnlock()

	// Return a copy of the cache
	cache := make(map[string]float64)
	for k, v := range at.peakPnLCache {
		cache[k] = v
	}
	return cache
}

// UpdatePeakPnL updates peak profit cache
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	if peak, exists := at.peakPnLCache[posKey]; exists {
		// Update peak (if long, take larger value; if short, currentPnLPct is negative, also compare)
		if currentPnLPct > peak {
			at.peakPnLCache[posKey] = currentPnLPct
		}
	} else {
		// First time recording
		at.peakPnLCache[posKey] = currentPnLPct
	}
}

// ClearPeakPnLCache clears peak cache for specified position
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := symbol + "_" + side
	delete(at.peakPnLCache, posKey)
}

// setPositionParams stores fixed params for a position (for API/UI, same as backtest). Call after successful open.
func (at *AutoTrader) setPositionParams(posKey string, stopLoss, takeProfit float64) {
	p := &positionParams{StopLoss: stopLoss, TakeProfit: takeProfit}
	symbol := strings.TrimSuffix(posKey, "_long")
	if symbol == posKey {
		symbol = strings.TrimSuffix(posKey, "_short")
	}
	if at.marketClient != nil {
		klines, err := at.marketClient.GetKlines(symbol, "15m", 50)
		if err == nil && len(klines) > 0 {
			period := 14
			if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
				rc := at.strategyEngine.GetConfig().RiskControl
				if rc.DynamicStopLoss != nil {
					if market.Normalize(symbol) == "BTCUSDT" || market.Normalize(symbol) == "ETHUSDT" {
						if rc.DynamicStopLoss.ATRPeriodBTCETH != nil {
							period = *rc.DynamicStopLoss.ATRPeriodBTCETH
						}
					} else {
						if rc.DynamicStopLoss.ATRPeriodAltcoin != nil {
							period = *rc.DynamicStopLoss.ATRPeriodAltcoin
						}
					}
				}
			}
			p.ATRPeriod = period
			p.ATRAtOpen = kernel.CalculateATR(klines, period)
		}
	}
	if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
		rc := at.strategyEngine.GetConfig().RiskControl
		if rc.DynamicStopLoss != nil && rc.DynamicStopLoss.ATREnabled != nil && *rc.DynamicStopLoss.ATREnabled {
			minS, maxS := 0.0, 0.0
			if rc.DynamicStopLoss.ATRMultiplierMin != nil {
				minS = *rc.DynamicStopLoss.ATRMultiplierMin
			}
			if rc.DynamicStopLoss.ATRMultiplierMax != nil {
				maxS = *rc.DynamicStopLoss.ATRMultiplierMax
			}
			if maxS > minS {
				p.ATRMultipleSL = (minS + maxS) / 2
			} else if minS > 0 {
				p.ATRMultipleSL = minS
			}
		}
		if rc.DynamicTakeProfit != nil && rc.DynamicTakeProfit.ATREnabled != nil && *rc.DynamicTakeProfit.ATREnabled {
			minT, maxT := 0.0, 0.0
			if rc.DynamicTakeProfit.ATRMultiplierMin != nil {
				minT = *rc.DynamicTakeProfit.ATRMultiplierMin
			}
			if rc.DynamicTakeProfit.ATRMultiplierMax != nil {
				maxT = *rc.DynamicTakeProfit.ATRMultiplierMax
			}
			if maxT > minT {
				p.ATRMultipleTP = (minT + maxT) / 2
			} else if minT > 0 {
				p.ATRMultipleTP = minT
			}
		}
	}
	at.positionParamsMu.Lock()
	defer at.positionParamsMu.Unlock()
	at.positionParamsMap[posKey] = p
}

// clearPositionParams removes stored params for a position. Call on close.
func (at *AutoTrader) clearPositionParams(posKey string) {
	at.positionParamsMu.Lock()
	defer at.positionParamsMu.Unlock()
	delete(at.positionParamsMap, posKey)
}

// adjustStopLossTakeProfitForActualEntry recalculates SL/TP based on actual entry price
// This ensures the risk/reward ratio is preserved when actual entry differs from AI analysis price
// Returns: adjustedSL, adjustedTP, priceDeviation%
func adjustStopLossTakeProfitForActualEntry(
	aiAnalysisPrice float64,
	actualEntryPrice float64,
	aiStopLoss float64,
	aiTakeProfit float64,
	side string,
) (float64, float64, float64) {
	if aiAnalysisPrice <= 0 || actualEntryPrice <= 0 {
		return aiStopLoss, aiTakeProfit, 0
	}

	// Calculate price deviation percentage
	priceDeviation := ((actualEntryPrice - aiAnalysisPrice) / aiAnalysisPrice) * 100

	// Calculate AI's SL/TP percentages relative to analysis price
	var slPct, tpPct float64
	if side == "long" || side == "LONG" {
		// Long: SL below entry, TP above entry
		slPct = (aiAnalysisPrice - aiStopLoss) / aiAnalysisPrice * 100   // positive value
		tpPct = (aiTakeProfit - aiAnalysisPrice) / aiAnalysisPrice * 100 // positive value
	} else {
		// Short: SL above entry, TP below entry
		slPct = (aiStopLoss - aiAnalysisPrice) / aiAnalysisPrice * 100   // positive value
		tpPct = (aiAnalysisPrice - aiTakeProfit) / aiAnalysisPrice * 100 // positive value
	}

	// Recalculate SL/TP using actual entry price with same percentages
	var adjustedSL, adjustedTP float64
	if side == "long" || side == "LONG" {
		adjustedSL = actualEntryPrice * (1 - slPct/100)
		adjustedTP = actualEntryPrice * (1 + tpPct/100)
	} else {
		adjustedSL = actualEntryPrice * (1 + slPct/100)
		adjustedTP = actualEntryPrice * (1 - tpPct/100)
	}

	return adjustedSL, adjustedTP, priceDeviation
}

func (at *AutoTrader) getScaledLevelsTaken(posKey string) []float64 {
	at.scaledLevelsTakenMu.RLock()
	defer at.scaledLevelsTakenMu.RUnlock()
	if taken, ok := at.scaledLevelsTaken[posKey]; ok {
		return append([]float64(nil), taken...)
	}
	return nil
}

func (at *AutoTrader) addScaledLevelTaken(posKey string, profitPercent float64) {
	at.scaledLevelsTakenMu.Lock()
	defer at.scaledLevelsTakenMu.Unlock()
	at.scaledLevelsTaken[posKey] = append(at.scaledLevelsTaken[posKey], profitPercent)
}

func (at *AutoTrader) clearScaledLevelsTakenForPosition(posKey string) {
	at.scaledLevelsTakenMu.Lock()
	defer at.scaledLevelsTakenMu.Unlock()
	delete(at.scaledLevelsTaken, posKey)
}

// recordAndConfirmOrder polls order status for actual fill data and records position
// action: open_long, open_short, close_long, close_short
// entryPrice: entry price when closing (0 when opening)
func (at *AutoTrader) recordAndConfirmOrder(orderResult map[string]interface{}, symbol, action string, quantity float64, price float64, leverage int, entryPrice float64) {
	if at.store == nil {
		return
	}

	// Get order ID (supports orderId and order_id for paper/simulation)
	var orderID string
	switch v := orderResult["orderId"].(type) {
	case int64:
		orderID = fmt.Sprintf("%d", v)
	case float64:
		orderID = fmt.Sprintf("%.0f", v)
	case string:
		orderID = v
	}
	if orderID == "" || orderID == "0" {
		if s, _ := orderResult["order_id"].(string); s != "" {
			orderID = s
		}
	}
	if orderID == "" || orderID == "0" {
		orderID = fmt.Sprintf("%v", orderResult["order_id"])
	}

	if orderID == "" || orderID == "0" {
		logger.Infof("  ⚠️ Order ID is empty, skipping record")
		return
	}

	// Determine positionSide
	var positionSide string
	switch action {
	case "open_long", "close_long":
		positionSide = "LONG"
	case "open_short", "close_short":
		positionSide = "SHORT"
	}

	var actualPrice = price
	var actualQty = quantity
	var fee float64

	// 实盘模拟：纸面订单立即成交，直接写入订单与持仓记录，不依赖 OrderSync
	if at.config.IsSimulation {
		if statusStr, _ := orderResult["status"].(string); statusStr == "FILLED" {
			orderRecord := at.createOrderRecord(orderID, symbol, action, positionSide, quantity, price, leverage)
			if err := at.store.Order().CreateOrder(orderRecord); err != nil {
				logger.Infof("  ⚠️ Failed to record paper order: %v", err)
			}
			if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, "FILLED", quantity, price, 0); err != nil {
				logger.Infof("  ⚠️ Failed to update paper order status: %v", err)
			}
			normalizedSymbolForPosition := market.Normalize(symbol)
			closeReason := at.getAndClearPendingCloseReason()
			if action == "close_long" || action == "close_short" {
				if closeReason == "" {
					closeReason = "sync"
				}
			}
			at.recordPositionChange(orderID, normalizedSymbolForPosition, positionSide, action, actualQty, actualPrice, leverage, entryPrice, 0, closeReason)
			logger.Infof("  📝 Paper order recorded: %s [%s] %s qty=%.4f", orderID, action, symbol, quantity)
		}
		return
	}

	// Exchanges with OrderSync: Skip immediate order recording, let OrderSync handle it
	// This ensures accurate data from GetTrades API and avoids duplicate records
	switch at.exchange {
	case "binance", "lighter", "hyperliquid", "bybit", "okx", "bitget", "aster", "kucoin", "gate":
		logger.Infof("  📝 Order submitted (id: %s), will be synced by OrderSync", orderID)
		return
	}

	// For exchanges without OrderSync (e.g., Binance): record immediately and poll for fill data
	orderRecord := at.createOrderRecord(orderID, symbol, action, positionSide, quantity, price, leverage)
	if err := at.store.Order().CreateOrder(orderRecord); err != nil {
		logger.Infof("  ⚠️ Failed to record order: %v", err)
	} else {
		logger.Infof("  📝 Order recorded: %s [%s] %s", orderID, action, symbol)
	}

	// Wait for order to be filled and get actual fill data
	time.Sleep(500 * time.Millisecond)
	for i := 0; i < 5; i++ {
		status, err := at.trader.GetOrderStatus(symbol, orderID)
		if err == nil {
			statusStr, _ := status["status"].(string)
			if statusStr == "FILLED" {
				// Get actual fill price
				if avgPrice, ok := status["avgPrice"].(float64); ok && avgPrice > 0 {
					actualPrice = avgPrice
				}
				// Get actual executed quantity
				if execQty, ok := status["executedQty"].(float64); ok && execQty > 0 {
					actualQty = execQty
				}
				// Get commission/fee
				if commission, ok := status["commission"].(float64); ok {
					fee = commission
				}
				logger.Infof("  ✅ Order filled: avgPrice=%.6f, qty=%.6f, fee=%.6f", actualPrice, actualQty, fee)

				// Update order status to FILLED
				if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, "FILLED", actualQty, actualPrice, fee); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
				}

				// Record fill details
				at.recordOrderFill(orderRecord.ID, orderID, symbol, action, actualPrice, actualQty, fee)
				break
			} else if statusStr == "CANCELED" || statusStr == "EXPIRED" || statusStr == "REJECTED" {
				logger.Infof("  ⚠️ Order %s, skipping position record", statusStr)

				// Update order status
				if err := at.store.Order().UpdateOrderStatus(orderRecord.ID, statusStr, 0, 0, 0); err != nil {
					logger.Infof("  ⚠️ Failed to update order status: %v", err)
				}
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Normalize symbol for position record consistency
	normalizedSymbolForPosition := market.Normalize(symbol)

	logger.Infof("  📝 Recording position (ID: %s, action: %s, price: %.6f, qty: %.6f, fee: %.4f)",
		orderID, action, actualPrice, actualQty, fee)

	closeReason := at.getAndClearPendingCloseReason()
	if (action == "close_long" || action == "close_short") && closeReason == "" {
		closeReason = "sync"
	}
	at.recordPositionChange(orderID, normalizedSymbolForPosition, positionSide, action, actualQty, actualPrice, leverage, entryPrice, fee, closeReason)

	// Send anonymous trade statistics for experience improvement (async, non-blocking)
	// This helps us understand overall product usage across all deployments
	experience.TrackTrade(experience.TradeEvent{
		Exchange:  at.exchange,
		TradeType: action,
		Symbol:    symbol,
		AmountUSD: actualPrice * actualQty,
		Leverage:  leverage,
		UserID:    at.userID,
		TraderID:  at.id,
	})
}

// recordPositionChange records position change (create record on open, update record on close)
// closeReason used when action is close_long/close_short (e.g. "system:sl:trailing", "ai", "manual", "sync")
func (at *AutoTrader) recordPositionChange(orderID, symbol, side, action string, quantity, price float64, leverage int, entryPrice float64, fee float64, closeReason string) {
	if at.store == nil {
		return
	}

	switch action {
	case "open_long", "open_short":
		// Open position: create new position record
		nowMs := time.Now().UTC().UnixMilli()
		pos := &store.TraderPosition{
			TraderID:     at.id,
			ExchangeID:   at.exchangeID, // Exchange account UUID
			ExchangeType: at.exchange,   // Exchange type: binance/bybit/okx/etc
			Symbol:       symbol,
			Side:         side, // LONG or SHORT
			Quantity:     quantity,
			EntryPrice:   price,
			EntryOrderID: orderID,
			EntryTime:    nowMs,
			Leverage:     leverage,
			Status:       "OPEN",
			CreatedAt:    nowMs,
			UpdatedAt:    nowMs,
		}
		if err := at.store.Position().Create(pos); err != nil {
			logger.Infof("  ⚠️ Failed to record position: %v", err)
		} else {
			logger.Infof("  📊 Position recorded [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
		}

	case "close_long", "close_short":
		// Close position using PositionBuilder for consistent handling
		// PositionBuilder will handle both cases:
		// 1. If open position exists: close it properly
		// 2. If no open position (e.g., table cleared): create a closed position record
		posBuilder := store.NewPositionBuilder(at.store.Position())
		if err := posBuilder.ProcessTrade(
			at.id, at.exchangeID, at.exchange,
			symbol, side, action,
			quantity, price, fee, 0, // realizedPnL will be calculated
			time.Now().UTC().UnixMilli(), orderID,
			closeReason,
		); err != nil {
			logger.Infof("  ⚠️ Failed to process close position: %v", err)
		} else {
			logger.Infof("  ✅ Position closed [%s] %s %s @ %.4f", at.id[:8], symbol, side, price)
		}
	}
}

// createOrderRecord creates an order record struct from order details
func (at *AutoTrader) createOrderRecord(orderID, symbol, action, positionSide string, quantity, price float64, leverage int) *store.TraderOrder {
	// Determine order type (market for auto trader)
	orderType := "MARKET"

	// Determine side (BUY/SELL)
	var side string
	switch action {
	case "open_long", "close_short":
		side = "BUY"
	case "open_short", "close_long":
		side = "SELL"
	}

	// Use action as orderAction directly (keep lowercase format)
	orderAction := action

	// Determine if it's a reduce only order
	reduceOnly := (action == "close_long" || action == "close_short")

	// Normalize symbol for consistency
	normalizedSymbol := market.Normalize(symbol)

	return &store.TraderOrder{
		TraderID:        at.id,
		ExchangeID:      at.exchangeID,
		ExchangeType:    at.exchange,
		ExchangeOrderID: orderID,
		Symbol:          normalizedSymbol,
		Side:            side,
		PositionSide:    positionSide,
		Type:            orderType,
		TimeInForce:     "GTC",
		Quantity:        quantity,
		Price:           price,
		Status:          "NEW",
		FilledQuantity:  0,
		AvgFillPrice:    0,
		Commission:      0,
		CommissionAsset: "USDT",
		Leverage:        leverage,
		ReduceOnly:      reduceOnly,
		ClosePosition:   reduceOnly,
		OrderAction:     orderAction,
		CreatedAt:       time.Now().UTC().UnixMilli(),
		UpdatedAt:       time.Now().UTC().UnixMilli(),
	}
}

// recordOrderFill records order fill/trade details
func (at *AutoTrader) recordOrderFill(orderRecordID int64, exchangeOrderID, symbol, action string, price, quantity, fee float64) {
	if at.store == nil {
		return
	}

	// Determine side (BUY/SELL)
	var side string
	switch action {
	case "open_long", "close_short":
		side = "BUY"
	case "open_short", "close_long":
		side = "SELL"
	}

	// Generate a simple trade ID (exchange doesn't always provide one)
	tradeID := fmt.Sprintf("%s-%d", exchangeOrderID, time.Now().UnixNano())

	// Normalize symbol for consistency
	normalizedSymbol := market.Normalize(symbol)

	fill := &store.TraderFill{
		TraderID:         at.id,
		ExchangeID:       at.exchangeID,
		ExchangeType:     at.exchange,
		OrderID:          orderRecordID,
		ExchangeOrderID:  exchangeOrderID,
		ExchangeTradeID:  tradeID,
		Symbol:           normalizedSymbol,
		Side:             side,
		Price:            price,
		Quantity:         quantity,
		QuoteQuantity:    price * quantity,
		Commission:       fee,
		CommissionAsset:  "USDT",
		RealizedPnL:      0, // Will be calculated for close orders
		IsMaker:          false, // Market orders are usually taker
		CreatedAt:        time.Now().UTC().UnixMilli(),
	}

	// Calculate realized PnL for close orders
	if action == "close_long" || action == "close_short" {
		// Try to get the entry price from the open position
		var positionSide string
		if action == "close_long" {
			positionSide = "LONG"
		} else {
			positionSide = "SHORT"
		}

		if openPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, symbol, positionSide); err == nil && openPos != nil {
			if positionSide == "LONG" {
				fill.RealizedPnL = (price - openPos.EntryPrice) * quantity
			} else {
				fill.RealizedPnL = (openPos.EntryPrice - price) * quantity
			}
		}
	}

	if err := at.store.Order().CreateFill(fill); err != nil {
		logger.Infof("  ⚠️ Failed to record fill: %v", err)
	} else {
		logger.Infof("  📋 Fill recorded: %.4f @ %.6f, fee: %.4f", quantity, price, fee)
	}
}

// ============================================================================
// Risk Control Helpers
// ============================================================================

// isBTCETH checks if a symbol is BTC or ETH
func isBTCETH(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	return strings.HasPrefix(symbol, "BTC") || strings.HasPrefix(symbol, "ETH")
}

// enforcePositionValueRatio checks and enforces position value ratio limits (CODE ENFORCED)
// Returns the adjusted position size (capped if necessary) and whether the position was capped
// positionSizeUSD: the original position size in USD
// equity: the account equity
// symbol: the trading symbol
func (at *AutoTrader) enforcePositionValueRatio(positionSizeUSD float64, equity float64, symbol string) (float64, bool) {
	if at.config.StrategyConfig == nil {
		return positionSizeUSD, false
	}

	riskControl := at.config.StrategyConfig.RiskControl

	// Get the appropriate position value ratio limit
	var maxPositionValueRatio float64
	if isBTCETH(symbol) {
		maxPositionValueRatio = riskControl.BTCETHMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 5.0 // Default: 5x for BTC/ETH
		}
	} else {
		maxPositionValueRatio = riskControl.AltcoinMaxPositionValueRatio
		if maxPositionValueRatio <= 0 {
			maxPositionValueRatio = 1.0 // Default: 1x for altcoins
		}
	}

	// Calculate max allowed position value = equity × ratio
	maxPositionValue := equity * maxPositionValueRatio

	// Check if position size exceeds limit
	if positionSizeUSD > maxPositionValue {
		logger.Infof("  ⚠️ [RISK CONTROL] Position %.2f USDT exceeds limit (equity %.2f × %.1fx = %.2f USDT max for %s), capping",
			positionSizeUSD, equity, maxPositionValueRatio, maxPositionValue, symbol)
		return maxPositionValue, true
	}

	return positionSizeUSD, false
}

// enforceMinPositionSize checks minimum position size (CODE ENFORCED)
func (at *AutoTrader) enforceMinPositionSize(positionSizeUSD float64) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	minSize := at.config.StrategyConfig.RiskControl.MinPositionSize
	if minSize <= 0 {
		minSize = 12 // Default: 12 USDT
	}

	if positionSizeUSD < minSize {
		return fmt.Errorf("❌ [RISK CONTROL] Position %.2f USDT below minimum (%.2f USDT)", positionSizeUSD, minSize)
	}
	return nil
}

// enforceMaxPositions checks maximum positions count (CODE ENFORCED)
func (at *AutoTrader) enforceMaxPositions(currentPositionCount int) error {
	if at.config.StrategyConfig == nil {
		return nil
	}

	maxPositions := at.config.StrategyConfig.RiskControl.MaxPositions
	if maxPositions <= 0 {
		maxPositions = 3 // Default: 3 positions
	}

	if currentPositionCount >= maxPositions {
		return fmt.Errorf("❌ [RISK CONTROL] Already at max positions (%d/%d)", currentPositionCount, maxPositions)
	}
	return nil
}

// getSideFromAction converts order action to side (BUY/SELL)
func getSideFromAction(action string) string {
	switch action {
	case "open_long", "close_short":
		return "BUY"
	case "open_short", "close_long":
		return "SELL"
	default:
		return "BUY"
	}
}

// GetOpenOrders returns open orders (pending SL/TP) from exchange
func (at *AutoTrader) GetOpenOrders(symbol string) ([]OpenOrder, error) {
	return at.trader.GetOpenOrders(symbol)
}

// checkDynamicStopLossTakeProfit checks all positions for dynamic stop loss and take profit triggers.
// 实盘与实盘模拟共用：positions 来自 at.trader.GetPositions()（交易所或 PaperTrader），后续判断与执行逻辑一致。
func (at *AutoTrader) checkDynamicStopLossTakeProfit() error {
	// Check if dynamic stop loss/take profit is enabled
	if at.strategyEngine == nil || at.strategyEngine.GetConfig() == nil {
		return nil
	}

	riskConfig := at.strategyEngine.GetConfig().RiskControl
	stopLossConfig := riskConfig.DynamicStopLoss
	takeProfitConfig := riskConfig.DynamicTakeProfit

	// Skip if both are disabled
	if (stopLossConfig == nil || !stopLossConfig.Enabled) && (takeProfitConfig == nil || !takeProfitConfig.Enabled) {
		return nil
	}

	// Get current positions
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	if len(positions) == 0 {
		return nil // No positions to check
	}

	// Create checkers
	var stopLossChecker *kernel.StopLossChecker
	var takeProfitChecker *kernel.TakeProfitChecker

	if stopLossConfig != nil && stopLossConfig.Enabled {
		stopLossChecker = kernel.NewStopLossChecker(stopLossConfig)
	}
	if takeProfitConfig != nil && takeProfitConfig.Enabled {
		takeProfitChecker = kernel.NewTakeProfitChecker(takeProfitConfig)
		scaledOn := takeProfitConfig.ScaledEnabled != nil && *takeProfitConfig.ScaledEnabled
		levelCount := len(takeProfitConfig.ScaledLevels)
		logger.Infof("📋 Take profit checker created: enabled=true, scaled_enabled=%v, scaled_levels=%d", scaledOn, levelCount)
	}

	currentPositionKeys := make(map[string]bool) // for cleaning slConfirmCount when position is closed
	// Check each position (use getPosStr/getPosFloat so paper and exchange formats both work)
	for _, pos := range positions {
		symbol := getPosStr(pos, "symbol", "")
		if symbol == "" {
			continue
		}
		side := getPosStr(pos, "side", "position_side")
		side = strings.ToLower(side)
		// 便于排查分层止盈：确认每个仓位都进入 SL/TP 检查
		logger.Infof("📋 TP/SL: checking position %s %s", symbol, side)
		entryPrice := getPosFloat(pos, "entryPrice", "entry_price")
		markPrice := getPosFloat(pos, "markPrice", "mark_price")
		quantity := getPosFloat(pos, "positionAmt", "position_amt")
		if quantity < 0 {
			quantity = -quantity
		}

		// Skip closed positions
		if quantity == 0 {
			continue
		}

		leverage := 10
		if lev := getPosFloat(pos, "leverage", ""); lev > 0 {
			leverage = int(lev)
		}

		unrealizedPnl := getPosFloat(pos, "unRealizedProfit", "unrealized_pnl")
		liquidationPrice := getPosFloat(pos, "liquidationPrice", "liquidation_price")
		marginUsed := (quantity * markPrice) / float64(leverage)

		// Calculate P&L percentage
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)

		// Get position update time (posKey must match positionFirstSeenTime: symbol_side lowercase)
		// 优先用持仓 map 的 update_time（纸面恢复后 GetPositions 会带真实入场时间），否则查 DB，避免重启后被误判为刚开仓导致最小持仓跳过、分层止盈不触发
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true
		updateTime := getPosInt64(pos, "update_time", "updateTime", "entry_time", "createdTime")
		if updateTime > 0 {
			if at.positionFirstSeenTime[posKey] == 0 {
				at.positionFirstSeenTime[posKey] = updateTime
			}
		} else {
			updateTime = at.positionFirstSeenTime[posKey]
		}
		if updateTime == 0 && at.store != nil {
			normalizedSymbol := market.Normalize(symbol)
			sideUpper := strings.ToUpper(side)
			dbPos, dbErr := at.store.Position().GetOpenPositionBySymbol(at.id, normalizedSymbol, sideUpper)
			if dbErr == nil && dbPos != nil && dbPos.EntryTime > 0 {
				updateTime = dbPos.EntryTime
				at.positionFirstSeenTime[posKey] = updateTime
				logger.Infof("📋 TP/SL: got entry time from DB for %s %s: entryTime=%d", symbol, side, updateTime)
			} else if at.config.IsSimulation {
				// Fallback: match from all open positions (handles symbol format differences)
				openList, _ := at.store.Position().GetOpenPositions(at.id)
				for _, o := range openList {
					if market.Normalize(o.Symbol) == normalizedSymbol && strings.ToUpper(o.Side) == sideUpper && o.EntryTime > 0 {
						updateTime = o.EntryTime
						at.positionFirstSeenTime[posKey] = updateTime
						logger.Infof("📋 TP/SL: got entry time from DB (list) for %s %s: entryTime=%d", symbol, side, updateTime)
						break
					}
				}
				if updateTime == 0 {
					logger.Infof("📋 TP/SL: no DB entry for %s %s (err=%v), using now as entry time", symbol, side, dbErr)
				}
			}
		}
		if updateTime == 0 {
			updateTime = time.Now().UnixMilli()
		}

		// Min hold: skip dynamic SL/TP if position opened too recently (same as backtest; 避免开仓即平仓)
		nowMs := time.Now().UTC().UnixMilli()
		holdDurationMs := nowMs - updateTime
		minHoldMs := 0.0
		if stopLossConfig != nil && stopLossConfig.Enabled && stopLossConfig.MinHoldMinutes > 0 {
			minHoldMs = stopLossConfig.MinHoldMinutes * 60 * 1000
		}
		if takeProfitConfig != nil && takeProfitConfig.Enabled && takeProfitConfig.MinHoldMinutes > 0 {
			tpMin := takeProfitConfig.MinHoldMinutes * 60 * 1000
			if tpMin > minHoldMs {
				minHoldMs = tpMin
			}
		}
		if minHoldMs > 0 && float64(holdDurationMs) < minHoldMs {
			logger.Infof("📋 TP/SL: skip %s %s due to min hold: holdDurationMs=%d, minHoldMs=%.0f (%.1f min left)", symbol, side, holdDurationMs, minHoldMs, (minHoldMs-float64(holdDurationMs))/60000)
			continue
		}

		// Get peak PnL
		at.peakPnLCacheMutex.RLock()
		peakPnlPct := at.peakPnLCache[posKey]
		at.peakPnLCacheMutex.RUnlock()

		// Update peak PnL if current is higher
		if pnlPct > peakPnlPct {
			at.peakPnLCacheMutex.Lock()
			at.peakPnLCache[posKey] = pnlPct
			peakPnlPct = pnlPct
			at.peakPnLCacheMutex.Unlock()
		}

		scaledTaken := at.getScaledLevelsTaken(posKey)
		positionInfo := kernel.PositionInfo{
			Symbol:            symbol,
			Side:              side,
			EntryPrice:        entryPrice,
			MarkPrice:         markPrice,
			Quantity:          quantity,
			Leverage:          leverage,
			UnrealizedPnL:     unrealizedPnl,
			UnrealizedPnLPct:  pnlPct,
			PeakPnLPct:        peakPnlPct,
			LiquidationPrice:  liquidationPrice,
			MarginUsed:        marginUsed,
			UpdateTime:        updateTime,
			ScaledLevelsTaken: scaledTaken,
		}

		// Get market data for ATR and support/resistance calculations
		klines, err := at.marketClient.GetKlines(symbol, "15m", 50)
		if err != nil {
			logger.Infof("⚠️  Failed to get klines for %s: %v", symbol, err)
			continue
		}

		// Calculate ATR (14) and longer-period ATR (28) for high-vol tolerance
		atr := kernel.CalculateATR(klines, 14)
		atrLong := kernel.CalculateATR(klines, 28)

		// Calculate highest price since entry (for trailing stop)
		// IMPORTANT: Only consider klines AFTER position entry to avoid using pre-entry extremes
		highestPrice := entryPrice
		entryTimeMs := updateTime // position entry time in ms
		if side == "long" {
			for _, k := range klines {
				// Only use klines that started after position entry
				if k.OpenTime >= entryTimeMs && k.High > highestPrice {
					highestPrice = k.High
				}
			}
		} else {
			// For short: find lowest price since entry (represents peak profit)
			highestPrice = entryPrice
			for _, k := range klines {
				if k.OpenTime >= entryTimeMs && k.Low < highestPrice {
					highestPrice = k.Low
				}
			}
		}

		// Find support/resistance levels
		// Long: needs support (for SL) and resistance (for TP)
		// Short: needs resistance (for SL) and support (for TP)
		supportLevel := kernel.FindSupportLevel(klines, markPrice, 30)
		resistanceLevel := kernel.FindResistanceLevel(klines, markPrice, 30)

		// 锁定利润阈值：达到此盈利后移动止损到盈亏平衡点（策略中 LockProfitPercent）
		breakevenLocked := takeProfitConfig != nil && takeProfitConfig.LockProfitPercent != nil &&
			*takeProfitConfig.LockProfitPercent > 0 && pnlPct >= *takeProfitConfig.LockProfitPercent
		if breakevenLocked {
			// 价格已回到盈亏平衡点下方（多）或上方（空）→ 按盈亏平衡止损
			if side == "long" && markPrice <= entryPrice {
				signal := &kernel.StopLossSignal{Triggered: true, Reason: "Breakeven stop (profit locked)", Price: entryPrice, Type: "breakeven"}
				kernel.LogStopLossCheck(symbol, signal)
				if err := at.executeStopLoss(&positionInfo, signal); err != nil {
					logger.Infof("❌ Failed to execute breakeven stop for %s: %v", symbol, err)
				}
				continue
			}
			if side == "short" && markPrice >= entryPrice {
				signal := &kernel.StopLossSignal{Triggered: true, Reason: "Breakeven stop (profit locked)", Price: entryPrice, Type: "breakeven"}
				kernel.LogStopLossCheck(symbol, signal)
				if err := at.executeStopLoss(&positionInfo, signal); err != nil {
					logger.Infof("❌ Failed to execute breakeven stop for %s: %v", symbol, err)
				}
				continue
			}
		}

		// Check stop loss
		// Long: use supportLevel (stop below support)
		// Short: use resistanceLevel (stop above resistance)
		if stopLossChecker != nil {
			slLevel := supportLevel
			if side == "short" {
				slLevel = resistanceLevel
			}
			signal := stopLossChecker.CheckStopLoss(&positionInfo, markPrice, highestPrice, atr, atrLong, slLevel)
			// 已锁定利润时，止损价不得劣于入场价（多：不低于入场；空：不高于入场）
			if signal.Triggered && breakevenLocked {
				if side == "long" && signal.Price < entryPrice {
					signal = &kernel.StopLossSignal{Triggered: true, Reason: signal.Reason + " (clamped to breakeven)", Price: entryPrice, Type: signal.Type}
				}
				if side == "short" && signal.Price > entryPrice {
					signal = &kernel.StopLossSignal{Triggered: true, Reason: signal.Reason + " (clamped to breakeven)", Price: entryPrice, Type: signal.Type}
				}
			}
			if signal.Triggered {
				// Confirm cycles: execute only after N consecutive cycles with SL condition met
				requiredCycles := 1
				if stopLossConfig.ConfirmCycles > 0 {
					requiredCycles = stopLossConfig.ConfirmCycles
				}
				if stopLossConfig.ATRToleranceEnabled != nil && *stopLossConfig.ATRToleranceEnabled && atrLong > 0 {
					highMult := 1.2
					if stopLossConfig.ATRHighMultiplier != nil {
						highMult = *stopLossConfig.ATRHighMultiplier
					}
					if atr > atrLong*highMult {
						requiredCycles++ // high volatility: require one more confirm
					}
				}
				at.slConfirmCountMu.Lock()
				at.slConfirmCount[posKey]++
				count := at.slConfirmCount[posKey]
				at.slConfirmCountMu.Unlock()
				if count >= requiredCycles {
					kernel.LogStopLossCheck(symbol, signal)
					if err := at.executeStopLoss(&positionInfo, signal); err != nil {
						logger.Infof("❌ Failed to execute stop loss for %s: %v", symbol, err)
					}
					at.slConfirmCountMu.Lock()
					delete(at.slConfirmCount, posKey)
					at.slConfirmCountMu.Unlock()
					continue
				}
				// Not yet enough consecutive confirms; skip execute and take-profit check this cycle
				continue
			}
			// Condition not triggered: reset consecutive count so we require a fresh run of N cycles
			at.slConfirmCountMu.Lock()
			delete(at.slConfirmCount, posKey)
			at.slConfirmCountMu.Unlock()
		}

		// Check take profit
		// Long: use resistanceLevel (take profit near resistance)
		// Short: use supportLevel (take profit near support)
		if takeProfitChecker != nil {
			tpLevel := resistanceLevel
			if side == "short" {
				tpLevel = supportLevel
			}
			// 便于排查分层止盈未激活：每周期打印当前盈亏与已触发档位
			scaledTaken := at.getScaledLevelsTaken(posKey)
			logger.Infof("📋 TP check %s %s: pnl=%.2f%%, entry=%.4f mark=%.4f, scaled_levels_taken=%d",
				symbol, side, pnlPct, entryPrice, markPrice, len(scaledTaken))
			signal := takeProfitChecker.CheckTakeProfit(&positionInfo, markPrice, atr, tpLevel)
			if signal.Triggered {
				kernel.LogTakeProfitCheck(symbol, signal)
				// Execute take profit
				if err := at.executeTakeProfit(&positionInfo, signal); err != nil {
					logger.Infof("❌ Failed to execute take profit for %s: %v", symbol, err)
				}
			}
		}
	}

	// Clean up slConfirmCount for positions that no longer exist
	at.slConfirmCountMu.Lock()
	for k := range at.slConfirmCount {
		if !currentPositionKeys[k] {
			delete(at.slConfirmCount, k)
		}
	}
	at.slConfirmCountMu.Unlock()

	return nil
}

// executeStopLoss executes stop loss for a position
func (at *AutoTrader) executeStopLoss(position *kernel.PositionInfo, signal *kernel.StopLossSignal) error {
	logger.Infof("🛑 Executing stop loss for %s %s: %s", position.Symbol, position.Side, signal.Reason)

	// Create close decision
	action := "close_long"
	if position.Side == "short" {
		action = "close_short"
	}

	decision := kernel.Decision{
		Symbol:    position.Symbol,
		Action:    action,
		Reasoning: fmt.Sprintf("Dynamic stop loss triggered: %s", signal.Reason),
	}

	// Execute the close order
	actionRecord := store.DecisionAction{
		Action:    action,
		Symbol:    position.Symbol,
		Reasoning: decision.Reasoning,
		Timestamp: time.Now().UTC(),
		Success:   false,
	}

	closeReason := "system:sl:" + signal.Type
	if signal.Type == "trailing" && signal.TrailingTier > 0 {
		closeReason = fmt.Sprintf("system:sl:trailing:L%d:%.2f:%.2f", signal.TrailingTier, signal.TrailingPeakPct, signal.TrailingRetracePct)
	}
	at.setPendingCloseReason(closeReason)

	// Pre-set closeReason on the OPEN position so OrderSync can pick it up
	if at.store != nil {
		normalizedSymbol := market.Normalize(position.Symbol)
		side := strings.ToUpper(position.Side)
		if err := at.store.Position().SetPendingCloseReasonBySymbol(at.id, normalizedSymbol, side, closeReason); err != nil {
			logger.Infof("  ⚠️ Failed to pre-set close reason: %v", err)
		}
	}

	if err := at.executeDecisionWithRecord(&decision, &actionRecord); err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	at.recordStrategyTriggeredClose(kernel.StrategyTriggeredClose{
		Symbol: position.Symbol, Side: position.Side, Reason: signal.Reason, Price: signal.Price,
	})
	logger.Infof("✓ Stop loss executed successfully for %s", position.Symbol)
	return nil
}

// executeTakeProfit executes take profit for a position (full or partial, same as backtest).
func (at *AutoTrader) executeTakeProfit(position *kernel.PositionInfo, signal *kernel.TakeProfitSignal) error {
	logger.Infof("💰 Executing take profit for %s %s: %s (close %.1f%%)", position.Symbol, position.Side, signal.Reason, signal.PartialPercent)

	action := "close_long"
	if position.Side == "short" {
		action = "close_short"
	}

	decision := kernel.Decision{
		Symbol:    position.Symbol,
		Action:    action,
		Reasoning: fmt.Sprintf("Dynamic take profit triggered: %s", signal.Reason),
	}
	if signal.PartialPercent > 0 && signal.PartialPercent < 100 {
		decision.CloseQuantity = position.Quantity * (signal.PartialPercent / 100)
		if decision.CloseQuantity <= 0 || decision.CloseQuantity > position.Quantity {
			decision.CloseQuantity = 0
		}
	}

	actionRecord := store.DecisionAction{
		Action:    action,
		Symbol:    position.Symbol,
		Reasoning: decision.Reasoning,
		Timestamp: time.Now().UTC(),
		Success:   false,
	}

	closeReason := "system:tp:" + signal.Type
	at.setPendingCloseReason(closeReason)

	// Pre-set closeReason on the OPEN position so OrderSync can pick it up (for full close only)
	// Partial closes don't change status to CLOSED, so this only matters for full close
	if at.store != nil && signal.PartialPercent >= 100 {
		normalizedSymbol := market.Normalize(position.Symbol)
		side := strings.ToUpper(position.Side)
		if err := at.store.Position().SetPendingCloseReasonBySymbol(at.id, normalizedSymbol, side, closeReason); err != nil {
			logger.Infof("  ⚠️ Failed to pre-set close reason: %v", err)
		}
	}

	if err := at.executeDecisionWithRecord(&decision, &actionRecord); err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	at.recordStrategyTriggeredClose(kernel.StrategyTriggeredClose{
		Symbol: position.Symbol, Side: position.Side, Reason: signal.Reason, Price: signal.Price,
	})
	posKey := position.Symbol + "_" + position.Side
	if signal.PartialPercent >= 100 {
		at.clearScaledLevelsTakenForPosition(posKey)
	} else if signal.Type == "scaled" && signal.PartialPercent > 0 {
		profitPct := 0.0
		if position.Side == "long" {
			profitPct = (signal.Price - position.EntryPrice) / position.EntryPrice * 100
		} else {
			profitPct = (position.EntryPrice - signal.Price) / position.EntryPrice * 100
		}
		at.addScaledLevelTaken(posKey, profitPct)
	}
	logger.Infof("✓ Take profit executed successfully for %s", position.Symbol)
	return nil
}

