package trader

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"nofx/experience"
	"nofx/kernel"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/monitor"
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
	"nofx/provider/binancedata"
	"nofx/provider/coinglass"
	"nofx/provider/coingecko"
	"nofx/provider/coinank"
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
	ScanInterval time.Duration // AI 决策周期（仅调用 AI 的间隔，如 10 分钟）
	// SystemInterval 系统周期（拉数据、方向池、系统开仓、止盈止损）；0=与 AI 周期一致（不分离）
	SystemInterval time.Duration
	// SLTPAnalysisInterval 持仓止盈止损专用 AI 分析周期；0=不启用，仅用主周期分析
	SLTPAnalysisInterval time.Duration

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

const maxCycleOutputHistory = 50 // 思维链形式保留最近 N 条周期输出

// systemCycleOutputEntry 单次系统周期输出记录（思维链一条）
type systemCycleOutputEntry struct {
	At           string `json:"at"`            // RFC3339
	CycleNumber  int    `json:"cycle_number"`  // 本会话第几次系统周期
	Summary      string `json:"summary"`       // 本周期输出摘要
}

// sltpCycleOutputEntry 单次止盈止损分析周期输出记录（思维链一条）
type sltpCycleOutputEntry struct {
	At           string                          `json:"at"`           // RFC3339
	ScheduledAt  string                          `json:"scheduled_at,omitempty"` // RFC3339（计划调度时刻，仅用于对照）
	CycleNumber  int                             `json:"cycle_number"` // 本会话第几次止盈止损周期
	Adjustments  []kernel.PositionSLTPAdjustment `json:"adjustments"`  // 本周期 AI 调节建议
	RawOutput    string                          `json:"raw_output,omitempty"` // 本周期 AI 原始输出（便于排查/复盘）
}

// sltpExitSignalState 维护每仓“结构化退场信号”的确认+迟滞状态（内存态，重启重置）
type sltpExitSignalState struct {
	PhaseLabel           string
	ExitBias             string
	StrengthEMA          float64
	ConfirmUp70          int // 连续 strength>=70 次数（用于 scale_out）
	ConfirmUp85          int // 连续 strength>=85 次数（用于 exit）
	ConfirmDown55        int // 连续 strength<=55 次数（用于降级/解除）
	LastActionLevel      int // 0=无 1=收紧 2=减仓 3=退出
	LastExecutedLevel    int // 已执行到的动作等级（避免 20s 检查重复触发下单）
	LastUpdatedAtUnixSec int64
}

func (at *AutoTrader) appendSystemCycleOutput(summary string) {
	at.systemCycleHistoryMu.Lock()
	defer at.systemCycleHistoryMu.Unlock()
	at.systemCycleHistory = append(at.systemCycleHistory, systemCycleOutputEntry{
		// 使用带时区偏移的 RFC3339，前端 new Date(...).toLocaleString() 会稳定显示为本机时间
		At:          time.Now().Format(time.RFC3339),
		CycleNumber: at.systemCycleCount,
		Summary:     summary,
	})
	if len(at.systemCycleHistory) > maxCycleOutputHistory {
		at.systemCycleHistory = at.systemCycleHistory[len(at.systemCycleHistory)-maxCycleOutputHistory:]
	}
}

// appendSLTPCycleOutput 记录止盈止损周期输出；界面展示时间使用实际执行时刻，scheduledAt 仅用于对照排查调度漂移。
func (at *AutoTrader) appendSLTPCycleOutput(adjustments []kernel.PositionSLTPAdjustment, rawOutput string, scheduledAt time.Time) {
	at.sltpCycleHistoryMu.Lock()
	defer at.sltpCycleHistoryMu.Unlock()
	// 深拷贝，避免后续被覆盖
	adjCopy := make([]kernel.PositionSLTPAdjustment, len(adjustments))
	copy(adjCopy, adjustments)
	at.sltpCycleHistory = append(at.sltpCycleHistory, sltpCycleOutputEntry{
		At:          time.Now().Format(time.RFC3339),
		ScheduledAt: scheduledAt.Format(time.RFC3339),
		CycleNumber: at.sltpAnalysisCycleCount,
		Adjustments: adjCopy,
		RawOutput:   rawOutput,
	})
	if len(at.sltpCycleHistory) > maxCycleOutputHistory {
		at.sltpCycleHistory = at.sltpCycleHistory[len(at.sltpCycleHistory)-maxCycleOutputHistory:]
	}
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
	callCount                 int  // AI call count (本会话 AI 决策周期数)
	systemCycleCount           int  // 本会话系统周期执行次数（仅用缓存+实时执行，不调 AI）
	sltpAnalysisCycleCount     int  // 本会话止盈止损分析周期执行次数（仅对持仓做 AI 分析，不写决策）
	positionFirstSeenTime      map[string]int64   // Position first seen time (symbol_side -> timestamp in milliseconds)
	stopMonitorCh         chan struct{}      // Used to stop monitoring goroutine
	monitorWg             sync.WaitGroup     // Used to wait for monitoring goroutine to finish
	peakPnLCache          map[string]float64   // Peak profit cache (symbol_side -> peak P&L %)
	peakPnLCacheMutex     sync.RWMutex        // Cache read-write lock
	slConfirmCount        map[string]int    // Consecutive cycles SL condition met (posKey -> count); execute only when >= ConfirmCycles
	slConfirmCountMu      sync.Mutex      // Protects slConfirmCount
	slFirstTriggeredAt   map[string]int64 // When ConfirmMinutes > 0, first time (ms) SL condition triggered per posKey
	slFirstTriggeredAtMu sync.Mutex      // Protects slFirstTriggeredAt
	tpTrailingFirstTriggeredAt   map[string]int64 // When TrailingTPConfirmMinutes > 0, first time trailing TP condition met
	tpTrailingFirstTriggeredAtMu sync.Mutex
	aiCloseConfirmCount   map[string]int      // Consecutive cycles AI requested close for a position (symbol_side -> count)
	aiCloseConfirmMu      sync.Mutex          // Protects aiCloseConfirmCount
	scaledLevelsTaken     map[string][]float64 // Scaled TP levels already taken (posKey -> profit percents)
	scaledLevelsTakenMu   sync.RWMutex        // For layered TP parity with backtest
	// Partial-close cooldown (per posKey + kind) to avoid duplicate partial closes in short window
	partialCloseCooldownMu sync.Mutex
	partialCloseCooldown   map[string]int64 // key=posKey+"|"+kind -> lastExecUnixMs
	// AI-selected per-position profiles (system-enforced, stored by posKey)
	positionProfileMu    sync.RWMutex
	positionRiskBucket   map[string]string // posKey -> low/medium/high
	positionTPProfile    map[string]string // posKey -> tp_profile
	positionSLProfile    map[string]string // posKey -> sl_profile
	positionTrailAgg     map[string]string // posKey -> low/medium/high
	positionKeyLevels    map[string][]string // posKey -> key_levels (compact)
	// 系统周期与 AI 周期分离时：上次 AI 结果缓存，供系统周期开仓使用
	lastAIDecisionCache *kernel.FullDecision
	lastAIDecisionMu    sync.RWMutex
	// 持仓期间 AI 建议的止盈/止损调节（震荡持仓、抓住趋势、锁住利润）；系统在边界内应用，不直接平仓
	positionSLTPAdjustment   map[string]kernel.PositionSLTPAdjustment // posKey -> adjustment
	positionSLTPAdjustmentMu sync.RWMutex
	// 结构化退场信号：确认+迟滞状态机（用于抗抖动，决定是否允许 scale_out/exit）
	sltpExitSignalState   map[string]*sltpExitSignalState
	sltpExitSignalStateMu sync.Mutex
	// 调节前系统策略基线（原始值），用于 API/前端展示「原始→调整后」
	positionSLTPBaseline   map[string]kernel.PositionSLTPBaseline // posKey -> baseline
	positionSLTPBaselineMu sync.RWMutex
	// 上次系统周期/止盈止损周期输出（兼容），供前端「查看输出内容」
	lastSystemCycleSummary   string
	lastSystemCycleSummaryMu sync.Mutex
	lastSLTPCycleOutput      []kernel.PositionSLTPAdjustment
	lastSLTPCycleOutputMu    sync.Mutex
	// 每周期输出历史，思维链形式：保留最近 N 条
	systemCycleHistory   []systemCycleOutputEntry
	systemCycleHistoryMu sync.Mutex
	sltpCycleHistory     []sltpCycleOutputEntry
	sltpCycleHistoryMu   sync.Mutex
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
	// 本周期市场摘要，供止盈止损专用分析补强 5 种场景（choppy_hold / high_vol_hold 等）
	lastMarketSummaryForSLTP   string
	lastMarketSummaryForSLTPMu sync.RWMutex
	// Coinglass WSS 客户端（可选）；启用时在 buildTradingContext 中懒加载并写入 ctx.CoinglassWSSSnapshot
	coinglassWSClient   *coinglass.WSClient
	coinglassWSClientMu sync.Mutex
}

var onceForceOrderWS sync.Once // 仅启动一次币安强平 WebSocket

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
		callCount:              0,
		systemCycleCount:       0,
		sltpAnalysisCycleCount: 0,
		isRunning:              false,
		positionFirstSeenTime: make(map[string]int64),
		stopMonitorCh:         make(chan struct{}),
		monitorWg:             sync.WaitGroup{},
		peakPnLCache:          make(map[string]float64),
		peakPnLCacheMutex:     sync.RWMutex{},
		slConfirmCount:        make(map[string]int),
		slFirstTriggeredAt:         make(map[string]int64),
		tpTrailingFirstTriggeredAt: make(map[string]int64),
		aiCloseConfirmCount:   make(map[string]int),
		scaledLevelsTaken:     make(map[string][]float64),
		scaledLevelsTakenMu:   sync.RWMutex{},
		partialCloseCooldown:  make(map[string]int64),
		positionRiskBucket:    make(map[string]string),
		positionTPProfile:     make(map[string]string),
		positionSLProfile:     make(map[string]string),
		positionTrailAgg:        make(map[string]string),
		positionKeyLevels:      make(map[string][]string),
		positionSLTPAdjustment: make(map[string]kernel.PositionSLTPAdjustment),
		sltpExitSignalState:    make(map[string]*sltpExitSignalState),
		positionSLTPBaseline:   make(map[string]kernel.PositionSLTPBaseline),
		positionParamsMap:      make(map[string]*positionParams),
		lastBalanceSyncTime:   time.Now(),
		userID:                userID,
	}, nil
}

const partialCloseCooldownMs = int64(45 * 1000) // 45s cooldown for partial closes (signal scale_out / layered TP)

func (at *AutoTrader) shouldCooldownPartialClose(posKey, kind string) bool {
	if posKey == "" || kind == "" {
		return false
	}
	key := posKey + "|" + kind
	now := time.Now().UnixMilli()
	at.partialCloseCooldownMu.Lock()
	defer at.partialCloseCooldownMu.Unlock()
	if last, ok := at.partialCloseCooldown[key]; ok && now-last < partialCloseCooldownMs {
		return true
	}
	at.partialCloseCooldown[key] = now
	return false
}

// Run runs the automatic trading main loop.
// 约定：实盘与实盘模拟共用同一套 AI 周期、提示词与止盈止损逻辑，任何功能修改需同时适用于两者；仅执行层（真实下单 vs 纸面）不同。
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
	// AI 与系统周期分离：SystemInterval>0 且 <ScanInterval 时，系统周期（数据/方向池/开仓/SL-TP）更频繁，AI 仍按 ScanInterval 调用以省 token
	useDualInterval := at.config.SystemInterval > 0 && at.config.SystemInterval < at.config.ScanInterval
	if useDualInterval {
		logger.Infof("⚙️  AI interval: %v (token 节省) | System interval: %v (数据/方向池/开仓/止盈止损)", at.config.ScanInterval, at.config.SystemInterval)
	} else {
		logger.Infof("⚙️  Scan interval: %v (AI cycle runs every this duration only; not tied to K-line)", at.config.ScanInterval)
	}
	logger.Info("🚀 AI-driven automatic trading system started")
	logger.Infof("💰 Initial balance: %.2f USDT", at.initialBalance)
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

	lastCycleStart := time.Now()
	lastSystemTick := time.Now()
	lastSLTPTick := time.Now()
	useSLTPAnalysis := at.config.SLTPAnalysisInterval > 0 && at.config.SLTPAnalysisInterval < at.config.ScanInterval
	if useSLTPAnalysis {
		logger.Infof("🛡️ [%s] SL/TP-only analysis interval: %v (positions only, real-time + prediction for adjustments)", at.name, at.config.SLTPAnalysisInterval)
	}

	// 执行顺序与开仓逻辑（三条件共振）：
	// ① 过滤机制：系统拉数据、跑 pipeline，根据实时数据判定币种进入方向池（多/空），得到 ToSubmitSymbols、DirectionLong/Short。
	// ② 实时数据传给 AI：AI 周期调用大模型，输入含实时数据（方向池、行情等），产出 symbol_predictions（predicted_direction、suggest_open 等）。
	// ③ 系统再根据实时数据判定方向，并校验三条件共振：实时方向（该币在对应方向池）+ AI 预测方向（up/down 一致）+ AI 建议开仓（suggest_open=true），满足才开仓，缺一不可。
	// 无 AI 预测缓存时系统不执行开仓；止盈/止损一旦触发必须执行（与是否有 AI 缓存无关）。
	// Execute first AI cycle immediately (so we have cached prediction for system-only cycles)
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

		now := time.Now()
		nextAI := lastCycleStart.Add(at.config.ScanInterval)
		var nextSystem time.Time
		if useDualInterval {
			nextSystem = lastSystemTick.Add(at.config.SystemInterval)
		} else {
			nextSystem = nextAI.Add(time.Hour) // 不分离时只按 AI 周期
		}
		var nextSLTP time.Time
		if useSLTPAnalysis {
			nextSLTP = lastSLTPTick.Add(at.config.SLTPAnalysisInterval)
		} else {
			nextSLTP = nextAI.Add(time.Hour)
		}

		// 等到下一个系统、SL/TP 分析或 AI 时刻
		sleepUntil := nextAI
		if useDualInterval && nextSystem.Before(sleepUntil) {
			sleepUntil = nextSystem
		}
		if useSLTPAnalysis && nextSLTP.Before(sleepUntil) {
			sleepUntil = nextSLTP
		}
		if now.Before(sleepUntil) {
			sleepDur := sleepUntil.Sub(now)
			logger.Infof("[%s] ⏳ Next: system=%s sltp=%s AI=%s (sleep %.1fs)", at.name,
				nextSystem.Format("15:04:05"), nextSLTP.Format("15:04:05"), nextAI.Format("15:04:05"), sleepDur.Seconds())
			select {
			case <-time.After(sleepDur):
			case <-at.stopMonitorCh:
				logger.Infof("[%s] ⏹ Stop signal received, exiting automatic trading main loop", at.name)
				return nil
			}
			now = time.Now()
		}

		// 系统周期：仅用「缓存的 AI 预测 + 实时」执行（③）；不调 AI。无缓存则不开仓，止盈/止损照常执行。
		if useDualInterval && !now.Before(nextSystem) {
			if err := at.runSystemCycle(); err != nil {
				logger.Infof("❌ System cycle failed: %v", err)
			}
			lastSystemTick = nextSystem
		}

		// 持仓止盈止损专用分析周期：仅对持仓做「实时+预测」分析，输出 position_sl_tp_adjustments，不写决策、不开平仓
		// 思维链时间戳用 nextSLTP（本周期调度时刻），保证界面展示与设定间隔（如 3 分钟）一致
		if useSLTPAnalysis && !now.Before(nextSLTP) {
			if err := at.runSLTPAnalysisCycle(nextSLTP); err != nil {
				logger.Infof("❌ SL/TP analysis cycle failed: %v", err)
			}
			lastSLTPTick = nextSLTP
		}

		// AI 周期：① 获取数据 ② AI 预测（并缓存）③ 用「AI 预测 + 实时」执行开仓；止盈/止损照常执行。
		// 本周期时间戳用 nextAI（本次调度时刻），保证界面展示为 10 分钟间隔且不重复、不少周期
		if !now.Before(nextAI) {
			cycleStart := nextAI
			lastCycleStart = nextAI
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

// runCycle 运行一个 AI 决策周期：先获取数据 → 先 AI 预测 → 再根据「AI 预测 + 实时」执行开仓；止盈/止损照常检查并执行。
// cycleStart 为本周期调度时刻（计划开始时间）；界面展示时间使用实际执行时刻，避免卡顿后“看起来仍按固定间隔推进”。
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
	logger.Infof("[%s] 🔄 runCycle #%d start (cycleStart=%s, config_interval=%v)", at.name, at.callCount, cycleStart.Format("15:04:05"), at.config.ScanInterval)

	// Create decision record（时间戳用实际执行时刻；计划时刻仅用于调度与日志）
	record := &store.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
		// 保留时区偏移（JSON 会携带 +08:00），前端显示与本机时间一致
		Timestamp:    time.Now(),
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

	// 3. Collect trading context first (needed to decide whether we have candidates and to run SL/TP after AI when we do)
	ctx, err := at.buildTradingContext("主周期")
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("Failed to build trading context: %v", err)
		at.saveDecision(record)
		return fmt.Errorf("failed to build trading context: %w", err)
	}
	ctx.TraderID = at.id
	// 缓存本周期市场摘要，供止盈止损专用分析（5 种场景）补强
	at.lastMarketSummaryForSLTPMu.Lock()
	at.lastMarketSummaryForSLTP = kernel.BuildCurrentCycleSummaryForSLTP(ctx)
	at.lastMarketSummaryForSLTPMu.Unlock()

	// Save equity snapshot independently (decoupled from AI decision, used for drawing profit curve)
	// NOTE: Must be called BEFORE candidate coins check to ensure equity is always recorded
	at.saveEquitySnapshot(ctx)

	// 无候选币时仍调用 AI（首周期或 oi_top/ai500 未就绪时），以便缓存 regime/scenario 供系统周期使用，仅跳过开仓执行
	noCandidates := len(ctx.CandidateCoins) == 0
	if noCandidates {
		logger.Infof("ℹ️  No candidate coins this cycle (e.g. first run or data not ready); will still call AI for cache, then skip entry execution")
	}
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	logger.Infof("📊 Account equity: %.2f USDT | Available: %.2f USDT | Positions: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 5. Use strategy engine to call AI for decision
	logger.Infof("🤖 Requesting AI analysis and decision... [Strategy Engine]")
	// 多空雷达配置：允许做多/做空参与 pipeline 过滤（从 DB 读取，与多空雷达页一致）
	var pipelineOpts *kernel.PipelineOptions
	if at.store != nil {
		if raw, _ := at.store.Trader().GetRadarConfigBytes(at.id); len(raw) > 0 {
			var radar struct {
				AllowLong  bool `json:"allow_long"`
				AllowShort bool `json:"allow_short"`
			}
			if json.Unmarshal(raw, &radar) == nil {
				pipelineOpts = &kernel.PipelineOptions{AllowLong: radar.AllowLong, AllowShort: radar.AllowShort}
			}
		}
	}
	if pipelineOpts == nil {
		pipelineOpts = &kernel.PipelineOptions{AllowLong: true, AllowShort: true}
	}
	aiDecision, err := kernel.GetFullDecisionWithStrategy(ctx, at.mcpClient, at.strategyEngine, "balanced", "", at.id, pipelineOpts)

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

	// 缓存本次 AI 结果，供「仅系统周期」开仓使用（系统周期不调 AI，用上次预测 + 实时 pipeline）
	at.lastAIDecisionMu.Lock()
	at.lastAIDecisionCache = aiDecision
	at.lastAIDecisionMu.Unlock()
	// 持仓止盈/止损 AI 调节：若已启用「止盈止损专用周期」（间隔以前端配置为准），则由该周期专责写入调节，主周期不再应用此处输出，避免重复
	useSLTPOnlyCycle := at.config.SLTPAnalysisInterval > 0 && at.config.SLTPAnalysisInterval < at.config.ScanInterval
	if !useSLTPOnlyCycle && len(aiDecision.PositionSLTPAdjustments) > 0 {
		at.positionSLTPAdjustmentMu.Lock()
		for _, adj := range aiDecision.PositionSLTPAdjustments {
			posKey := market.Normalize(adj.Symbol) + "_" + strings.ToLower(adj.Side)
			at.positionSLTPAdjustment[posKey] = adj
		}
		at.positionSLTPAdjustmentMu.Unlock()
	}

	// 5b. Build AI trend_view per position + global scenario for SL/TP modulation
	trendViewByPosKey := make(map[string]string)
	for _, pos := range ctx.Positions {
		posKey := market.Normalize(pos.Symbol) + "_" + strings.ToLower(pos.Side)
		for _, d := range aiDecision.Decisions {
			if d.Action != "hold" || market.Normalize(d.Symbol) != market.Normalize(pos.Symbol) {
				continue
			}
			tv := strings.TrimSpace(strings.ToLower(d.TrendView))
			if tv == "trend_intact" || tv == "choppy" || tv == "reversing" {
				trendViewByPosKey[posKey] = tv
			}
			break
		}
	}
	globalScenario := strings.TrimSpace(strings.ToLower(aiDecision.Scenario))
	if err := at.checkDynamicStopLossTakeProfit(trendViewByPosKey, globalScenario); err != nil {
		logger.Infof("⚠️  Failed to check dynamic stop loss/take profit: %v", err)
	}

	// 无候选币时：已调用 AI 并缓存，止盈止损已检查，仅跳过开仓/平仓执行，直接保存决策并返回
	if noCandidates {
		record.Success = true
		record.ExecutionLog = append(record.ExecutionLog, "No candidate coins; AI called for cache (regime/scenario), entry execution skipped")
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

		// AI 仅预测模式：不执行 AI 的任何开平仓动作，由系统根据预测 + pipeline 决策
		if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil && at.strategyEngine.GetConfig().RiskControl.AIPredictOnly {
			if d.Action == "open_long" || d.Action == "open_short" || d.Action == "close_long" || d.Action == "close_short" {
				msg := fmt.Sprintf("AI %s %s skipped (ai_predict_only=true, system decides from prediction)", d.Symbol, d.Action)
				logger.Infof("⏭ %s", msg)
				record.ExecutionLog = append(record.ExecutionLog, msg)
				record.Decisions = append(record.Decisions, actionRecord)
				continue
			}
		}

		// 系统执行开仓模式：不执行 AI 的开仓建议，开仓由系统根据 pipeline 执行
		systemExecutesEntry := false
		if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
			systemExecutesEntry = at.strategyEngine.GetConfig().RiskControl.SystemExecutesEntry
		}
		if systemExecutesEntry && (d.Action == "open_long" || d.Action == "open_short") {
			msg := fmt.Sprintf("AI open %s %s skipped (system_executes_entry=true, system will open from pipeline)", d.Symbol, d.Action)
			logger.Infof("⏭ %s", msg)
			record.ExecutionLog = append(record.ExecutionLog, msg)
			record.Decisions = append(record.Decisions, actionRecord)
			continue
		}

		// AI 平仓权限：当 AIOnlyEntry=true 或 AllowAIClose=false 时，不执行 AI 的平仓建议，由策略动态 SL/TP 负责平仓
		if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
			rc := at.strategyEngine.GetConfig().RiskControl
			if rc.AIOnlyEntry || !rc.AllowAIClose {
				if d.Action == "close_long" || d.Action == "close_short" {
					msg := fmt.Sprintf("AI close %s %s skipped (ai_only_entry=%v, allow_ai_close=%v, strategy handles exit)", d.Symbol, d.Action, rc.AIOnlyEntry, rc.AllowAIClose)
					logger.Infof("⏭ %s", msg)
					record.ExecutionLog = append(record.ExecutionLog, msg)
					record.Decisions = append(record.Decisions, actionRecord)
					continue
				}
			}
			// AI 平仓约束：仅当置信度≥门槛且（若要求）exit_reason 为 take_profit|stop_loss|prediction_mismatch 之一时才执行
			if d.Action == "close_long" || d.Action == "close_short" {
				if rc.MinConfidenceForAIClose > 0 && d.Confidence < rc.MinConfidenceForAIClose {
					msg := fmt.Sprintf("AI close %s %s skipped (confidence %d < min_for_ai_close %d)", d.Symbol, d.Action, d.Confidence, rc.MinConfidenceForAIClose)
					logger.Infof("⏭ %s", msg)
					record.ExecutionLog = append(record.ExecutionLog, msg)
					record.Decisions = append(record.Decisions, actionRecord)
					continue
				}
				if rc.RequireExitReasonForAIClose {
					reason := strings.TrimSpace(strings.ToLower(d.ExitReason))
					switch reason {
					case "take_profit", "stop_loss", "prediction_mismatch":
						// 通过
					default:
						msg := fmt.Sprintf("AI close %s %s skipped (exit_reason=%q required: take_profit|stop_loss|prediction_mismatch)", d.Symbol, d.Action, d.ExitReason)
						logger.Infof("⏭ %s", msg)
						record.ExecutionLog = append(record.ExecutionLog, msg)
						record.Decisions = append(record.Decisions, actionRecord)
						continue
					}
				}
			}
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

		// 额外补强：按 regime 与极端资金费率/多空比得到有效最低置信度及是否禁止开多/开空
		effectiveMinConf := 0
		if d.Action == "open_long" || d.Action == "open_short" {
			cfg := at.strategyEngine.GetConfig()
			if cfg != nil {
				rc := &cfg.RiskControl
				baseMinConf := rc.MinConfidence
				if baseMinConf <= 0 {
					baseMinConf = 70
				}
				regimeEnabled := rc.RegimeAdjustEnabled != nil && *rc.RegimeAdjustEnabled
				regimeMap := rc.RegimeMinConfidenceMap
				var extreme *store.ExtremeFundingRule
				if rc.ExtremeFundingRule != nil && rc.ExtremeFundingRule.Enabled {
					extreme = rc.ExtremeFundingRule
				}
				eff, blockLong, blockShort := kernel.EffectiveOpenConstraints(ctx, d.Symbol, aiDecision.MarketRegime, baseMinConf, regimeEnabled, regimeMap, extreme)
				effectiveMinConf = eff
				if d.Action == "open_long" && blockLong {
					msg := fmt.Sprintf("AI open_long %s skipped (extreme funding/LS rule: block open long)", d.Symbol)
					logger.Infof("⏭ %s", msg)
					record.ExecutionLog = append(record.ExecutionLog, msg)
					record.Decisions = append(record.Decisions, actionRecord)
					continue
				}
				if d.Action == "open_short" && blockShort {
					msg := fmt.Sprintf("AI open_short %s skipped (extreme funding/LS rule: block open short)", d.Symbol)
					logger.Infof("⏭ %s", msg)
					record.ExecutionLog = append(record.ExecutionLog, msg)
					record.Decisions = append(record.Decisions, actionRecord)
					continue
				}
			}
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord, effectiveMinConf); err != nil {
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

	// 系统执行开仓：根据 pipeline 状态由系统执行开仓（SystemExecutesEntry 或 AIPredictOnly 时：不执行 AI 的 open，在此处按方向池 + 可选 AI 预测过滤 执行）
	var runSystemEntry bool
	if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
		rc := at.strategyEngine.GetConfig().RiskControl
		runSystemEntry = rc.SystemExecutesEntry || rc.AIPredictOnly
	}
	if runSystemEntry {
		if aiDecision != nil && aiDecision.RiskAlert {
			record.ExecutionLog = append(record.ExecutionLog, "⏭ AI risk_alert=true，本周期不执行系统开仓")
			logger.Infof("⏭ [%s] AI risk_alert=true, skipping system opens this cycle", at.name)
		} else {
		state := kernel.GetPipelineState(at.id)
		if state != nil {
			var systemEntries []kernel.SystemEntry
			cfg := at.strategyEngine.GetConfig()
			// 不用 AI 缓存开仓，仅采用同一周期的方向池 + AI 分析（预测+建议开仓）；本周期缺条件则不开仓（GetSystemEntryListFromPrediction 仅返回三条件共振的标的）
			if cfg.RiskControl.AIPredictOnly && aiDecision != nil && len(aiDecision.SymbolPredictions) > 0 {
				baseMinConf := cfg.RiskControl.MinConfidence
				if baseMinConf <= 0 {
					baseMinConf = 70
				}
				// Regime 与开仓置信度严格绑定：震荡/高波/反转时使用提高后的有效最低置信度，避免部分入口绕过
				marketRegimeForList := ""
				if aiDecision != nil {
					marketRegimeForList = aiDecision.MarketRegime
				}
				regimeEnabled := cfg.RiskControl.RegimeAdjustEnabled != nil && *cfg.RiskControl.RegimeAdjustEnabled
				regimeMap := cfg.RiskControl.RegimeMinConfidenceMap
				effMinConf, _, _ := kernel.EffectiveOpenConstraints(ctx, "", marketRegimeForList, baseMinConf, regimeEnabled, regimeMap, nil)
				minStrengthToOpen := 0.0
				if cfg.MultilayerFilter != nil && cfg.MultilayerFilter.DirectionPool != nil && cfg.MultilayerFilter.DirectionPool.MinStrengthPctToOpen > 0 {
					minStrengthToOpen = cfg.MultilayerFilter.DirectionPool.MinStrengthPctToOpen
				}
				systemEntries = kernel.GetSystemEntryListFromPrediction(state, pipelineOpts, aiDecision.SymbolPredictions, effMinConf, minStrengthToOpen)
				logger.Infof("📋 [预测定多空] 三条件共振生成系统开仓列表，共 %d 条（实时方向+AI方向+AI建议开仓）", len(systemEntries))
			} else {
				systemEntries = kernel.GetSystemEntryList(state, pipelineOpts)
			}
			equity := ctx.Account.TotalEquity
			if equity <= 0 {
				equity = ctx.Account.AvailableBalance
			}
			// 额外补强：按 regime 与极端资金费率/多空比得到有效最低置信度及是否禁止开多/开空（与 AI 开仓共用逻辑）
			regimeEnabled := cfg.RiskControl.RegimeAdjustEnabled != nil && *cfg.RiskControl.RegimeAdjustEnabled
			regimeMap := cfg.RiskControl.RegimeMinConfidenceMap
			baseMinConf := cfg.RiskControl.MinConfidence
			if baseMinConf <= 0 {
				baseMinConf = 70
			}
			var extreme *store.ExtremeFundingRule
			if cfg.RiskControl.ExtremeFundingRule != nil && cfg.RiskControl.ExtremeFundingRule.Enabled {
				extreme = cfg.RiskControl.ExtremeFundingRule
			}
			marketRegime := ""
			if aiDecision != nil {
				marketRegime = aiDecision.MarketRegime
			}

			for _, ent := range systemEntries {
				at.isRunningMutex.RLock()
				running = at.isRunning
				at.isRunningMutex.RUnlock()
				if !running {
					break
				}
				// 实时数据定是否执行：regime/极端资金费率/多空比、OI 对齐等
				eff, blockLong, blockShort := kernel.EffectiveOpenConstraints(ctx, ent.Symbol, marketRegime, baseMinConf, regimeEnabled, regimeMap, extreme)
				if ent.Action == "open_long" && blockLong {
					logger.Infof("⏭ System open_long %s skipped (extreme funding/LS rule: block open long)", ent.Symbol)
					record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("⏭ system open_long %s skipped (extreme rule)", ent.Symbol))
					continue
				}
				if ent.Action == "open_short" && blockShort {
					logger.Infof("⏭ System open_short %s skipped (extreme funding/LS rule: block open short)", ent.Symbol)
					record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("⏭ system open_short %s skipped (extreme rule)", ent.Symbol))
					continue
				}
				effectiveMinConfSys := eff
				if effectiveMinConfSys <= 0 {
					effectiveMinConfSys = baseMinConf
				}
				// 用策略默认参数构建系统开仓决策
				positionSizeUSD := equity * cfg.RiskControl.AltcoinMaxPositionValueRatio
				if market.Normalize(ent.Symbol) == "BTCUSDT" || market.Normalize(ent.Symbol) == "ETHUSDT" {
					positionSizeUSD = equity * cfg.RiskControl.BTCETHMaxPositionValueRatio
				}
				if positionSizeUSD < cfg.RiskControl.MinPositionSize {
					positionSizeUSD = cfg.RiskControl.MinPositionSize
				}
				lev := cfg.RiskControl.AltcoinMaxLeverage
				if market.Normalize(ent.Symbol) == "BTCUSDT" || market.Normalize(ent.Symbol) == "ETHUSDT" {
					lev = cfg.RiskControl.BTCETHMaxLeverage
				}
				if lev <= 0 {
					lev = 2
				}
				var sl, tp float64
				if data, ok := ctx.MarketDataMap[ent.Symbol]; ok && data.CurrentPrice > 0 {
					price := data.CurrentPrice
					if cfg.RiskControl.DynamicStopLoss != nil && cfg.RiskControl.DynamicStopLoss.InitialStopPercent > 0 {
						pct := cfg.RiskControl.DynamicStopLoss.InitialStopPercent / 100
						if ent.Action == "open_long" {
							sl = price * (1 - pct)
						} else {
							sl = price * (1 + pct)
						}
					}
					if cfg.RiskControl.DynamicTakeProfit != nil && len(cfg.RiskControl.DynamicTakeProfit.ScaledLevels) > 0 {
						firstPct := cfg.RiskControl.DynamicTakeProfit.ScaledLevels[0].ProfitPercent / 100
						if ent.Action == "open_long" {
							tp = price * (1 + firstPct)
						} else {
							tp = price * (1 - firstPct)
						}
					}
				}
				// SL 回退：InitialStopPercent=0 时，用「第一档止盈÷最小盈亏比」使默认组合满足 3:1，避免长期 0.5:1 无法开仓
				if sl <= 0 && ctx.MarketDataMap[ent.Symbol] != nil && ctx.MarketDataMap[ent.Symbol].CurrentPrice > 0 {
					price := ctx.MarketDataMap[ent.Symbol].CurrentPrice
					tpPct := 0.06
					if cfg.RiskControl.DynamicTakeProfit != nil && len(cfg.RiskControl.DynamicTakeProfit.ScaledLevels) > 0 {
						tpPct = cfg.RiskControl.DynamicTakeProfit.ScaledLevels[0].ProfitPercent / 100
					}
					minRR := cfg.RiskControl.MinRiskRewardRatio
					if minRR <= 0 {
						minRR = 3.0
					}
					slPct := tpPct / minRR
					if ent.Action == "open_long" {
						sl = price * (1 - slPct)
					} else {
						sl = price * (1 + slPct)
					}
				}
				if tp <= 0 && ctx.MarketDataMap[ent.Symbol] != nil && ctx.MarketDataMap[ent.Symbol].CurrentPrice > 0 {
					price := ctx.MarketDataMap[ent.Symbol].CurrentPrice
					pct := 0.06
					if ent.Action == "open_long" {
						tp = price * (1 + pct)
					} else {
						tp = price * (1 - pct)
					}
				}
				sysDecision := kernel.Decision{
					Symbol:         ent.Symbol,
					Action:         ent.Action,
					Leverage:       lev,
					PositionSizeUSD: positionSizeUSD,
					StopLoss:       sl,
					TakeProfit:     tp,
					Confidence:     effectiveMinConfSys,
					Reasoning:      "system entry from pipeline (AI only assists)",
				}
				if sysDecision.Confidence <= 0 {
					sysDecision.Confidence = 70
				}
				actionRecord := store.DecisionAction{
					Action:     ent.Action,
					Symbol:     ent.Symbol,
					Leverage:   lev,
					StopLoss:   sl,
					TakeProfit: tp,
					Confidence: sysDecision.Confidence,
					Reasoning:  sysDecision.Reasoning,
					Timestamp:  time.Now().UTC(),
					Success:    false,
				}
				var execErr error
				if ent.Action == "open_long" {
					execErr = at.executeOpenLongWithRecord(&sysDecision, &actionRecord, effectiveMinConfSys)
				} else {
					execErr = at.executeOpenShortWithRecord(&sysDecision, &actionRecord, effectiveMinConfSys)
				}
				if execErr != nil {
					logger.Infof("❌ System open %s %s failed: %v", ent.Symbol, ent.Action, execErr)
					actionRecord.Error = execErr.Error()
					record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ system open %s %s failed: %v", ent.Symbol, ent.Action, execErr))
				} else if actionRecord.Error != "" {
					record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("⏭ system open %s %s skipped: %s", ent.Symbol, ent.Action, actionRecord.Error))
				} else {
					actionRecord.Success = true
					record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ system open %s %s succeeded", ent.Symbol, ent.Action))
					time.Sleep(1 * time.Second)
				}
				record.Decisions = append(record.Decisions, actionRecord)
			}
		}
		}
	}

	// 9. Save decision record
	if err := at.saveDecision(record); err != nil {
		logger.Infof("⚠ Failed to save decision record: %v", err)
	}

	return nil
}

// runSystemCycle 仅系统周期：拉数据、跑 pipeline，更新方向池（供雷达展示）；检查止盈止损（触发则必须执行）。
// 不调 AI，不写决策记录。不开仓（不用 AI 缓存）；开仓仅用同一周期的方向池+AI 分析，在 runCycle（AI 周期）内执行，本周期缺条件则不开仓。
func (at *AutoTrader) runSystemCycle() error {
	at.isRunningMutex.RLock()
	running := at.isRunning
	at.isRunningMutex.RUnlock()
	if !running {
		return nil
	}
	at.systemCycleCount++
	oldClosesLen := at.strategyTriggeredClosesLen()
	var deferredOpens []string
	var deferredEntryCount int
	var needDeferredAppend bool

	ctx, err := at.buildTradingContext("系统周期")
	if err != nil {
		return err
	}
	ctx.TraderID = at.id
	at.lastMarketSummaryForSLTPMu.Lock()
	at.lastMarketSummaryForSLTP = kernel.BuildCurrentCycleSummaryForSLTP(ctx)
	at.lastMarketSummaryForSLTPMu.Unlock()
	at.saveEquitySnapshot(ctx)
	candidateCount := len(ctx.CandidateCoins)
	if candidateCount == 0 {
		_ = at.checkDynamicStopLossTakeProfit(nil, "")
		s := "候选币数: 0；已检查止盈止损。"
		at.lastSystemCycleSummaryMu.Lock()
		at.lastSystemCycleSummary = s
		at.lastSystemCycleSummaryMu.Unlock()
		at.appendSystemCycleOutput(s)
		return nil
	}
	var pipelineOpts *kernel.PipelineOptions
	if at.store != nil {
		if raw, _ := at.store.Trader().GetRadarConfigBytes(at.id); len(raw) > 0 {
			var radar struct {
				AllowLong  bool `json:"allow_long"`
				AllowShort bool `json:"allow_short"`
			}
			if json.Unmarshal(raw, &radar) == nil {
				pipelineOpts = &kernel.PipelineOptions{AllowLong: radar.AllowLong, AllowShort: radar.AllowShort}
			}
		}
	}
	if pipelineOpts == nil {
		pipelineOpts = &kernel.PipelineOptions{AllowLong: true, AllowShort: true}
	}
	if err := kernel.PrepareContextForPipeline(ctx, at.strategyEngine, at.id, pipelineOpts); err != nil {
		logger.Infof("⚠️ [%s] System cycle PrepareContextForPipeline: %v", at.name, err)
		_ = at.checkDynamicStopLossTakeProfit(nil, "")
		s := fmt.Sprintf("候选币数: %d；PrepareContextForPipeline 失败；已检查止盈止损。", candidateCount)
		at.lastSystemCycleSummaryMu.Lock()
		at.lastSystemCycleSummary = s
		at.lastSystemCycleSummaryMu.Unlock()
		at.appendSystemCycleOutput(s)
		return nil
	}
	cfg := at.strategyEngine.GetConfig()
	if cfg == nil {
		_ = at.checkDynamicStopLossTakeProfit(nil, "")
		s := fmt.Sprintf("候选币数: %d；策略配置为空；已检查止盈止损。", candidateCount)
		at.lastSystemCycleSummaryMu.Lock()
		at.lastSystemCycleSummary = s
		at.lastSystemCycleSummaryMu.Unlock()
		at.appendSystemCycleOutput(s)
		return nil
	}
	rc := cfg.RiskControl
	// 三条件共振开仓仅在 AI 周期执行，保证同一周期内「系统获取数据→传 AI→等 AI 给出建议与预测→系统实时分析」对应，不混用本周期方向池与上周期 AI
	runSystemEntry := rc.SystemExecutesEntry
	if rc.AIPredictOnly {
		// 「AI 预测 + 实时」模式：系统周期只更新方向池与止盈止损，不开仓；开仓仅在 runCycle（AI 周期）内用同周期的 pipeline + AI 结果执行
		runSystemEntry = false
		logger.Infof("⏭ [%s] System cycle: direction pool updated for display; opens only in AI cycle (same-cycle system+AI). SL/TP still run.", at.name)
	}
	if runSystemEntry {
		at.lastAIDecisionMu.RLock()
		cached := at.lastAIDecisionCache
		at.lastAIDecisionMu.RUnlock()
		if cached != nil && cached.RiskAlert {
			logger.Infof("⏭ [%s] System cycle: skip opens (cached AI risk_alert=true)", at.name)
			s := fmt.Sprintf("候选币数: %d；AI 风险预警，跳过开仓；已检查止盈止损。", candidateCount)
			at.lastSystemCycleSummaryMu.Lock()
			at.lastSystemCycleSummary = s
			at.lastSystemCycleSummaryMu.Unlock()
			at.appendSystemCycleOutput(s)
		} else {
			state := kernel.GetPipelineState(at.id)
			if state != nil {
				systemEntries := kernel.GetSystemEntryList(state, pipelineOpts)
				marketRegime := ""
				if cached != nil {
					marketRegime = cached.MarketRegime
				}
				equity := ctx.Account.TotalEquity
				if equity <= 0 {
					equity = ctx.Account.AvailableBalance
				}
				regimeEnabled := rc.RegimeAdjustEnabled != nil && *rc.RegimeAdjustEnabled
				regimeMap := rc.RegimeMinConfidenceMap
				baseMinConf := rc.MinConfidence
				if baseMinConf <= 0 {
					baseMinConf = 70
				}
				var extreme *store.ExtremeFundingRule
				if rc.ExtremeFundingRule != nil && rc.ExtremeFundingRule.Enabled {
					extreme = rc.ExtremeFundingRule
				}
				entryCount := len(systemEntries)
				for _, ent := range systemEntries {
					at.isRunningMutex.RLock()
					running = at.isRunning
					at.isRunningMutex.RUnlock()
					if !running {
						break
					}
					eff, blockLong, blockShort := kernel.EffectiveOpenConstraints(ctx, ent.Symbol, marketRegime, baseMinConf, regimeEnabled, regimeMap, extreme)
					if (ent.Action == "open_long" && blockLong) || (ent.Action == "open_short" && blockShort) {
						continue
					}
					effectiveMinConfSys := eff
					if effectiveMinConfSys <= 0 {
						effectiveMinConfSys = baseMinConf
					}
					positionSizeUSD := equity * rc.AltcoinMaxPositionValueRatio
					if market.Normalize(ent.Symbol) == "BTCUSDT" || market.Normalize(ent.Symbol) == "ETHUSDT" {
						positionSizeUSD = equity * rc.BTCETHMaxPositionValueRatio
					}
					if positionSizeUSD < rc.MinPositionSize {
						positionSizeUSD = rc.MinPositionSize
					}
					lev := rc.AltcoinMaxLeverage
					if market.Normalize(ent.Symbol) == "BTCUSDT" || market.Normalize(ent.Symbol) == "ETHUSDT" {
						lev = rc.BTCETHMaxLeverage
					}
					if lev <= 0 {
						lev = 2
					}
					var sl, tp float64
					if data, ok := ctx.MarketDataMap[ent.Symbol]; ok && data.CurrentPrice > 0 {
						price := data.CurrentPrice
						if rc.DynamicStopLoss != nil && rc.DynamicStopLoss.InitialStopPercent > 0 {
							pct := rc.DynamicStopLoss.InitialStopPercent / 100
							if ent.Action == "open_long" {
								sl = price * (1 - pct)
							} else {
								sl = price * (1 + pct)
							}
						}
						if rc.DynamicTakeProfit != nil && len(rc.DynamicTakeProfit.ScaledLevels) > 0 {
							firstPct := rc.DynamicTakeProfit.ScaledLevels[0].ProfitPercent / 100
							if ent.Action == "open_long" {
								tp = price * (1 + firstPct)
							} else {
								tp = price * (1 - firstPct)
							}
						}
					}
					if sl <= 0 && ctx.MarketDataMap[ent.Symbol] != nil && ctx.MarketDataMap[ent.Symbol].CurrentPrice > 0 {
						price := ctx.MarketDataMap[ent.Symbol].CurrentPrice
						tpPct := 0.06
						if rc.DynamicTakeProfit != nil && len(rc.DynamicTakeProfit.ScaledLevels) > 0 {
							tpPct = rc.DynamicTakeProfit.ScaledLevels[0].ProfitPercent / 100
						}
						minRR := rc.MinRiskRewardRatio
						if minRR <= 0 {
							minRR = 3.0
						}
						slPct := tpPct / minRR
						if ent.Action == "open_long" {
							sl = price * (1 - slPct)
						} else {
							sl = price * (1 + slPct)
						}
					}
					if tp <= 0 && ctx.MarketDataMap[ent.Symbol] != nil && ctx.MarketDataMap[ent.Symbol].CurrentPrice > 0 {
						price := ctx.MarketDataMap[ent.Symbol].CurrentPrice
						pct := 0.06
						if ent.Action == "open_long" {
							tp = price * (1 + pct)
						} else {
							tp = price * (1 - pct)
						}
					}
					sysDecision := kernel.Decision{
						Symbol:          ent.Symbol,
						Action:          ent.Action,
						Leverage:        lev,
						PositionSizeUSD: positionSizeUSD,
						StopLoss:        sl,
						TakeProfit:      tp,
						Confidence:      effectiveMinConfSys,
						Reasoning:       "system entry (system cycle, cached AI)",
					}
					if sysDecision.Confidence <= 0 {
						sysDecision.Confidence = 70
					}
					actionRecord := store.DecisionAction{
						Action: ent.Action, Symbol: ent.Symbol, Leverage: lev, StopLoss: sl, TakeProfit: tp,
						Confidence: sysDecision.Confidence, Reasoning: sysDecision.Reasoning, Timestamp: time.Now().UTC(), Success: false,
					}
					var execErr error
					if ent.Action == "open_long" {
						execErr = at.executeOpenLongWithRecord(&sysDecision, &actionRecord, effectiveMinConfSys)
					} else {
						execErr = at.executeOpenShortWithRecord(&sysDecision, &actionRecord, effectiveMinConfSys)
					}
					if execErr != nil {
						logger.Infof("❌ [%s] System cycle open %s %s failed: %v", at.name, ent.Symbol, ent.Action, execErr)
					} else if actionRecord.Error != "" {
						logger.Infof("⏭ [%s] System cycle open %s %s skipped: %s", at.name, ent.Symbol, ent.Action, actionRecord.Error)
					} else {
						logger.Infof("✓ [%s] System cycle open %s %s succeeded", at.name, ent.Symbol, ent.Action)
						if ent.Action == "open_long" {
							deferredOpens = append(deferredOpens, market.Normalize(ent.Symbol)+" LONG")
						} else {
							deferredOpens = append(deferredOpens, market.Normalize(ent.Symbol)+" SHORT")
						}
						time.Sleep(1 * time.Second)
					}
				}
				deferredEntryCount = entryCount
				needDeferredAppend = true
			} else {
				s := fmt.Sprintf("候选币数: %d；无 pipeline 状态；已检查止盈止损。", candidateCount)
				at.lastSystemCycleSummaryMu.Lock()
				at.lastSystemCycleSummary = s
				at.lastSystemCycleSummaryMu.Unlock()
				at.appendSystemCycleOutput(s)
			}
		}
	} else {
		s := fmt.Sprintf("候选币数: %d；已检查止盈止损。", candidateCount)
		at.lastSystemCycleSummaryMu.Lock()
		at.lastSystemCycleSummary = s
		at.lastSystemCycleSummaryMu.Unlock()
		at.appendSystemCycleOutput(s)
	}
	_ = at.checkDynamicStopLossTakeProfit(nil, "")
	if needDeferredAppend {
		copy := at.getStrategyTriggeredClosesCopy()
		var newCloses []string
		if len(copy) > oldClosesLen {
			for _, c := range copy[oldClosesLen:] {
				newCloses = append(newCloses, c.Symbol+" CLOSE")
			}
		}
		parts := []string{fmt.Sprintf("候选币数: %d；待开仓数: %d", candidateCount, deferredEntryCount)}
		if len(deferredOpens) > 0 {
			parts = append(parts, "本周期开仓: "+strings.Join(deferredOpens, ", "))
		}
		if len(newCloses) > 0 {
			parts = append(parts, "本周期平仓: "+strings.Join(newCloses, ", "))
		}
		parts = append(parts, "已检查止盈止损。")
		s := strings.Join(parts, "；")
		at.lastSystemCycleSummaryMu.Lock()
		at.lastSystemCycleSummary = s
		at.lastSystemCycleSummaryMu.Unlock()
		at.appendSystemCycleOutput(s)
	}
	return nil
}

// runSLTPAnalysisCycle 持仓止盈止损专用分析：仅拉取持仓与必要数据，调用 AI 输出 position_sl_tp_adjustments，不写决策、不开平仓
// scheduledAt 为本周期调度时刻，用于思维链 At 展示，使界面间隔与配置一致
func (at *AutoTrader) runSLTPAnalysisCycle(scheduledAt time.Time) error {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}
	at.sltpAnalysisCycleCount++
	nowMs := time.Now().UTC().UnixMilli()
	var list []kernel.SLTPPositionInfo
	baselinesToStore := make(map[string]kernel.PositionSLTPBaseline)
	var riskConfig *store.RiskControlConfig
	if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
		riskConfig = &at.strategyEngine.GetConfig().RiskControl
	}
	for _, pos := range positions {
		symbol := getPosStr(pos, "symbol", "")
		if symbol == "" {
			continue
		}
		side := getPosStr(pos, "side", "position_side")
		side = strings.ToLower(side)
		normalizedSymbol := market.Normalize(symbol)
		posKey := normalizedSymbol + "_" + side
		entryPrice := getPosFloat(pos, "entryPrice", "entry_price")
		markPrice := getPosFloat(pos, "markPrice", "mark_price")
		quantity := getPosFloat(pos, "positionAmt", "position_amt")
		if quantity < 0 {
			quantity = -quantity
		}
		if quantity == 0 {
			continue
		}
		leverage := getPosFloat(pos, "leverage", "")
		if leverage <= 0 {
			leverage = 10
		}
		unrealizedPnl := getPosFloat(pos, "unRealizedProfit", "unrealized_pnl")
		marginUsed := (quantity * markPrice) / leverage
		if marginUsed <= 0 {
			marginUsed = 1
		}
		pnlPct := calculatePnLPercentage(unrealizedPnl, marginUsed)
		updateTime := getPosInt64(pos, "update_time", "updateTime", "entry_time", "createdTime")
		if updateTime <= 0 {
			updateTime = nowMs
		}
		holdMinutes := float64(nowMs-updateTime) / 60000.0
		atr := 0.0
		if at.marketClient != nil {
			klines, kerr := at.marketClient.GetKlines(symbol, "15m", 50)
			if kerr == nil && len(klines) > 0 {
				atr = kernel.CalculateATR(klines, 14)
			}
		}
		info := kernel.SLTPPositionInfo{
			Symbol:      normalizedSymbol,
			Side:        side,
			EntryPrice:  entryPrice,
			MarkPrice:   markPrice,
			PnlPct:      pnlPct,
			HoldMinutes: holdMinutes,
			ATR:         atr,
		}
		baseline := kernel.PositionSLTPBaseline{Symbol: normalizedSymbol, Side: side}
		if riskConfig != nil {
			at.positionProfileMu.RLock()
			slName := strings.TrimSpace(strings.ToLower(at.positionSLProfile[posKey]))
			tpName := strings.TrimSpace(strings.ToLower(at.positionTPProfile[posKey]))
			trailAgg := strings.TrimSpace(strings.ToLower(at.positionTrailAgg[posKey]))
			at.positionProfileMu.RUnlock()
			atrMultMin, atrMultMax := 1.2, 3.0
			lockMin, lockMax := 1.0, 5.0
			confirmBase := 1
			if riskConfig.DynamicStopLoss != nil && riskConfig.DynamicStopLoss.Enabled {
				if riskConfig.DynamicStopLoss.ATRMultiplierMin != nil {
					atrMultMin = *riskConfig.DynamicStopLoss.ATRMultiplierMin
				}
				if riskConfig.DynamicStopLoss.ATRMultiplierMax != nil {
					atrMultMax = *riskConfig.DynamicStopLoss.ATRMultiplierMax
				}
				if riskConfig.DynamicStopLoss.ConfirmCycles > 0 {
					confirmBase = riskConfig.DynamicStopLoss.ConfirmCycles
				}
			}
			if slName != "" && riskConfig.SLProfiles != nil {
				if prof, ok := riskConfig.SLProfiles[slName]; ok && prof.Enabled {
					if prof.ATRMultiplierMin != nil {
						atrMultMin = *prof.ATRMultiplierMin
					}
					if prof.ATRMultiplierMax != nil {
						atrMultMax = *prof.ATRMultiplierMax
					}
					if prof.ConfirmCycles > 0 {
						confirmBase = prof.ConfirmCycles
					}
				}
			}
			if riskConfig.DynamicTakeProfit != nil && riskConfig.DynamicTakeProfit.Enabled && riskConfig.DynamicTakeProfit.LockProfitPercent != nil {
				lp := *riskConfig.DynamicTakeProfit.LockProfitPercent
				if lp >= 1 && lp <= 5 {
					lockMin, lockMax = 1.0, 5.0
				}
			}
			if tpName != "" && riskConfig.TPProfiles != nil {
				if prof, ok := riskConfig.TPProfiles[tpName]; ok && prof.Enabled && prof.LockProfitPercent != nil {
					lp := *prof.LockProfitPercent
					if lp < 1 {
						lp = 1
					}
					if lp > 5 {
						lp = 5
					}
					lockMin, lockMax = 1.0, 5.0
				}
			}
			info.ATRMultMin, info.ATRMultMax = atrMultMin, atrMultMax
			info.LockProfitPctMin, info.LockProfitPctMax = lockMin, lockMax
			info.ConfirmCyclesBase = confirmBase
			if trailAgg != "" {
				info.TrailCurrent = trailAgg
			} else {
				info.TrailCurrent = "medium"
			}
			baseline.TrailAggressiveness = info.TrailCurrent
			baseline.ATRMultSL = (atrMultMin + atrMultMax) / 2
			baseline.LockProfitPct = (lockMin + lockMax) / 2
			if riskConfig.DynamicTakeProfit != nil && riskConfig.DynamicTakeProfit.LockProfitPercent != nil {
				baseline.LockProfitPct = *riskConfig.DynamicTakeProfit.LockProfitPercent
			}
			if tpName != "" && riskConfig.TPProfiles != nil {
				if prof, ok := riskConfig.TPProfiles[tpName]; ok && prof.LockProfitPercent != nil {
					baseline.LockProfitPct = *prof.LockProfitPercent
				}
			}
			baseline.ConfirmCycles = confirmBase
		}
		// 当前生效参数与上一轮 AI 建议：若有则传给 AI，便于续推或微调
		at.positionSLTPAdjustmentMu.RLock()
		adj, hasAdj := at.positionSLTPAdjustment[posKey]
		at.positionSLTPAdjustmentMu.RUnlock()
		if hasAdj {
			if adj.TrailAggressiveness != "" {
				info.AppliedTrailAggressiveness = adj.TrailAggressiveness
			}
			if adj.ATRMultSL > 0 {
				info.AppliedATRMultSL = adj.ATRMultSL
			}
			if adj.LockProfitPct > 0 {
				info.AppliedLockProfitPct = adj.LockProfitPct
			}
			if adj.Advice != "" {
				info.LastAdvice = adj.Advice
			}
		}
		list = append(list, info)
		baselinesToStore[posKey] = baseline
	}
	if len(list) == 0 {
		return nil
	}
	regime, scenario := "", ""
	at.lastAIDecisionMu.RLock()
	if at.lastAIDecisionCache != nil {
		regime = at.lastAIDecisionCache.MarketRegime
		scenario = at.lastAIDecisionCache.Scenario
	}
	at.lastAIDecisionMu.RUnlock()
	// 止盈止损 5 种场景需要「实时+预测」：本周期拉取实时市场数据生成摘要，避免用主周期缓存导致场景判断滞后
	var summary string
	if at.strategyEngine != nil {
		summary = at.strategyEngine.BuildLiveMarketSummaryForSLTP(at.id)
	}
	if summary == "" {
		at.lastMarketSummaryForSLTPMu.RLock()
		summary = at.lastMarketSummaryForSLTP
		at.lastMarketSummaryForSLTPMu.RUnlock()
	}
	adj, raw, err := kernel.RunSLTPOnlyAnalysis(at.mcpClient, list, regime, scenario, summary)
	if err != nil {
		return err
	}
	if len(adj) > 0 {
		at.positionSLTPAdjustmentMu.Lock()
		at.positionSLTPBaselineMu.Lock()
		for _, a := range adj {
			posKey := market.Normalize(a.Symbol) + "_" + strings.ToLower(a.Side)
			at.positionSLTPAdjustment[posKey] = a
			if b, ok := baselinesToStore[posKey]; ok {
				at.positionSLTPBaseline[posKey] = b
			}
		}
		at.positionSLTPBaselineMu.Unlock()
		at.positionSLTPAdjustmentMu.Unlock()
		logger.Infof("🛡️ [%s] SL/TP-only analysis: updated %d position adjustments (with strategy bounds)", at.name, len(adj))
	}
	at.lastSLTPCycleOutputMu.Lock()
	at.lastSLTPCycleOutput = adj
	at.lastSLTPCycleOutputMu.Unlock()
	// 思维链：每周期都记录（含无调节时），时间戳用调度时刻便于展示与设定间隔一致
	at.appendSLTPCycleOutput(adj, raw, scheduledAt)
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

// strategyTriggeredClosesLen 返回当前策略触发平仓数量（不清空），供系统周期汇总用
func (at *AutoTrader) strategyTriggeredClosesLen() int {
	at.strategyTriggeredClosesMu.Lock()
	defer at.strategyTriggeredClosesMu.Unlock()
	return len(at.strategyTriggeredCloses)
}

// getStrategyTriggeredClosesCopy 返回当前策略触发平仓列表的副本（不清空）
func (at *AutoTrader) getStrategyTriggeredClosesCopy() []kernel.StrategyTriggeredClose {
	at.strategyTriggeredClosesMu.Lock()
	defer at.strategyTriggeredClosesMu.Unlock()
	if len(at.strategyTriggeredCloses) == 0 {
		return nil
	}
	out := make([]kernel.StrategyTriggeredClose, len(at.strategyTriggeredCloses))
	copy(out, at.strategyTriggeredCloses)
	return out
}

// buildTradingContext builds trading context. flow 用于 Coinglass 错峰与数据统计（主周期/系统周期），在 FillCoinglassData 前会据此做 WaitFlowStagger。
func (at *AutoTrader) buildTradingContext(flow string) (*kernel.Context, error) {
	// 1. Get account information
	t0 := time.Now()
	balance, err := at.trader.GetBalance()
	durationMs := time.Since(t0).Milliseconds()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	monitor.RecordDataCall("exchange", "GetAccount", flow, at.id, err == nil, errMsg, durationMs)
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
	t0 = time.Now()
	positions, err := at.trader.GetPositions()
	durationMs = time.Since(t0).Milliseconds()
	errMsg = ""
	if err != nil {
		errMsg = err.Error()
	}
	monitor.RecordDataCall("exchange", "GetPositions", flow, at.id, err == nil, errMsg, durationMs)
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
		// posKey 须与 checkDynamicStopLossTakeProfit/runSLTPAnalysisCycle/平仓清理一致：Normalize(symbol)_side
		posKey := market.Normalize(symbol) + "_" + strings.ToLower(side)
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

	// 6. Build context（Flow 在 FillCoinglassData 前已设，保证错峰与数据统计正确）
	ctx := &kernel.Context{
		Flow:             flow,
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
		t0 := time.Now()
		ctx.OIRankingData = at.strategyEngine.FetchOIRankingData()
		monitor.RecordDataCall("nofxos", "OIRanking", flow, at.id, ctx.OIRankingData != nil, "", time.Since(t0).Milliseconds())
		if ctx.OIRankingData != nil {
			logger.Infof("📊 [%s] OI ranking data ready: %d top, %d low positions",
				at.name, len(ctx.OIRankingData.TopPositions), len(ctx.OIRankingData.LowPositions))
		}
	}

	// 10. Get NetFlow ranking data (market-wide fund flow)
	if strategyConfig.Indicators.EnableNetFlowRanking {
		logger.Infof("💰 [%s] Fetching NetFlow ranking data...", at.name)
		t0 := time.Now()
		ctx.NetFlowRankingData = at.strategyEngine.FetchNetFlowRankingData()
		monitor.RecordDataCall("nofxos", "NetFlowRanking", flow, at.id, ctx.NetFlowRankingData != nil, "", time.Since(t0).Milliseconds())
		if ctx.NetFlowRankingData != nil {
			logger.Infof("💰 [%s] NetFlow ranking data ready: inst_in=%d, inst_out=%d",
				at.name, len(ctx.NetFlowRankingData.InstitutionFutureTop), len(ctx.NetFlowRankingData.InstitutionFutureLow))
		}
	}

	// 11. Get Price ranking data (market-wide gainers/losers)
	if strategyConfig.Indicators.EnablePriceRanking {
		logger.Infof("📈 [%s] Fetching Price ranking data...", at.name)
		t0 := time.Now()
		ctx.PriceRankingData = at.strategyEngine.FetchPriceRankingData()
		monitor.RecordDataCall("nofxos", "PriceRanking", flow, at.id, ctx.PriceRankingData != nil, "", time.Since(t0).Milliseconds())
		if ctx.PriceRankingData != nil {
			logger.Infof("📈 [%s] Price ranking data ready for %d durations",
				at.name, len(ctx.PriceRankingData.Durations))
		}
	}

	// 11.5 Coinglass 市场数据（经 KeyStore 中转）：OI、资金费率、多空比、强平、恐惧贪婪、BTC 占比、山寨季、ETF 资金流，写入 ctx 供数据补强与 AI
	// FillCoinglassData 为同步，返回后 ctx 已为本周期完整拉取结果，后续 pipeline/AI 仅使用此完整 ctx，不传递不完整信息
	if strategyConfig.Indicators.EnableCoinglassData {
		at.strategyEngine.FillCoinglassData(ctx)
		n := len(ctx.CoinglassOISummaries) + len(ctx.CoinglassFundingMap) + len(ctx.CoinglassLongShortMap)
		if ctx.CoinglassLiquidation != nil {
			n++
		}
		if ctx.FearGreedValue > 0 || ctx.FearGreedClassification != "" {
			n++
		}
		if n > 0 {
			logger.Infof("📊 [%s] Coinglass data ready via KeyStore: OI %d, funding %d, long/short %d, liquidation %v, fear-greed %v, ETF %v",
				at.name, len(ctx.CoinglassOISummaries), len(ctx.CoinglassFundingMap), len(ctx.CoinglassLongShortMap),
				ctx.CoinglassLiquidation != nil, ctx.FearGreedClassification != "", ctx.ETFFlowBTCRecent != 0 || ctx.ETFFlowETHRecent != 0)
		}
		// Coinglass WSS 实时：可选启用，懒加载客户端并写入 ctx.CoinglassWSSSnapshot 补强 AI
		if strategyConfig.Indicators.EnableCoinglassWSS && strategyConfig.Indicators.CoinglassAPIKey != "" {
			t0 := time.Now()
			at.coinglassWSClientMu.Lock()
			if at.coinglassWSClient == nil {
				at.coinglassWSClient = coinglass.NewWSClient("", strategyConfig.Indicators.CoinglassAPIKey)
				if err := at.coinglassWSClient.Connect(); err != nil {
					logger.Warnf("[%s] Coinglass WSS connect: %v", at.name, err)
				} else {
					go at.coinglassWSClient.Run()
					logger.Infof("[%s] Coinglass WSS started (funding/liquidation/OI/price)", at.name)
				}
			}
			gotAny := false
			if at.coinglassWSClient != nil {
				ctx.CoinglassWSSSnapshot = make(map[string]string)
				for _, ch := range []string{coinglass.ChannelFundingRate, coinglass.ChannelLiquidation, coinglass.ChannelOpenInterest, coinglass.ChannelPrice} {
					if b, ok := at.coinglassWSClient.GetLatest(ch); ok && len(b) > 0 {
						ctx.CoinglassWSSSnapshot[ch] = string(b)
						gotAny = true
					}
				}
			}
			at.coinglassWSClientMu.Unlock()
			monitor.RecordDataCall("coinglass_wss", "WSS", flow, at.id, gotAny, "", time.Since(t0).Milliseconds())
		}
	}

	// 12. Binance 衍生数据（多空比、资金费率、Taker）— 以币安为主增强市场判断
	ind := strategyConfig.Indicators
	if ind.EnableBinanceLongShortRatio || ind.EnableBinanceFundingHistory || ind.EnableBinanceTakerVolume {
		symbolsSet := make(map[string]bool)
		for _, p := range positionInfos {
			symbolsSet[market.Normalize(p.Symbol)] = true
		}
		for _, c := range candidateCoins {
			symbolsSet[market.Normalize(c.Symbol)] = true
		}
		if len(symbolsSet) == 0 {
			symbolsSet["BTCUSDT"] = true
			symbolsSet["ETHUSDT"] = true
		}
		symbolsList := make([]string, 0, len(symbolsSet))
		for s := range symbolsSet {
			symbolsList = append(symbolsList, s)
		}
		if len(symbolsList) > 12 {
			symbolsList = symbolsList[:12]
		}
		bc := binancedata.NewClient()
		periodLS := ind.BinanceLongShortPeriod
		if periodLS == "" {
			periodLS = "15m"
		}
		periodTaker := ind.BinanceTakerPeriod
		if periodTaker == "" {
			periodTaker = "15m"
		}
		for _, sym := range symbolsList {
			if ind.EnableBinanceLongShortRatio {
				t0 := time.Now()
				global, errLS := bc.GetGlobalLongShortAccountRatio(sym, periodLS, 1)
				top, _ := bc.GetTopLongShortAccountRatio(sym, periodLS, 1)
				errMsg := ""
				if errLS != nil {
					errMsg = errLS.Error()
				}
				monitor.RecordDataCall("binancedata", "LongShortRatio", flow, at.id, errLS == nil && len(global) > 0, errMsg, time.Since(t0).Milliseconds())
				if len(global) > 0 {
					if ctx.BinanceLongShortMap == nil {
						ctx.BinanceLongShortMap = make(map[string]*kernel.BinanceLongShortSnapshot)
					}
					g := global[0]
					snap := &kernel.BinanceLongShortSnapshot{
						LongShortRatio: g.LongShortRatio,
						LongAccount:    g.LongAccount,
						ShortAccount:   g.ShortAccount,
						Timestamp:      g.Timestamp,
					}
					if len(top) > 0 {
						snap.TopLongShortRatio = top[0].LongShortRatio
						snap.TopLongAccount = top[0].LongAccount
						snap.TopShortAccount = top[0].ShortAccount
					}
					ctx.BinanceLongShortMap[sym] = snap
				}
			}
			if ind.EnableBinanceFundingHistory {
				t0 := time.Now()
				prem, err := bc.GetPremiumIndex(sym)
				errMsg := ""
				if err != nil {
					errMsg = err.Error()
				}
				monitor.RecordDataCall("binancedata", "FundingRate", flow, at.id, err == nil && prem != nil, errMsg, time.Since(t0).Milliseconds())
				if err == nil && prem != nil {
					if ctx.BinanceFundingMap == nil {
						ctx.BinanceFundingMap = make(map[string]*kernel.BinanceFundingSnapshot)
					}
					ctx.BinanceFundingMap[sym] = &kernel.BinanceFundingSnapshot{
						LastFundingRate: prem.LastFundingRate,
						NextFundingTime: prem.NextFundingTime,
						MarkPrice:       prem.MarkPrice,
						Time:            prem.Time,
					}
				}
			}
			if ind.EnableBinanceTakerVolume {
				t0 := time.Now()
				taker, errTaker := bc.GetTakerLongShortRatio(sym, periodTaker, 1)
				errMsg := ""
				if errTaker != nil {
					errMsg = errTaker.Error()
				}
				monitor.RecordDataCall("binancedata", "TakerVolume", flow, at.id, errTaker == nil && len(taker) > 0, errMsg, time.Since(t0).Milliseconds())
				if len(taker) > 0 {
					if ctx.BinanceTakerMap == nil {
						ctx.BinanceTakerMap = make(map[string]*kernel.BinanceTakerSnapshot)
					}
					t := taker[0]
					ctx.BinanceTakerMap[sym] = &kernel.BinanceTakerSnapshot{
						BuySellRatio: t.BuySellRatio,
						BuyVol:       t.BuyVol,
						SellVol:      t.SellVol,
						Timestamp:    t.Timestamp,
					}
				}
			}
		}
		if ctx.BinanceLongShortMap != nil || ctx.BinanceFundingMap != nil || ctx.BinanceTakerMap != nil {
			logger.Infof("📊 [%s] Binance derivatives data ready (L/S:%d Funding:%d Taker:%d)",
				at.name, len(ctx.BinanceLongShortMap), len(ctx.BinanceFundingMap), len(ctx.BinanceTakerMap))
		}
	}

	// 12.1 数据补强：资金费率近 8h 均值、Basis、BTC 占比、强平聚合、CoinAnk 清算
	symbolsSetExtra := make(map[string]bool)
	for _, p := range positionInfos {
		symbolsSetExtra[market.Normalize(p.Symbol)] = true
	}
	for _, c := range candidateCoins {
		symbolsSetExtra[market.Normalize(c.Symbol)] = true
	}
	if len(symbolsSetExtra) == 0 {
		symbolsSetExtra["BTCUSDT"] = true
		symbolsSetExtra["ETHUSDT"] = true
	}
	symbolsListExtra := make([]string, 0, len(symbolsSetExtra))
	for s := range symbolsSetExtra {
		symbolsListExtra = append(symbolsListExtra, s)
	}
	if len(symbolsListExtra) > 12 {
		symbolsListExtra = symbolsListExtra[:12]
	}
	bc := binancedata.NewClient()
	if ind.EnableBinanceFundingRateHistory {
		if ctx.BinanceFundingRateAvg8h == nil {
			ctx.BinanceFundingRateAvg8h = make(map[string]float64)
		}
		if ctx.FundingRateHistoryLast8 == nil {
			ctx.FundingRateHistoryLast8 = make(map[string][]float64)
		}
		for _, sym := range symbolsListExtra {
			rates, err := bc.GetFundingRateHistory(sym, 24)
			if err != nil || len(rates) == 0 {
				continue
			}
			n := 3
			if len(rates) < n {
				n = len(rates)
			}
			var sum float64
			for i := len(rates) - n; i < len(rates); i++ {
				sum += rates[i].FundingRate
			}
			ctx.BinanceFundingRateAvg8h[sym] = sum / float64(n)
			// 近 8 期资金费供 AI 看趋势（紧凑，不增 K 线 token）
			last := 8
			if len(rates) < last {
				last = len(rates)
			}
			hist := make([]float64, last)
			for i := 0; i < last; i++ {
				hist[i] = rates[len(rates)-last+i].FundingRate
			}
			ctx.FundingRateHistoryLast8[sym] = hist
		}
	}
	if ind.EnableBasis {
		if ctx.BasisMap == nil {
			ctx.BasisMap = make(map[string]float64)
		}
		for _, sym := range symbolsListExtra {
			prem, err := bc.GetPremiumIndex(sym)
			if err != nil || prem == nil {
				continue
			}
			t0 := time.Now()
			spot, err := binancedata.GetSpotPrice(sym)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			monitor.RecordDataCall("binancedata", "SpotPrice", flow, at.id, err == nil && spot > 0, errMsg, time.Since(t0).Milliseconds())
			if err != nil || spot <= 0 {
				continue
			}
			ctx.BasisMap[sym] = (prem.MarkPrice - spot) / spot * 100
		}
	}
	if ind.EnableBTCDominance {
		cg := coingecko.NewClient()
		if pct, err := cg.GetBTCDominance(); err == nil {
			ctx.BTCDominancePct = pct
		}
	}
	if ind.EnableBinanceWSForceOrder {
		onceForceOrderWS.Do(func() { binancedata.RunForceOrderWS() })
		t0 := time.Now()
		binancedata.RefreshLiquidationAgg()
		agg := binancedata.GetLiquidationAgg()
		monitor.RecordDataCall("binancedata", "LiquidationAgg", flow, at.id, agg != nil, "", time.Since(t0).Milliseconds())
		if agg != nil {
			ctx.LiquidationAgg = agg
		}
	}
	if ind.EnableCoinAnkLiquidation && ind.CoinAnkAPIKey != "" {
		coinankURL := ind.CoinAnkURL
		if coinankURL == "" {
			coinankURL = "https://open-api.coinank.com"
		}
		ca := coinank.NewCoinankClient(coinankURL, ind.CoinAnkAPIKey)
		stats, err := ca.LiquidationExchangeStatistics(context.Background(), "BTC")
		if err == nil && stats != nil {
			ctx.LiquidationAgg = &kernel.LiquidationAggSnapshot{
				Long1hUSD:  stats.OneH.LongTurnover,
				Short1hUSD: stats.OneH.ShortTurnover,
				Long4hUSD:  stats.Two4H.LongTurnover / 6,
				Short4hUSD: stats.Two4H.ShortTurnover / 6,
				Source:     "coinank",
				UpdatedAt:  time.Now().UnixMilli(),
			}
		}
	}

	// 本周期或 30s 后台触发的策略平仓，传给 AI 以便分析链中有平仓记录
	ctx.StrategyTriggeredCloses = at.getAndClearStrategyTriggeredCloses()
	if len(ctx.StrategyTriggeredCloses) > 0 {
		logger.Infof("📤 [%s] Passing %d strategy-triggered close(s) to AI context", at.name, len(ctx.StrategyTriggeredCloses))
	}

	return ctx, nil
}

// executeDecisionWithRecord executes AI decision and records detailed information.
// effectiveMinConf: 当 > 0 时用于开仓置信度校验（覆盖策略 MinConfidence）；0 表示使用策略配置。
func (at *AutoTrader) executeDecisionWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, effectiveMinConf int) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord, effectiveMinConf)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord, effectiveMinConf)
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

// ExecuteDecision executes a trading decision from external sources
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

	// Execute the decision (external call: use config MinConfidence)
	err := at.executeDecisionWithRecord(d, actionRecord, 0)
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

// executeOpenLongWithRecord executes open long position and records detailed information.
// effectiveMinConf: 当 > 0 时用于置信度校验（覆盖策略 MinConfidence）；0 表示使用策略配置。
func (at *AutoTrader) executeOpenLongWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, effectiveMinConf int) error {
	logger.Infof("  📈 Open long: %s", decision.Symbol)

	// [CODE ENFORCED] Min confidence: reject open if AI confidence below strategy minimum (or regime/extreme-adjusted)
	minConfidence := effectiveMinConf
	if minConfidence <= 0 && at.strategyEngine != nil {
		if cfg := at.strategyEngine.GetConfig(); cfg != nil && cfg.RiskControl.MinConfidence > 0 {
			minConfidence = cfg.RiskControl.MinConfidence
		}
	}
	if minConfidence > 0 && decision.Confidence < minConfidence {
		actionRecord.Error = fmt.Sprintf("置信度 %d 低于最小要求 %d，已拒绝开仓", decision.Confidence, minConfidence)
		logger.Infof("  ⛔ %s", actionRecord.Error)
		return nil
	}

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

	// AI position sizing via discrete risk bucket (system-enforced).
	// If enabled and AI provides RiskBucket (or provides invalid), map to equity ratio and override PositionSizeUSD.
	if at.strategyEngine != nil {
		if cfg := at.strategyEngine.GetConfig(); cfg != nil {
			b := cfg.RiskControl.PositionSizeBuckets
			if b != nil && b.Enabled {
				bucket := strings.TrimSpace(strings.ToLower(decision.RiskBucket))
				if bucket == "" {
					bucket = strings.TrimSpace(strings.ToLower(b.DefaultBucket))
				}
				// If confidence is low, force default bucket
				if b.MinBucketConfidence > 0 && decision.Confidence > 0 && decision.Confidence < b.MinBucketConfidence {
					bucket = strings.TrimSpace(strings.ToLower(b.DefaultBucket))
				}
				// Clamp bucket by MaxBucket ordering low<medium<high when configured
				clampBucket := func(x string) string {
					switch x {
					case "low", "medium", "high":
						return x
					default:
						return ""
					}
				}
				maxB := clampBucket(strings.TrimSpace(strings.ToLower(b.MaxBucket)))
				if maxB != "" {
					order := map[string]int{"low": 1, "medium": 2, "high": 3}
					if order[bucket] > 0 && order[maxB] > 0 && order[bucket] > order[maxB] {
						bucket = maxB
					}
				}
				ratio, ok := b.Buckets[bucket]
				if !ok || ratio <= 0 {
					ratio = b.Buckets[strings.TrimSpace(strings.ToLower(b.DefaultBucket))]
				}
				if ratio > 0 {
					decision.PositionSizeUSD = equity * ratio
					decision.RiskBucket = bucket
				}
			} else {
				// 仓位档位关闭：使用 AI 的 position_size_usd；若未提供则按策略比例回退
				if decision.PositionSizeUSD <= 0 {
					ratio := cfg.RiskControl.AltcoinMaxPositionValueRatio
					if market.Normalize(decision.Symbol) == "BTCUSDT" || market.Normalize(decision.Symbol) == "ETHUSDT" {
						ratio = cfg.RiskControl.BTCETHMaxPositionValueRatio
					}
					if ratio > 0 {
						decision.PositionSizeUSD = equity * ratio
						logger.Infof("  ℹ️ Position size from AI missing/zero, using strategy ratio → %.2f USDT", decision.PositionSizeUSD)
					}
				}
			}
		}
	}

	// 若 AI/档位给出的仓位小于最小要求且权益足够，则抬到最小仓位
	if at.strategyEngine != nil {
		if cfg := at.strategyEngine.GetConfig(); cfg != nil {
			minSize := cfg.RiskControl.MinPositionSize
			if minSize <= 0 {
				minSize = 12
			}
			if minSize > 0 && decision.PositionSizeUSD > 0 && decision.PositionSizeUSD < minSize && equity >= minSize {
				logger.Infof("  ℹ️ Position size %.2f below minimum %.2f, bumping to min size", decision.PositionSizeUSD, minSize)
				decision.PositionSizeUSD = minSize
			}
		}
	}

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
	actualEntryPrice := marketData.CurrentPrice
	actionRecord.Price = actualEntryPrice

	// Adjust SL/TP based on actual entry price (preserve risk/reward ratio) — do before open so we can enforce min ratio
	aiAnalysisPrice := decision.Price
	if aiAnalysisPrice <= 0 {
		aiAnalysisPrice = actualEntryPrice
	}
	adjustedSL, adjustedTP, priceDeviation := adjustStopLossTakeProfitForActualEntry(
		aiAnalysisPrice, actualEntryPrice, decision.StopLoss, decision.TakeProfit, "long",
	)

	// [CODE ENFORCED] Min risk-reward ratio: 不满足则拒绝开仓，不修改 SL/TP
	// 盈亏比 = 止盈距离/止损距离。多: 止盈距离=TP-入场, 止损距离=入场-SL → ratio=(TP-entry)/(entry-SL)
	if decision.StopLoss > 0 && decision.TakeProfit > 0 {
		slDist := actualEntryPrice - adjustedSL   // 多单: 入场到止损的距离(>0)
		tpDist := adjustedTP - actualEntryPrice   // 多单: 入场到止盈的距离(>0)
		if slDist > 0 && tpDist > 0 {
			ratio := tpDist / slDist
			minRatio := 0.0
			if at.strategyEngine != nil {
				if cfg := at.strategyEngine.GetConfig(); cfg != nil && cfg.RiskControl.MinRiskRewardRatio > 0 {
					minRatio = cfg.RiskControl.MinRiskRewardRatio
				}
			}
			if minRatio > 0 && ratio < minRatio {
				actionRecord.Error = fmt.Sprintf("盈亏比 %.2f:1 低于最小要求 %.1f:1，已拒绝开仓", ratio, minRatio)
				logger.Infof("  ⛔ %s", actionRecord.Error)
				return nil
			}
		}
	}

	// Log adjustment if significant deviation
	if math.Abs(priceDeviation) > 0.1 {
		logger.Infof("  📊 SL/TP adjusted for actual entry: AI price %.6f → actual %.6f (%.2f%% deviation)",
			aiAnalysisPrice, actualEntryPrice, priceDeviation)
		logger.Infof("     SL: %.6f → %.6f, TP: %.6f → %.6f",
			decision.StopLoss, adjustedSL, decision.TakeProfit, adjustedTP)
	}
	if math.Abs(priceDeviation) > 2.0 {
		logger.Infof("  ⚠️ Large price deviation (%.2f%%) - market moved significantly since AI analysis", priceDeviation)
	}

	// 第三层开单前校验：信号年龄<5分钟且标的在本周期待提交列表中（按截图设计）
	if at.strategyEngine != nil {
		if cfg := at.strategyEngine.GetConfig(); cfg != nil && cfg.MultilayerFilter != nil && cfg.MultilayerFilter.Enabled {
			if !kernel.ValidateLayer3(decision.Symbol, "long", at.id, cfg.MultilayerFilter.Layer3) {
				actionRecord.Error = "第三层未过：信号超时、不在待提交列表、OI 未对齐或强度不足，已拒绝开多"
				logger.Infof("  ⛔ %s", actionRecord.Error)
				return nil
			}
		}
	}

	// Set margin mode
	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
	}

	// Open position (use effective leverage so display matches AI/strategy)
	order, err := at.trader.OpenLong(decision.Symbol, quantity, leverage)
	if err != nil {
		return err
	}

	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f, leverage: %dx", order["orderId"], quantity, leverage)

	at.recordAndConfirmOrder(order, decision.Symbol, "open_long", quantity, actualEntryPrice, leverage, 0)

	posKey := market.Normalize(decision.Symbol) + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
	// Save AI-selected profiles for this position (if provided)
	at.positionProfileMu.Lock()
	if decision.RiskBucket != "" {
		at.positionRiskBucket[posKey] = strings.TrimSpace(strings.ToLower(decision.RiskBucket))
	}
	if decision.TPProfile != "" {
		at.positionTPProfile[posKey] = strings.TrimSpace(strings.ToLower(decision.TPProfile))
	}
	if decision.SLProfile != "" {
		at.positionSLProfile[posKey] = strings.TrimSpace(strings.ToLower(decision.SLProfile))
	}
	if decision.TrailAggressiveness != "" {
		at.positionTrailAgg[posKey] = strings.TrimSpace(strings.ToLower(decision.TrailAggressiveness))
	}
	if len(decision.KeyLevels) > 0 {
		levels := decision.KeyLevels
		if len(levels) > 3 {
			levels = levels[:3]
		}
		at.positionKeyLevels[posKey] = append([]string(nil), levels...)
	}
	at.positionProfileMu.Unlock()

	// Persist adjusted SL/TP to decision record so UI shows same ratio we set on exchange
	actionRecord.StopLoss = adjustedSL
	actionRecord.TakeProfit = adjustedTP

	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, adjustedSL); err != nil {
		logger.Infof("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, adjustedTP); err != nil {
		logger.Infof("  ⚠ Failed to set take profit: %v", err)
	}

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

// executeOpenShortWithRecord executes open short position and records detailed information.
// effectiveMinConf: 当 > 0 时用于置信度校验（覆盖策略 MinConfidence）；0 表示使用策略配置。
func (at *AutoTrader) executeOpenShortWithRecord(decision *kernel.Decision, actionRecord *store.DecisionAction, effectiveMinConf int) error {
	logger.Infof("  📉 Open short: %s", decision.Symbol)

	// [CODE ENFORCED] Min confidence: reject open if AI confidence below strategy minimum (or regime/extreme-adjusted)
	minConfidence := effectiveMinConf
	if minConfidence <= 0 && at.strategyEngine != nil {
		if cfg := at.strategyEngine.GetConfig(); cfg != nil && cfg.RiskControl.MinConfidence > 0 {
			minConfidence = cfg.RiskControl.MinConfidence
		}
	}
	if minConfidence > 0 && decision.Confidence < minConfidence {
		actionRecord.Error = fmt.Sprintf("置信度 %d 低于最小要求 %d，已拒绝开仓", decision.Confidence, minConfidence)
		logger.Infof("  ⛔ %s", actionRecord.Error)
		return nil
	}

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

	// AI position sizing via discrete risk bucket (system-enforced).
	if at.strategyEngine != nil {
		if cfg := at.strategyEngine.GetConfig(); cfg != nil {
			b := cfg.RiskControl.PositionSizeBuckets
			if b != nil && b.Enabled {
				bucket := strings.TrimSpace(strings.ToLower(decision.RiskBucket))
				if bucket == "" {
					bucket = strings.TrimSpace(strings.ToLower(b.DefaultBucket))
				}
				if b.MinBucketConfidence > 0 && decision.Confidence > 0 && decision.Confidence < b.MinBucketConfidence {
					bucket = strings.TrimSpace(strings.ToLower(b.DefaultBucket))
				}
				clampBucket := func(x string) string {
					switch x {
					case "low", "medium", "high":
						return x
					default:
						return ""
					}
				}
				maxB := clampBucket(strings.TrimSpace(strings.ToLower(b.MaxBucket)))
				if maxB != "" {
					order := map[string]int{"low": 1, "medium": 2, "high": 3}
					if order[bucket] > 0 && order[maxB] > 0 && order[bucket] > order[maxB] {
						bucket = maxB
					}
				}
				ratio, ok := b.Buckets[bucket]
				if !ok || ratio <= 0 {
					ratio = b.Buckets[strings.TrimSpace(strings.ToLower(b.DefaultBucket))]
				}
				if ratio > 0 {
					decision.PositionSizeUSD = equity * ratio
					decision.RiskBucket = bucket
				}
			} else {
				// 仓位档位关闭：使用 AI 的 position_size_usd；若未提供则按策略比例回退
				if decision.PositionSizeUSD <= 0 {
					ratio := cfg.RiskControl.AltcoinMaxPositionValueRatio
					if market.Normalize(decision.Symbol) == "BTCUSDT" || market.Normalize(decision.Symbol) == "ETHUSDT" {
						ratio = cfg.RiskControl.BTCETHMaxPositionValueRatio
					}
					if ratio > 0 {
						decision.PositionSizeUSD = equity * ratio
						logger.Infof("  ℹ️ Position size from AI missing/zero, using strategy ratio → %.2f USDT", decision.PositionSizeUSD)
					}
				}
			}
		}
	}

	// 若 AI/档位给出的仓位小于最小要求且权益足够，则抬到最小仓位
	if at.strategyEngine != nil {
		if cfg := at.strategyEngine.GetConfig(); cfg != nil {
			minSize := cfg.RiskControl.MinPositionSize
			if minSize <= 0 {
				minSize = 12
			}
			if minSize > 0 && decision.PositionSizeUSD > 0 && decision.PositionSizeUSD < minSize && equity >= minSize {
				logger.Infof("  ℹ️ Position size %.2f below minimum %.2f, bumping to min size", decision.PositionSizeUSD, minSize)
				decision.PositionSizeUSD = minSize
			}
		}
	}

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
	actualEntryPrice := marketData.CurrentPrice
	actionRecord.Price = actualEntryPrice

	// Adjust SL/TP based on actual entry price (preserve risk/reward ratio) — do before open so we can enforce min ratio
	aiAnalysisPrice := decision.Price
	if aiAnalysisPrice <= 0 {
		aiAnalysisPrice = actualEntryPrice
	}
	adjustedSL, adjustedTP, priceDeviation := adjustStopLossTakeProfitForActualEntry(
		aiAnalysisPrice, actualEntryPrice, decision.StopLoss, decision.TakeProfit, "short",
	)

	// [CODE ENFORCED] Min risk-reward ratio: 不满足则拒绝开仓，不修改 SL/TP（short: SL 在价格上方）
	// 盈亏比 = 止盈距离/止损距离。空: 止盈距离=入场-TP, 止损距离=SL-入场 → ratio=(entry-TP)/(SL-entry)
	if decision.StopLoss > 0 && decision.TakeProfit > 0 {
		slDist := adjustedSL - actualEntryPrice   // 空单: 入场到止损的距离(>0)
		tpDist := actualEntryPrice - adjustedTP   // 空单: 入场到止盈的距离(>0)
		if slDist > 0 && tpDist > 0 {
			ratio := tpDist / slDist
			minRatio := 0.0
			if at.strategyEngine != nil {
				if cfg := at.strategyEngine.GetConfig(); cfg != nil && cfg.RiskControl.MinRiskRewardRatio > 0 {
					minRatio = cfg.RiskControl.MinRiskRewardRatio
				}
			}
			if minRatio > 0 && ratio < minRatio {
				actionRecord.Error = fmt.Sprintf("盈亏比 %.2f:1 低于最小要求 %.1f:1，已拒绝开仓", ratio, minRatio)
				logger.Infof("  ⛔ %s", actionRecord.Error)
				return nil
			}
		}
	}

	if math.Abs(priceDeviation) > 0.1 {
		logger.Infof("  📊 SL/TP adjusted for actual entry: AI price %.6f → actual %.6f (%.2f%% deviation)",
			aiAnalysisPrice, actualEntryPrice, priceDeviation)
		logger.Infof("     SL: %.6f → %.6f, TP: %.6f → %.6f",
			decision.StopLoss, adjustedSL, decision.TakeProfit, adjustedTP)
	}
	if math.Abs(priceDeviation) > 2.0 {
		logger.Infof("  ⚠️ Large price deviation (%.2f%%) - market moved significantly since AI analysis", priceDeviation)
	}

	// 第三层开单前校验：信号年龄<5分钟且标的在本周期待提交列表中（按截图设计）
	if at.strategyEngine != nil {
		if cfg := at.strategyEngine.GetConfig(); cfg != nil && cfg.MultilayerFilter != nil && cfg.MultilayerFilter.Enabled {
			if !kernel.ValidateLayer3(decision.Symbol, "short", at.id, cfg.MultilayerFilter.Layer3) {
				actionRecord.Error = "第三层未过：信号超时、不在待提交列表、OI 未对齐或强度不足，已拒绝开空"
				logger.Infof("  ⛔ %s", actionRecord.Error)
				return nil
			}
		}
	}

	if err := at.trader.SetMarginMode(decision.Symbol, at.config.IsCrossMargin); err != nil {
		logger.Infof("  ⚠️ Failed to set margin mode: %v", err)
	}

	order, err := at.trader.OpenShort(decision.Symbol, quantity, leverage)
	if err != nil {
		return err
	}

	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	logger.Infof("  ✓ Position opened successfully, order ID: %v, quantity: %.4f, leverage: %dx", order["orderId"], quantity, leverage)

	at.recordAndConfirmOrder(order, decision.Symbol, "open_short", quantity, actualEntryPrice, leverage, 0)

	posKey := market.Normalize(decision.Symbol) + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
	at.positionProfileMu.Lock()
	if decision.RiskBucket != "" {
		at.positionRiskBucket[posKey] = strings.TrimSpace(strings.ToLower(decision.RiskBucket))
	}
	if decision.TPProfile != "" {
		at.positionTPProfile[posKey] = strings.TrimSpace(strings.ToLower(decision.TPProfile))
	}
	if decision.SLProfile != "" {
		at.positionSLProfile[posKey] = strings.TrimSpace(strings.ToLower(decision.SLProfile))
	}
	if decision.TrailAggressiveness != "" {
		at.positionTrailAgg[posKey] = strings.TrimSpace(strings.ToLower(decision.TrailAggressiveness))
	}
	if len(decision.KeyLevels) > 0 {
		levels := decision.KeyLevels
		if len(levels) > 3 {
			levels = levels[:3]
		}
		at.positionKeyLevels[posKey] = append([]string(nil), levels...)
	}
	at.positionProfileMu.Unlock()

	actionRecord.StopLoss = adjustedSL
	actionRecord.TakeProfit = adjustedTP

	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, adjustedSL); err != nil {
		logger.Infof("  ⚠ Failed to set stop loss: %v", err)
	}
	if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, adjustedTP); err != nil {
		logger.Infof("  ⚠ Failed to set take profit: %v", err)
	}

	at.setPositionParams(posKey, adjustedSL, adjustedTP)

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
	posKey := market.Normalize(decision.Symbol) + "_long"
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
		// Clear AI-selected per-position profiles
		at.positionProfileMu.Lock()
		delete(at.positionRiskBucket, posKey)
		delete(at.positionTPProfile, posKey)
			delete(at.positionSLProfile, posKey)
			delete(at.positionTrailAgg, posKey)
			delete(at.positionKeyLevels, posKey)
			at.positionSLTPAdjustmentMu.Lock()
			delete(at.positionSLTPAdjustment, posKey)
			at.positionSLTPBaselineMu.Lock()
			delete(at.positionSLTPBaseline, posKey)
			at.positionSLTPBaselineMu.Unlock()
			at.positionSLTPAdjustmentMu.Unlock()
		at.positionProfileMu.Unlock()
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
	posKey := market.Normalize(decision.Symbol) + "_short"
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
		at.positionProfileMu.Lock()
		delete(at.positionRiskBucket, posKey)
		delete(at.positionTPProfile, posKey)
			delete(at.positionSLProfile, posKey)
			delete(at.positionTrailAgg, posKey)
			delete(at.positionKeyLevels, posKey)
			at.positionSLTPAdjustmentMu.Lock()
			delete(at.positionSLTPAdjustment, posKey)
			at.positionSLTPBaselineMu.Lock()
			delete(at.positionSLTPBaseline, posKey)
			at.positionSLTPBaselineMu.Unlock()
			at.positionSLTPAdjustmentMu.Unlock()
		at.positionProfileMu.Unlock()
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
	if at.config.SystemInterval > 0 {
		result["system_interval"] = at.config.SystemInterval.String()
	}
	if at.config.SLTPAnalysisInterval > 0 {
		result["sltp_analysis_interval"] = at.config.SLTPAnalysisInterval.String()
	}
	result["system_cycle_count"] = at.systemCycleCount
	result["sltp_analysis_cycle_count"] = at.sltpAnalysisCycleCount
	at.lastSystemCycleSummaryMu.Lock()
	result["last_system_cycle_summary"] = at.lastSystemCycleSummary
	at.lastSystemCycleSummaryMu.Unlock()
	at.lastSLTPCycleOutputMu.Lock()
	if len(at.lastSLTPCycleOutput) > 0 {
		result["last_sltp_cycle_output"] = at.lastSLTPCycleOutput
	}
	at.lastSLTPCycleOutputMu.Unlock()
	at.systemCycleHistoryMu.Lock()
	result["system_cycle_output_history"] = at.systemCycleHistory
	at.systemCycleHistoryMu.Unlock()
	at.sltpCycleHistoryMu.Lock()
	result["sltp_cycle_output_history"] = at.sltpCycleHistory
	at.sltpCycleHistoryMu.Unlock()

	// 供 latest-analysis API：当前止盈止损调节与基线（原始→调整后展示）
	at.positionSLTPAdjustmentMu.RLock()
	at.positionSLTPBaselineMu.RLock()
	if len(at.positionSLTPAdjustment) > 0 {
		adjList := make([]kernel.PositionSLTPAdjustment, 0, len(at.positionSLTPAdjustment))
		baseList := make([]kernel.PositionSLTPBaseline, 0, len(at.positionSLTPBaseline))
		for k, a := range at.positionSLTPAdjustment {
			adjList = append(adjList, a)
			if b, ok := at.positionSLTPBaseline[k]; ok {
				baseList = append(baseList, b)
			}
		}
		result["position_sl_tp_adjustments"] = adjList
		result["position_sl_tp_baselines"] = baseList
	}
	at.positionSLTPBaselineMu.RUnlock()
	at.positionSLTPAdjustmentMu.RUnlock()

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
		posKey := market.Normalize(symbol) + "_" + sideNorm

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

// sltpCheckInterval 策略止盈/止损检查间隔，独立于 AI 决策间隔，触发即平仓；20s 在实时性与接口/负载间折中
const sltpCheckInterval = 20 * time.Second

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
				if err := at.checkDynamicStopLossTakeProfit(nil, ""); err != nil {
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

// UpdatePeakPnL updates peak profit cache (posKey 与 checkDynamicStopLossTakeProfit 等一致：Normalize(symbol)_side)
func (at *AutoTrader) UpdatePeakPnL(symbol, side string, currentPnLPct float64) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := market.Normalize(symbol) + "_" + strings.ToLower(side)
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

// ClearPeakPnLCache clears peak cache for specified position (posKey 与其它 map 一致：Normalize(symbol)_side)
func (at *AutoTrader) ClearPeakPnLCache(symbol, side string) {
	at.peakPnLCacheMutex.Lock()
	defer at.peakPnLCacheMutex.Unlock()

	posKey := market.Normalize(symbol) + "_" + strings.ToLower(side)
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

	// Exchanges with OrderSync: still write order immediately so it shows in UI; OrderSync will skip if exists
	switch at.exchange {
	case "binance", "lighter", "hyperliquid", "bybit", "okx", "bitget", "aster", "kucoin", "gate":
		orderRecord := at.createOrderRecord(orderID, symbol, action, positionSide, quantity, price, leverage)
		if err := at.store.Order().CreateOrder(orderRecord); err != nil {
			logger.Infof("  ⚠️ Failed to record order (OrderSync will sync): %v", err)
		} else {
			logger.Infof("  📝 Order recorded (id: %s), OrderSync may fill details later", orderID)
		}
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

// updateSLTPExitSignalState 更新每仓结构化退场信号状态机（确认+迟滞），返回当前动作等级
// actionLevel: 0=无 1=收紧 2=减仓 3=退出
func (at *AutoTrader) updateSLTPExitSignalState(posKey, phaseLabel, exitBias, invalidationLevel, rationale string, strength int) (actionLevel int, strengthEMA float64) {
	strength = int(math.Max(0, math.Min(100, float64(strength))))
	nowSec := time.Now().UTC().Unix()

	desired := 0
	switch strings.TrimSpace(strings.ToLower(exitBias)) {
	case "exit":
		desired = 3
	case "scale_out":
		desired = 2
	case "tighten":
		desired = 1
	default:
		desired = 0
	}

	var (
		stCopy sltpExitSignalState
		doPersist bool
	)
	at.sltpExitSignalStateMu.Lock()
	st := at.sltpExitSignalState[posKey]
	if st == nil {
		st = &sltpExitSignalState{}
		at.sltpExitSignalState[posKey] = st
	}

	// EMA smoothing (alpha=0.5, simple and responsive)
	if st.StrengthEMA <= 0 {
		st.StrengthEMA = float64(strength)
	} else {
		st.StrengthEMA = 0.5*float64(strength) + 0.5*st.StrengthEMA
	}

	if strength >= 70 {
		st.ConfirmUp70++
	} else {
		st.ConfirmUp70 = 0
	}
	if strength >= 85 {
		st.ConfirmUp85++
	} else {
		st.ConfirmUp85 = 0
	}
	if strength <= 55 {
		st.ConfirmDown55++
	} else {
		st.ConfirmDown55 = 0
	}

	// Upgrade rules (confirmation)
	level := st.LastActionLevel
	if desired >= 1 && level < 1 {
		level = 1
	}
	if desired >= 2 && strength >= 70 && st.ConfirmUp70 >= 2 && level < 2 {
		level = 2
	}
	if desired >= 3 && strength >= 85 && st.ConfirmUp85 >= 3 && level < 3 {
		level = 3
	}

	// Downgrade rules (hysteresis)
	// If invalidation pressure weakens consistently, allow downgrade from 2/3 to 1.
	if level >= 2 && st.ConfirmDown55 >= 3 {
		level = 1
		// keep ConfirmUp counters; next upgrades still require confirmation
	}
	// Allow further downgrade to 0 only when desired is 0 and pressure stays low.
	if level == 1 && desired == 0 && st.ConfirmDown55 >= 3 {
		level = 0
	}

	st.LastActionLevel = level
	st.PhaseLabel = strings.TrimSpace(phaseLabel)
	st.ExitBias = strings.TrimSpace(exitBias)
	st.LastUpdatedAtUnixSec = nowSec
	// copy for persistence outside lock
	stCopy = *st
	doPersist = true
	at.sltpExitSignalStateMu.Unlock()

	_ = invalidationLevel
	_ = rationale

	// Persist state to DB (survive restarts). Best-effort only.
	if doPersist && at.store != nil && posKey != "" {
		parts := strings.SplitN(posKey, "_", 2)
		if len(parts) == 2 {
			sym := parts[0]
			side := strings.ToUpper(parts[1])
			if b, err := json.Marshal(stCopy); err == nil {
				_ = at.store.Position().SetSLTPExitSignalStateAndTimeBySymbol(at.id, sym, side, string(b), time.Now().UTC().UnixMilli())
			}
		}
	}
	return st.LastActionLevel, st.StrengthEMA
}

// getSLTPExitState 返回该仓位的动作等级与已执行等级（供是否触发 scale_out/exit 判断）
func (at *AutoTrader) getSLTPExitState(posKey string) (actionLevel, lastExecutedLevel int) {
	// Fast path: memory
	at.sltpExitSignalStateMu.Lock()
	st := at.sltpExitSignalState[posKey]
	at.sltpExitSignalStateMu.Unlock()
	if st != nil {
		return st.LastActionLevel, st.LastExecutedLevel
	}
	// Lazy load from DB (best-effort)
	if at.store != nil && posKey != "" {
		parts := strings.SplitN(posKey, "_", 2)
		if len(parts) == 2 {
			sym := parts[0]
			side := strings.ToUpper(parts[1])
			dbPos, err := at.store.Position().GetOpenPositionBySymbol(at.id, sym, side)
			if err == nil && dbPos != nil && dbPos.SLTPExitSignalState != "" {
				var loaded sltpExitSignalState
				if json.Unmarshal([]byte(dbPos.SLTPExitSignalState), &loaded) == nil {
					at.sltpExitSignalStateMu.Lock()
					at.sltpExitSignalState[posKey] = &loaded
					at.sltpExitSignalStateMu.Unlock()
					return loaded.LastActionLevel, loaded.LastExecutedLevel
				}
			}
		}
	}
	return 0, 0
}

// setSLTPExitExecutedLevel 在成功执行 scale_out/exit 后更新已执行等级；level=3 时清除该仓位状态便于下次开仓重新累计
func (at *AutoTrader) setSLTPExitExecutedLevel(posKey string, level int) {
	var (
		stateJSON string
		sym       string
		side      string
	)
	at.sltpExitSignalStateMu.Lock()
	st := at.sltpExitSignalState[posKey]
	if st != nil {
		st.LastExecutedLevel = level
		if level >= 3 {
			delete(at.sltpExitSignalState, posKey)
		} else if b, err := json.Marshal(*st); err == nil {
			stateJSON = string(b)
		}
	}
	at.sltpExitSignalStateMu.Unlock()

	// Persist executed level update (best-effort)
	if at.store != nil && posKey != "" {
		parts := strings.SplitN(posKey, "_", 2)
		if len(parts) == 2 {
			sym = parts[0]
			side = strings.ToUpper(parts[1])
			if level >= 3 {
				_ = at.store.Position().SetSLTPExitSignalStateAndTimeBySymbol(at.id, sym, side, "", time.Now().UTC().UnixMilli())
			} else if stateJSON != "" {
				_ = at.store.Position().SetSLTPExitSignalStateAndTimeBySymbol(at.id, sym, side, stateJSON, time.Now().UTC().UnixMilli())
			}
		}
	}
}

// checkDynamicStopLossTakeProfit checks all positions for dynamic stop loss and take profit triggers.
// 止盈/止损一旦触发，系统必须执行，与是否有 AI 预测缓存无关（开仓才要求「AI 预测 + 实时」）。
// 实盘与实盘模拟共用：positions 来自 at.trader.GetPositions()（交易所或 PaperTrader），后续判断与执行逻辑一致。
// trendViewByPosKey: optional. When non-nil, key = symbol_side (e.g. "BTCUSDT_long"); value = "trend_intact"|"choppy"|"reversing".
// Used to modulate SL confirm: reversing→faster stop; trend_intact/choppy→one more confirm to reduce oscillation wash.
// globalScenario: optional. When equals \"reversal\" and ScenarioAdjustEnabled is true, one more confirm cycle is removed to react faster to true reversals.
func (at *AutoTrader) checkDynamicStopLossTakeProfit(trendViewByPosKey map[string]string, globalScenario string) error {
	// Check if dynamic stop loss/take profit is enabled
	if at.strategyEngine == nil || at.strategyEngine.GetConfig() == nil {
		return nil
	}

	riskConfig := at.strategyEngine.GetConfig().RiskControl
	baseStopLossConfig := riskConfig.DynamicStopLoss
	baseTakeProfitConfig := riskConfig.DynamicTakeProfit

	// Skip if both are disabled
	if (baseStopLossConfig == nil || !baseStopLossConfig.Enabled) && (baseTakeProfitConfig == nil || !baseTakeProfitConfig.Enabled) {
		return nil
	}

	// Get current positions（按截图「实时价格更好掌握盈亏」：每次检查时拉取最新持仓与 mark 价；RealtimePrice.FetchBeforeSLTPCheck 启用时本处即为 SL/TP 判断前的一次拉取）
	positions, err := at.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	if len(positions) == 0 {
		return nil // No positions to check
	}

	// helpers: per-position profile override (AI participates, system enforces bounds)
	deepCopyTrailingLevels := func(in []store.TrailingStopLevel) []store.TrailingStopLevel {
		if len(in) == 0 {
			return nil
		}
		out := make([]store.TrailingStopLevel, len(in))
		copy(out, in)
		return out
	}
	deepCopyScaledLevels := func(in []store.ScaledTakeProfitLevel) []store.ScaledTakeProfitLevel {
		if len(in) == 0 {
			return nil
		}
		out := make([]store.ScaledTakeProfitLevel, len(in))
		copy(out, in)
		return out
	}
	applyTrailAgg := func(sl *store.DynamicStopLossConfig, agg string) {
		if sl == nil || sl.TrailingEnabled == nil || !*sl.TrailingEnabled || len(sl.TrailingLevels) == 0 {
			return
		}
		m := 1.0
		switch strings.TrimSpace(strings.ToLower(agg)) {
		case "high":
			m = 0.8 // tighter trailing (more aggressive)
		case "low":
			m = 1.2 // looser trailing
		case "medium", "":
			m = 1.0
		default:
			m = 1.0
		}
		for i := range sl.TrailingLevels {
			sl.TrailingLevels[i].TrailingPercent = sl.TrailingLevels[i].TrailingPercent * m
		}
	}

	boolPtr := func(v bool) *bool { return &v }

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

		// Get position update time (posKey 须与 runSLTPAnalysisCycle/平仓清理等一致：Normalize(symbol)_side)
		// 优先用持仓 map 的 update_time（纸面恢复后 GetPositions 会带真实入场时间），否则查 DB，避免重启后被误判为刚开仓导致最小持仓跳过、分层止盈不触发
		posKey := market.Normalize(symbol) + "_" + side
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

		// Effective SL/TP configs per-position (AI profiles + trail aggressiveness; system-enforced)
		effSL := (*store.DynamicStopLossConfig)(nil)
		effTP := (*store.DynamicTakeProfitConfig)(nil)
		if baseStopLossConfig != nil && baseStopLossConfig.Enabled {
			cpy := *baseStopLossConfig
			cpy.TrailingLevels = deepCopyTrailingLevels(baseStopLossConfig.TrailingLevels)
			effSL = &cpy
		}
		if baseTakeProfitConfig != nil && baseTakeProfitConfig.Enabled {
			cpy := *baseTakeProfitConfig
			cpy.ScaledLevels = deepCopyScaledLevels(baseTakeProfitConfig.ScaledLevels)
			effTP = &cpy
		}
		at.positionProfileMu.RLock()
		tpName := strings.TrimSpace(strings.ToLower(at.positionTPProfile[posKey]))
		slName := strings.TrimSpace(strings.ToLower(at.positionSLProfile[posKey]))
		trailAgg := strings.TrimSpace(strings.ToLower(at.positionTrailAgg[posKey]))
		at.positionProfileMu.RUnlock()
		if at.strategyEngine != nil && at.strategyEngine.GetConfig() != nil {
			rc := at.strategyEngine.GetConfig().RiskControl
			if tpName != "" && effTP != nil && rc.TPProfiles != nil {
				if prof, ok := rc.TPProfiles[tpName]; ok && prof.Enabled {
					if len(prof.ScaledLevels) > 0 {
						effTP.ScaledLevels = deepCopyScaledLevels(prof.ScaledLevels)
						if effTP.ScaledEnabled == nil {
							effTP.ScaledEnabled = boolPtr(true)
						} else {
							*effTP.ScaledEnabled = true
						}
					}
					if prof.MinHoldMinutes > 0 {
						effTP.MinHoldMinutes = prof.MinHoldMinutes
					}
				}
			}
			if slName != "" && effSL != nil && rc.SLProfiles != nil {
				if prof, ok := rc.SLProfiles[slName]; ok && prof.Enabled {
					effSL.MinHoldMinutes = prof.MinHoldMinutes
					effSL.ConfirmCycles = prof.ConfirmCycles
					effSL.ATREnabled = prof.ATREnabled
					effSL.ATRMultiplierMin = prof.ATRMultiplierMin
					effSL.ATRMultiplierMax = prof.ATRMultiplierMax
					effSL.ATRToleranceEnabled = prof.ATRToleranceEnabled
					effSL.ATRHighMultiplier = prof.ATRHighMultiplier
					effSL.TrailingEnabled = prof.TrailingEnabled
					effSL.TrailingLevels = deepCopyTrailingLevels(prof.TrailingLevels)
					effSL.AdverseExitWhenNeverProfitATR = prof.AdverseExitWhenNeverProfitATR
					effSL.TrailingStopOnlyAfterFirstScaledTP = prof.TrailingStopOnlyAfterFirstScaledTP
				}
			}
		}
		if effSL != nil {
			applyTrailAgg(effSL, trailAgg)
		}
		// 应用持仓期间 AI 调节（震荡持仓、抓住趋势、锁住利润）；系统边界内生效，不直接平仓
		at.positionSLTPAdjustmentMu.RLock()
		adj, hasAdj := at.positionSLTPAdjustment[posKey]
		at.positionSLTPAdjustmentMu.RUnlock()
		if hasAdj {
			// update structured exit signal state (confirmation+hysteresis). Action mapping is handled later in this function.
			_, _ = at.updateSLTPExitSignalState(posKey, adj.PhaseLabel, adj.ExitBias, adj.InvalidationLevel, adj.Rationale, adj.InvalidationStrength)
			if adj.TrailAggressiveness != "" {
				applyTrailAgg(effSL, strings.TrimSpace(strings.ToLower(adj.TrailAggressiveness)))
			}
			if effSL != nil && adj.ATRMultSL > 0 {
				minM := 1.2
				maxM := 3.0
				if effSL.ATRMultiplierMin != nil {
					minM = *effSL.ATRMultiplierMin
				}
				if effSL.ATRMultiplierMax != nil {
					maxM = *effSL.ATRMultiplierMax
				}
				mult := adj.ATRMultSL
				if mult < minM {
					mult = minM
				}
				if mult > maxM {
					mult = maxM
				}
				effSL.ATRMultiplierMin = &mult
				effSL.ATRMultiplierMax = &mult
			}
			if effTP != nil && adj.LockProfitPct > 0 {
				pct := adj.LockProfitPct
				if pct < 1 {
					pct = 1
				}
				if pct > 5 {
					pct = 5
				}
				effTP.LockProfitPercent = &pct
			}
		}
		var stopLossChecker *kernel.StopLossChecker
		var takeProfitChecker *kernel.TakeProfitChecker
		if effSL != nil && effSL.Enabled {
			stopLossChecker = kernel.NewStopLossChecker(effSL)
		}
		if effTP != nil && effTP.Enabled {
			takeProfitChecker = kernel.NewTakeProfitChecker(effTP)
		}

		// Min hold: skip dynamic SL/TP if position opened too recently (same as backtest; 避免开仓即平仓)
		nowMs := time.Now().UTC().UnixMilli()
		holdDurationMs := nowMs - updateTime
		minHoldMs := 0.0
		if effSL != nil && effSL.Enabled && effSL.MinHoldMinutes > 0 {
			minHoldMs = effSL.MinHoldMinutes * 60 * 1000
		}
		if effTP != nil && effTP.Enabled && effTP.MinHoldMinutes > 0 {
			tpMin := effTP.MinHoldMinutes * 60 * 1000
			if tpMin > minHoldMs {
				minHoldMs = tpMin
			}
		}
		if minHoldMs > 0 && float64(holdDurationMs) < minHoldMs {
			// Allow emergency exit (signal level=3) to bypass MinHold; other actions remain blocked.
			actionLevel, lastExecutedLevel := at.getSLTPExitState(posKey)
			if hasAdj && actionLevel >= 3 && lastExecutedLevel < 3 {
				closeReason := "system:signal:exit"
				signalReason := fmt.Sprintf("Structural exit signal bypass MinHold: %s (level %d)", closeReason, actionLevel)
				triggerDetail := buildSignalExitDetail(signalReason, &adj)
				at.setPendingCloseReason(closeReason)
				if at.store != nil {
					normalizedSymbol := market.Normalize(symbol)
					sideStr := strings.ToUpper(side)
					if err := at.store.Position().SetPendingCloseReasonAndDetailBySymbol(at.id, normalizedSymbol, sideStr, closeReason, true, triggerDetail); err != nil {
						logger.Infof("  ⚠️ Failed to pre-set close reason (signal bypass minhold): %v", err)
					}
				}
				action := "close_long"
				if side == "short" {
					action = "close_short"
				}
				decision := kernel.Decision{Symbol: symbol, Action: action, Reasoning: signalReason}
				actionRecord := store.DecisionAction{
					Action:    action,
					Symbol:    symbol,
					Reasoning: decision.Reasoning,
					Timestamp: time.Now().UTC(),
					Success:   false,
				}
				if err := at.executeDecisionWithRecord(&decision, &actionRecord, 0); err != nil {
					logger.Infof("❌ Failed to execute signal exit (bypass MinHold) for %s: %v", symbol, err)
				} else {
					at.setSLTPExitExecutedLevel(posKey, 3)
					at.recordStrategyTriggeredClose(kernel.StrategyTriggeredClose{
						Symbol: symbol, Side: side, Reason: signalReason, Price: markPrice,
					})
					logger.Infof("✓ Signal exit executed (bypass MinHold) for %s %s", symbol, side)
					continue
				}
			}
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

		// Use effective per-position configs for the rest of this loop
		stopLossConfig := effSL
		takeProfitConfig := effTP

		// 结构化退场信号：若状态机给出 scale_out(2) 或 exit(3)，且尚未执行到该等级，则执行减仓/全平并写交易历史
		actionLevel, lastExecutedLevel := at.getSLTPExitState(posKey)
		if hasAdj && actionLevel >= 2 && lastExecutedLevel < actionLevel {
			if actionLevel == 2 && at.shouldCooldownPartialClose(posKey, "signal_scale_out") {
				logger.Infof("⏳ TP/SL: skip scale_out due to cooldown for %s %s", symbol, side)
			} else {
			closeReason := "system:signal:scale_out"
			if actionLevel >= 3 {
				closeReason = "system:signal:exit"
			}
			signalReason := fmt.Sprintf("Structural exit signal: %s (level %d)", closeReason, actionLevel)
			triggerDetail := buildSignalExitDetail(signalReason, &adj)
			at.setPendingCloseReason(closeReason)
			if at.store != nil {
				normalizedSymbol := market.Normalize(symbol)
				sideStr := strings.ToUpper(side)
				if err := at.store.Position().SetPendingCloseReasonAndDetailBySymbol(at.id, normalizedSymbol, sideStr, closeReason, true, triggerDetail); err != nil {
					logger.Infof("  ⚠️ Failed to pre-set close reason (signal): %v", err)
				}
			}
			action := "close_long"
			if side == "short" {
				action = "close_short"
			}
			decision := kernel.Decision{
				Symbol:     symbol,
				Action:     action,
				Reasoning:  signalReason,
			}
			if actionLevel == 2 {
				// scale_out: 减仓 50%
				decision.CloseQuantity = quantity * 0.5
				if decision.CloseQuantity <= 0 || decision.CloseQuantity >= quantity {
					decision.CloseQuantity = 0
				}
			}
			actionRecord := store.DecisionAction{
				Action:    action,
				Symbol:    symbol,
				Reasoning: decision.Reasoning,
				Timestamp: time.Now().UTC(),
				Success:   false,
			}
			if err := at.executeDecisionWithRecord(&decision, &actionRecord, 0); err != nil {
				logger.Infof("❌ Failed to execute signal exit for %s (level %d): %v", symbol, actionLevel, err)
			} else {
				at.setSLTPExitExecutedLevel(posKey, actionLevel)
				at.recordStrategyTriggeredClose(kernel.StrategyTriggeredClose{
					Symbol: symbol, Side: side, Reason: signalReason, Price: markPrice,
				})
				logger.Infof("✓ Signal exit executed for %s %s: %s", symbol, side, closeReason)
				continue
			}
			}
		}

		// Get market data for ATR and support/resistance calculations (timeframe from config, default 15m)
		tf := "15m"
		if stopLossConfig != nil && stopLossConfig.KlinesTimeframe != "" {
			tf = stopLossConfig.KlinesTimeframe
			if tf != "15m" && tf != "1h" {
				tf = "15m"
			}
		}
		klines, err := at.marketClient.GetKlines(symbol, tf, 50)
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

		// Find support/resistance levels (optionally use EMA20 as structure level)
		supportLevel := kernel.FindSupportLevel(klines, markPrice, 30)
		resistanceLevel := kernel.FindResistanceLevel(klines, markPrice, 30)
		if stopLossConfig != nil && stopLossConfig.SupportResistanceUseEMA20 != nil && *stopLossConfig.SupportResistanceUseEMA20 {
			ema20 := kernel.CalculateEMA(klines, 20)
			if ema20 > 0 {
				// Long: support = EMA20 (break below = stop); Short: resistance = EMA20 (break above = stop)
				if side == "long" {
					supportLevel = ema20
				} else {
					resistanceLevel = ema20
				}
			}
		}

		// 锁定利润阈值：达到此盈利后移动止损到盈亏平衡点（策略中 LockProfitPercent）
		breakevenLocked := takeProfitConfig != nil && takeProfitConfig.LockProfitPercent != nil &&
			*takeProfitConfig.LockProfitPercent > 0 && pnlPct >= *takeProfitConfig.LockProfitPercent
		if breakevenLocked {
			// 价格已回到盈亏平衡点下方（多）或上方（空）→ 按盈亏平衡止损
			var breakevenDetail string
			if hasAdj {
				breakevenDetail = buildSLTPTriggerDetail("sl", "breakeven", entryPrice, &adj)
			}
			if side == "long" && markPrice <= entryPrice {
				signal := &kernel.StopLossSignal{Triggered: true, Reason: "Breakeven stop (profit locked)", Price: entryPrice, Type: "breakeven"}
				kernel.LogStopLossCheck(symbol, signal)
				if err := at.executeStopLoss(&positionInfo, signal, hasAdj, breakevenDetail); err != nil {
					logger.Infof("❌ Failed to execute breakeven stop for %s: %v", symbol, err)
				}
				continue
			}
			if side == "short" && markPrice >= entryPrice {
				signal := &kernel.StopLossSignal{Triggered: true, Reason: "Breakeven stop (profit locked)", Price: entryPrice, Type: "breakeven"}
				kernel.LogStopLossCheck(symbol, signal)
				if err := at.executeStopLoss(&positionInfo, signal, hasAdj, breakevenDetail); err != nil {
					logger.Infof("❌ Failed to execute breakeven stop for %s: %v", symbol, err)
				}
				continue
			}
		}

		// 先检查止盈、再检查止损：同一价位同时满足时优先兑现利润，避免「本可止盈却被追踪止损平仓」
		if takeProfitChecker != nil {
			scaledTaken := at.getScaledLevelsTaken(posKey)
			logger.Infof("📋 TP check %s %s: pnl=%.2f%%, entry=%.4f mark=%.4f, scaled_levels_taken=%d",
				symbol, side, pnlPct, entryPrice, markPrice, len(scaledTaken))
			tpLevel := resistanceLevel
			if side == "short" {
				tpLevel = supportLevel
			}
			signal := takeProfitChecker.CheckTakeProfit(&positionInfo, markPrice, highestPrice, atr, atrLong, tpLevel)
			if !signal.Triggered {
				at.tpTrailingFirstTriggeredAtMu.Lock()
				delete(at.tpTrailingFirstTriggeredAt, posKey)
				at.tpTrailingFirstTriggeredAtMu.Unlock()
			} else {
				if signal.Type == "trailing_tp" && takeProfitConfig.TrailingTPConfirmMinutes > 0 {
					at.tpTrailingFirstTriggeredAtMu.Lock()
					firstAt, ok := at.tpTrailingFirstTriggeredAt[posKey]
					if !ok {
						at.tpTrailingFirstTriggeredAt[posKey] = time.Now().UTC().UnixMilli()
						at.tpTrailingFirstTriggeredAtMu.Unlock()
						continue
					}
					elapsedMs := time.Now().UTC().UnixMilli() - firstAt
					requiredMs := int64(takeProfitConfig.TrailingTPConfirmMinutes * 60 * 1000)
					at.tpTrailingFirstTriggeredAtMu.Unlock()
					if elapsedMs < requiredMs {
						continue
					}
					at.tpTrailingFirstTriggeredAtMu.Lock()
					delete(at.tpTrailingFirstTriggeredAt, posKey)
					at.tpTrailingFirstTriggeredAtMu.Unlock()
				}
				kernel.LogTakeProfitCheck(symbol, signal)
				var tpDetail string
				if hasAdj {
					tpDetail = buildSLTPTriggerDetail("tp", signal.Type, signal.Price, &adj)
				}
				if err := at.executeTakeProfit(&positionInfo, signal, hasAdj, tpDetail); err != nil {
					logger.Infof("❌ Failed to execute take profit for %s: %v", symbol, err)
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
				confirmMinutes := 0.0
				if stopLossConfig.ConfirmMinutes > 0 {
					confirmMinutes = stopLossConfig.ConfirmMinutes
				}
				if confirmMinutes > 0 {
					// Confirm by real time: require condition to hold for ConfirmMinutes
					nowMs := time.Now().UTC().UnixMilli()
					at.slFirstTriggeredAtMu.Lock()
					firstMs := at.slFirstTriggeredAt[posKey]
					if firstMs == 0 {
						at.slFirstTriggeredAt[posKey] = nowMs
						firstMs = nowMs
					}
					elapsedMin := float64(nowMs-firstMs) / 60000.0
					at.slFirstTriggeredAtMu.Unlock()
					if elapsedMin >= confirmMinutes {
						kernel.LogStopLossCheck(symbol, signal)
						var slDetail string
						if hasAdj {
							slDetail = buildSLTPTriggerDetail("sl", signal.Type, signal.Price, &adj)
						}
						if err := at.executeStopLoss(&positionInfo, signal, hasAdj, slDetail); err != nil {
							logger.Infof("❌ Failed to execute stop loss for %s: %v", symbol, err)
						}
						at.slFirstTriggeredAtMu.Lock()
						delete(at.slFirstTriggeredAt, posKey)
						at.slFirstTriggeredAtMu.Unlock()
						continue
					}
					continue
				}
				// Confirm by cycles: execute only after N consecutive cycles with SL condition met
				requiredCycles := 1
				if stopLossConfig.ConfirmCycles > 0 {
					requiredCycles = stopLossConfig.ConfirmCycles
				}
				if hasAdj {
					requiredCycles += adj.ConfirmCyclesDelta
					if requiredCycles < 1 {
						requiredCycles = 1
					}
					if requiredCycles > 3 {
						requiredCycles = 3
					}
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
				// AI trend view: modulate confirm to balance "early cut" vs "oscillation wash"
				if trendViewByPosKey != nil {
					if tv, ok := trendViewByPosKey[posKey]; ok {
						switch tv {
						case "reversing":
							if requiredCycles > 1 {
								requiredCycles--
								logger.Infof("📋 SL confirm: %s %s trend_view=reversing → requiredCycles %d", symbol, side, requiredCycles)
							}
						case "trend_intact", "choppy":
							requiredCycles++
							logger.Infof("📋 SL confirm: %s %s trend_view=%s → requiredCycles %d", symbol, side, tv, requiredCycles)
						}
					}
				}
				// Global scenario: when scenario indicates reversal and config allows adjustment, further reduce confirm cycles by 1 (min 1)
				if stopLossConfig.ScenarioAdjustEnabled != nil && *stopLossConfig.ScenarioAdjustEnabled {
					if globalScenario == "reversal" && requiredCycles > 1 {
						requiredCycles--
						logger.Infof("📋 SL confirm: %s %s scenario=reversal → requiredCycles %d", symbol, side, requiredCycles)
					}
				}
				// ConfirmCycles counts only in AI main cycle to avoid double-counting; system cycle is expected to use ConfirmMinutes.
				if trendViewByPosKey == nil {
					continue
				}
				at.slConfirmCountMu.Lock()
				at.slConfirmCount[posKey]++
				count := at.slConfirmCount[posKey]
				at.slConfirmCountMu.Unlock()
				if count >= requiredCycles {
					kernel.LogStopLossCheck(symbol, signal)
					var slDetail string
					if hasAdj {
						slDetail = buildSLTPTriggerDetail("sl", signal.Type, signal.Price, &adj)
					}
					if err := at.executeStopLoss(&positionInfo, signal, hasAdj, slDetail); err != nil {
						logger.Infof("❌ Failed to execute stop loss for %s: %v", symbol, err)
					}
					at.slConfirmCountMu.Lock()
					delete(at.slConfirmCount, posKey)
					at.slConfirmCountMu.Unlock()
					continue
				}
				continue
			}
			// Condition not triggered: reset consecutive count and first-trigger time
			at.slConfirmCountMu.Lock()
			delete(at.slConfirmCount, posKey)
			at.slConfirmCountMu.Unlock()
			at.slFirstTriggeredAtMu.Lock()
			delete(at.slFirstTriggeredAt, posKey)
			at.slFirstTriggeredAtMu.Unlock()
		}
	}

	// Clean up slConfirmCount and slFirstTriggeredAt for positions that no longer exist
	at.slConfirmCountMu.Lock()
	for k := range at.slConfirmCount {
		if !currentPositionKeys[k] {
			delete(at.slConfirmCount, k)
		}
	}
	at.slConfirmCountMu.Unlock()
	at.slFirstTriggeredAtMu.Lock()
	for k := range at.slFirstTriggeredAt {
		if !currentPositionKeys[k] {
			delete(at.slFirstTriggeredAt, k)
		}
	}
	at.slFirstTriggeredAtMu.Unlock()
	at.tpTrailingFirstTriggeredAtMu.Lock()
	for k := range at.tpTrailingFirstTriggeredAt {
		if !currentPositionKeys[k] {
			delete(at.tpTrailingFirstTriggeredAt, k)
		}
	}
	at.tpTrailingFirstTriggeredAtMu.Unlock()

	return nil
}

// buildSLTPTriggerDetail 构建平仓触发详情 JSON，供交易历史展示「是否由 AI 调节参数触发」及数值
func buildSLTPTriggerDetail(trigger, signalType string, triggerPrice float64, adj *kernel.PositionSLTPAdjustment) string {
	m := map[string]interface{}{
		"trigger":        trigger,
		"type":           signalType,
		"trigger_price":  triggerPrice,
	}
	if adj != nil {
		if adj.TrailAggressiveness != "" {
			m["trail_aggressiveness"] = adj.TrailAggressiveness
		}
		if adj.ATRMultSL > 0 {
			m["atr_mult_sl"] = adj.ATRMultSL
		}
		if adj.LockProfitPct > 0 {
			m["lock_profit_pct"] = adj.LockProfitPct
		}
		if adj.Advice != "" {
			m["advice"] = adj.Advice
		}
		if adj.PhaseLabel != "" {
			m["phase_label"] = adj.PhaseLabel
		}
		if adj.InvalidationStrength > 0 {
			m["invalidation_strength"] = adj.InvalidationStrength
		}
		if adj.InvalidationLevel != "" {
			m["invalidation_level"] = adj.InvalidationLevel
		}
		if adj.ExitBias != "" {
			m["exit_bias"] = adj.ExitBias
		}
		if adj.Rationale != "" {
			m["rationale"] = adj.Rationale
		}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// buildSignalExitDetail 构建结构化信号退场（scale_out/exit）的详情 JSON
func buildSignalExitDetail(signalReason string, adj *kernel.PositionSLTPAdjustment) string {
	m := map[string]interface{}{
		"trigger": "signal",
		"reason":  signalReason,
	}
	if adj != nil {
		if adj.PhaseLabel != "" {
			m["phase_label"] = adj.PhaseLabel
		}
		if adj.InvalidationStrength > 0 {
			m["invalidation_strength"] = adj.InvalidationStrength
		}
		if adj.InvalidationLevel != "" {
			m["invalidation_level"] = adj.InvalidationLevel
		}
		if adj.ExitBias != "" {
			m["exit_bias"] = adj.ExitBias
		}
		if adj.Rationale != "" {
			m["rationale"] = adj.Rationale
		}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// executeStopLoss executes stop loss for a position.
// aiAdjusted and triggerDetail: 若本次触发使用了 AI 调节参数，传入 true 与详情 JSON，供历史展示
func (at *AutoTrader) executeStopLoss(position *kernel.PositionInfo, signal *kernel.StopLossSignal, aiAdjusted bool, triggerDetail string) error {
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

	if at.store != nil {
		normalizedSymbol := market.Normalize(position.Symbol)
		side := strings.ToUpper(position.Side)
		if aiAdjusted && triggerDetail != "" {
			if err := at.store.Position().SetPendingCloseReasonAndDetailBySymbol(at.id, normalizedSymbol, side, closeReason, true, triggerDetail); err != nil {
				logger.Infof("  ⚠️ Failed to pre-set close reason (AI adj): %v", err)
			}
		} else if err := at.store.Position().SetPendingCloseReasonBySymbol(at.id, normalizedSymbol, side, closeReason); err != nil {
			logger.Infof("  ⚠️ Failed to pre-set close reason: %v", err)
		}
	}

	if err := at.executeDecisionWithRecord(&decision, &actionRecord, 0); err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	at.recordStrategyTriggeredClose(kernel.StrategyTriggeredClose{
		Symbol: position.Symbol, Side: position.Side, Reason: signal.Reason, Price: signal.Price,
	})
	logger.Infof("✓ Stop loss executed successfully for %s", position.Symbol)
	return nil
}

// executeTakeProfit executes take profit for a position (full or partial, same as backtest).
// aiAdjusted and triggerDetail: 若本次触发使用了 AI 调节参数，传入 true 与详情 JSON，供历史展示
func (at *AutoTrader) executeTakeProfit(position *kernel.PositionInfo, signal *kernel.TakeProfitSignal, aiAdjusted bool, triggerDetail string) error {
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

	// Cooldown for partial closes to avoid duplicate partial executions in short window (e.g., fast cycles / delayed sync).
	posKey := market.Normalize(position.Symbol) + "_" + strings.ToLower(position.Side)
	if decision.CloseQuantity > 0 && decision.CloseQuantity < position.Quantity {
		if at.shouldCooldownPartialClose(posKey, "tp_partial") {
			logger.Infof("⏳ TP: skip partial close due to cooldown for %s %s", position.Symbol, position.Side)
			return nil
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

	if at.store != nil {
		normalizedSymbol := market.Normalize(position.Symbol)
		side := strings.ToUpper(position.Side)
		if aiAdjusted && triggerDetail != "" {
			if err := at.store.Position().SetPendingCloseReasonAndDetailBySymbol(at.id, normalizedSymbol, side, closeReason, true, triggerDetail); err != nil {
				logger.Infof("  ⚠️ Failed to pre-set close reason (AI adj): %v", err)
			}
		} else if err := at.store.Position().SetPendingCloseReasonBySymbol(at.id, normalizedSymbol, side, closeReason); err != nil {
			logger.Infof("  ⚠️ Failed to pre-set close reason: %v", err)
		}
	}

	if err := at.executeDecisionWithRecord(&decision, &actionRecord, 0); err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	at.recordStrategyTriggeredClose(kernel.StrategyTriggeredClose{
		Symbol: position.Symbol, Side: position.Side, Reason: signal.Reason, Price: signal.Price,
	})
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

