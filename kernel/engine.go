package kernel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/monitor"
	"nofx/provider/coinglass"
	"nofx/provider/nofxos"
	"nofx/security"
	"nofx/store"
	"regexp"
	"strings"
	"time"
)

// ============================================================================
// Pre-compiled regular expressions (performance optimization)
// ============================================================================

var (
	// Safe regex: precisely match ```json code blocks
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	// 无 "json" 的代码块（部分模型如 deepseek-reasoner 只输出 ``` ... ```）
	reJSONFencePlain = regexp.MustCompile("(?is)```\\s*\\n?\\s*([\\s\\S]*?)\\s*```")
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// XML tag extraction (supports any characters in reasoning chain)
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
	reAnalysisTag  = regexp.MustCompile(`(?s)<analysis>(.*?)</analysis>`)
)

// ============================================================================
// AI API Call with Prompt Caching
// ============================================================================

// callAIWithCaching calls AI API with prompt caching enabled for system prompt.
// runID is optional (e.g. backtest run_id); when set, token usage is recorded per-run for GET /api/ai-usage?run_id=
func callAIWithCaching(mcpClient mcp.AIClient, systemPrompt, userPrompt, runID string) (string, error) {
	// Try to use the advanced Request API with caching
	// If the client doesn't support it, fall back to simple CallWithMessages

	// Build request with caching enabled for system prompt
	systemMsg := mcp.NewSystemMessageWithCache(systemPrompt)
	userMsg := mcp.NewUserMessage(userPrompt)

	req, buildErr := mcp.NewRequestBuilder().
		AddMessage(systemMsg).
		AddMessage(userMsg).
		WithStream(false). // 非流式：便于返回完整 usage（含 Prompt 缓存读取/创建），大模型已支持足够输出长度
		Build()
	if buildErr != nil {
		logger.Infof("⚠️  Request build failed, falling back to simple API: %v", buildErr)
		return mcpClient.CallWithMessages(systemPrompt, userPrompt)
	}
	req.RunID = runID

	// Try to call with Request API (supports caching)
	result, err := mcpClient.CallWithRequest(req)
	if err != nil {
		// If Request API fails, fall back to simple API
		logger.Infof("⚠️  Request API failed, falling back to simple API: %v", err)
		return mcpClient.CallWithMessages(systemPrompt, userPrompt)
	}
	
	return result, nil
}

// ============================================================================
// Type Definitions
// ============================================================================

// PositionInfo position information
type PositionInfo struct {
	Symbol             string    `json:"symbol"`
	Side               string    `json:"side"` // "long" or "short"
	EntryPrice         float64   `json:"entry_price"`
	MarkPrice          float64   `json:"mark_price"`
	Quantity           float64   `json:"quantity"`
	Leverage           int       `json:"leverage"`
	UnrealizedPnL      float64   `json:"unrealized_pnl"`
	UnrealizedPnLPct   float64   `json:"unrealized_pnl_pct"`
	PeakPnLPct         float64   `json:"peak_pnl_pct"` // Historical peak profit percentage
	LiquidationPrice   float64   `json:"liquidation_price"`
	MarginUsed         float64   `json:"margin_used"`
	UpdateTime         int64     `json:"update_time"` // Position update timestamp (milliseconds)
	ScaledLevelsTaken  []float64 `json:"scaled_levels_taken,omitempty"` // Profit percents already taken (for layered TP)
}

// AccountInfo account information
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // Account equity
	AvailableBalance float64 `json:"available_balance"` // Available balance
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized profit/loss
	TotalPnL         float64 `json:"total_pnl"`         // Total profit/loss
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total profit/loss percentage
	MarginUsed       float64 `json:"margin_used"`       // Used margin
	MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin usage rate
	PositionCount    int     `json:"position_count"`    // Number of positions
}

// CandidateCoin candidate coin (from coin pool)
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // Sources: "ai500" and/or "oi_top"
}

// OITopData open interest growth top data (for AI decision reference)
type OITopData struct {
	Rank              int     // OI Top ranking
	OIDeltaPercent    float64 // Open interest change percentage (1 hour)
	OIDeltaValue      float64 // Open interest change value
	PriceDeltaPercent float64 // Price change percentage
}

// TradingStats trading statistics (for AI input)
type TradingStats struct {
	TotalTrades    int     `json:"total_trades"`     // Total number of trades (closed)
	WinRate        float64 `json:"win_rate"`         // Win rate (%)
	ProfitFactor   float64 `json:"profit_factor"`    // Profit factor
	SharpeRatio    float64 `json:"sharpe_ratio"`     // Sharpe ratio
	TotalPnL       float64 `json:"total_pnl"`        // Total profit/loss
	AvgWin         float64 `json:"avg_win"`          // Average win
	AvgLoss        float64 `json:"avg_loss"`         // Average loss
	MaxDrawdownPct float64 `json:"max_drawdown_pct"` // Maximum drawdown (%)
}

// RecentOrder recently completed order (for AI input)
type RecentOrder struct {
	Symbol       string  `json:"symbol"`        // Trading pair
	Side         string  `json:"side"`          // long/short
	EntryPrice   float64 `json:"entry_price"`   // Entry price
	ExitPrice    float64 `json:"exit_price"`    // Exit price
	RealizedPnL  float64 `json:"realized_pnl"`  // Realized profit/loss
	PnLPct       float64 `json:"pnl_pct"`       // Profit/loss percentage
	EntryTime    string  `json:"entry_time"`    // Entry time
	ExitTime     string  `json:"exit_time"`     // Exit time
	HoldDuration string  `json:"hold_duration"` // Hold duration, e.g. "2h30m"
}

// StrategyTriggeredClose 本周期内由策略（动态止损/止盈）触发的平仓，供 AI 提示与决策记录
type StrategyTriggeredClose struct {
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	Reason string  `json:"reason"` // e.g. "initial_stop", "trailing_stop", "fixed_tp"
	Price  float64 `json:"price"`
}

// Context trading context (complete information passed to AI)
type Context struct {
	CurrentTime             string                             `json:"current_time"`
	RuntimeMinutes          int                                `json:"runtime_minutes"`
	CallCount               int                                `json:"call_count"`
	Flow                    string                             `json:"-"` // 数据统计用：主周期/系统周期/止盈止损调整 等
	TraderID                string                             `json:"-"` // 数据统计用：交易员 ID
	Account                 AccountInfo                        `json:"account"`
	Positions               []PositionInfo                     `json:"positions"`
	CandidateCoins          []CandidateCoin                    `json:"candidate_coins"`
	// 系统方向池判定结果（与 pipeline state 同步），供 AI 对齐三条件共振：仅当 AI 预测方向与系统方向一致且 suggest_open=true 才开仓
	DirectionPoolLong      []DirectionPoolItem                `json:"-"`
	DirectionPoolShort     []DirectionPoolItem                 `json:"-"`
	PromptVariant           string                             `json:"prompt_variant,omitempty"`
	StrategyTriggeredCloses []StrategyTriggeredClose            `json:"-"` // 本周期已由策略触发的平仓，写入 prompt 与决策记录
	TradingStats            *TradingStats                      `json:"trading_stats,omitempty"`
	RecentOrders            []RecentOrder                      `json:"recent_orders,omitempty"`
	MarketDataMap           map[string]*market.Data            `json:"-"`
	MultiTFMarket   map[string]map[string]*market.Data `json:"-"`
	OITopDataMap    map[string]*OITopData              `json:"-"`
	QuantDataMap    map[string]*QuantData              `json:"-"`
	OIRankingData      *nofxos.OIRankingData      `json:"-"` // Market-wide OI ranking data
	NetFlowRankingData *nofxos.NetFlowRankingData `json:"-"` // Market-wide fund flow ranking data
	PriceRankingData   *nofxos.PriceRankingData   `json:"-"` // Market-wide price gainers/losers
	// Binance derivatives (when enabled): 以币安为主增强市场判断
	BinanceLongShortMap map[string]*BinanceLongShortSnapshot `json:"-"` // symbol -> 多空比快照
	BinanceFundingMap   map[string]*BinanceFundingSnapshot   `json:"-"` // symbol -> 资金费率快照
	BinanceTakerMap     map[string]*BinanceTakerSnapshot      `json:"-"` // symbol -> Taker 买卖比快照
	// 数据补强
	BinanceFundingRateAvg8h map[string]float64  `json:"-"` // symbol -> 近 8h 资金费率均值（小数）
	FundingRateHistoryLast8 map[string][]float64 `json:"-"` // symbol -> 近 8 期资金费率（供 AI 看趋势，不增 K 线 token）
	BasisMap                map[string]float64 `json:"-"` // symbol -> 永续-现货价差百分比，如 0.05 表示 0.05%
	BTCDominancePct         float64            `json:"-"` // BTC 市值占比，如 54.5 表示 54.5%
	LiquidationAgg          *LiquidationAggSnapshot `json:"-"` // 近 1h/4h 强平聚合（来自 WS 或 CoinAnk）
	// Coinglass（经 KeyStore 中转）各类数据，用于数据补强与 AI 参考
	CoinglassOISummaries      []CoinglassOISummary    `json:"-"`
	CoinglassFundingMap      map[string]float64      `json:"-"` // symbol -> 多所/OI 加权最新资金费率（小数）
	CoinglassLongShortMap    map[string]*BinanceLongShortSnapshot `json:"-"` // 多所多空比（与 Binance 结构一致）
	CoinglassLiquidation     *LiquidationAggSnapshot `json:"-"` // 多所强平聚合（Source=coinglass）
	FearGreedValue           int                     `json:"-"` // 恐惧贪婪指数 0-100
	FearGreedClassification  string                 `json:"-"` // 如 Extreme Fear / Greed
	AltcoinSeasonIndex       int                     `json:"-"` // 山寨季指数 0-100
	ETFFlowBTCRecent         float64                 `json:"-"` // 近期 BTC ETF 净流入（美元）
	ETFFlowETHRecent         float64                 `json:"-"` // 近期 ETH ETF 净流入（美元）
	// Coinglass WSS 实时快照（channel -> 最新一条 JSON 字符串），可选；用于补强 AI 实时与预测
	CoinglassWSSSnapshot     map[string]string       `json:"-"`
	// Coinglass 鲸鱼指数 / CGDI（多空扩散）/ CDRI（衍生品风险）：最新值，用于方向池加成与 AI
	CoinglassWhaleIndex      float64                `json:"-"`
	CoinglassCGDI            float64                `json:"-"`
	CoinglassCDRI            float64                `json:"-"`
	// Coinglass 强平热力图关键价位摘要（一句），补强 scenario / key_levels
	CoinglassLiquidationKeyLevels string             `json:"-"`
	BTCETHLeverage     int                          `json:"-"`
	AltcoinLeverage int                                `json:"-"`
	Timeframes      []string                           `json:"-"`
}

// CoinglassOISummary 单币种 OI 汇总（来自 Coinglass 聚合 "All" 行，经 KeyStore 获取）
type CoinglassOISummary struct {
	Symbol          string  `json:"symbol"`
	OpenInterestUSD float64 `json:"open_interest_usd"`
	Change1hPct     float64 `json:"change_1h_pct"`
	Change4hPct     float64 `json:"change_4h_pct"`
}

// BinanceLongShortSnapshot 币安多空比快照（全账户 + 大户，供 AI 与过滤参考）
type BinanceLongShortSnapshot struct {
	LongShortRatio     float64 `json:"long_short_ratio"`      // 全账户多空比
	LongAccount        float64 `json:"long_account"`          // 多头账户占比
	ShortAccount       float64 `json:"short_account"`         // 空头账户占比
	TopLongShortRatio  float64 `json:"top_long_short_ratio"`  // 大户多空比（可选）
	TopLongAccount     float64 `json:"top_long_account"`
	TopShortAccount    float64 `json:"top_short_account"`
	Timestamp          int64   `json:"timestamp"`
}

// BinanceFundingSnapshot 币安资金费率快照
type BinanceFundingSnapshot struct {
	LastFundingRate float64 `json:"last_funding_rate"`
	NextFundingTime int64   `json:"next_funding_time"`
	MarkPrice       float64 `json:"mark_price"`
	Time            int64   `json:"time"`
}

// BinanceTakerSnapshot 币安 Taker 买卖比快照
type BinanceTakerSnapshot struct {
	BuySellRatio float64 `json:"buy_sell_ratio"` // >1 买多
	BuyVol       float64 `json:"buy_vol"`
	SellVol      float64 `json:"sell_vol"`
	Timestamp    int64   `json:"timestamp"`
}

// LiquidationAggSnapshot 近 1h/4h/24h 强平聚合（来自币安 WS 或 CoinAnk）
type LiquidationAggSnapshot struct {
	Long1hUSD   float64 `json:"long_1h_usd"`   // 近 1h 多单强平金额（美元）
	Short1hUSD  float64 `json:"short_1h_usd"`
	Long4hUSD   float64 `json:"long_4h_usd"`
	Short4hUSD  float64 `json:"short_4h_usd"`
	Long24hUSD  float64 `json:"long_24h_usd"`  // 近 24h（可选，WS 持续运行时有效）
	Short24hUSD float64 `json:"short_24h_usd"`
	Source      string  `json:"source"` // "binance_ws" | "coinank"
	UpdatedAt   int64   `json:"updated_at"`
}

// Decision AI trading decision
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // Standard: "open_long", "open_short", "close_long", "close_short", "hold", "wait"
	// Grid actions: "place_buy_limit", "place_sell_limit", "cancel_order", "cancel_all_orders", "pause_grid", "resume_grid", "adjust_grid"

	// Opening position parameters
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`
	// Position sizing & SL/TP profiles (AI participates, system enforces bounds)
	RiskBucket          string   `json:"risk_bucket,omitempty"`          // low | medium | high (system maps to equity ratio)
	TPProfile           string   `json:"tp_profile,omitempty"`           // tp_conservative | tp_balanced | tp_aggressive ...
	SLProfile           string   `json:"sl_profile,omitempty"`           // sl_tight | sl_normal | sl_loose ...
	TrailAggressiveness string   `json:"trail_aggressiveness,omitempty"` // low | medium | high
	KeyLevels           []string `json:"key_levels,omitempty"`           // per-symbol key levels (optional; compact)

	// Close position: when > 0, close this quantity (partial close); 0 = close all
	CloseQuantity float64 `json:"close_quantity,omitempty"`
	// ExitReason: when action is close_long/close_short，AI 应填写退出原因（结合历史+实时+预测，认为与预测不符时止盈/止损）。取值: take_profit | stop_loss | prediction_mismatch
	ExitReason string `json:"exit_reason,omitempty"`

	// Grid trading parameters
	Price      float64 `json:"price,omitempty"`       // Limit order price (for grid)
	Quantity   float64 `json:"quantity,omitempty"`    // Order quantity (for grid)
	LevelIndex int     `json:"level_index,omitempty"` // Grid level index
	OrderID    string  `json:"order_id,omitempty"`    // Order ID (for cancel)

	// Common parameters
	Confidence int     `json:"confidence,omitempty"` // Confidence level (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // Maximum USD risk
	Reasoning  string  `json:"reasoning"`

	// Holding-period trend view: only meaningful when action is "hold" for an existing position.
	// Used by strategy to modulate SL confirm (reversing→faster stop; trend_intact/choppy→one more confirm to reduce wash).
	TrendView string `json:"trend_view,omitempty"` // "trend_intact" | "choppy" | "reversing"
}

// FullDecision AI's complete decision (including chain of thought)
type FullDecision struct {
	SystemPrompt        string     `json:"system_prompt"`
	UserPrompt          string     `json:"user_prompt"`
	CoTTrace            string     `json:"cot_trace"`
	Decisions           []Decision `json:"decisions"`
	RawResponse         string     `json:"raw_response"`
	Timestamp           time.Time  `json:"timestamp"`
	AIRequestDurationMs int64      `json:"ai_request_duration_ms,omitempty"`
	// AI 仅作辅助时的四项输出（不参与开平仓执行，供分析与风控参考）
	MarketSummary string `json:"market_summary,omitempty"` // 本周期市场摘要
	MarketRegime  string `json:"market_regime,omitempty"`  // 市场状态：trend_up/trend_down/ranging/high_volatility/reversal
	RiskAlert     bool   `json:"risk_alert,omitempty"`     // 为 true 时本周期建议不新开仓
	// 预测性辅助输出：整体市场短期预期与关键位（不直接驱动执行，供策略与风控参考）
	NearTermOutlook string   `json:"near_term_outlook,omitempty"` // 未来 1～2 根 K 或本 session 的一句话预期
	Scenario        string   `json:"scenario,omitempty"`          // continuation | reversal | range 等整体情景
	KeyLevels       []string `json:"key_levels,omitempty"`        // 关键支撑/阻力位列表
	// SymbolPredictions: 按标的的预测（AIPredictOnly 模式下必填）；系统据此决定是否执行 pipeline 给出的开仓及动态参数
	SymbolPredictions []SymbolPrediction `json:"symbol_predictions,omitempty"`
	// SymbolStructureSignals: 结构化阶段/证伪信号（用于“趋势何时被证伪/何时应收紧/减仓/退出”），供系统执行层与 SL/TP 调整使用
	SymbolStructureSignals []SymbolStructureSignal `json:"symbol_structure_signals,omitempty"`
	// PositionSLTPAdjustments: 持仓期间 AI 建议的止盈/止损参数调节（震荡持仓、抓住趋势、锁住利润）；系统在边界内应用，不直接平仓
	PositionSLTPAdjustments []PositionSLTPAdjustment `json:"position_sl_tp_adjustments,omitempty"`
}

// SymbolPrediction 单标的预测（AIPredictOnly 时由 AI 输出，系统据此决策）
type SymbolPrediction struct {
	Symbol             string `json:"symbol"`                        // 标的，如 BTCUSDT
	PredictedDirection string `json:"predicted_direction,omitempty"` // up | down | neutral
	Confidence         int    `json:"confidence,omitempty"`         // 0-100
	SuggestExit        bool   `json:"suggest_exit,omitempty"`        // 是否建议该持仓平仓（系统可据此或仅用 TP/SL）
	SuggestOpen        *bool  `json:"suggest_open,omitempty"`        // 是否建议本周期开仓（与实时方向、AI预测方向三条件共振才开仓；nil 视为不要求）
}

// SymbolStructureSignal 为「结构证伪/阶段标签/退场倾向」提供结构化输出（不直接代表下单指令）
type SymbolStructureSignal struct {
	Symbol                string `json:"symbol"`                            // 标的，如 BTCUSDT
	PhaseLabel            string `json:"phase_label,omitempty"`              // trend | late_trend | range | high_vol | reversal_risk | neutral
	InvalidationLevel     string `json:"invalidation_level,omitempty"`       // 证伪条件/关键位（字符串表达，允许价格/区间/规则）
	InvalidationStrength  int    `json:"invalidation_strength,omitempty"`    // 0-100（越高越偏向退场）
	ExitBias              string `json:"exit_bias,omitempty"`                // hold | tighten | scale_out | exit
	Rationale             string `json:"rationale,omitempty"`                // 一句解释（供 UI/日志）
}

// AnalysisSnapshot 从 AI 原始响应中解析出的分析快照（供雷达/实时+AI 展示）
type AnalysisSnapshot struct {
	MarketRegime            string                   `json:"market_regime,omitempty"`
	Scenario                string                   `json:"scenario,omitempty"`
	MarketSummary           string                   `json:"market_summary,omitempty"`
	RiskAlert               bool                     `json:"risk_alert,omitempty"`
	SymbolPredictions       []SymbolPrediction       `json:"symbol_predictions,omitempty"`
	SymbolStructureSignals  []SymbolStructureSignal  `json:"symbol_structure_signals,omitempty"`
	PositionSLTPAdjustments []PositionSLTPAdjustment  `json:"position_sl_tp_adjustments,omitempty"`
}

// PositionSLTPAdjustment 持仓期间 AI 建议的止盈/止损参数调节（系统在边界内应用，不直接平仓）
// 规则：震荡持仓=放宽/多确认；抓住趋势=收紧追踪；锁住利润=建议锁利阈值
type PositionSLTPAdjustment struct {
	Symbol               string  `json:"symbol"`
	Side                 string  `json:"side"` // long | short
	Advice               string  `json:"advice,omitempty"`                 // choppy_hold | trend_ride | lock_profit
	TrailAggressiveness  string  `json:"trail_aggressiveness,omitempty"`   // low | medium | high
	ATRMultSL            float64 `json:"atr_mult_sl,omitempty"`            // 建议 ATR 止损乘数，系统裁剪到 [min,max]
	LockProfitPct        float64 `json:"lock_profit_pct,omitempty"`         // 建议达到该盈利%后锁利（移动止损到保本）
	ConfirmCyclesDelta   int     `json:"confirm_cycles_delta,omitempty"`   // 在基础 ConfirmCycles 上加减
	// 结构化退场信号（可选）：用于把“趋势结束点”表达成证伪条件与退场倾向；执行层会做确认+迟滞后再落地动作
	PhaseLabel           string  `json:"phase_label,omitempty"`            // trend | late_trend | range | high_vol | reversal_risk | neutral
	InvalidationLevel    string  `json:"invalidation_level,omitempty"`     // 如 "break_below 26250" / "lose_4h_ema20"
	InvalidationStrength int     `json:"invalidation_strength,omitempty"`  // 0-100
	ExitBias             string  `json:"exit_bias,omitempty"`              // hold | tighten | scale_out | exit
	Rationale            string  `json:"rationale,omitempty"`              // 一句解释
}

// PositionSLTPBaseline 系统策略下该仓位的原始参数（调节前的基线），用于展示「原始→调整后」
type PositionSLTPBaseline struct {
	Symbol              string  `json:"symbol"`
	Side                string  `json:"side"`
	TrailAggressiveness string  `json:"trail_aggressiveness,omitempty"`
	ATRMultSL           float64 `json:"atr_mult_sl,omitempty"`
	LockProfitPct       float64 `json:"lock_profit_pct,omitempty"`
	ConfirmCycles       int     `json:"confirm_cycles,omitempty"`
}

// QuantData quantitative data structure (fund flow, position changes, price changes)
type QuantData struct {
	Symbol      string             `json:"symbol"`
	Price       float64            `json:"price"`
	Netflow     *NetflowData       `json:"netflow,omitempty"`
	OI          map[string]*OIData `json:"oi,omitempty"`
	PriceChange map[string]float64 `json:"price_change,omitempty"`
}

type NetflowData struct {
	Institution *FlowTypeData `json:"institution,omitempty"`
	Personal    *FlowTypeData `json:"personal,omitempty"`
}

type FlowTypeData struct {
	Future map[string]float64 `json:"future,omitempty"`
	Spot   map[string]float64 `json:"spot,omitempty"`
}

type OIData struct {
	CurrentOI float64                 `json:"current_oi"`
	Delta     map[string]*OIDeltaData `json:"delta,omitempty"`
}

type OIDeltaData struct {
	OIDelta        float64 `json:"oi_delta"`
	OIDeltaValue   float64 `json:"oi_delta_value"`
	OIDeltaPercent float64 `json:"oi_delta_percent"`
}

// ============================================================================
// StrategyEngine - Core Strategy Execution Engine
// ============================================================================

// StrategyEngine strategy execution engine
type StrategyEngine struct {
	config          *store.StrategyConfig
	nofxosClient     *nofxos.Client
	coinglassClient *coinglass.Client
}

// NewStrategyEngine creates strategy execution engine
func NewStrategyEngine(config *store.StrategyConfig) *StrategyEngine {
	// Create NofxOS client with API key from config
	apiKey := config.Indicators.NofxOSAPIKey
	if apiKey == "" {
		apiKey = nofxos.DefaultAuthKey
	}
	client := nofxos.NewClient(nofxos.DefaultBaseURL, apiKey)

	// Coinglass（KeyStore 中转站）客户端：仅当启用且配置了 API Key 时创建
	var coinglassClient *coinglass.Client
	if config.Indicators.EnableCoinglassData && config.Indicators.CoinglassAPIKey != "" {
		limit := config.Indicators.CoinglassRateLimitPerMin
		if limit <= 0 {
			limit = coinglass.DefaultRateLimitPerMin
		}
		coinglassClient = coinglass.NewClient(config.Indicators.CoinglassProxyURL, config.Indicators.CoinglassAPIKey, limit)
	}

	return &StrategyEngine{
		config:          config,
		nofxosClient:    client,
		coinglassClient: coinglassClient,
	}
}

// GetRiskControlConfig gets risk control configuration
func (e *StrategyEngine) GetRiskControlConfig() store.RiskControlConfig {
	return e.config.RiskControl
}

// GetLanguage returns the language from config or falls back to auto-detection
func (e *StrategyEngine) GetLanguage() Language {
	switch e.config.Language {
	case "zh":
		return LangChinese
	case "en":
		return LangEnglish
	default:
		// Fall back to auto-detection from prompt content for backward compatibility
		return detectLanguage(e.config.PromptSections.RoleDefinition)
	}
}

// GetConfig gets complete strategy configuration
func (e *StrategyEngine) GetConfig() *store.StrategyConfig {
	return e.config
}

// FillCoinglassOIData 通过 KeyStore 中转站拉取 Coinglass OI 数据并写入 ctx.CoinglassOISummaries（用于数据补强与 AI）
func (e *StrategyEngine) FillCoinglassOIData(ctx *Context) {
	e.FillCoinglassData(ctx)
}

func recordCoinglass(ctx *Context, dataType string, err error, durationMs int64) {
	if ctx == nil {
		return
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	flow := ctx.Flow
	if flow == "" {
		flow = "主周期"
	}
	monitor.RecordDataCall("coinglass", dataType, flow, ctx.TraderID, err == nil, errMsg, durationMs)
}

// FillCoinglassData 拉取并填充 Coinglass 全部已接入数据：OI、资金费率、多空比、强平、恐惧贪婪、BTC 占比、山寨季、ETF 资金流。
// 按 ctx.Flow 错峰（系统周期/主周期/止盈止损调整），且受客户端每分钟请求数限速。
// 本函数为同步：仅当本流程内所有 Coinglass 请求均执行完毕（含限速等待）后才返回，调用方必须在返回后再将 ctx 用于系统分析或 AI，保证传入的是本周期完整拉取结果。
func (e *StrategyEngine) FillCoinglassData(ctx *Context) {
	if e.coinglassClient == nil || ctx == nil {
		return
	}
	e.coinglassClient.WaitFlowStagger(ctx.Flow)
	// 1. OI 汇总（BTC/ETH）
	for _, sym := range []string{"BTC", "ETH"} {
		t0 := time.Now()
		resp, err := e.coinglassClient.GetOpenInterestExchangeList(sym)
		recordCoinglass(ctx, "GetOpenInterestExchangeList", err, time.Since(t0).Milliseconds())
		if err != nil {
			logger.Warnf("Coinglass OI exchange-list %s: %v", sym, err)
			continue
		}
		for _, item := range resp.Data {
			if item.Exchange == "All" {
				ctx.CoinglassOISummaries = append(ctx.CoinglassOISummaries, CoinglassOISummary{
					Symbol:          item.Symbol,
					OpenInterestUSD: item.OpenInterestUSD,
					Change1hPct:     item.OpenInterestChangePercent1h,
					Change4hPct:     item.OpenInterestChangePercent4h,
				})
				break
			}
		}
	}
	// 2. 资金费率（exchange-list 多所，取 All 或均值）
	if ctx.CoinglassFundingMap == nil {
		ctx.CoinglassFundingMap = make(map[string]float64)
	}
	for _, sym := range []string{"BTC", "ETH"} {
		t0 := time.Now()
		frResp, err := e.coinglassClient.GetFundingRateExchangeList(sym)
		recordCoinglass(ctx, "GetFundingRateExchangeList", err, time.Since(t0).Milliseconds())
		if err != nil {
			logger.Warnf("Coinglass funding exchange-list %s: %v", sym, err)
			continue
		}
		for _, s := range frResp.Data {
			if s.Symbol != sym {
				continue
			}
			list := s.StablecoinMarginList
			if len(list) == 0 {
				list = s.TokenMarginList
			}
			if len(list) == 0 {
				continue
			}
			var sum float64
			for _, ex := range list {
				sum += ex.FundingRate
			}
			ctx.CoinglassFundingMap[sym] = sum / float64(len(list))
			break
		}
	}
	// 3. 多空比（global + top 最近一条）
	if ctx.CoinglassLongShortMap == nil {
		ctx.CoinglassLongShortMap = make(map[string]*BinanceLongShortSnapshot)
	}
	// 多所多空：先 BTC/ETH，再从候选取最多 8 个标的（补强方向池与过滤）
	symbolsForLS := []string{"BTC", "ETH"}
	seen := map[string]bool{"BTC": true, "ETH": true}
	for _, c := range ctx.CandidateCoins {
		if len(symbolsForLS) >= 8 {
			break
		}
		base := symbolToCoinglassBase(c.Symbol)
		if base != "" && !seen[base] {
			seen[base] = true
			symbolsForLS = append(symbolsForLS, base)
		}
	}
	if ctx.BinanceLongShortMap == nil {
		ctx.BinanceLongShortMap = make(map[string]*BinanceLongShortSnapshot)
	}
	for _, sym := range symbolsForLS {
		t0 := time.Now()
		gResp, err := e.coinglassClient.GetGlobalLongShortAccountRatioHistory(sym, "1h", 1)
		recordCoinglass(ctx, "GetGlobalLongShortAccountRatioHistory", err, time.Since(t0).Milliseconds())
		if err != nil {
			logger.Warnf("Coinglass global-long-short %s: %v", sym, err)
			continue
		}
		t0 = time.Now()
		tResp, err := e.coinglassClient.GetTopLongShortAccountRatioHistory(sym, "1h", 1)
		recordCoinglass(ctx, "GetTopLongShortAccountRatioHistory", err, time.Since(t0).Milliseconds())
		if err != nil {
			logger.Warnf("Coinglass top-long-short %s: %v", sym, err)
		}
		ls := &BinanceLongShortSnapshot{Timestamp: time.Now().UnixMilli()}
		if len(gResp.Data) > 0 {
			p := gResp.Data[len(gResp.Data)-1]
			ls.LongAccount = p.LongAccount
			ls.ShortAccount = p.ShortAccount
			if p.ShortAccount > 0 {
				ls.LongShortRatio = p.LongAccount / p.ShortAccount
			}
		}
		if tResp != nil && len(tResp.Data) > 0 {
			p := tResp.Data[len(tResp.Data)-1]
			ls.TopLongAccount = p.LongAccount
			ls.TopShortAccount = p.ShortAccount
			if p.ShortAccount > 0 {
				ls.TopLongShortRatio = p.LongAccount / p.ShortAccount
			}
		}
		ctx.CoinglassLongShortMap[sym] = ls
		// 同时写入 BinanceLongShortMap，key 用交易对（BTCUSDT）以便 pipeline/方向池按标的查找
		for _, pair := range []string{sym + "USDT", sym} {
			ctx.BinanceLongShortMap[pair] = ls
		}
	}
	// 4. 强平聚合（BTC 1h 最近 24 条：最后 1 条=1h，最后 4 条=4h，全部=24h）
	t0 := time.Now()
	liqResp, err := e.coinglassClient.GetLiquidationAggregatedHistory("BTC", "1h", 24)
	recordCoinglass(ctx, "GetLiquidationAggregatedHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass liquidation aggregated-history: %v", err)
	} else if len(liqResp.Data) > 0 {
		agg := &LiquidationAggSnapshot{Source: "coinglass", UpdatedAt: time.Now().UnixMilli()}
		last := liqResp.Data[len(liqResp.Data)-1]
		agg.Long1hUSD = last.LongVol
		agg.Short1hUSD = last.ShortVol
		for i := len(liqResp.Data) - 1; i >= 0 && i >= len(liqResp.Data)-4; i-- {
			agg.Long4hUSD += liqResp.Data[i].LongVol
			agg.Short4hUSD += liqResp.Data[i].ShortVol
		}
		for _, p := range liqResp.Data {
			agg.Long24hUSD += p.LongVol
			agg.Short24hUSD += p.ShortVol
		}
		ctx.CoinglassLiquidation = agg
	}
	// 5. 恐惧贪婪指数
	t0 = time.Now()
	fgResp, err := e.coinglassClient.GetFearGreedHistory(1)
	recordCoinglass(ctx, "GetFearGreedHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass fear-greed-history: %v", err)
	} else if len(fgResp.Data) > 0 {
		ctx.FearGreedValue = fgResp.Data[len(fgResp.Data)-1].Value
		ctx.FearGreedClassification = fgResp.Data[len(fgResp.Data)-1].ValueCN
		if ctx.FearGreedClassification == "" {
			ctx.FearGreedClassification = fearGreedLabel(ctx.FearGreedValue)
		}
	}
	// 6. BTC 市值占比（Coinglass 有则覆盖）
	t0 = time.Now()
	pct, errBD := e.coinglassClient.GetBitcoinDominance()
	recordCoinglass(ctx, "GetBitcoinDominance", errBD, time.Since(t0).Milliseconds())
	if errBD == nil && pct > 0 {
		ctx.BTCDominancePct = pct
	}
	// 7. 山寨季指数
	t0 = time.Now()
	altResp, err := e.coinglassClient.GetAltcoinSeasonHistory(1)
	recordCoinglass(ctx, "GetAltcoinSeasonHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass altcoin-season: %v", err)
	} else if len(altResp.Data) > 0 {
		ctx.AltcoinSeasonIndex = altResp.Data[len(altResp.Data)-1].AltcoinIndex
	}
	// 8. ETF 资金流（BTC/ETH 最近一条或近 24h 汇总）
	t0 = time.Now()
	btcFlow, err := e.coinglassClient.GetETFBitcoinFlowHistory(7)
	recordCoinglass(ctx, "GetETFBitcoinFlowHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass etf bitcoin flow-history: %v", err)
	} else if len(btcFlow.Data) > 0 {
		for _, p := range btcFlow.Data {
			ctx.ETFFlowBTCRecent += p.Flow
		}
		if ctx.ETFFlowBTCRecent == 0 {
			ctx.ETFFlowBTCRecent = btcFlow.Data[len(btcFlow.Data)-1].Flow
		}
	}
	t0 = time.Now()
	ethFlow, err := e.coinglassClient.GetETFEthereumFlowHistory(7)
	recordCoinglass(ctx, "GetETFEthereumFlowHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass etf ethereum flow-history: %v", err)
	} else if len(ethFlow.Data) > 0 {
		for _, p := range ethFlow.Data {
			ctx.ETFFlowETHRecent += p.Flow
		}
		if ctx.ETFFlowETHRecent == 0 {
			ctx.ETFFlowETHRecent = ethFlow.Data[len(ethFlow.Data)-1].Flow
		}
	}
	// 9. 鲸鱼指数 / CGDI（多空扩散）/ CDRI（衍生品风险）— 方向池与 AI 补强
	t0 = time.Now()
	whaleResp, err := e.coinglassClient.GetWhaleIndexHistory("Binance", "BTC", "1h", 1)
	recordCoinglass(ctx, "GetWhaleIndexHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass whale-index: %v", err)
	} else if len(whaleResp.Data) > 0 {
		ctx.CoinglassWhaleIndex = whaleResp.Data[len(whaleResp.Data)-1].WhaleIndexValue
	}
	t0 = time.Now()
	cgdiResp, err := e.coinglassClient.GetCGDIIndexHistory("BTC", 1)
	recordCoinglass(ctx, "GetCGDIIndexHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass cgdi-index: %v", err)
	} else if len(cgdiResp.Data) > 0 {
		ctx.CoinglassCGDI = cgdiResp.Data[len(cgdiResp.Data)-1].CGDIIndexValue
	}
	t0 = time.Now()
	cdriResp, err := e.coinglassClient.GetCDRIIndexHistory("BTC", 1)
	recordCoinglass(ctx, "GetCDRIIndexHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass cdri-index: %v", err)
	} else if len(cdriResp.Data) > 0 {
		ctx.CoinglassCDRI = cdriResp.Data[len(cdriResp.Data)-1].CDRIIndexValue
	}
	// 10. 强平热力图关键价位（补强 scenario / key_levels）
	t0 = time.Now()
	heatmapResp, err := e.coinglassClient.GetLiquidationAggregatedHeatmap("BTC", "24h")
	recordCoinglass(ctx, "GetLiquidationAggregatedHeatmap", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass liquidation aggregated-heatmap: %v", err)
	} else {
		ctx.CoinglassLiquidationKeyLevels = heatmapResp.HeatmapToKeyLevelsSummary("BTC", "24h")
	}
}

// FillCoinglassDataForSLTP 拉取止盈止损 5 种场景所需的完整 Coinglass 数据并写入 ctx，满足 AI 实时+预测：强平 1h/4h/24h、OI 变化、资金费率、多空比、恐惧贪婪、山寨季、CGDI、CDRI、鲸鱼指数、ETF 资金流、BTC 占比、强平热力图。
// 按 ctx.Flow 错峰，且受客户端每分钟请求数限速。
// 本函数为同步：仅当本流程内全部 Coinglass 请求执行完毕后才返回，调用方必须在返回后再用 ctx 生成 summary 或传入 AI，保证传入的是完整拉取结果。
func (e *StrategyEngine) FillCoinglassDataForSLTP(ctx *Context) {
	if e.coinglassClient == nil || ctx == nil {
		return
	}
	e.coinglassClient.WaitFlowStagger(ctx.Flow)
	// 1. 强平聚合（BTC 1h 最近 24 条：最后 1 条=1h，最后 4 条=4h，全部=24h）
	t0 := time.Now()
	liqResp, err := e.coinglassClient.GetLiquidationAggregatedHistory("BTC", "1h", 24)
	recordCoinglass(ctx, "GetLiquidationAggregatedHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP liquidation: %v", err)
	} else if len(liqResp.Data) > 0 {
		agg := &LiquidationAggSnapshot{Source: "coinglass", UpdatedAt: time.Now().UnixMilli()}
		last := liqResp.Data[len(liqResp.Data)-1]
		agg.Long1hUSD = last.LongVol
		agg.Short1hUSD = last.ShortVol
		for i := len(liqResp.Data) - 1; i >= 0 && i >= len(liqResp.Data)-4; i-- {
			agg.Long4hUSD += liqResp.Data[i].LongVol
			agg.Short4hUSD += liqResp.Data[i].ShortVol
		}
		for _, p := range liqResp.Data {
			agg.Long24hUSD += p.LongVol
			agg.Short24hUSD += p.ShortVol
		}
		ctx.CoinglassLiquidation = agg
	}
	// 2. OI 汇总（BTC，用于趋势强度/衰竭判断）
	for _, sym := range []string{"BTC"} {
		t0 = time.Now()
		resp, err := e.coinglassClient.GetOpenInterestExchangeList(sym)
		recordCoinglass(ctx, "GetOpenInterestExchangeList", err, time.Since(t0).Milliseconds())
		if err != nil {
			logger.Warnf("Coinglass SLTP OI %s: %v", sym, err)
			continue
		}
		for _, item := range resp.Data {
			if item.Exchange == "All" {
				ctx.CoinglassOISummaries = append(ctx.CoinglassOISummaries, CoinglassOISummary{
					Symbol:          item.Symbol,
					OpenInterestUSD: item.OpenInterestUSD,
					Change1hPct:     item.OpenInterestChangePercent1h,
					Change4hPct:     item.OpenInterestChangePercent4h,
				})
				break
			}
		}
	}
	// 3. 资金费率（BTC，多所）
	t0 = time.Now()
	frResp, err := e.coinglassClient.GetFundingRateExchangeList("BTC")
	recordCoinglass(ctx, "GetFundingRateExchangeList", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP funding BTC: %v", err)
	} else {
		if ctx.CoinglassFundingMap == nil {
			ctx.CoinglassFundingMap = make(map[string]float64)
		}
		for _, s := range frResp.Data {
			if s.Symbol != "BTC" {
				continue
			}
			list := s.StablecoinMarginList
			if len(list) == 0 {
				list = s.TokenMarginList
			}
			if len(list) == 0 {
				continue
			}
			var sum float64
			for _, ex := range list {
				sum += ex.FundingRate
			}
			ctx.CoinglassFundingMap["BTC"] = sum / float64(len(list))
			break
		}
	}
	// 4. 多空比（BTC，全账户+大户）
	t0 = time.Now()
	gResp, err := e.coinglassClient.GetGlobalLongShortAccountRatioHistory("BTC", "1h", 1)
	recordCoinglass(ctx, "GetGlobalLongShortAccountRatioHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP global-long-short BTC: %v", err)
	}
	t0 = time.Now()
	tResp, _ := e.coinglassClient.GetTopLongShortAccountRatioHistory("BTC", "1h", 1)
	recordCoinglass(ctx, "GetTopLongShortAccountRatioHistory", nil, time.Since(t0).Milliseconds())
	ls := &BinanceLongShortSnapshot{Timestamp: time.Now().UnixMilli()}
	if gResp != nil && len(gResp.Data) > 0 {
		p := gResp.Data[len(gResp.Data)-1]
		ls.LongAccount = p.LongAccount
		ls.ShortAccount = p.ShortAccount
		if p.ShortAccount > 0 {
			ls.LongShortRatio = p.LongAccount / p.ShortAccount
		}
	}
	if tResp != nil && len(tResp.Data) > 0 {
		p := tResp.Data[len(tResp.Data)-1]
		ls.TopLongAccount = p.LongAccount
		ls.TopShortAccount = p.ShortAccount
		if p.ShortAccount > 0 {
			ls.TopLongShortRatio = p.LongAccount / p.ShortAccount
		}
	}
	if ctx.CoinglassLongShortMap == nil {
		ctx.CoinglassLongShortMap = make(map[string]*BinanceLongShortSnapshot)
	}
	ctx.CoinglassLongShortMap["BTC"] = ls
	if ctx.BinanceLongShortMap == nil {
		ctx.BinanceLongShortMap = make(map[string]*BinanceLongShortSnapshot)
	}
	ctx.BinanceLongShortMap["BTCUSDT"] = ls
	ctx.BinanceLongShortMap["BTC"] = ls
	// 5. 恐惧贪婪
	t0 = time.Now()
	fgResp, err := e.coinglassClient.GetFearGreedHistory(1)
	recordCoinglass(ctx, "GetFearGreedHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP fear-greed: %v", err)
	} else if len(fgResp.Data) > 0 {
		ctx.FearGreedValue = fgResp.Data[len(fgResp.Data)-1].Value
		ctx.FearGreedClassification = fgResp.Data[len(fgResp.Data)-1].ValueCN
		if ctx.FearGreedClassification == "" {
			ctx.FearGreedClassification = fearGreedLabel(ctx.FearGreedValue)
		}
	}
	// 6. BTC 市值占比
	t0 = time.Now()
	pct, errBD := e.coinglassClient.GetBitcoinDominance()
	recordCoinglass(ctx, "GetBitcoinDominance", errBD, time.Since(t0).Milliseconds())
	if errBD == nil && pct > 0 {
		ctx.BTCDominancePct = pct
	}
	// 7. 山寨季
	t0 = time.Now()
	altResp, err := e.coinglassClient.GetAltcoinSeasonHistory(1)
	recordCoinglass(ctx, "GetAltcoinSeasonHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP altcoin-season: %v", err)
	} else if len(altResp.Data) > 0 {
		ctx.AltcoinSeasonIndex = altResp.Data[len(altResp.Data)-1].AltcoinIndex
	}
	// 8. ETF 资金流（BTC/ETH）
	t0 = time.Now()
	btcFlow, err := e.coinglassClient.GetETFBitcoinFlowHistory(7)
	recordCoinglass(ctx, "GetETFBitcoinFlowHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP etf btc flow: %v", err)
	} else if len(btcFlow.Data) > 0 {
		for _, p := range btcFlow.Data {
			ctx.ETFFlowBTCRecent += p.Flow
		}
		if ctx.ETFFlowBTCRecent == 0 {
			ctx.ETFFlowBTCRecent = btcFlow.Data[len(btcFlow.Data)-1].Flow
		}
	}
	t0 = time.Now()
	ethFlow, err := e.coinglassClient.GetETFEthereumFlowHistory(7)
	recordCoinglass(ctx, "GetETFEthereumFlowHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP etf eth flow: %v", err)
	} else if len(ethFlow.Data) > 0 {
		for _, p := range ethFlow.Data {
			ctx.ETFFlowETHRecent += p.Flow
		}
		if ctx.ETFFlowETHRecent == 0 {
			ctx.ETFFlowETHRecent = ethFlow.Data[len(ethFlow.Data)-1].Flow
		}
	}
	// 9. 鲸鱼指数 / CGDI / CDRI
	t0 = time.Now()
	whaleResp, err := e.coinglassClient.GetWhaleIndexHistory("Binance", "BTC", "1h", 1)
	recordCoinglass(ctx, "GetWhaleIndexHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP whale: %v", err)
	} else if len(whaleResp.Data) > 0 {
		ctx.CoinglassWhaleIndex = whaleResp.Data[len(whaleResp.Data)-1].WhaleIndexValue
	}
	t0 = time.Now()
	cgdiResp, err := e.coinglassClient.GetCGDIIndexHistory("BTC", 1)
	recordCoinglass(ctx, "GetCGDIIndexHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP cgdi: %v", err)
	} else if len(cgdiResp.Data) > 0 {
		ctx.CoinglassCGDI = cgdiResp.Data[len(cgdiResp.Data)-1].CGDIIndexValue
	}
	t0 = time.Now()
	cdriResp, err := e.coinglassClient.GetCDRIIndexHistory("BTC", 1)
	recordCoinglass(ctx, "GetCDRIIndexHistory", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP cdri: %v", err)
	} else if len(cdriResp.Data) > 0 {
		ctx.CoinglassCDRI = cdriResp.Data[len(cdriResp.Data)-1].CDRIIndexValue
	}
	// 10. 强平热力图关键价位
	t0 = time.Now()
	heatmapResp, err := e.coinglassClient.GetLiquidationAggregatedHeatmap("BTC", "24h")
	recordCoinglass(ctx, "GetLiquidationAggregatedHeatmap", err, time.Since(t0).Milliseconds())
	if err != nil {
		logger.Warnf("Coinglass SLTP heatmap: %v", err)
	} else {
		ctx.CoinglassLiquidationKeyLevels = heatmapResp.HeatmapToKeyLevelsSummary("BTC", "24h")
	}
}

// FormatMarketContextForSLTP 将 Context 格式化为多行市场上下文，供止盈止损 5 种场景做「实时+预测」判断（与主周期数据补强对齐）。
func FormatMarketContextForSLTP(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	var lines []string
	// BTC 行情：1h/4h/24h/7d、MACD、RSI
	if btc := ctx.MarketDataMap["BTCUSDT"]; btc != nil {
		line := fmt.Sprintf("BTC: 1h %+.2f%% 4h %+.2f%%", btc.PriceChange1h, btc.PriceChange4h)
		if btc.PriceChange24h != 0 || btc.PriceChange7d != 0 {
			line += fmt.Sprintf(" 24h %+.2f%% 7d %+.2f%%", btc.PriceChange24h, btc.PriceChange7d)
		}
		if btc.CurrentMACD != 0 || btc.CurrentRSI7 != 0 {
			line += fmt.Sprintf(" | MACD %.4f RSI %.2f", btc.CurrentMACD, btc.CurrentRSI7)
		}
		lines = append(lines, line)
	}
	if ctx.BTCDominancePct > 0 {
		lines = append(lines, fmt.Sprintf("BTC dominance %.2f%%", ctx.BTCDominancePct))
	}
	if ctx.FearGreedValue > 0 || ctx.FearGreedClassification != "" {
		lines = append(lines, fmt.Sprintf("Fear&Greed %d %s", ctx.FearGreedValue, ctx.FearGreedClassification))
	}
	if ctx.AltcoinSeasonIndex > 0 {
		lines = append(lines, fmt.Sprintf("AltcoinSeason %d", ctx.AltcoinSeasonIndex))
	}
	// OI 变化（趋势/衰竭）
	for _, oi := range ctx.CoinglassOISummaries {
		lines = append(lines, fmt.Sprintf("OI %s 1h %+.2f%% 4h %+.2f%%", oi.Symbol, oi.Change1hPct, oi.Change4hPct))
		break
	}
	// 资金费率、多空比
	if r, ok := ctx.CoinglassFundingMap["BTC"]; ok && r != 0 {
		lines = append(lines, fmt.Sprintf("Funding BTC %.4f%%", r*100))
	}
	if ls := ctx.CoinglassLongShortMap["BTC"]; ls != nil && (ls.LongAccount != 0 || ls.ShortAccount != 0) {
		lines = append(lines, fmt.Sprintf("L/S BTC 多%.0f%% 空%.0f%% 大户多%.0f%%", ls.LongAccount*100, ls.ShortAccount*100, ls.TopLongAccount*100))
	}
	// 强平 1h/4h/24h
	if ctx.CoinglassLiquidation != nil {
		liq := ctx.CoinglassLiquidation
		lines = append(lines, fmt.Sprintf("Liq 1h L%.0f S%.0f 4h L%.0f S%.0f 24h L%.0f S%.0f USD",
			liq.Long1hUSD, liq.Short1hUSD, liq.Long4hUSD, liq.Short4hUSD, liq.Long24hUSD, liq.Short24hUSD))
	}
	if ctx.CoinglassCGDI != 0 || ctx.CoinglassCDRI != 0 {
		lines = append(lines, fmt.Sprintf("CGDI %.2f CDRI %.2f", ctx.CoinglassCGDI, ctx.CoinglassCDRI))
	}
	if ctx.CoinglassWhaleIndex != 0 {
		lines = append(lines, fmt.Sprintf("WhaleIndex %.2f", ctx.CoinglassWhaleIndex))
	}
	if ctx.ETFFlowBTCRecent != 0 || ctx.ETFFlowETHRecent != 0 {
		lines = append(lines, fmt.Sprintf("ETF flow BTC %.0f ETH %.0f", ctx.ETFFlowBTCRecent, ctx.ETFFlowETHRecent))
	}
	if ctx.CoinglassLiquidationKeyLevels != "" {
		lines = append(lines, "Liq levels: "+ctx.CoinglassLiquidationKeyLevels)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// BuildLiveMarketSummaryForSLTP 拉取当前实时市场数据并生成完整市场上下文（多行），供止盈止损 5 种场景做「实时+预测」判断。与主周期缓存解耦，保证 SL/TP 周期内看到的是本周期市场快照。
// FillCoinglassDataForSLTP 返回后 ctx 已完整填充，再格式化为 summary 传入 AI，不传递未拉齐的不完整信息。
func (e *StrategyEngine) BuildLiveMarketSummaryForSLTP(traderID string) string {
	ctx := &Context{TraderID: traderID, Flow: "止盈止损调整"}
	// BTC 行情（含 1h/4h/24h/7d、MACD、RSI）
	t0 := time.Now()
	btcData, err := e.FetchMarketData("BTCUSDT")
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	dur := time.Since(t0).Milliseconds()
	monitor.RecordDataCall("market", "Get", "止盈止损调整", traderID, err == nil, errMsg, dur)
	monitor.RecordDataCall("market", "GetKlines", "止盈止损调整", traderID, err == nil, errMsg, dur)
	if err != nil {
		logger.Warnf("BuildLiveMarketSummaryForSLTP: fetch BTCUSDT: %v", err)
	} else if btcData != nil {
		if ctx.MarketDataMap == nil {
			ctx.MarketDataMap = make(map[string]*market.Data)
		}
		ctx.MarketDataMap["BTCUSDT"] = btcData
	}
	e.FillCoinglassDataForSLTP(ctx) // 同步拉齐本流程全部 Coinglass 后再用 ctx
	s := FormatMarketContextForSLTP(ctx)
	if s != "" {
		return s
	}
	// 无 Coinglass 时退回一句摘要
	return BuildCurrentCycleSummaryForSLTP(ctx)
}

// symbolToCoinglassBase 从交易对取 Coinglass 币种符号（如 BTCUSDT -> BTC），用于多所多空拉取
func symbolToCoinglassBase(symbol string) string {
	s := strings.TrimSpace(strings.ToUpper(symbol))
	if strings.HasSuffix(s, "USDT") {
		return s[:len(s)-4]
	}
	return s
}

func fearGreedLabel(v int) string {
	if v <= 24 {
		return "Extreme Fear"
	}
	if v <= 44 {
		return "Fear"
	}
	if v <= 55 {
		return "Neutral"
	}
	if v <= 75 {
		return "Greed"
	}
	return "Extreme Greed"
}

// ============================================================================
// Entry Functions - Main API
// ============================================================================

// GetFullDecision gets AI's complete trading decision (batch analysis of all coins and positions)
// Uses default strategy configuration - for production use GetFullDecisionWithStrategy with explicit config
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	defaultConfig := store.GetDefaultStrategyConfig("en")
	engine := NewStrategyEngine(&defaultConfig)
	return GetFullDecisionWithStrategy(ctx, mcpClient, engine, "", "", "", nil)
}

// GetFullDecisionWithStrategy uses StrategyEngine to get AI decision (unified prompt generation).
// runID is optional (e.g. backtest run_id); when set, token usage is stored per-run for GET /api/ai-usage?run_id=
// traderID is optional; when set and strategy uses multilayer_filter, RunPipeline is executed and state is stored for radar API.
// opts 可选，与多空雷达「允许做多/做空」一致，传入 nil 时默认允许多空。
// 调用方须在传入前完成 ctx 的完整填充（含 Coinglass 的 FillCoinglassData），保证传入系统/AI 的为完整信息，不传不完整数据。
func GetFullDecisionWithStrategy(ctx *Context, mcpClient mcp.AIClient, engine *StrategyEngine, variant, runID, traderID string, opts *PipelineOptions) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if engine == nil {
		defaultConfig := store.GetDefaultStrategyConfig("en")
		engine = NewStrategyEngine(&defaultConfig)
	}

	// 1. Fetch market data using strategy config
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataWithStrategy(ctx, engine); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	}

	// Ensure OITopDataMap is initialized
	if ctx.OITopDataMap == nil {
		ctx.OITopDataMap = make(map[string]*OITopData)
		oiPositions, err := engine.nofxosClient.GetOITopPositions()
		if err == nil {
			for _, pos := range oiPositions {
				ctx.OITopDataMap[pos.Symbol] = &OITopData{
					Rank:              pos.Rank,
					OIDeltaPercent:    pos.OIDeltaPercent,
					OIDeltaValue:      pos.OIDeltaValue,
					PriceDeltaPercent: pos.PriceDeltaPercent,
				}
			}
		}
	}

	// 1.5 多层过滤与方向池：若策略启用则执行 pipeline，并仅对「待提交」候选做后续 AI 与下单
	config := engine.GetConfig()
	if config.StrategyMode == "multilayer_filter" && config.MultilayerFilter != nil && config.MultilayerFilter.Enabled {
		state, err := RunPipeline(ctx, config, traderID, opts)
		if err != nil {
			logger.Infof("⚠️ Pipeline run error (continuing with all candidates): %v", err)
		} else if state != nil {
			newCoins := make([]CandidateCoin, 0, len(state.ToSubmitSymbols))
			for _, sym := range state.ToSubmitSymbols {
				newCoins = append(newCoins, CandidateCoin{Symbol: sym, Sources: []string{"pipeline"}})
			}
			ctx.CandidateCoins = newCoins
			ctx.DirectionPoolLong = state.DirectionLong
			ctx.DirectionPoolShort = state.DirectionShort
		}
	}

	// 2. Build System Prompt using strategy engine (includes Schema for caching)
	riskConfig := engine.GetRiskControlConfig()
	systemPrompt := engine.BuildSystemPrompt(ctx.Account.TotalEquity, variant)

	// 3. Build User Prompt using strategy engine
	userPrompt := engine.BuildUserPrompt(ctx)

	// 4. Call AI API with prompt caching enabled
	aiCallStart := time.Now()
	aiResponse, err := callAIWithCaching(mcpClient, systemPrompt, userPrompt, runID)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		return nil, fmt.Errorf("AI API call failed: %w", err)
	}

	// 5. Parse AI response
	decision, err := parseFullDecisionResponse(
		aiResponse,
		ctx.Account.TotalEquity,
		riskConfig.BTCETHMaxLeverage,
		riskConfig.AltcoinMaxLeverage,
		riskConfig.BTCETHMaxPositionValueRatio,
		riskConfig.AltcoinMaxPositionValueRatio,
	)

	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt
		decision.UserPrompt = userPrompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
		decision.RawResponse = aiResponse
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

// PrepareContextForPipeline 拉取行情并执行 pipeline（不调 AI），供「仅系统周期」使用。调用后 GetPipelineState(traderID) 可读最新方向池与待提交列表。
func PrepareContextForPipeline(ctx *Context, engine *StrategyEngine, traderID string, opts *PipelineOptions) error {
	if ctx == nil || engine == nil {
		return fmt.Errorf("context or engine is nil")
	}
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataWithStrategy(ctx, engine); err != nil {
			return err
		}
	}
	config := engine.GetConfig()
	if config.StrategyMode != "multilayer_filter" || config.MultilayerFilter == nil || !config.MultilayerFilter.Enabled {
		return nil
	}
	state, err := RunPipeline(ctx, config, traderID, opts)
	if err != nil {
		return err
	}
	if state != nil {
		newCoins := make([]CandidateCoin, 0, len(state.ToSubmitSymbols))
		for _, sym := range state.ToSubmitSymbols {
			newCoins = append(newCoins, CandidateCoin{Symbol: sym, Sources: []string{"pipeline"}})
		}
		ctx.CandidateCoins = newCoins
		ctx.DirectionPoolLong = state.DirectionLong
		ctx.DirectionPoolShort = state.DirectionShort
	}
	return nil
}

// ============================================================================
// Market Data Fetching
// ============================================================================

// fetchMarketDataWithStrategy fetches market data using strategy config (multiple timeframes).
// 仅当有持仓或候选币时才会实际请求 K 线（按标的循环）；无持仓且无候选时不会发请求，但会记一条「本流程已执行」便于数据统计展示。
func fetchMarketDataWithStrategy(ctx *Context, engine *StrategyEngine) error {
	config := engine.GetConfig()
	ctx.MarketDataMap = make(map[string]*market.Data)

	timeframes := config.Indicators.Klines.SelectedTimeframes
	primaryTimeframe := config.Indicators.Klines.PrimaryTimeframe
	klineCount := config.Indicators.Klines.PrimaryCount

	// Compatible with old configuration
	if len(timeframes) == 0 {
		if primaryTimeframe != "" {
			timeframes = append(timeframes, primaryTimeframe)
		} else {
			timeframes = append(timeframes, "3m")
		}
		if config.Indicators.Klines.LongerTimeframe != "" {
			timeframes = append(timeframes, config.Indicators.Klines.LongerTimeframe)
		}
	}
	if primaryTimeframe == "" {
		primaryTimeframe = timeframes[0]
	}
	if klineCount <= 0 {
		klineCount = 30
	}

	logger.Infof("📊 Strategy timeframes: %v, Primary: %s, Kline count: %d", timeframes, primaryTimeframe, klineCount)

	// 无持仓且无候选时也记一条 market Get，便于数据统计中「过滤机制」显示本流程已执行（实际请求数=0）
	if len(ctx.Positions) == 0 && len(ctx.CandidateCoins) == 0 {
		monitor.RecordDataCall("market", "Get", ctx.Flow, ctx.TraderID, true, "", 0)
		monitor.RecordDataCall("market", "GetKlines", ctx.Flow, ctx.TraderID, true, "", 0)
	}

	// 1. First fetch data for position coins (must fetch)
	var positionFetchFailed []string
	for _, pos := range ctx.Positions {
		t0 := time.Now()
		data, err := market.GetWithTimeframes(pos.Symbol, timeframes, primaryTimeframe, klineCount)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		dur := time.Since(t0).Milliseconds()
		monitor.RecordDataCall("market", "Get", ctx.Flow, ctx.TraderID, err == nil, errMsg, dur)
		monitor.RecordDataCall("market", "GetKlines", ctx.Flow, ctx.TraderID, err == nil, errMsg, dur)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for position %s: %v", pos.Symbol, err)
			positionFetchFailed = append(positionFetchFailed, pos.Symbol)
			continue
		}
		ctx.MarketDataMap[pos.Symbol] = data
	}
	if len(positionFetchFailed) > 0 {
		logger.Infof("⛔ Missing market data for %d position(s): %v (will continue, but AI SL/TP and trend_view quality may degrade)",
			len(positionFetchFailed), positionFetchFailed)
	}

	// 2. Fetch data for candidate coins (cap count to control prompt size / token usage)
	// 回测只对 r.cfg.Symbols 拉行情（通常 3～5 个），实盘/模拟候选列表可能很多，故此处限制写入 prompt 的候选数，减轻 token 差距
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	const minOIThresholdMillions = 15.0 // 15M USD minimum open interest value
	maxCandidateCoinsForPrompt := config.Indicators.Klines.MaxCoinsInPrompt
	if maxCandidateCoinsForPrompt <= 0 {
		maxCandidateCoinsForPrompt = 8 // default
	}

	candidateCoinsAdded := 0
	skippedFetchFail := 0
	skippedOILow := 0
	for _, coin := range ctx.CandidateCoins {
		if _, exists := ctx.MarketDataMap[coin.Symbol]; exists {
			continue
		}
		if positionSymbols[coin.Symbol] {
			continue
		}
		if candidateCoinsAdded >= maxCandidateCoinsForPrompt {
			logger.Infof("📊 Capping candidate coins in prompt at %d (total candidates: %d) to keep token usage close to backtest",
				maxCandidateCoinsForPrompt, len(ctx.CandidateCoins))
			break
		}

		t0 := time.Now()
		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primaryTimeframe, klineCount)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		dur := time.Since(t0).Milliseconds()
		monitor.RecordDataCall("market", "Get", ctx.Flow, ctx.TraderID, err == nil, errMsg, dur)
		monitor.RecordDataCall("market", "GetKlines", ctx.Flow, ctx.TraderID, err == nil, errMsg, dur)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for %s: %v", coin.Symbol, err)
			skippedFetchFail++
			continue
		}

		// Liquidity filter (skip for xyz dex assets - they don't have OI data from Binance)
		isXyzAsset := market.IsXyzDexAsset(coin.Symbol)
		if !isXyzAsset && data.OpenInterest != nil && data.CurrentPrice > 0 {
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000
			if oiValueInMillions < minOIThresholdMillions {
				logger.Infof("⚠️  %s OI value too low (%.2fM USD < %.1fM), skipping coin",
					coin.Symbol, oiValueInMillions, minOIThresholdMillions)
				skippedOILow++
				continue
			}
		}

		ctx.MarketDataMap[coin.Symbol] = data
		candidateCoinsAdded++
	}

	// 实际写入 Prompt 的候选数可能小于「写入 Prompt 的候选币数」：候选列表不足、拉取失败或 OI 过滤会导致更少
	logger.Infof("📊 Market data: %d positions + %d candidate coins in prompt (max candidates=%d; total candidates=%d; skipped: fetch_fail=%d, OI_low=%d)",
		len(positionSymbols), candidateCoinsAdded, maxCandidateCoinsForPrompt, len(ctx.CandidateCoins), skippedFetchFail, skippedOILow)
	if len(ctx.CandidateCoins) < maxCandidateCoinsForPrompt && (skippedFetchFail == 0 && skippedOILow == 0) {
		logger.Infof("📊 Candidate list has only %d coins (max_coins_in_prompt=%d); add more static coins or check signal source (AI500/OI) if you expect more", len(ctx.CandidateCoins), maxCandidateCoinsForPrompt)
	}
	return nil
}

// ============================================================================
// Candidate Coins
// ============================================================================

// GetCandidateCoins gets candidate coins based on strategy configuration
func (e *StrategyEngine) GetCandidateCoins() ([]CandidateCoin, error) {
	var candidates []CandidateCoin
	symbolSources := make(map[string][]string)

	coinSource := e.config.CoinSource

	switch coinSource.SourceType {
	case "static":
		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"static"},
			})
		}

		return e.filterExcludedCoins(candidates), nil

	case "ai500":
		// 检查 use_ai500 标志，如果为 false 则回退到静态币种
		if !coinSource.UseAI500 {
			logger.Infof("⚠️  source_type is 'ai500' but use_ai500 is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getAI500Coins(coinSource.AI500Limit)
		if err != nil {
			return nil, err
		}
		// 空列表是正常情况，直接返回
		return e.filterExcludedCoins(coins), nil

	case "oi_top":
		// 检查 use_oi_top 标志，如果为 false 则回退到静态币种
		if !coinSource.UseOITop {
			logger.Infof("⚠️  source_type is 'oi_top' but use_oi_top is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getOITopCoins(coinSource.OITopLimit)
		if err != nil {
			return nil, err
		}
		// 空列表是正常情况，直接返回
		return e.filterExcludedCoins(coins), nil

	case "oi_low":
		// 持仓减少榜，适合做空
		if !coinSource.UseOILow {
			logger.Infof("⚠️  source_type is 'oi_low' but use_oi_low is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return e.filterExcludedCoins(candidates), nil
		}
		coins, err := e.getOILowCoins(coinSource.OILowLimit)
		if err != nil {
			return nil, err
		}
		// 空列表是正常情况，直接返回
		return e.filterExcludedCoins(coins), nil

	case "mixed":
		if coinSource.UseAI500 {
			poolCoins, err := e.getAI500Coins(coinSource.AI500Limit)
			if err != nil {
				logger.Infof("⚠️  Failed to get AI500 coins: %v", err)
			} else {
				for _, coin := range poolCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "ai500")
				}
			}
		}

		if coinSource.UseOITop {
			oiCoins, err := e.getOITopCoins(coinSource.OITopLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Top: %v", err)
			} else {
				for _, coin := range oiCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "oi_top")
				}
			}
		}

		if coinSource.UseOILow {
			oiLowCoins, err := e.getOILowCoins(coinSource.OILowLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Low: %v", err)
			} else {
				for _, coin := range oiLowCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "oi_low")
				}
			}
		}

		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			if _, exists := symbolSources[symbol]; !exists {
				symbolSources[symbol] = []string{"static"}
			} else {
				symbolSources[symbol] = append(symbolSources[symbol], "static")
			}
		}

		for symbol, sources := range symbolSources {
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: sources,
			})
		}
		return e.filterExcludedCoins(candidates), nil

	default:
		return nil, fmt.Errorf("unknown coin source type: %s", coinSource.SourceType)
	}
}

// filterExcludedCoins removes excluded coins from the candidates list
func (e *StrategyEngine) filterExcludedCoins(candidates []CandidateCoin) []CandidateCoin {
	if len(e.config.CoinSource.ExcludedCoins) == 0 {
		return candidates
	}

	// Build excluded set for O(1) lookup
	excluded := make(map[string]bool)
	for _, coin := range e.config.CoinSource.ExcludedCoins {
		normalized := market.Normalize(coin)
		excluded[normalized] = true
	}

	// Filter out excluded coins
	filtered := make([]CandidateCoin, 0, len(candidates))
	for _, c := range candidates {
		if !excluded[c.Symbol] {
			filtered = append(filtered, c)
		} else {
			logger.Infof("🚫 Excluded coin: %s", c.Symbol)
		}
	}

	return filtered
}

func (e *StrategyEngine) getAI500Coins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 30
	}

	symbols, err := e.nofxosClient.GetTopRatedCoins(limit)
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"ai500"},
		})
	}
	return candidates, nil
}

func (e *StrategyEngine) getOITopCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 10
	}

	positions, err := e.nofxosClient.GetOITopPositions()
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for i, pos := range positions {
		if i >= limit {
			break
		}
		symbol := market.Normalize(pos.Symbol)
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"oi_top"},
		})
	}
	return candidates, nil
}

func (e *StrategyEngine) getOILowCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 10
	}

	positions, err := e.nofxosClient.GetOILowPositions()
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for i, pos := range positions {
		if i >= limit {
			break
		}
		symbol := market.Normalize(pos.Symbol)
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{"oi_low"},
		})
	}
	return candidates, nil
}

// ============================================================================
// External & Quant Data
// ============================================================================

// FetchMarketData fetches market data based on strategy configuration
func (e *StrategyEngine) FetchMarketData(symbol string) (*market.Data, error) {
	return market.Get(symbol)
}

// FetchExternalData fetches external data sources
func (e *StrategyEngine) FetchExternalData() (map[string]interface{}, error) {
	externalData := make(map[string]interface{})

	for _, source := range e.config.Indicators.ExternalDataSources {
		data, err := e.fetchSingleExternalSource(source)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch external data source [%s]: %v", source.Name, err)
			continue
		}
		externalData[source.Name] = data
	}

	return externalData, nil
}

func (e *StrategyEngine) fetchSingleExternalSource(source store.ExternalDataSource) (interface{}, error) {
	// SSRF Protection: Validate URL before making request
	if err := security.ValidateURL(source.URL); err != nil {
		return nil, fmt.Errorf("external source URL validation failed: %w", err)
	}

	timeout := time.Duration(source.RefreshSecs) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Use SSRF-safe HTTP client
	client := security.SafeHTTPClient(timeout)

	req, err := http.NewRequest(source.Method, source.URL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range source.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if source.DataPath != "" {
		result = extractJSONPath(result, source.DataPath)
	}

	return result, nil
}

func extractJSONPath(data interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

// FetchQuantData fetches quantitative data for a single coin
func (e *StrategyEngine) FetchQuantData(symbol string) (*QuantData, error) {
	if !e.config.Indicators.EnableQuantData {
		return nil, nil
	}

	// Use nofxos client with unified API key
	include := "oi,price"
	if e.config.Indicators.EnableQuantNetflow {
		include = "netflow,oi,price"
	}

	nofxosData, err := e.nofxosClient.GetCoinData(symbol, include)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quant data: %w", err)
	}

	if nofxosData == nil {
		return nil, nil
	}

	// Convert nofxos.QuantData to kernel.QuantData
	quantData := &QuantData{
		Symbol:      nofxosData.Symbol,
		Price:       nofxosData.Price,
		PriceChange: nofxosData.PriceChange,
	}

	// Convert OI data
	if nofxosData.OI != nil {
		quantData.OI = make(map[string]*OIData)
		for exchange, oiData := range nofxosData.OI {
			if oiData != nil {
				kData := &OIData{
					CurrentOI: oiData.CurrentOI,
				}
				if oiData.Delta != nil {
					kData.Delta = make(map[string]*OIDeltaData)
					for dur, delta := range oiData.Delta {
						if delta != nil {
							kData.Delta[dur] = &OIDeltaData{
								OIDelta:        delta.OIDelta,
								OIDeltaValue:   delta.OIDeltaValue,
								OIDeltaPercent: delta.OIDeltaPercent,
							}
						}
					}
				}
				quantData.OI[exchange] = kData
			}
		}
	}

	// Convert Netflow data
	if nofxosData.Netflow != nil {
		quantData.Netflow = &NetflowData{}
		if nofxosData.Netflow.Institution != nil {
			quantData.Netflow.Institution = &FlowTypeData{
				Future: nofxosData.Netflow.Institution.Future,
				Spot:   nofxosData.Netflow.Institution.Spot,
			}
		}
		if nofxosData.Netflow.Personal != nil {
			quantData.Netflow.Personal = &FlowTypeData{
				Future: nofxosData.Netflow.Personal.Future,
				Spot:   nofxosData.Netflow.Personal.Spot,
			}
		}
	}

	return quantData, nil
}

// FetchQuantDataBatch batch fetches quantitative data
func (e *StrategyEngine) FetchQuantDataBatch(symbols []string) map[string]*QuantData {
	result := make(map[string]*QuantData)

	if !e.config.Indicators.EnableQuantData {
		return result
	}

	for _, symbol := range symbols {
		data, err := e.FetchQuantData(symbol)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch quantitative data for %s: %v", symbol, err)
			continue
		}
		if data != nil {
			result[symbol] = data
		}
	}

	return result
}

// FetchOIRankingData fetches market-wide OI ranking data
func (e *StrategyEngine) FetchOIRankingData() *nofxos.OIRankingData {
	indicators := e.config.Indicators
	if !indicators.EnableOIRanking {
		return nil
	}

	duration := indicators.OIRankingDuration
	if duration == "" {
		duration = "1h"
	}

	limit := indicators.OIRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("📊 Fetching OI ranking data (duration: %s, limit: %d)", duration, limit)

	data, err := e.nofxosClient.GetOIRanking(duration, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch OI ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ OI ranking data ready: %d top, %d low positions",
		len(data.TopPositions), len(data.LowPositions))

	return data
}

// FetchNetFlowRankingData fetches market-wide NetFlow ranking data
func (e *StrategyEngine) FetchNetFlowRankingData() *nofxos.NetFlowRankingData {
	indicators := e.config.Indicators
	if !indicators.EnableNetFlowRanking {
		return nil
	}

	duration := indicators.NetFlowRankingDuration
	if duration == "" {
		duration = "1h"
	}

	limit := indicators.NetFlowRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("💰 Fetching NetFlow ranking data (duration: %s, limit: %d)", duration, limit)

	data, err := e.nofxosClient.GetNetFlowRanking(duration, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch NetFlow ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ NetFlow ranking data ready: inst_in=%d, inst_out=%d, retail_in=%d, retail_out=%d",
		len(data.InstitutionFutureTop), len(data.InstitutionFutureLow),
		len(data.PersonalFutureTop), len(data.PersonalFutureLow))

	return data
}

// FetchPriceRankingData fetches market-wide price ranking data (gainers/losers)
func (e *StrategyEngine) FetchPriceRankingData() *nofxos.PriceRankingData {
	indicators := e.config.Indicators
	if !indicators.EnablePriceRanking {
		return nil
	}

	durations := indicators.PriceRankingDuration
	if durations == "" {
		durations = "1h"
	}

	limit := indicators.PriceRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("📈 Fetching Price ranking data (durations: %s, limit: %d)", durations, limit)

	data, err := e.nofxosClient.GetPriceRanking(durations, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch Price ranking data: %v", err)
		return nil
	}

	logger.Infof("✓ Price ranking data ready for %d durations", len(data.Durations))

	return data
}

// ============================================================================
// Prompt Building - System Prompt
// ============================================================================

// BuildSystemPrompt builds System Prompt according to strategy configuration.
// Used for backtest, live, and paper/simulation; no branch by IsSimulation — 实盘与实盘模拟共用同一套提示词。
// This includes Schema prompt for better prompt caching efficiency.
func (e *StrategyEngine) BuildSystemPrompt(accountEquity float64, variant string) string {
	var sb strings.Builder
	riskControl := e.config.RiskControl
	promptSections := e.config.PromptSections

	// 0. Data Dictionary & Schema (static content, will be cached)
	lang := e.GetLanguage()
	schemaPrompt := GetSchemaPrompt(lang)
	sb.WriteString(schemaPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("---\n\n")

	// 1. Role definition (editable)
	if promptSections.RoleDefinition != "" {
		sb.WriteString(promptSections.RoleDefinition)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# You are a professional cryptocurrency trading AI\n\n")
		sb.WriteString("Your task is to make trading decisions based on provided market data.\n\n")
	}

	// 2. Trading mode variant
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "aggressive":
		sb.WriteString("## Mode: Aggressive\n- Prioritize capturing trend breakouts, can build positions in batches when confidence ≥ 70\n- Allow higher positions, but must strictly set stop-loss and explain risk-reward ratio\n\n")
	case "conservative":
		sb.WriteString("## Mode: Conservative\n- Only open positions when multiple signals resonate\n- Prioritize cash preservation, must pause for multiple periods after consecutive losses\n\n")
	case "scalping":
		sb.WriteString("## Mode: Scalping\n- Focus on short-term momentum, smaller profit targets but require quick action\n- If price doesn't move as expected within two bars, immediately reduce position or stop-loss\n\n")
	}

	// 3. Hard constraints (risk control)
	btcEthPosValueRatio := riskControl.BTCETHMaxPositionValueRatio
	if btcEthPosValueRatio <= 0 {
		btcEthPosValueRatio = 5.0
	}
	altcoinPosValueRatio := riskControl.AltcoinMaxPositionValueRatio
	if altcoinPosValueRatio <= 0 {
		altcoinPosValueRatio = 1.0
	}

	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("## CODE ENFORCED (Backend validation, cannot be bypassed):\n")
	sb.WriteString(fmt.Sprintf("- Max Positions: %d coins simultaneously\n", riskControl.MaxPositions))
	sb.WriteString("- **When current position count already equals Max Positions, do NOT output open_long or open_short for any new symbol; only hold, close_long, close_short for existing positions are allowed.**\n")
	if riskControl.SystemExecutesEntry {
		if lang == LangChinese {
			sb.WriteString("- **【系统执行开仓】本周期开仓由系统根据规则执行，你仅作辅助分析。请勿输出 open_long 或 open_short；仅对已有持仓输出 hold + trend_view，对候选输出 wait 或 reasoning。**\n")
		} else {
			sb.WriteString("- **[System executes entry] Entry is executed by the system from rules this cycle; you only assist with analysis. Do NOT output open_long or open_short; output hold + trend_view for existing positions, wait or reasoning for candidates.**\n")
		}
	}
	if riskControl.AIPredictOnly {
		if lang == LangChinese {
			sb.WriteString("- **【AI 仅预测】你禁止输出 open_long、open_short、close_long、close_short。仅可输出 hold 或 wait。开平仓完全由系统根据你的预测信息（market_regime、scenario、symbol_predictions 等）与规则执行。你必须在 <analysis> 中输出 symbol_predictions 数组，见下方格式。**\n")
		} else {
			sb.WriteString("- **[AI predict only] You must NOT output open_long, open_short, close_long, or close_short. Only output hold or wait. All entry/exit is decided by the system from your prediction (market_regime, scenario, symbol_predictions, etc.). You MUST output symbol_predictions in <analysis>; see format below.**\n")
		}
	}
	sb.WriteString(fmt.Sprintf("- Position Value Limit (Altcoins): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*altcoinPosValueRatio, accountEquity, altcoinPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (BTC/ETH): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*btcEthPosValueRatio, accountEquity, btcEthPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
	sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT\n\n", riskControl.MinPositionSize))

	sb.WriteString("## AI GUIDED (Recommended, you should follow):\n")
	sb.WriteString(fmt.Sprintf("- Trading Leverage: Altcoins max %dx | BTC/ETH max %dx\n",
		riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
	sb.WriteString(fmt.Sprintf("- Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss)\n", riskControl.MinRiskRewardRatio))
	sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d to open position\n\n", riskControl.MinConfidence))

	// Position sizing guidance
	sb.WriteString("## Position Sizing Guidance\n")
	sb.WriteString("Calculate `position_size_usd` based on your confidence and the Position Value Limits above:\n")
	sb.WriteString("- High confidence (≥85): Use 80-100%% of max position value limit\n")
	sb.WriteString("- Medium confidence (70-84): Use 50-80%% of max position value limit\n")
	sb.WriteString("- Low confidence (60-69): Use 30-50%% of max position value limit\n")
	sb.WriteString(fmt.Sprintf("- Example: With equity %.0f and BTC/ETH ratio %.1fx, max is %.0f USDT\n",
		accountEquity, btcEthPosValueRatio, accountEquity*btcEthPosValueRatio))
	sb.WriteString("- **DO NOT** just use available_balance as position_size_usd. Use the Position Value Limits!\n\n")

	// 4. Trading frequency (editable)
	if promptSections.TradingFrequency != "" {
		sb.WriteString(promptSections.TradingFrequency)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# ⏱️ Trading Frequency Awareness\n\n")
		sb.WriteString("- Excellent traders: 2-4 trades/day ≈ 0.1-0.2 trades/hour\n")
		sb.WriteString("- >2 trades/hour = Overtrading\n")
		sb.WriteString("- Single position hold time ≥ 30-60 minutes\n")
		sb.WriteString("If you find yourself trading every period → standards too low; if closing positions < 30 minutes → too impatient.\n\n")
	}

	// 5. Entry standards (editable)
	if promptSections.EntryStandards != "" {
		sb.WriteString(promptSections.EntryStandards)
		sb.WriteString("\n\nYou have the following indicator data:\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\n**Confidence ≥ %d** required to open positions.\n\n", riskControl.MinConfidence))
	} else {
		sb.WriteString("# 🎯 Entry Standards (Strict)\n\n")
		sb.WriteString("Only open positions when multiple signals resonate. You have:\n")
		e.writeAvailableIndicators(&sb)
		sb.WriteString(fmt.Sprintf("\nFeel free to use any effective analysis method, but **confidence ≥ %d** required to open positions; avoid low-quality behaviors such as single indicators, contradictory signals, sideways consolidation, reopening immediately after closing, etc.\n\n", riskControl.MinConfidence))
	}

	// 5b. 当启用 AI 开仓时：约束开仓决策，禁止模棱两可
	if !riskControl.SystemExecutesEntry {
		if lang == LangChinese {
			sb.WriteString("# ⚠️ 开仓决策约束（必须遵守）\n\n")
			sb.WriteString("- **明确性**：仅当多周期、量能、OI/流向等**至少 3 类信号共振**且能在 reasoning 中写出**具体依据**时，才可输出 open_long/open_short；否则一律输出 **wait** 并说明缺少哪类信号。\n")
			sb.WriteString(fmt.Sprintf("- **禁止模糊**：禁止基于单一指标或「可能」「或许」「观望」式理由开仓。confidence 必须为**具体数字**且 **≥ %d**。\n", riskControl.MinConfidence))
			sb.WriteString("- **不确定时**：若信号矛盾或依据不足，必须输出 wait，不得输出 open。\n\n")
		} else {
			sb.WriteString("# ⚠️ Entry Decision Rules (Mandatory)\n\n")
			sb.WriteString("- **Unambiguous**: Only output open_long/open_short when **at least 3 signal categories** (e.g. multi-timeframe, volume, OI/flow) align and you can state **concrete reasons** in reasoning; otherwise output **wait** and state which signals are missing.\n")
			sb.WriteString(fmt.Sprintf("- **No vague opens**: Do not open on a single indicator or \"maybe\" reasoning. confidence must be a **number** and **≥ %d**.\n", riskControl.MinConfidence))
			sb.WriteString("- **When unsure**: If signals conflict or evidence is weak, output wait; do not output open.\n\n")
		}
	}

	// 6. Decision process & entry order (strategy philosophy: direction first, ranging can still open)
	if promptSections.DecisionProcess != "" {
		sb.WriteString(promptSections.DecisionProcess)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("# 📋 Decision Process\n\n")
		sb.WriteString("1. Check positions → Should we take profit/stop-loss\n")
		sb.WriteString("2. **Short-term outlook first**: Before deciding any action, form a short-term outlook (next 1–2 bars / this session) for each symbol based on multi-timeframe trend, volume, OI/flow, funding and liquidation data; summarize it as `near_term_outlook`, `scenario` (continuation/reversal/range) and 1–2 key levels.\n")
		sb.WriteString("3. **Entry order (hard)**: First determine **direction from 4h/1h** (data has 4h dir, 1h dir, 1h vs 4h aligned); then find **entry on primary timeframe (e.g. 15m)**. When 1h and 4h are not aligned, use **smaller position or higher confidence**—do not refuse to open; in clear range still open near range low (long) or high (short).\n")
		sb.WriteString("4. Scan candidate coins + multi-timeframe → Are there strong signals\n")
		sb.WriteString("5. Output structured JSON first, then write chain of thought (never omit the JSON)\n\n")
	}

	// 6a. 开仓三原则（趋势、动能、多周期）：与用户经验一致，供模型在 reasoning 中对照
	if lang == LangChinese {
		sb.WriteString("# 🎯 开仓三原则（须在 reasoning 中对照）\n\n")
		sb.WriteString("- **趋势**：震荡/混沌行情方向不明确，做单运气成分大；此类 regime（如 ranging、high_volatility）下应谨慎开仓或提高置信度要求。\n")
		sb.WriteString("- **动能**：是否同向加强或反向减弱；仅当多空一侧动能占优且可持续时，才倾向该方向开仓。\n")
		sb.WriteString("- **多周期**：4h/1h/短周期至少两周期方向一致时才视为方向明确；多周期不一致时倾向轻仓或提高置信度，仍可开仓但需在 reasoning 中说明。\n\n")
	} else {
		sb.WriteString("# 🎯 Entry Principles (align reasoning with these)\n\n")
		sb.WriteString("- **Trend**: In choppy/ranging regimes direction is unclear; be cautious or require higher confidence (e.g. ranging, high_volatility).\n")
		sb.WriteString("- **Momentum**: Prefer opening when momentum is strengthening in the same direction or weakening in the opposite; one side should clearly dominate.\n")
		sb.WriteString("- **Multi-timeframe**: At least two of (4h, 1h, short-term) should align in direction; when they don't, use smaller size or higher confidence and state why in reasoning.\n\n")
	}

	// 6b. 分析行为规范：约束 regime/scenario 等分析结论的得出方式，降低 AI 自主性带来的随意性
	if lang == LangChinese {
		sb.WriteString("# 📐 分析行为规范（必须遵守）\n\n")
		sb.WriteString("你在 <analysis> 中输出的 **market_regime**、**scenario**、**near_term_outlook** 等将影响开仓门槛与止盈止损逻辑，必须与本周期 prompt 中的数据一致、可追溯。\n\n")
		sb.WriteString("- **数据锚定**：market_regime 与 scenario 必须**依据本周期 prompt 中给出的数据**（如 BTC 1h/4h 涨跌、资金费率、强平、多空比、OI 等）得出，不得与这些数据明显矛盾（例如数据显式偏空时不得仅凭主观输出 trend_up）。\n")
		sb.WriteString("- **多因子**：判定 market_regime 时至少综合**多周期价格方向、资金/强平/情绪中的至少两类**，禁止仅凭单一指标（如只看 1h 涨跌）就输出 regime。\n")
		sb.WriteString("- **一致性**：scenario 须与 near_term_outlook 一致；若 risk_alert=true 则不得对任何标的设 suggest_open=true；symbol_predictions 的 predicted_direction 须与你在 regime/outlook 上的整体判断相符。\n")
		sb.WriteString("- **本周期重判**：每个周期都应根据**当前周期**的数据重新判断 regime 与 scenario，不得机械沿用上一周期或未结合本周期数据即输出相同结论。\n")
		sb.WriteString("- **冲突时说明**：当数据冲突（如 1h 涨、4h 跌）时，应在 reasoning 或 market_summary 中简要说明你如何权衡、优先了哪类数据，再给出 regime/scenario。\n\n")
	} else {
		sb.WriteString("# 📐 Analysis Behavior Norms (Mandatory)\n\n")
		sb.WriteString("Your **market_regime**, **scenario**, **near_term_outlook** in <analysis> affect entry constraints and SL/TP logic; they must be consistent with and traceable to the data in this cycle's prompt.\n\n")
		sb.WriteString("- **Data grounding**: market_regime and scenario MUST be derived from **data provided in this cycle's prompt** (e.g. BTC 1h/4h change, funding, liquidation, long/short, OI). Do not output conclusions that plainly contradict that data (e.g. do not output trend_up when data is clearly bearish).\n")
		sb.WriteString("- **Multi-factor**: When assigning market_regime, consider **at least two of**: multi-timeframe price direction, funding/liquidation/sentiment. Do not rely on a single metric (e.g. 1h change only).\n")
		sb.WriteString("- **Consistency**: scenario must align with near_term_outlook; if risk_alert=true do not set suggest_open=true for any symbol; predicted_direction in symbol_predictions must be consistent with your overall regime/outlook.\n")
		sb.WriteString("- **Re-evaluate each cycle**: Re-assess regime and scenario from **this cycle's** data every time; do not copy the previous cycle's output without re-checking against current data.\n")
		sb.WriteString("- **When data conflicts**: If data conflicts (e.g. 1h up, 4h down), briefly state in reasoning or market_summary how you weighed them and which data you prioritized before giving regime/scenario.\n\n")
	}

	// 7. Output format
	sb.WriteString("# Output Format (Strictly Follow)\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON, avoiding parsing errors**\n\n")
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("Step 1: JSON decision array (MUST output this first; if unsure, output a single wait decision)\n\n")
	sb.WriteString("```json\n[\n")
	// Use the actual configured position value ratio for BTC/ETH in the example
	examplePositionSize := accountEquity * btcEthPosValueRatio
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300},\n",
		riskControl.BTCETHMaxLeverage, examplePositionSize))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\", \"exit_reason\": \"prediction_mismatch\", \"confidence\": 75}\n")
	sb.WriteString("  {\"symbol\": \"BTCUSDT\", \"action\": \"hold\", \"trend_view\": \"trend_intact\", \"reasoning\": \"...\"}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Step 2: Your chain of thought analysis (keep concise; do not include JSON here)\n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("## Field Description\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	if lang == LangChinese {
		sb.WriteString("- **close_long/close_short 约束**：仅当**结合历史数据、实时数据与你的预测**后，你认为**此交易与预测不符**（如趋势反转、目标达成、关键位跌破）时，才可建议平仓。必须输出 **exit_reason**：**take_profit**（止盈/锁定利润）| **stop_loss**（止损/结构破坏）| **prediction_mismatch**（预期改变、交易不再成立）。禁止基于模糊或单根K线理由建议平仓。\n")
	} else {
		sb.WriteString("- When action is **close_long** or **close_short**: You may suggest closing only when, **after combining historical data, real-time data, and your prediction**, you conclude that **the trade no longer matches the outlook** (e.g. trend reversed, target reached, or key level broken). You MUST output **exit_reason**: **take_profit** (profit target met or lock gain) | **stop_loss** (loss cut or structure broken) | **prediction_mismatch** (outlook changed, trade no longer valid). Do not suggest close on vague or single-candle moves.\n")
	}
	sb.WriteString("- When action is **hold** for an **existing position**, you MUST output **trend_view**: one of **trend_intact** (trend still valid) | **choppy** (ranging/oscillating) | **reversing** (trend reversing or structure broken). Strategy uses it to adjust SL confirm: reversing→faster stop; trend_intact/choppy→one more confirm to reduce oscillation wash.\n")
	sb.WriteString(fmt.Sprintf("- `confidence`: 0-100 (opening recommended ≥ %d)\n", riskControl.MinConfidence))
	sb.WriteString("- Required when opening: leverage, position_size_usd (or risk_bucket when buckets enabled), stop_loss, take_profit, confidence, risk_usd\n")
	if riskControl.PositionSizeBuckets == nil || !riskControl.PositionSizeBuckets.Enabled {
		if lang == LangChinese {
			sb.WriteString("- **仓位档位已关闭**：开仓时必须由你设置 `position_size_usd`（系统会按最大仓位比例上限做裁剪）。\n")
		} else {
			sb.WriteString("- **Position size buckets are disabled**: you must set `position_size_usd` for each open (system will cap by max position value ratio).\n")
		}
	}
	sb.WriteString("- Optional when opening (STRICT enums):\n")
	sb.WriteString("  - `risk_bucket`: low | medium | high (only when position size buckets enabled; system maps to ratio)\n")
	sb.WriteString("  - `tp_profile`: tp_conservative | tp_balanced | tp_aggressive\n")
	sb.WriteString("  - `sl_profile`: sl_tight | sl_normal | sl_loose\n")
	sb.WriteString("  - `trail_aggressiveness`: low | medium | high\n")
	sb.WriteString("  - `key_levels`: array of 1–3 key levels (strings), optional\n")
	sb.WriteString("- **IMPORTANT**: All numeric values must be calculated numbers, NOT formulas/expressions (e.g., use `27.76` not `3000 * 0.01`)\n\n")
	sb.WriteString("## <analysis> block (required structure)\n")
	if lang == LangChinese {
		sb.WriteString("在 <decision> 之后**必须**输出 <analysis>（或 <outlook>），其中 JSON **必须包含**以下字段，建议按顺序组织分析：① 市场环境 ② 短期预期 ③ 标的结论。\n\n")
		sb.WriteString("**必填**：\n")
		sb.WriteString("- `market_regime`：市场状态，必填。取值：trend_up / trend_down / ranging / high_volatility / reversal。\n")
		sb.WriteString("- `scenario`：整体情景，必填。取值：continuation / reversal / range 等，用于辅助止盈止损与持仓决策。\n")
		sb.WriteString("- `risk_alert`：布尔，必填。true 表示建议本周期不新开仓；无特别风险时填 false。\n")
		if riskControl.AIPredictOnly {
			sb.WriteString("- `symbol_predictions`：按标的的预测数组，必填。每项含 `symbol`、`predicted_direction`（up/down/neutral）、`confidence`（0-100）、`suggest_exit`（该持仓是否建议平仓）、`suggest_open`（是否建议本周期开仓）。**原则**：仅当该标的的预测方向与上方「系统方向池判定」一致（多池→up、空池→down）且置信度≥策略要求时设 suggest_open=true，否则设 false；开仓须三条件共振（实时方向+AI预测方向+AI建议开仓）。系统据此决定是否执行开仓及动态参数。\n")
		}
		sb.WriteString("\n**可选**：\n")
		sb.WriteString("- `market_summary`：本周期市场摘要，一两句话。\n")
		sb.WriteString("- `near_term_outlook`：未来 1～2 根 K 或本 session 的整体预期（方向、空间、主要依据）。\n")
		sb.WriteString("- `key_levels`：1～2 个关键支撑/阻力或目标价位。\n")
		sb.WriteString("- `symbol_structure_signals`：结构化“阶段标签+证伪点+退场倾向”数组（建议输出，用于更好把握趋势结束点）。每项：symbol、phase_label（trend/late_trend/range/high_vol/reversal_risk/neutral）、invalidation_level（关键位/证伪规则）、invalidation_strength（0-100）、exit_bias（hold/tighten/scale_out/exit）、rationale（一句解释）。**抗抖动**：证伪与 exit_bias 必须以 1h/4h 结构为主，15m 只能小幅微调；避免来回反复，优先输出可被验证的证伪条件。\n")
		sb.WriteString("- `position_sl_tp_adjustments`：持仓期间止盈/止损参数调节（系统在边界内应用）。规则：choppy_hold→trail=low 或 confirm_cycles_delta=+1；trend_ride→trail=high；lock_profit→lock_profit_pct=2～3；trend_weakening→收紧或 confirm_cycles_delta=-1；high_vol_hold→confirm_cycles_delta=+1 或 atr_mult_sl 上限。每项：symbol、side、advice、trail_aggressiveness、atr_mult_sl、lock_profit_pct、confirm_cycles_delta。\n")
		sb.WriteString("\n示例：\n```json\n{\"market_regime\": \"trend_down\", \"scenario\": \"continuation\", \"risk_alert\": false, \"market_summary\": \"...\", \"key_levels\": [\"88000\"]")
		if riskControl.AIPredictOnly {
			sb.WriteString(", \"symbol_predictions\": [{\"symbol\": \"BTCUSDT\", \"predicted_direction\": \"down\", \"confidence\": 75, \"suggest_exit\": false, \"suggest_open\": true}, {\"symbol\": \"ETHUSDT\", \"predicted_direction\": \"up\", \"confidence\": 70, \"suggest_exit\": false, \"suggest_open\": false}]")
		}
		sb.WriteString("}\n```\n\n")
	} else {
		sb.WriteString("After <decision> you **must** output <analysis> (or <outlook>) with JSON that **must include** the following fields. Suggested analysis order: ① market environment ② short-term outlook ③ symbol conclusions.\n\n")
		sb.WriteString("**Required**:\n")
		sb.WriteString("- `market_regime`: required. One of: trend_up / trend_down / ranging / high_volatility / reversal.\n")
		sb.WriteString("- `scenario`: required. One of: continuation / reversal / range, for SL/TP and holding decisions.\n")
		sb.WriteString("- `risk_alert`: required boolean. true = suggest no new entries this cycle; use false when no special risk.\n")
		if riskControl.AIPredictOnly {
			sb.WriteString("- `symbol_predictions`: required array. Each: `symbol`, `predicted_direction` (up/down/neutral), `confidence` (0-100), `suggest_exit`, `suggest_open`. **Rule**: set suggest_open=true only when predicted_direction matches the \"System direction pool\" above (long pool→up, short pool→down) and confidence≥strategy minimum; otherwise false. Entry requires 3-way resonance (system direction + AI direction + suggest_open). System uses this for entries and dynamic params.\n")
		}
		sb.WriteString("\n**Optional**:\n")
		sb.WriteString("- `market_summary`: 1–2 sentence cycle summary.\n")
		sb.WriteString("- `near_term_outlook`: short-term outlook for next 1–2 bars or this session.\n")
		sb.WriteString("- `key_levels`: 1–2 key support/resistance or target levels.\n")
		sb.WriteString("- `symbol_structure_signals`: structured phase/invalidation/exit-bias array (recommended, used to better capture trend endings). Each: symbol, phase_label (trend/late_trend/range/high_vol/reversal_risk/neutral), invalidation_level, invalidation_strength (0-100), exit_bias (hold/tighten/scale_out/exit), rationale (1 sentence). **Anti-noise**: invalidation and exit_bias must be driven mainly by 1h/4h structure; 15m only for small nudges; avoid flip-flopping, prefer verifiable invalidation conditions.\n")
		sb.WriteString("- `position_sl_tp_adjustments`: per-position SL/TP parameter suggestions (system applies within bounds). Rules: choppy_hold→trail=low or confirm_cycles_delta=+1; trend_ride→trail=high; lock_profit→lock_profit_pct=2–3; trend_weakening→tighten or confirm_cycles_delta=-1; high_vol_hold→confirm_cycles_delta=+1 or atr_mult_sl upper. Fields: symbol, side, advice, trail_aggressiveness, atr_mult_sl, lock_profit_pct, confirm_cycles_delta.\n")
		sb.WriteString("\nExample:\n```json\n{\"market_regime\": \"trend_down\", \"scenario\": \"continuation\", \"risk_alert\": false, \"market_summary\": \"...\", \"key_levels\": [\"88000\"]")
		if riskControl.AIPredictOnly {
			sb.WriteString(", \"symbol_predictions\": [{\"symbol\": \"BTCUSDT\", \"predicted_direction\": \"down\", \"confidence\": 75, \"suggest_exit\": false, \"suggest_open\": true}, {\"symbol\": \"ETHUSDT\", \"predicted_direction\": \"up\", \"confidence\": 70, \"suggest_exit\": false, \"suggest_open\": false}]")
		}
		sb.WriteString("}\n```\n\n")
	}

	// 8. Custom Prompt
	if e.config.CustomPrompt != "" {
		sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		sb.WriteString(e.config.CustomPrompt)
		sb.WriteString("\n\n")
		sb.WriteString("Note: The above personalized strategy is a supplement to the basic rules and cannot violate the basic risk control principles.\n")
	}

	return sb.String()
}

func (e *StrategyEngine) writeAvailableIndicators(sb *strings.Builder) {
	indicators := e.config.Indicators
	kline := indicators.Klines

	sb.WriteString(fmt.Sprintf("- %s price series", kline.PrimaryTimeframe))
	if kline.EnableMultiTimeframe {
		sb.WriteString(fmt.Sprintf(" + %s K-line series\n", kline.LongerTimeframe))
	} else {
		sb.WriteString("\n")
	}

	if indicators.EnableEMA {
		sb.WriteString("- EMA indicators")
		if len(indicators.EMAPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.EMAPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableMACD {
		sb.WriteString("- MACD indicators\n")
	}

	if indicators.EnableRSI {
		sb.WriteString("- RSI indicators")
		if len(indicators.RSIPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.RSIPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableATR {
		sb.WriteString("- ATR indicators")
		if len(indicators.ATRPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.ATRPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableBOLL {
		sb.WriteString("- Bollinger Bands (BOLL) - Upper/Middle/Lower bands")
		if len(indicators.BOLLPeriods) > 0 {
			sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.BOLLPeriods))
		}
		sb.WriteString("\n")
	}

	if indicators.EnableVolume {
		sb.WriteString("- Volume data\n")
	}

	if indicators.EnableOI {
		sb.WriteString("- Open Interest (OI) data\n")
	}

	if indicators.EnableFundingRate {
		sb.WriteString("- Funding rate\n")
	}

	if len(e.config.CoinSource.StaticCoins) > 0 || e.config.CoinSource.UseAI500 || e.config.CoinSource.UseOITop {
		sb.WriteString("- AI500 / OI_Top filter tags (if available)\n")
	}

	if indicators.EnableQuantData {
		sb.WriteString("- Quantitative data (institutional/retail fund flow, position changes, multi-period price changes)\n")
	}
}

// ============================================================================
// Prompt Building - User Prompt
// ============================================================================

// BuildUserPrompt builds User Prompt based on strategy configuration
func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// System status
	sb.WriteString(fmt.Sprintf("Time: %s | Period: #%d | Runtime: %d minutes\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC market
	if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		sb.WriteString(fmt.Sprintf("BTC: %.2f (1h: %+.2f%%, 4h: %+.2f%%) | MACD: %.4f | RSI: %.2f\n\n",
			btcData.CurrentPrice, btcData.PriceChange1h, btcData.PriceChange4h,
			btcData.CurrentMACD, btcData.CurrentRSI7))
	}

	// Short-term predictive summary (heuristic outlook based on current context; AI must still form its own detailed outlook)
	if ctx.TradingStats != nil || len(ctx.Positions) > 0 || len(ctx.CandidateCoins) > 0 {
		lang := e.GetLanguage()
		if lang == LangChinese {
			sb.WriteString("## 预测参考摘要（由系统粗略归纳，供你形成 near_term_outlook 与 scenario）：\n")
		} else {
			sb.WriteString("## Short-term predictive summary (rough system hints to help you form near_term_outlook and scenario):\n")
		}
		if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
			if lang == LangChinese {
				sb.WriteString(fmt.Sprintf("- BTC：1h 涨跌 %+0.2f%%，4h 涨跌 %+0.2f%%", btcData.PriceChange1h, btcData.PriceChange4h))
				if btcData.PriceChange24h != 0 || btcData.PriceChange7d != 0 {
					sb.WriteString(fmt.Sprintf("，24h %+0.2f%% 7d %+0.2f%%", btcData.PriceChange24h, btcData.PriceChange7d))
				}
				sb.WriteString("；\n")
			} else {
				sb.WriteString(fmt.Sprintf("- BTC: 1h %+0.2f%% 4h %+0.2f%%", btcData.PriceChange1h, btcData.PriceChange4h))
				if btcData.PriceChange24h != 0 || btcData.PriceChange7d != 0 {
					sb.WriteString(fmt.Sprintf(" 24h %+0.2f%% 7d %+0.2f%%", btcData.PriceChange24h, btcData.PriceChange7d))
				}
				sb.WriteString(";\n")
			}
		}
		if ctx.BTCDominancePct > 0 {
			if lang == LangChinese {
				sb.WriteString(fmt.Sprintf("- BTC Dominance：当前 %.2f%%；\n", ctx.BTCDominancePct))
			} else {
				sb.WriteString(fmt.Sprintf("- BTC Dominance: current %.2f%%;\n", ctx.BTCDominancePct))
			}
		}
		if ctx.LiquidationAgg != nil {
			if lang == LangChinese {
				sb.WriteString("- 最近存在显著强平聚合区，可结合价格与 OI 判断短期波动与反向可能。\n")
			} else {
				sb.WriteString("- Recent liquidation clusters exist; combine with price and OI to judge short-term volatility and reversal risk.\n")
			}
		}
		sb.WriteString("\n")
	}

	// Account information
	sb.WriteString(fmt.Sprintf("Account: Equity %.2f | Balance %.2f (%.1f%%) | PnL %+.2f%% | Margin %.1f%% | Positions %d\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	maxPos := 3
	if e.config.RiskControl.MaxPositions > 0 {
		maxPos = e.config.RiskControl.MaxPositions
	}
	if ctx.Account.PositionCount >= maxPos {
		if e.GetLanguage() == LangChinese {
			sb.WriteString(fmt.Sprintf("**约束：当前持仓 %d / 最大 %d（已达上限）。本周期不得对新标的输出 open_long 或 open_short，仅可对现有持仓输出 hold、close_long、close_short。**\n\n", ctx.Account.PositionCount, maxPos))
		} else {
			sb.WriteString(fmt.Sprintf("**Constraint: Current positions %d / Max %d (at limit). This period do NOT output open_long or open_short for new symbols; only hold, close_long, or close_short for existing positions.**\n\n", ctx.Account.PositionCount, maxPos))
		}
	}

	// 系统执行开仓模式：AI 仅作辅助分析量化数据，不输出 open_long/open_short；开仓由系统根据多层过滤/方向池执行
	if e.config.RiskControl.SystemExecutesEntry {
		if e.GetLanguage() == LangChinese {
			sb.WriteString("**当前为「系统执行开仓」模式：你仅作为辅助，分析量化数据与市场趋势。请勿输出 open_long 或 open_short；开仓动作由系统根据多层过滤与方向池规则执行。**\n\n")
			sb.WriteString("**你只需**：对已有持仓输出 hold 并附带 **trend_view**（trend_intact/choppy/reversing），供策略调节止损确认；对候选币可输出 wait 或简短 reasoning，不要输出开仓动作。**\n\n")
		} else {
			sb.WriteString("**System-executes-entry mode: You only assist by analyzing quantitative data and market trend. Do NOT output open_long or open_short; entry is executed by the system based on multilayer filter and direction pool rules.**\n\n")
			sb.WriteString("**You should**: For existing positions output hold with **trend_view** (trend_intact/choppy/reversing) for SL modulation; for candidates you may output wait or brief reasoning—do not output entry actions.**\n\n")
		}
	} else if e.config.RiskControl.AIOnlyEntry {
		// AI 仅开仓模式：对已有持仓一律输出 hold，平仓由策略动态 SL/TP 执行；并对每个持仓输出 trend_view 供策略调节止损确认
		if e.GetLanguage() == LangChinese {
			sb.WriteString("**当前为「AI 仅开仓」模式：你只负责预测市场、决定开仓方向与开仓时的止损/止盈参数；持仓的平仓完全由策略（动态止损、追踪止损、分层止盈）执行。对已有持仓请一律输出 hold，不要输出 close_long 或 close_short。**\n\n")
			if len(ctx.Positions) > 0 {
				sb.WriteString("**对每个已有持仓，若输出 hold，必须同时输出 trend_view**（三选一）：**trend_intact**（趋势仍在）/ **choppy**（震荡）/ **reversing**（反转中）。策略会根据 trend_view 调节止损确认次数：reversing 时更快止损，trend_intact 或 choppy 时多确认一周期以减少震荡被洗。\n\n")
				sb.WriteString("**注意**：你的分析仅用于**实时市场趋势区分**。市场多为波动，随时可能出现结构反转、趋势破坏等情况；**当识别到结构反转或趋势破坏时，请输出 trend_view=reversing**，策略将减少止损确认周期、更快执行止损。每周期都需对持仓做趋势/震荡/反转判断。\n\n")
			}
		} else {
			sb.WriteString("**AI-only-entry mode: You only predict market and decide entry direction/params; strategy handles all exits (dynamic SL/TP, trailing, scaled TP). For existing positions always output hold, do NOT output close_long or close_short.**\n\n")
			if len(ctx.Positions) > 0 {
				sb.WriteString("**For each existing position, when outputting hold, you MUST also output trend_view** (one of): **trend_intact** (trend still valid) | **choppy** (ranging/oscillating) | **reversing** (reversal or structure broken). Strategy uses it to adjust SL confirm: reversing→faster stop; trend_intact/choppy→one more confirm to reduce oscillation wash.\n\n")
				sb.WriteString("**Note**: Your analysis is for **real-time market trend distinction** only. Markets are volatile and structure reversal can happen anytime; **when you identify structure reversal or trend break, output trend_view=reversing** so the strategy will require fewer SL confirm cycles and exit faster. Re-assess trend/choppy/reversing for each position every cycle.\n\n")
			}
		}
	}

	// 本周期已由策略触发的平仓（动态止损/止盈），供 AI 思维链与后续决策参考
	if len(ctx.StrategyTriggeredCloses) > 0 {
		if e.GetLanguage() == LangChinese {
			sb.WriteString("## 本周期已由策略触发的平仓（动态止损/止盈）\n")
			for _, c := range ctx.StrategyTriggeredCloses {
				sb.WriteString(fmt.Sprintf("- %s %s 已平仓 @ %.4f（原因: %s）\n", c.Symbol, c.Side, c.Price, c.Reason))
			}
		} else {
			sb.WriteString("## Strategy-triggered closes this period (dynamic SL/TP)\n")
			for _, c := range ctx.StrategyTriggeredCloses {
				sb.WriteString(fmt.Sprintf("- %s %s closed @ %.4f (reason: %s)\n", c.Symbol, c.Side, c.Price, c.Reason))
			}
		}
		sb.WriteString("\n")
	}

	// 策略动态止损/止盈配置：供 reasoning 中明确写出初始止损、止盈、ATR倍数/周期，并说明追踪止损与分层止盈由策略执行
	sb.WriteString(e.formatStrategyDynamicSLTP())
	sb.WriteString("\n")

	// Recently completed orders (placed before positions to ensure visibility)
	if len(ctx.RecentOrders) > 0 {
		sb.WriteString("## Recent Completed Trades\n")
		for i, order := range ctx.RecentOrders {
			resultStr := "Profit"
			if order.RealizedPnL < 0 {
				resultStr = "Loss"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Exit %.4f | %s: %+.2f USDT (%+.2f%%) | %s→%s (%s)\n",
				i+1, order.Symbol, order.Side,
				order.EntryPrice, order.ExitPrice,
				resultStr, order.RealizedPnL, order.PnLPct,
				order.EntryTime, order.ExitTime, order.HoldDuration))
		}
		sb.WriteString("\n")
	}

	// Historical trading statistics (helps AI understand past performance)
	if ctx.TradingStats != nil && ctx.TradingStats.TotalTrades > 0 {
		// Get language from strategy config
		lang := e.GetLanguage()

		// Win/Loss ratio
		var winLossRatio float64
		if ctx.TradingStats.AvgLoss > 0 {
			winLossRatio = ctx.TradingStats.AvgWin / ctx.TradingStats.AvgLoss
		}

		if lang == LangChinese {
			sb.WriteString("## 历史交易统计\n")
			sb.WriteString(fmt.Sprintf("总交易: %d 笔 | 盈利因子: %.2f | 夏普比率: %.2f | 盈亏比: %.2f\n",
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.ProfitFactor,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("总盈亏: %+.2f USDT | 平均盈利: +%.2f | 平均亏损: -%.2f | 最大回撤: %.1f%%\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("表现: 良好 - 保持当前策略\n")
			} else if ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("表现: 需改进 - 提高盈亏比，优化止盈止损\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("表现: 风险偏高 - 减少仓位，控制回撤\n")
			} else {
				sb.WriteString("表现: 正常 - 有优化空间\n")
			}
		} else {
			sb.WriteString("## Historical Trading Statistics\n")
			sb.WriteString(fmt.Sprintf("Total Trades: %d | Profit Factor: %.2f | Sharpe: %.2f | Win/Loss Ratio: %.2f\n",
				ctx.TradingStats.TotalTrades,
				ctx.TradingStats.ProfitFactor,
				ctx.TradingStats.SharpeRatio,
				winLossRatio))
			sb.WriteString(fmt.Sprintf("Total PnL: %+.2f USDT | Avg Win: +%.2f | Avg Loss: -%.2f | Max Drawdown: %.1f%%\n",
				ctx.TradingStats.TotalPnL,
				ctx.TradingStats.AvgWin,
				ctx.TradingStats.AvgLoss,
				ctx.TradingStats.MaxDrawdownPct))

			// Performance hints based on profit factor, sharpe, and drawdown
			if ctx.TradingStats.ProfitFactor >= 1.5 && ctx.TradingStats.SharpeRatio >= 1 {
				sb.WriteString("Performance: GOOD - maintain current strategy\n")
			} else if ctx.TradingStats.ProfitFactor < 1 {
				sb.WriteString("Performance: NEEDS IMPROVEMENT - improve win/loss ratio, optimize TP/SL\n")
			} else if ctx.TradingStats.MaxDrawdownPct > 30 {
				sb.WriteString("Performance: HIGH RISK - reduce position size, control drawdown\n")
			} else {
				sb.WriteString("Performance: NORMAL - room for optimization\n")
			}
		}
		sb.WriteString("\n")
	}

	// Position information
	if len(ctx.Positions) > 0 {
		sb.WriteString("## Current Positions\n")
		for i, pos := range ctx.Positions {
			sb.WriteString(e.formatPositionInfo(i+1, pos, ctx))
		}
	} else {
		sb.WriteString("Current Positions: None\n\n")
	}

	// Candidate coins (exclude coins already in positions to avoid duplicate data)
	// 输入 token 差异说明：回测时 MarketDataMap 仅包含 r.cfg.Symbols（如 3～5 个），故只有这些币种会写入 prompt；
	// 实盘/模拟时 MarketDataMap = 持仓 + 最多 maxCandidateCoinsForPrompt 个候选，写入的币种更多，且还有 RecentOrders、TradingStats，
	// 所以即使「扫描到的候选数量」相同，实盘/模拟的 input token 仍会明显多于回测。
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		// Normalize symbol to handle both "ETH" and "ETHUSDT" formats
		normalizedSymbol := market.Normalize(pos.Symbol)
		positionSymbols[normalizedSymbol] = true
	}

	// 候选数量以当前列表为准（启用多层过滤时即为「待提交」数量）
	candidateCount := len(ctx.CandidateCoins)
	if e.config.StrategyMode == "multilayer_filter" && e.config.MultilayerFilter != nil && e.config.MultilayerFilter.Enabled {
		if e.GetLanguage() == LangChinese {
			sb.WriteString("以下候选已通过多层过滤（第一层条件 + 第二层因子/可靠度/信心 + 方向），仅可对下列标的建议开仓。\n\n")
		} else {
			sb.WriteString("The candidate coins below have passed the multilayer filter (Layer1 + Layer2 factors/reliability/confidence + direction). Only suggest open for these symbols.\n\n")
		}
		// 系统方向池判定：供 AI 对齐三条件共振（实时方向 = 系统判定多/空；AI 的 predicted_direction 与 suggest_open 须与此一致才开仓）
		if len(ctx.DirectionPoolLong) > 0 || len(ctx.DirectionPoolShort) > 0 {
			longStr := make(map[string]float64)
			for _, i := range ctx.DirectionPoolLong {
				longStr[market.Normalize(i.Symbol)] = i.StrengthPct
			}
			shortStr := make(map[string]float64)
			for _, i := range ctx.DirectionPoolShort {
				shortStr[market.Normalize(i.Symbol)] = i.StrengthPct
			}
			if e.GetLanguage() == LangChinese {
				sb.WriteString("**系统方向池判定**（开仓须三条件共振：①与此方向一致 ②AI 预测方向一致 ③AI 建议开仓=true）：\n")
			} else {
				sb.WriteString("**System direction pool** (entry requires 3-way resonance: 1 match this direction, 2 AI predicted_direction matches, 3 suggest_open=true):\n")
			}
			var lines []string
			zh := e.GetLanguage() == LangChinese
			for _, coin := range ctx.CandidateCoins {
				n := market.Normalize(coin.Symbol)
				lpct, inLong := longStr[n]
				spct, inShort := shortStr[n]
				if inLong && inShort {
					if zh {
						lines = append(lines, fmt.Sprintf("%s 多%.0f%% / 空%.0f%%", coin.Symbol, lpct, spct))
					} else {
						lines = append(lines, fmt.Sprintf("%s long %.0f%% / short %.0f%%", coin.Symbol, lpct, spct))
					}
				} else if inLong {
					if zh {
						lines = append(lines, fmt.Sprintf("%s 多 %.0f%%", coin.Symbol, lpct))
					} else {
						lines = append(lines, fmt.Sprintf("%s long %.0f%%", coin.Symbol, lpct))
					}
				} else if inShort {
					if zh {
						lines = append(lines, fmt.Sprintf("%s 空 %.0f%%", coin.Symbol, spct))
					} else {
						lines = append(lines, fmt.Sprintf("%s short %.0f%%", coin.Symbol, spct))
					}
				}
			}
			if len(lines) > 0 {
				sb.WriteString(strings.Join(lines, "；") + "\n\n")
			}
		}
	}
	sb.WriteString(fmt.Sprintf("## Candidate Coins (%d coins)\n\n", candidateCount))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		// Skip if this coin is already a position (data already shown in positions section)
		normalizedCoinSymbol := market.Normalize(coin.Symbol)
		if positionSymbols[normalizedCoinSymbol] {
			continue
		}

		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			continue
		}
		displayedCount++

		sourceTags := e.formatCoinSourceTag(coin.Sources)
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[coin.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}
	// 本周期若无任何候选币有行情数据，显式说明原因，避免 AI 误以为「有候选但被隐藏」或误报无数据
	if displayedCount == 0 && len(ctx.CandidateCoins) > 0 {
		if e.GetLanguage() == LangChinese {
			sb.WriteString("（本周期暂无候选币种 K 线/技术数据，可能因行情接口暂时失败或候选拉取被过滤；请仅基于当前持仓与下方排行榜信息决策。）\n\n")
		} else {
			sb.WriteString("(No candidate coin kline/technical data this cycle—fetch may have failed or been filtered; base decision only on current positions and ranking data below.)\n\n")
		}
	}
	sb.WriteString("\n")

	// Get language for market data formatting
	nofxosLang := nofxos.LangEnglish
	if e.GetLanguage() == LangChinese {
		nofxosLang = nofxos.LangChinese
	}

	// OI Ranking data (market-wide open interest changes)
	if ctx.OIRankingData != nil {
		sb.WriteString(nofxos.FormatOIRankingForAI(ctx.OIRankingData, nofxosLang))
	}

	// NetFlow Ranking data (market-wide fund flow)
	if ctx.NetFlowRankingData != nil {
		sb.WriteString(nofxos.FormatNetFlowRankingForAI(ctx.NetFlowRankingData, nofxosLang))
	}

	// Price Ranking data (market-wide gainers/losers)
	if ctx.PriceRankingData != nil {
		sb.WriteString(nofxos.FormatPriceRankingForAI(ctx.PriceRankingData, nofxosLang))
	}

	// 多源数据说明：同一指标可能同时出现单所与多所/聚合，说明优先参考规则以减少干扰
	hasBinanceOrLiq := len(ctx.BinanceLongShortMap) > 0 || len(ctx.BinanceFundingMap) > 0 || len(ctx.BinanceTakerMap) > 0 ||
		len(ctx.BinanceFundingRateAvg8h) > 0 || ctx.LiquidationAgg != nil
	hasCoinglass := len(ctx.CoinglassFundingMap) > 0 || len(ctx.CoinglassLongShortMap) > 0 || ctx.CoinglassLiquidation != nil
	if hasBinanceOrLiq && hasCoinglass {
		if e.GetLanguage() == LangChinese {
			sb.WriteString("**说明**：以下同一指标可能同时出现单所（币安）与多所/聚合（Coinglass）。做方向与 regime 判断时优先参考多所/聚合数据，单所可作补充或交叉验证。\n\n")
		} else {
			sb.WriteString("**Note**: The same metric may appear from single-exchange (Binance) and multi-exchange/aggregate (Coinglass). Prefer multi-exchange/aggregate for direction and regime; use single-exchange as supplement or cross-check.\n\n")
		}
	}

	// Binance 衍生数据（多空比、资金费率、Taker）— 以币安为主增强市场判断
	if len(ctx.BinanceLongShortMap) > 0 || len(ctx.BinanceFundingMap) > 0 || len(ctx.BinanceTakerMap) > 0 {
		sb.WriteString(e.formatBinanceDerivativesForAI(ctx))
	}

	// 数据补强：资金费率 8h 均值、Basis、BTC 占比、强平聚合
	if len(ctx.BinanceFundingRateAvg8h) > 0 || len(ctx.BasisMap) > 0 || ctx.BTCDominancePct > 0 || ctx.LiquidationAgg != nil {
		sb.WriteString(e.formatDataStrengtheningForAI(ctx))
	}

	sb.WriteString("---\n\n")
	sb.WriteString("Now please analyze and output your decision (Chain of Thought + JSON)\n")

	return sb.String()
}

func (e *StrategyEngine) formatPositionInfo(index int, pos PositionInfo, ctx *Context) string {
	var sb strings.Builder

	holdingDuration := ""
	durationMin := int64(0)
	if pos.UpdateTime > 0 {
		durationMs := time.Now().UnixMilli() - pos.UpdateTime
		durationMin = durationMs / (1000 * 60)
		if durationMin < 60 {
			holdingDuration = fmt.Sprintf(" | Holding Duration %d min", durationMin)
		} else {
			durationHour := durationMin / 60
			durationMinRemainder := durationMin % 60
			holdingDuration = fmt.Sprintf(" | Holding Duration %dh %dm", durationHour, durationMinRemainder)
		}
	}

	positionValue := pos.Quantity * pos.MarkPrice
	if positionValue < 0 {
		positionValue = -positionValue
	}

	sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.4f Current %.4f | Qty %.4f | Position Value %.2f USDT | PnL%+.2f%% | PnL Amount%+.2f USDT | Peak PnL%.2f%% | Leverage %dx | Margin %.0f | Liq Price %.4f%s\n\n",
		index, pos.Symbol, strings.ToUpper(pos.Side),
		pos.EntryPrice, pos.MarkPrice, pos.Quantity, positionValue, pos.UnrealizedPnLPct, pos.UnrealizedPnL, pos.PeakPnLPct,
		pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration))

	// 显式说明是否已满最小持仓，避免 AI 推理时误写「未达到最小持仓」导致与真实执行逻辑矛盾
	minHoldMinutes := 0.0
	rc := e.config.RiskControl
	if sl := rc.DynamicStopLoss; sl != nil && sl.Enabled && sl.MinHoldMinutes > 0 {
		minHoldMinutes = sl.MinHoldMinutes
	}
	if tp := rc.DynamicTakeProfit; tp != nil && tp.Enabled && tp.MinHoldMinutes > 0 && tp.MinHoldMinutes > minHoldMinutes {
		minHoldMinutes = tp.MinHoldMinutes
	}
	if minHoldMinutes > 0 {
		zh := e.GetLanguage() == LangChinese
		if float64(durationMin) >= minHoldMinutes {
			if zh {
				sb.WriteString(fmt.Sprintf("   → 本仓位已满最小持仓时间（当前 %d 分钟 ≥ 最小 %.0f 分钟），策略会参与止损/止盈检查。\n\n", durationMin, minHoldMinutes))
			} else {
				sb.WriteString(fmt.Sprintf("   → This position has met min hold (current %d min ≥ %.0f min); strategy will run SL/TP checks.\n\n", durationMin, minHoldMinutes))
			}
		} else {
			if zh {
				sb.WriteString(fmt.Sprintf("   → 本仓位未满最小持仓时间（当前 %d 分钟 < 最小 %.0f 分钟），本周期策略不参与止损/止盈。\n\n", durationMin, minHoldMinutes))
			} else {
				sb.WriteString(fmt.Sprintf("   → This position has not met min hold (current %d min < %.0f min); strategy will skip SL/TP this cycle.\n\n", durationMin, minHoldMinutes))
			}
		}
	}

	if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
		sb.WriteString(e.formatMarketData(marketData))

		if ctx.QuantDataMap != nil {
			if quantData, hasQuant := ctx.QuantDataMap[pos.Symbol]; hasQuant {
				sb.WriteString(e.formatQuantData(quantData))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatStrategyDynamicSLTP returns a short summary of strategy dynamic SL/TP for the AI prompt,
// so reasoning can explicitly mention 初始止损、止盈、ATR倍数/周期 and that 追踪止损/分层止盈由策略执行.
func (e *StrategyEngine) formatStrategyDynamicSLTP() string {
	rc := e.config.RiskControl
	sl := rc.DynamicStopLoss
	tp := rc.DynamicTakeProfit
	if (sl == nil || !sl.Enabled) && (tp == nil || !tp.Enabled) {
		return ""
	}
	zh := e.GetLanguage() == LangChinese
	var sb strings.Builder
	if zh {
		sb.WriteString("## 策略动态止损/止盈配置（无固定初始止损；请在 reasoning 中写出止损价、止盈价、ATR倍数与周期，并说明追踪止损与分层止盈由策略执行）\n")
		sb.WriteString("- 执行顺序：先止盈检查再止损；止盈内为 分层→回撤→固定→ATR→阻力。\n")
	} else {
		sb.WriteString("## Strategy dynamic SL/TP config (no fixed initial stop; in reasoning state stop price, take profit, ATR multiplier & period; state that trailing stop and scaled take profit are executed by strategy)\n")
		sb.WriteString("- Execution order: TP check first, then SL; within TP: scaled → trailing_tp → fixed → atr → resistance.\n")
	}
	if sl != nil && sl.Enabled {
		if zh {
			sb.WriteString(fmt.Sprintf("- 最小持仓: %.0f 分钟（未满不触发动态止损）", sl.MinHoldMinutes))
		} else {
			sb.WriteString(fmt.Sprintf("- Min hold: %.0f min (no dynamic SL before this)", sl.MinHoldMinutes))
		}
		if sl.TrailingEnabled != nil && *sl.TrailingEnabled && len(sl.TrailingLevels) > 0 {
			if zh {
				sb.WriteString(" | 追踪止损(分层): 开")
			} else {
				sb.WriteString(" | Trailing stop (tiered): on")
			}
		}
		if sl.ATREnabled != nil && *sl.ATREnabled {
			min, max := 0.0, 0.0
			if sl.ATRMultiplierMin != nil {
				min = *sl.ATRMultiplierMin
			}
			if sl.ATRMultiplierMax != nil {
				max = *sl.ATRMultiplierMax
			}
			if zh {
				sb.WriteString(fmt.Sprintf(" | ATR止损倍数: %.1f–%.1f", min, max))
			} else {
				sb.WriteString(fmt.Sprintf(" | ATR stop multiplier: %.1f–%.1f", min, max))
			}
		}
		if sl.SupportResistanceEnabled != nil && *sl.SupportResistanceEnabled {
			if zh {
				sb.WriteString(" | 支撑阻力止损: 开")
			} else {
				sb.WriteString(" | Support/Resistance stop: on")
			}
			if sl.SupportResistanceBuffer != nil && *sl.SupportResistanceBuffer > 0 {
				if zh {
					sb.WriteString(fmt.Sprintf("(缓冲%.2f%%)", *sl.SupportResistanceBuffer))
				} else {
					sb.WriteString(fmt.Sprintf("(buffer %.2f%%)", *sl.SupportResistanceBuffer))
				}
			}
		}
		if sl.AdverseExitWhenNeverProfitATR != nil && *sl.AdverseExitWhenNeverProfitATR > 0 {
			if zh {
				sb.WriteString(fmt.Sprintf(" | 从未浮盈+反向≥%.1f×ATR早退: 开", *sl.AdverseExitWhenNeverProfitATR))
			} else {
				sb.WriteString(fmt.Sprintf(" | Adverse exit when never profit + reverse ≥%.1f×ATR: on", *sl.AdverseExitWhenNeverProfitATR))
			}
			if sl.AdverseExitWhenNeverProfitATRAltcoin != nil && *sl.AdverseExitWhenNeverProfitATRAltcoin > 0 {
				if zh {
					sb.WriteString(fmt.Sprintf("(山寨%.1f×)", *sl.AdverseExitWhenNeverProfitATRAltcoin))
				} else {
					sb.WriteString(fmt.Sprintf("(altcoin %.1f×)", *sl.AdverseExitWhenNeverProfitATRAltcoin))
				}
			}
			if sl.AdverseExitRequireATRSpike != nil && *sl.AdverseExitRequireATRSpike {
				if zh {
					sb.WriteString(" | 逆势早退需ATR骤升")
				} else {
					sb.WriteString(" | adverse exit requires ATR spike")
				}
			}
		}
		if sl.KlinesTimeframe != "" && sl.KlinesTimeframe != "15m" {
			if zh {
				sb.WriteString(fmt.Sprintf(" | 止损K线周期: %s", sl.KlinesTimeframe))
			} else {
				sb.WriteString(fmt.Sprintf(" | SL klines: %s", sl.KlinesTimeframe))
			}
		}
		if sl.SupportResistanceUseEMA20 != nil && *sl.SupportResistanceUseEMA20 {
			if zh {
				sb.WriteString(" | 支撑/阻力用EMA20")
			} else {
				sb.WriteString(" | S/R use EMA20")
			}
		}
		if sl.ConfirmMinutes > 0 {
			if zh {
				sb.WriteString(fmt.Sprintf(" | 确认时长: %.0f分钟", sl.ConfirmMinutes))
			} else {
				sb.WriteString(fmt.Sprintf(" | Confirm: %.0f min", sl.ConfirmMinutes))
			}
		}
		sb.WriteString("\n")
	}
	if tp != nil && tp.Enabled {
		if zh {
			sb.WriteString(fmt.Sprintf("- 止盈: 最小持仓 %.0f 分钟", tp.MinHoldMinutes))
		} else {
			sb.WriteString(fmt.Sprintf("- Take profit: min hold %.0f min", tp.MinHoldMinutes))
		}
		if tp.MinProfitPercentToAllowTP != nil && *tp.MinProfitPercentToAllowTP > 0 {
			if zh {
				sb.WriteString(fmt.Sprintf(" | 最低盈利%.1f%%才止盈", *tp.MinProfitPercentToAllowTP))
			} else {
				sb.WriteString(fmt.Sprintf(" | min profit %.1f%% to allow TP", *tp.MinProfitPercentToAllowTP))
			}
		}
		// 与执行逻辑一致：有档位即视为分层止盈启用（ScaledEnabled 为 nil 时也展示）
		scaledEffective := len(tp.ScaledLevels) > 0 && (tp.ScaledEnabled == nil || *tp.ScaledEnabled)
		if scaledEffective {
			if zh {
				sb.WriteString(" | 分层止盈: ")
			} else {
				sb.WriteString(" | Scaled TP: ")
			}
			for i, lv := range tp.ScaledLevels {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%.1f%%平%.0f%%", lv.ProfitPercent, lv.ClosePercent))
			}
		}
		if tp.ATREnabled != nil && *tp.ATREnabled {
			min, max := 0.0, 0.0
			if tp.ATRMultiplierMin != nil {
				min = *tp.ATRMultiplierMin
			}
			if tp.ATRMultiplierMax != nil {
				max = *tp.ATRMultiplierMax
			}
			if zh {
				sb.WriteString(fmt.Sprintf(" | ATR止盈倍数: %.1f–%.1f", min, max))
			} else {
				sb.WriteString(fmt.Sprintf(" | ATR TP multiplier: %.1f–%.1f", min, max))
			}
			if tp.ATRUseMaxInHighVolatility != nil && *tp.ATRUseMaxInHighVolatility {
				th := 1.2
				if tp.ATRHighVolatilityThreshold != nil {
					th = *tp.ATRHighVolatilityThreshold
				}
				if zh {
					sb.WriteString(fmt.Sprintf(" (高波动用Max,阈值%.1f)", th))
				} else {
					sb.WriteString(fmt.Sprintf(" (high vol use max, th %.1f)", th))
				}
			}
		}
		if tp.ResistanceEnabled != nil && *tp.ResistanceEnabled {
			if zh {
				sb.WriteString(" | 阻力位止盈: 开")
			} else {
				sb.WriteString(" | Resistance take profit: on")
			}
			if tp.ResistanceBuffer != nil && *tp.ResistanceBuffer > 0 {
				if zh {
					sb.WriteString(fmt.Sprintf("(缓冲%.2f%%)", *tp.ResistanceBuffer))
				} else {
					sb.WriteString(fmt.Sprintf("(buffer %.2f%%)", *tp.ResistanceBuffer))
				}
			}
		}
		if tp.TrailingTPEnabled != nil && *tp.TrailingTPEnabled {
			act := 2.0
			if tp.TrailingTPActivateProfitPct != nil {
				act = *tp.TrailingTPActivateProfitPct
			}
			ret := 1.5
			if tp.TrailingTPRetracePct != nil {
				ret = *tp.TrailingTPRetracePct
			}
			if zh {
				sb.WriteString(fmt.Sprintf(" | 回撤止盈: 激活≥%.1f%% 回撤≥%.1f%%", act, ret))
			} else {
				sb.WriteString(fmt.Sprintf(" | Trailing TP: activate ≥%.1f%%, retrace ≥%.1f%%", act, ret))
			}
			if (tp.TrailingTPActivateProfitPctAltcoin != nil && *tp.TrailingTPActivateProfitPctAltcoin > 0) || (tp.TrailingTPRetracePctAltcoin != nil && *tp.TrailingTPRetracePctAltcoin > 0) {
				if zh {
					sb.WriteString("(山寨另设)")
				} else {
					sb.WriteString("(altcoin separate)")
				}
			}
		}
		if tp.LockProfitPercent != nil && *tp.LockProfitPercent > 0 {
			if zh {
				sb.WriteString(fmt.Sprintf(" | 锁定利润: 达%.1f%%(保证金%%)后移动止损到盈亏平衡", *tp.LockProfitPercent))
			} else {
				sb.WriteString(fmt.Sprintf(" | Lock profit: move stop to breakeven after %.1f%% (margin%%)", *tp.LockProfitPercent))
			}
		}
		sb.WriteString("\n")
	}
	if zh {
		sb.WriteString("- 口径说明：止盈档位与回撤止盈均为价格%；锁定利润阈值为保证金收益率%。\n")
	} else {
		sb.WriteString("- Note: TP levels and trailing TP use price%%; lock profit threshold uses margin%%.\n")
	}
	return sb.String()
}

func (e *StrategyEngine) formatCoinSourceTag(sources []string) string {
	if len(sources) > 1 {
		// 多信号源组合
		hasAI500 := false
		hasOITop := false
		hasOILow := false
		for _, s := range sources {
			switch s {
			case "ai500":
				hasAI500 = true
			case "oi_top":
				hasOITop = true
			case "oi_low":
				hasOILow = true
			}
		}
		if hasAI500 && hasOITop {
			return " (AI500+OI_Top dual signal)"
		}
		if hasAI500 && hasOILow {
			return " (AI500+OI_Low dual signal)"
		}
		if hasOITop && hasOILow {
			return " (OI_Top+OI_Low)"
		}
		return " (Multiple sources)"
	} else if len(sources) == 1 {
		switch sources[0] {
		case "ai500":
			return " (AI500)"
		case "oi_top":
			return " (OI_Top 持仓增加)"
		case "oi_low":
			return " (OI_Low 持仓减少)"
		case "static":
			return " (Manual selection)"
		}
	}
	return ""
}

// ============================================================================
// Market Data Formatting
// ============================================================================

func (e *StrategyEngine) formatMarketData(data *market.Data) string {
	var sb strings.Builder
	indicators := e.config.Indicators

	// 明确标注币种
	sb.WriteString(fmt.Sprintf("=== %s Market Data ===\n\n", data.Symbol))
	// 显式 4h/1h 方向与是否同向，便于先定方向再在 15m 找入场（策略理念：方向正确优先，震荡也做方向判断）
	const dirThreshold = 0.15
	dir4h := "ranging"
	if data.PriceChange4h > dirThreshold {
		dir4h = "up"
	} else if data.PriceChange4h < -dirThreshold {
		dir4h = "down"
	}
	dir1h := "ranging"
	if data.PriceChange1h > dirThreshold {
		dir1h = "up"
	} else if data.PriceChange1h < -dirThreshold {
		dir1h = "down"
	}
	sameDir := (dir4h == dir1h) || (dir4h != "ranging" && dir1h != "ranging" && (data.PriceChange4h > 0) == (data.PriceChange1h > 0))
	if e.GetLanguage() == LangChinese {
		d4, d1 := dir4h, dir1h
		if d4 == "up" {
			d4 = "多"
		} else if d4 == "down" {
			d4 = "空"
		} else {
			d4 = "震荡"
		}
		if d1 == "up" {
			d1 = "多"
		} else if d1 == "down" {
			d1 = "空"
		} else {
			d1 = "震荡"
		}
		sb.WriteString(fmt.Sprintf("多周期涨跌: 1h %+.2f%% 4h %+.2f%%", data.PriceChange1h, data.PriceChange4h))
		if data.PriceChange24h != 0 || data.PriceChange7d != 0 {
			sb.WriteString(fmt.Sprintf(" | 24h %+.2f%% 7d %+.2f%%", data.PriceChange24h, data.PriceChange7d))
		}
		sb.WriteString("（负=偏空 正=偏多）\n")
		sb.WriteString(fmt.Sprintf("4h方向: %s | 1h方向: %s | 1h与4h同向: %s（请先据此定多空方向，再在主周期找入场；多周期不一致时倾向轻仓或提高置信度，仍可开仓）\n\n", d4, d1, map[bool]string{true: "是", false: "否"}[sameDir]))
	} else {
		sb.WriteString(fmt.Sprintf("Multi-timeframe: 1h %+.2f%% 4h %+.2f%%", data.PriceChange1h, data.PriceChange4h))
		if data.PriceChange24h != 0 || data.PriceChange7d != 0 {
			sb.WriteString(fmt.Sprintf(" | 24h %+.2f%% 7d %+.2f%%", data.PriceChange24h, data.PriceChange7d))
		}
		sb.WriteString(" (negative=bearish, positive=bullish)\n")
		sb.WriteString(fmt.Sprintf("4h dir: %s | 1h dir: %s | 1h vs 4h aligned: %s (set direction from 4h/1h first, then find entry on primary TF; when not aligned use smaller size or higher confidence, still may open e.g. in clear range)\n\n", dir4h, dir1h, map[bool]string{true: "yes", false: "no"}[sameDir]))
	}
	sb.WriteString(fmt.Sprintf("current_price = %.4f", data.CurrentPrice))

	if indicators.EnableEMA {
		sb.WriteString(fmt.Sprintf(", current_ema20 = %.3f", data.CurrentEMA20))
	}

	if indicators.EnableMACD {
		sb.WriteString(fmt.Sprintf(", current_macd = %.3f", data.CurrentMACD))
	}

	if indicators.EnableRSI {
		sb.WriteString(fmt.Sprintf(", current_rsi7 = %.3f", data.CurrentRSI7))
	}

	sb.WriteString("\n\n")

	if indicators.EnableOI || indicators.EnableFundingRate {
		sb.WriteString(fmt.Sprintf("Additional data for %s:\n\n", data.Symbol))

		if indicators.EnableOI && data.OpenInterest != nil {
			sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n",
				data.OpenInterest.Latest, data.OpenInterest.Average))
		}

		if indicators.EnableFundingRate {
			sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
		}
	}

	if len(data.TimeframeData) > 0 {
		primaryTf := indicators.Klines.PrimaryTimeframe
		if primaryTf == "" && len(indicators.Klines.SelectedTimeframes) > 0 {
			primaryTf = indicators.Klines.SelectedTimeframes[0]
		}
		compactNonPrimary := indicators.CompactNonPrimaryTimeframe
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				isPrimary := (tf == primaryTf)
				if compactNonPrimary && !isPrimary {
					sb.WriteString(fmt.Sprintf("=== %s (compact) ===\n", strings.ToUpper(tf)))
					e.formatTimeframeSeriesDataCompact(&sb, tfData, indicators)
				} else {
					sb.WriteString(fmt.Sprintf("=== %s Timeframe (oldest → latest) ===\n\n", strings.ToUpper(tf)))
					e.formatTimeframeSeriesData(&sb, tfData, indicators)
				}
			}
		}
	} else {
		// Compatible with old data format
		if data.IntradaySeries != nil {
			klineConfig := indicators.Klines
			sb.WriteString(fmt.Sprintf("Intraday series (%s intervals, oldest → latest):\n\n", klineConfig.PrimaryTimeframe))

			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}

			if indicators.EnableEMA && len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA indicators (20-period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}

			if indicators.EnableMACD && len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}

			if indicators.EnableRSI {
				if len(data.IntradaySeries.RSI7Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (7-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
				}
				if len(data.IntradaySeries.RSI14Values) > 0 {
					sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
				}
			}

			if indicators.EnableVolume && len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3m ATR (14-period): %.3f\n\n", data.IntradaySeries.ATR14))
			}
		}

		if data.LongerTermContext != nil && indicators.Klines.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf("Longer-term context (%s timeframe):\n\n", indicators.Klines.LongerTimeframe))

			if indicators.EnableEMA {
				sb.WriteString(fmt.Sprintf("20-Period EMA: %.3f vs. 50-Period EMA: %.3f\n\n",
					data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))
			}

			if indicators.EnableATR {
				sb.WriteString(fmt.Sprintf("3-Period ATR: %.3f vs. 14-Period ATR: %.3f\n\n",
					data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))
			}

			if indicators.EnableVolume {
				sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
					data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))
			}

			if indicators.EnableMACD && len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}

			if indicators.EnableRSI && len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}
		}
	}

	return sb.String()
}

// formatTimeframeSeriesDataCompact writes a one-line summary for token saving (latest close, EMA20, EMA50, ATR14).
func (e *StrategyEngine) formatTimeframeSeriesDataCompact(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig) {
	closeStr := "n/a"
	if len(data.Klines) > 0 {
		closeStr = fmt.Sprintf("%.4f", data.Klines[len(data.Klines)-1].Close)
	} else if len(data.MidPrices) > 0 {
		closeStr = fmt.Sprintf("%.4f", data.MidPrices[len(data.MidPrices)-1])
	}
	ema20Str, ema50Str := "", ""
	if indicators.EnableEMA {
		if len(data.EMA20Values) > 0 {
			ema20Str = fmt.Sprintf(" EMA20=%.3f", data.EMA20Values[len(data.EMA20Values)-1])
		}
		if len(data.EMA50Values) > 0 {
			ema50Str = fmt.Sprintf(" EMA50=%.3f", data.EMA50Values[len(data.EMA50Values)-1])
		}
	}
	atrStr := ""
	if indicators.EnableATR && data.ATR14 > 0 {
		atrStr = fmt.Sprintf(" ATR14=%.4f", data.ATR14)
	}
	sb.WriteString(fmt.Sprintf("Close=%s%s%s%s\n\n", closeStr, ema20Str, ema50Str, atrStr))
}

func (e *StrategyEngine) formatTimeframeSeriesData(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig) {
	if len(data.Klines) > 0 {
		sb.WriteString("Time(UTC)      Open      High      Low       Close     Volume\n")
		for i, k := range data.Klines {
			t := time.Unix(k.Time/1000, 0).UTC()
			timeStr := t.Format("01-02 15:04")
			marker := ""
			if i == len(data.Klines)-1 {
				marker = "  <- current"
			}
			sb.WriteString(fmt.Sprintf("%-14s %-9.4f %-9.4f %-9.4f %-9.4f %-12.2f%s\n",
				timeStr, k.Open, k.High, k.Low, k.Close, k.Volume, marker))
		}
		sb.WriteString("\n")
	} else if len(data.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.MidPrices)))
		if indicators.EnableVolume && len(data.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.Volume)))
		}
	}

	if indicators.EnableEMA {
		if len(data.EMA20Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA20: %s\n", formatFloatSlice(data.EMA20Values)))
		}
		if len(data.EMA50Values) > 0 {
			sb.WriteString(fmt.Sprintf("EMA50: %s\n", formatFloatSlice(data.EMA50Values)))
		}
	}

	if indicators.EnableMACD && len(data.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD: %s\n", formatFloatSlice(data.MACDValues)))
	}

	if indicators.EnableRSI {
		if len(data.RSI7Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI7: %s\n", formatFloatSlice(data.RSI7Values)))
		}
		if len(data.RSI14Values) > 0 {
			sb.WriteString(fmt.Sprintf("RSI14: %s\n", formatFloatSlice(data.RSI14Values)))
		}
	}

	if indicators.EnableATR && data.ATR14 > 0 {
		sb.WriteString(fmt.Sprintf("ATR14: %.4f\n", data.ATR14))
	}

	if indicators.EnableBOLL && len(data.BOLLUpper) > 0 {
		sb.WriteString(fmt.Sprintf("BOLL Upper: %s\n", formatFloatSlice(data.BOLLUpper)))
		sb.WriteString(fmt.Sprintf("BOLL Middle: %s\n", formatFloatSlice(data.BOLLMiddle)))
		sb.WriteString(fmt.Sprintf("BOLL Lower: %s\n", formatFloatSlice(data.BOLLLower)))
	}

	sb.WriteString("\n")
}

func (e *StrategyEngine) formatQuantData(data *QuantData) string {
	if data == nil {
		return ""
	}

	indicators := e.config.Indicators
	if !indicators.EnableQuantOI && !indicators.EnableQuantNetflow {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s Quantitative Data:\n", data.Symbol))

	if len(data.PriceChange) > 0 {
		sb.WriteString("Price Change: ")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}
		parts := []string{}
		for _, tf := range timeframes {
			if v, ok := data.PriceChange[tf]; ok {
				parts = append(parts, fmt.Sprintf("%s: %+.4f%%", tf, v*100))
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
	}

	if indicators.EnableQuantNetflow && data.Netflow != nil {
		sb.WriteString("Fund Flow (Netflow):\n")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}

		if data.Netflow.Institution != nil {
			if data.Netflow.Institution.Future != nil && len(data.Netflow.Institution.Future) > 0 {
				sb.WriteString("  Institutional Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if data.Netflow.Institution.Spot != nil && len(data.Netflow.Institution.Spot) > 0 {
				sb.WriteString("  Institutional Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}

		if data.Netflow.Personal != nil {
			if data.Netflow.Personal.Future != nil && len(data.Netflow.Personal.Future) > 0 {
				sb.WriteString("  Retail Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if data.Netflow.Personal.Spot != nil && len(data.Netflow.Personal.Spot) > 0 {
				sb.WriteString("  Retail Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}
	}

	if indicators.EnableQuantOI && len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if len(oiData.Delta) > 0 {
				sb.WriteString(fmt.Sprintf("Open Interest (%s):\n", exchange))
				for _, tf := range []string{"5m", "15m", "1h", "4h", "12h", "24h"} {
					if d, ok := oiData.Delta[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %+.4f%% (%s)\n", tf, d.OIDeltaPercent, formatFlowValue(d.OIDeltaValue)))
					}
				}
			}
		}
	}

	return sb.String()
}

// formatBinanceDerivativesForAI 将 Context 中的币安多空比/资金费率/Taker 格式化为 AI 可读文本
func (e *StrategyEngine) formatBinanceDerivativesForAI(ctx *Context) string {
	var sb strings.Builder
	lang := e.GetLanguage()
	if lang == LangChinese {
		sb.WriteString("## 币安衍生数据（多空比 / 资金费率 / Taker）\n\n")
	} else {
		sb.WriteString("## Binance Derivatives (Long/Short, Funding, Taker)\n\n")
	}
	// 多空比
	for sym, ls := range ctx.BinanceLongShortMap {
		if ls == nil {
			continue
		}
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **%s** 全账户多空比: %.4f (多%.2f%% / 空%.2f%%)", sym, ls.LongShortRatio, ls.LongAccount*100, ls.ShortAccount*100))
			if ls.TopLongShortRatio > 0 {
				sb.WriteString(fmt.Sprintf(" | 大户多空比: %.4f", ls.TopLongShortRatio))
			}
			sb.WriteString("\n")
		} else {
			sb.WriteString(fmt.Sprintf("- **%s** L/S ratio: %.4f (long %.2f%% / short %.2f%%)", sym, ls.LongShortRatio, ls.LongAccount*100, ls.ShortAccount*100))
			if ls.TopLongShortRatio > 0 {
				sb.WriteString(fmt.Sprintf(" | top L/S: %.4f", ls.TopLongShortRatio))
			}
			sb.WriteString("\n")
		}
	}
	// 资金费率
	for sym, f := range ctx.BinanceFundingMap {
		if f == nil {
			continue
		}
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **%s** 资金费率: %.4f%% | 标记价: %.4f\n", sym, f.LastFundingRate*100, f.MarkPrice))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s** funding: %.4f%% | mark: %.4f\n", sym, f.LastFundingRate*100, f.MarkPrice))
		}
	}
	// Taker
	for sym, t := range ctx.BinanceTakerMap {
		if t == nil {
			continue
		}
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **%s** Taker买卖比: %.4f (买%.0f / 卖%.0f)\n", sym, t.BuySellRatio, t.BuyVol, t.SellVol))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s** Taker buy/sell ratio: %.4f (buy %.0f / sell %.0f)\n", sym, t.BuySellRatio, t.BuyVol, t.SellVol))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// formatDataStrengtheningForAI 将资金费率 8h 均值、Basis、BTC 占比、强平聚合格式化为 AI 可读
func (e *StrategyEngine) formatDataStrengtheningForAI(ctx *Context) string {
	var sb strings.Builder
	lang := e.GetLanguage()
	if lang == LangChinese {
		sb.WriteString("## 数据补强（资金费率历史 / Basis / BTC 占比 / 强平）\n\n")
	} else {
		sb.WriteString("## Data strengthening (Funding 8h avg / Basis / BTC dominance / Liquidation)\n\n")
	}
	if len(ctx.BinanceFundingRateAvg8h) > 0 {
		if lang == LangChinese {
			sb.WriteString("- **资金费率近 8h 均值**: ")
		} else {
			sb.WriteString("- **Funding rate 8h avg**: ")
		}
		first := true
		for sym, avg := range ctx.BinanceFundingRateAvg8h {
			if !first {
				sb.WriteString(" | ")
			}
			sb.WriteString(fmt.Sprintf("%s %.4f%%", sym, avg*100))
			first = false
		}
		sb.WriteString("\n")
	}
	if len(ctx.FundingRateHistoryLast8) > 0 {
		if lang == LangChinese {
			sb.WriteString("- **资金费近 8 期（由旧到新）**: ")
		} else {
			sb.WriteString("- **Funding last 8 periods (old→new)**: ")
		}
		first := true
		for sym, arr := range ctx.FundingRateHistoryLast8 {
			if len(arr) == 0 {
				continue
			}
			if !first {
				sb.WriteString(" | ")
			}
			parts := make([]string, len(arr))
			for i, r := range arr {
				parts[i] = fmt.Sprintf("%+.3f%%", r*100)
			}
			sb.WriteString(fmt.Sprintf("%s [%s]", sym, strings.Join(parts, ",")))
			first = false
		}
		sb.WriteString("\n")
	}
	if len(ctx.BasisMap) > 0 {
		if lang == LangChinese {
			sb.WriteString("- **永续-现货价差 Basis**: ")
		} else {
			sb.WriteString("- **Basis (perpetual-spot)**: ")
		}
		first := true
		for sym, pct := range ctx.BasisMap {
			if !first {
				sb.WriteString(" | ")
			}
			sb.WriteString(fmt.Sprintf("%s %.3f%%", sym, pct))
			first = false
		}
		sb.WriteString("\n")
	}
	if ctx.BTCDominancePct > 0 {
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **BTC 市值占比**: %.2f%%\n", ctx.BTCDominancePct))
		} else {
			sb.WriteString(fmt.Sprintf("- **BTC dominance**: %.2f%%\n", ctx.BTCDominancePct))
		}
	}
	if ctx.LiquidationAgg != nil {
		liq := ctx.LiquidationAgg
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **强平聚合 (%s)** 1h 多: %.0f 空: %.0f | 4h 多: %.0f 空: %.0f",
				liq.Source, liq.Long1hUSD, liq.Short1hUSD, liq.Long4hUSD, liq.Short4hUSD))
			if liq.Long24hUSD != 0 || liq.Short24hUSD != 0 {
				sb.WriteString(fmt.Sprintf(" | 24h 多: %.0f 空: %.0f", liq.Long24hUSD, liq.Short24hUSD))
			}
			sb.WriteString(" USD\n")
		} else {
			sb.WriteString(fmt.Sprintf("- **Liquidation (%s)** 1h L: %.0f S: %.0f | 4h L: %.0f S: %.0f",
				liq.Source, liq.Long1hUSD, liq.Short1hUSD, liq.Long4hUSD, liq.Short4hUSD))
			if liq.Long24hUSD != 0 || liq.Short24hUSD != 0 {
				sb.WriteString(fmt.Sprintf(" | 24h L: %.0f S: %.0f", liq.Long24hUSD, liq.Short24hUSD))
			}
			sb.WriteString(" USD\n")
		}
	}
	if len(ctx.CoinglassOISummaries) > 0 {
		if lang == LangChinese {
			sb.WriteString("- **Coinglass OI（经 KeyStore 中转）**: ")
		} else {
			sb.WriteString("- **Coinglass OI (via KeyStore proxy)**: ")
		}
		for i, s := range ctx.CoinglassOISummaries {
			if i > 0 {
				sb.WriteString(" | ")
			}
			b := s.OpenInterestUSD / 1e9
			sb.WriteString(fmt.Sprintf("%s %.2fB USD, 1h %+.2f%%, 4h %+.2f%%", s.Symbol, b, s.Change1hPct, s.Change4hPct))
		}
		sb.WriteString("\n")
	}
	// Coinglass 资金费率（多所）
	if len(ctx.CoinglassFundingMap) > 0 {
		if lang == LangChinese {
			sb.WriteString("- **Coinglass 资金费率（多所）**: ")
		} else {
			sb.WriteString("- **Coinglass funding (multi-exchange)**: ")
		}
		first := true
		for sym, rate := range ctx.CoinglassFundingMap {
			if !first {
				sb.WriteString(" | ")
			}
			sb.WriteString(fmt.Sprintf("%s %.4f%%", sym, rate*100))
			first = false
		}
		sb.WriteString("\n")
	}
	// Coinglass 多空比
	if len(ctx.CoinglassLongShortMap) > 0 {
		if lang == LangChinese {
			sb.WriteString("- **Coinglass 多空比（全账户/大户）**: ")
		} else {
			sb.WriteString("- **Coinglass long/short (global/top)**: ")
		}
		first := true
		for sym, ls := range ctx.CoinglassLongShortMap {
			if ls == nil {
				continue
			}
			if !first {
				sb.WriteString(" | ")
			}
			sb.WriteString(fmt.Sprintf("%s 全账户多%.0f%% 大户多%.0f%%", sym, ls.LongAccount*100, ls.TopLongAccount*100))
			first = false
		}
		sb.WriteString("\n")
	}
	// Coinglass 强平聚合
	if ctx.CoinglassLiquidation != nil {
		liq := ctx.CoinglassLiquidation
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **Coinglass 强平聚合** 1h 多: %.0f 空: %.0f | 4h 多: %.0f 空: %.0f | 24h 多: %.0f 空: %.0f USD\n",
				liq.Long1hUSD, liq.Short1hUSD, liq.Long4hUSD, liq.Short4hUSD, liq.Long24hUSD, liq.Short24hUSD))
		} else {
			sb.WriteString(fmt.Sprintf("- **Coinglass liquidation** 1h L: %.0f S: %.0f | 4h L: %.0f S: %.0f | 24h L: %.0f S: %.0f USD\n",
				liq.Long1hUSD, liq.Short1hUSD, liq.Long4hUSD, liq.Short4hUSD, liq.Long24hUSD, liq.Short24hUSD))
		}
	}
	// 恐惧贪婪指数
	if ctx.FearGreedValue > 0 || ctx.FearGreedClassification != "" {
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **恐惧贪婪指数**: %d %s\n", ctx.FearGreedValue, ctx.FearGreedClassification))
		} else {
			sb.WriteString(fmt.Sprintf("- **Fear & Greed Index**: %d %s\n", ctx.FearGreedValue, ctx.FearGreedClassification))
		}
	}
	// 山寨季指数
	if ctx.AltcoinSeasonIndex > 0 {
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **山寨季指数**: %d\n", ctx.AltcoinSeasonIndex))
		} else {
			sb.WriteString(fmt.Sprintf("- **Altcoin Season Index**: %d\n", ctx.AltcoinSeasonIndex))
		}
	}
	// ETF 资金流
	if ctx.ETFFlowBTCRecent != 0 || ctx.ETFFlowETHRecent != 0 {
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **ETF 资金流（近期）**: BTC %s USD | ETH %s USD\n", formatFlowValue(ctx.ETFFlowBTCRecent), formatFlowValue(ctx.ETFFlowETHRecent)))
		} else {
			sb.WriteString(fmt.Sprintf("- **ETF flow (recent)**: BTC %s USD | ETH %s USD\n", formatFlowValue(ctx.ETFFlowBTCRecent), formatFlowValue(ctx.ETFFlowETHRecent)))
		}
	}
	// 鲸鱼指数 / CGDI / CDRI
	if ctx.CoinglassWhaleIndex != 0 || ctx.CoinglassCGDI != 0 || ctx.CoinglassCDRI != 0 {
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **鲸鱼指数/CGDI/CDRI**: Whale %.2f | CGDI(多空扩散) %.2f | CDRI(衍生品风险) %.2f\n", ctx.CoinglassWhaleIndex, ctx.CoinglassCGDI, ctx.CoinglassCDRI))
		} else {
			sb.WriteString(fmt.Sprintf("- **Whale/CGDI/CDRI**: Whale %.2f | CGDI %.2f | CDRI %.2f\n", ctx.CoinglassWhaleIndex, ctx.CoinglassCGDI, ctx.CoinglassCDRI))
		}
	}
	// 强平热力图关键价位
	if ctx.CoinglassLiquidationKeyLevels != "" {
		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **强平热力图关键价位**: %s\n", ctx.CoinglassLiquidationKeyLevels))
		} else {
			sb.WriteString(fmt.Sprintf("- **Liquidation heatmap key levels**: %s\n", ctx.CoinglassLiquidationKeyLevels))
		}
	}
	// Coinglass WSS 实时快照（若存在）
	if len(ctx.CoinglassWSSSnapshot) > 0 {
		if lang == LangChinese {
			sb.WriteString("- **Coinglass WSS 实时**: 已接入融资率/清算/OI/价格频道，见原始数据参考\n")
		} else {
			sb.WriteString("- **Coinglass WSS real-time**: funding/liquidation/OI/price channels available\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func formatFlowValue(v float64) string {
	sign := ""
	if v >= 0 {
		sign = "+"
	}
	absV := v
	if absV < 0 {
		absV = -absV
	}
	if absV >= 1e9 {
		return fmt.Sprintf("%s%.2fB", sign, v/1e9)
	} else if absV >= 1e6 {
		return fmt.Sprintf("%s%.2fM", sign, v/1e6)
	} else if absV >= 1e3 {
		return fmt.Sprintf("%s%.2fK", sign, v/1e3)
	}
	return fmt.Sprintf("%s%.2f", sign, v)
}

func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.4f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}

// ============================================================================
// AI Response Parsing
// ============================================================================

func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) (*FullDecision, error) {
	cotTrace := extractCoTTrace(aiResponse)

	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", err)
	}

	fd := &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}
	// 解析可选的 <analysis>：market_summary, market_regime, risk_alert, near_term_outlook, scenario, key_levels
	if match := reAnalysisTag.FindStringSubmatch(aiResponse); len(match) > 1 {
		analysisPart := strings.TrimSpace(match[1])
		// 允许 ```json ... ``` 包裹
		if idx := strings.Index(analysisPart, "{"); idx >= 0 {
			end := strings.LastIndex(analysisPart, "}")
			if end > idx {
				analysisPart = analysisPart[idx : end+1]
			}
		}
		var analysis struct {
			MarketSummary               string                    `json:"market_summary"`
			MarketRegime                string                    `json:"market_regime"`
			RiskAlert                   *bool                     `json:"risk_alert"`
			NearTermOutlook             string                    `json:"near_term_outlook"`
			Scenario                    string                    `json:"scenario"`
			KeyLevels                   []string                  `json:"key_levels"`
			SymbolPredictions           []SymbolPrediction        `json:"symbol_predictions"`
			SymbolStructureSignals      []SymbolStructureSignal   `json:"symbol_structure_signals"`
			PositionSLTPAdjustments     []PositionSLTPAdjustment   `json:"position_sl_tp_adjustments"`
		}
		if err := json.Unmarshal([]byte(analysisPart), &analysis); err == nil {
			fd.MarketSummary = strings.TrimSpace(analysis.MarketSummary)
			fd.MarketRegime = strings.TrimSpace(analysis.MarketRegime)
			if analysis.RiskAlert != nil && *analysis.RiskAlert {
				fd.RiskAlert = analysis.RiskAlert != nil && *analysis.RiskAlert
			}
			fd.NearTermOutlook = strings.TrimSpace(analysis.NearTermOutlook)
			fd.Scenario = strings.TrimSpace(analysis.Scenario)
			if len(analysis.KeyLevels) > 0 {
				fd.KeyLevels = analysis.KeyLevels
			}
			if len(analysis.SymbolPredictions) > 0 {
				fd.SymbolPredictions = analysis.SymbolPredictions
			}
			if len(analysis.SymbolStructureSignals) > 0 {
				fd.SymbolStructureSignals = analysis.SymbolStructureSignals
			}
			if len(analysis.PositionSLTPAdjustments) > 0 {
				fd.PositionSLTPAdjustments = analysis.PositionSLTPAdjustments
			}
		}
	}
	return fd, nil
}

// ParseAnalysisFromRawResponse 从 AI 原始响应中仅解析 <analysis> 块，供 API 返回最新分析快照（雷达页「实时数据+AI预测」展示）
func ParseAnalysisFromRawResponse(rawResponse string) *AnalysisSnapshot {
	if rawResponse == "" {
		return nil
	}
	if match := reAnalysisTag.FindStringSubmatch(rawResponse); len(match) > 1 {
		analysisPart := strings.TrimSpace(match[1])
		if idx := strings.Index(analysisPart, "{"); idx >= 0 {
			if end := strings.LastIndex(analysisPart, "}"); end > idx {
				analysisPart = analysisPart[idx : end+1]
			}
		}
		var analysis struct {
			MarketSummary           string                   `json:"market_summary"`
			MarketRegime            string                   `json:"market_regime"`
			RiskAlert               *bool                    `json:"risk_alert"`
			Scenario                string                   `json:"scenario"`
			SymbolPredictions       []SymbolPrediction       `json:"symbol_predictions"`
			SymbolStructureSignals  []SymbolStructureSignal  `json:"symbol_structure_signals"`
			PositionSLTPAdjustments []PositionSLTPAdjustment  `json:"position_sl_tp_adjustments"`
		}
		if err := json.Unmarshal([]byte(analysisPart), &analysis); err == nil {
			out := &AnalysisSnapshot{
				MarketRegime:            strings.TrimSpace(analysis.MarketRegime),
				Scenario:                strings.TrimSpace(analysis.Scenario),
				MarketSummary:           strings.TrimSpace(analysis.MarketSummary),
				SymbolPredictions:       analysis.SymbolPredictions,
				SymbolStructureSignals:  analysis.SymbolStructureSignals,
				PositionSLTPAdjustments: analysis.PositionSLTPAdjustments,
			}
			if analysis.RiskAlert != nil && *analysis.RiskAlert {
				out.RiskAlert = true
			}
			return out
		}
	}
	return nil
}

// SLTPPositionInfo 供「仅止盈止损分析」使用的持仓摘要（轻量）
type SLTPPositionInfo struct {
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	EntryPrice   float64 `json:"entry_price"`
	MarkPrice    float64 `json:"mark_price"`
	PnlPct       float64 `json:"pnl_pct"`
	HoldMinutes  float64 `json:"hold_minutes"`
	ATR          float64 `json:"atr,omitempty"`
	// 系统策略边界（传给 AI，便于在边界内建议）
	ATRMultMin        float64 `json:"atr_mult_min,omitempty"`
	ATRMultMax        float64 `json:"atr_mult_max,omitempty"`
	LockProfitPctMin  float64 `json:"lock_profit_pct_min,omitempty"`
	LockProfitPctMax  float64 `json:"lock_profit_pct_max,omitempty"`
	ConfirmCyclesBase int     `json:"confirm_cycles_base,omitempty"`
	TrailCurrent      string  `json:"trail_current,omitempty"` // 策略/档位默认追踪档位，如 low/medium/high
	// 当前生效参数（来自上一轮 AI 调整，若存在则执行层正在使用；传给 AI 便于续推或微调）
	AppliedTrailAggressiveness string  `json:"applied_trail_aggressiveness,omitempty"`
	AppliedATRMultSL           float64 `json:"applied_atr_mult_sl,omitempty"`
	AppliedLockProfitPct       float64 `json:"applied_lock_profit_pct,omitempty"`
	LastAdvice                 string  `json:"last_advice,omitempty"` // 上一轮 AI 建议场景，如 choppy_hold
}

// BuildSLTPOnlyPrompts 构建仅用于持仓止盈止损参数调节的轻量 prompt；若 position 带 Bounds 则写入 prompt，AI 须在边界内建议。
// currentCycleSummary 为本周期实时市场上下文（可为多行），含 BTC 1h/4h/24h/7d、MACD/RSI、资金费率、多空比、强平 1h/4h/24h、OI 变化、情绪与关键价位等，用于 5 种场景判断。
func BuildSLTPOnlyPrompts(positions []SLTPPositionInfo, regime, scenario, currentCycleSummary string) (systemPrompt, userPrompt string) {
	systemPrompt = `You are a risk assistant. Output ONLY a JSON object inside <analysis></analysis>. No opening/closing decisions.
You will be given system strategy BOUNDS per position (atr_mult_sl range, lock_profit_pct range, confirm_cycles base). You MUST suggest values within these bounds only.
Rules for position_sl_tp_adjustments (one per position):
- choppy_hold: price choppy / ranging → loosen (trail_aggressiveness=low or confirm_cycles_delta=+1)
- trend_ride: trend intact → tighten (trail_aggressiveness=high)
- lock_profit: in profit → suggest lock_profit_pct within bounds (e.g. 2~3)
- trend_weakening: momentum fading → tighten or lower lock_profit_pct
- high_vol_hold: volatility spike but direction ok → confirm_cycles_delta=+1 or atr_mult_sl at upper bound
Use a "base template + small adjustments" philosophy:
- Treat the provided bounds as the base template for this position and the current_applied values (if any) as the current working settings.
- Between consecutive analyses, prefer SMALL, incremental changes (e.g. slightly nudging atr_mult_sl or lock_profit_pct within the range) instead of jumping from the minimum directly to the maximum or radically changing trail_aggressiveness.
- Avoid frequent back-and-forth changes: only move parameters noticeably when market conditions clearly change (e.g. from choppy/ranging to strong trend, or vice versa).
Time in position (hold_minutes) is a SOFT signal, not a hard stop:
- Do NOT recommend exit only because the position has been open for a long time.
- When a position has been held for many cycles with little profit or with large giveback from previous profit, you may tighten stops or raise lock_profit_pct to reduce risk, but still base decisions mainly on price structure and volatility, not time alone.
In each position_sl_tp_adjustments item, also output structured exit signals (used by the system with confirmation+hysteresis before any scale_out/exit):
- phase_label: trend | late_trend | range | high_vol | reversal_risk | neutral
- invalidation_level: verifiable invalidation condition or key level (string)
- invalidation_strength: 0-100 (higher = stronger invalidation / more exit pressure)
- exit_bias: hold | tighten | scale_out | exit
- rationale: 1 sentence
Anti-noise rules for exit signals:
- Base invalidation and exit_bias mainly on 1h/4h structure; 15m is only for small nudges.
- Avoid flip-flopping. Only output scale_out/exit when structure invalidation is clear; otherwise tighten/hold.
Current cycle market context (if provided) is real-time data for this cycle, possibly multi-line. It may include: BTC 1h/4h/24h/7d price change, MACD, RSI, dominance, Fear&Greed, AltcoinSeason, OI 1h/4h change, Funding, Long/Short ratio, Liquidation 1h/4h/24h (L/S USD), CGDI, CDRI, WhaleIndex, ETF flow, key levels. Use it to distinguish choppy vs trend vs high volatility and to choose the appropriate advice (choppy_hold / trend_ride / lock_profit / trend_weakening / high_vol_hold) per position.
When a position shows "current_applied (from previous AI)" with trail / atr_mult_sl / lock_profit_pct / last_advice, those are the parameters currently in effect (from your last adjustment). Use them to decide whether to keep or change; you may suggest the same or new values within bounds.
Output format: <analysis>{"position_sl_tp_adjustments":[{"symbol":"X","side":"long","advice":"choppy_hold","trail_aggressiveness":"low",...}]}</analysis>`

	userParts := []string{"Current positions (for SL/TP adjustment only). Suggest only within the given bounds:"}
	for _, p := range positions {
		line := fmt.Sprintf("- %s %s entry=%.4f mark=%.4f pnl_pct=%.2f hold_mins=%.0f atr=%.4f",
			p.Symbol, p.Side, p.EntryPrice, p.MarkPrice, p.PnlPct, p.HoldMinutes, p.ATR)
		if p.ATRMultMin > 0 || p.ATRMultMax > 0 || p.LockProfitPctMin > 0 || p.LockProfitPctMax > 0 || p.ConfirmCyclesBase > 0 {
			line += fmt.Sprintf(" | bounds: atr_mult_sl [%.2f, %.2f], lock_profit_pct [%.1f, %.1f]%%, confirm_cycles_base %d",
				p.ATRMultMin, p.ATRMultMax, p.LockProfitPctMin, p.LockProfitPctMax, p.ConfirmCyclesBase)
			if p.TrailCurrent != "" {
				line += ", trail_current=" + p.TrailCurrent
			}
			line += " (suggest within these bounds)"
		}
		// 当前生效参数与上一轮 AI 建议（若有），便于 AI 续推或微调
		if p.AppliedTrailAggressiveness != "" || p.AppliedATRMultSL > 0 || p.AppliedLockProfitPct > 0 || p.LastAdvice != "" {
			line += " | current_applied (from previous AI):"
			if p.AppliedTrailAggressiveness != "" {
				line += " trail=" + p.AppliedTrailAggressiveness
			}
			if p.AppliedATRMultSL > 0 {
				line += fmt.Sprintf(" atr_mult_sl=%.2f", p.AppliedATRMultSL)
			}
			if p.AppliedLockProfitPct > 0 {
				line += fmt.Sprintf(" lock_profit_pct=%.1f%%", p.AppliedLockProfitPct)
			}
			if p.LastAdvice != "" {
				line += " last_advice=" + p.LastAdvice
			}
		}
		userParts = append(userParts, line)
	}
	userParts = append(userParts, fmt.Sprintf("Last regime=%s scenario=%s. Output only <analysis> with position_sl_tp_adjustments.", regime, scenario))
	if currentCycleSummary != "" {
		userParts = append(userParts, "Current cycle market context (real-time, for choppy_hold / trend_ride / lock_profit / trend_weakening / high_vol_hold):")
		userParts = append(userParts, currentCycleSummary)
	}
	userPrompt = strings.Join(userParts, "\n")
	return systemPrompt, userPrompt
}

// ParsePositionSLTPAdjustmentsFromRawResponse 从 AI 原始响应中仅解析 position_sl_tp_adjustments（用于独立 SL/TP 分析周期）
func ParsePositionSLTPAdjustmentsFromRawResponse(raw string) []PositionSLTPAdjustment {
	if raw == "" {
		return nil
	}
	match := reAnalysisTag.FindStringSubmatch(raw)
	if len(match) < 2 {
		return nil
	}
	analysisPart := strings.TrimSpace(match[1])
	if idx := strings.Index(analysisPart, "{"); idx >= 0 {
		if end := strings.LastIndex(analysisPart, "}"); end > idx {
			analysisPart = analysisPart[idx : end+1]
		}
	}
	var out struct {
		PositionSLTPAdjustments []PositionSLTPAdjustment `json:"position_sl_tp_adjustments"`
	}
	if err := json.Unmarshal([]byte(analysisPart), &out); err != nil {
		return nil
	}
	return out.PositionSLTPAdjustments
}

// BuildCurrentCycleSummaryForSLTP 从 Context 生成一句本周期市场摘要，供止盈止损 5 种场景判断补强（choppy_hold / high_vol_hold / trend_ride 等）。
func BuildCurrentCycleSummaryForSLTP(ctx *Context) string {
	if ctx == nil {
		return ""
	}
	var parts []string
	if btc := ctx.MarketDataMap["BTCUSDT"]; btc != nil {
		parts = append(parts, fmt.Sprintf("BTC 1h %+.2f%% 4h %+.2f%%", btc.PriceChange1h, btc.PriceChange4h))
	}
	if ctx.FearGreedValue > 0 || ctx.FearGreedClassification != "" {
		parts = append(parts, fmt.Sprintf("Fear&Greed %d %s", ctx.FearGreedValue, ctx.FearGreedClassification))
	}
	if ctx.AltcoinSeasonIndex > 0 {
		parts = append(parts, fmt.Sprintf("AltcoinSeason %d", ctx.AltcoinSeasonIndex))
	}
	if ctx.CoinglassLiquidation != nil && (ctx.CoinglassLiquidation.Long1hUSD+ctx.CoinglassLiquidation.Short1hUSD) > 0 {
		parts = append(parts, fmt.Sprintf("Liq 1h L%.0f S%.0f", ctx.CoinglassLiquidation.Long1hUSD, ctx.CoinglassLiquidation.Short1hUSD))
	}
	if ctx.CoinglassCGDI != 0 || ctx.CoinglassCDRI != 0 {
		parts = append(parts, fmt.Sprintf("CGDI %.2f CDRI %.2f", ctx.CoinglassCGDI, ctx.CoinglassCDRI))
	}
	if ctx.CoinglassLiquidationKeyLevels != "" {
		parts = append(parts, "Liq levels: "+ctx.CoinglassLiquidationKeyLevels)
	}
	return strings.Join(parts, "; ")
}

// RunSLTPOnlyAnalysis 仅做持仓止盈止损参数分析：轻量 prompt、只返回 position_sl_tp_adjustments（不写决策记录、不开平仓）。
// currentCycleSummary 为本周期市场摘要，可为空。
func RunSLTPOnlyAnalysis(mcpClient mcp.AIClient, positions []SLTPPositionInfo, regime, scenario, currentCycleSummary string) (adjustments []PositionSLTPAdjustment, raw string, err error) {
	if len(positions) == 0 {
		return nil, "", nil
	}
	sys, user := BuildSLTPOnlyPrompts(positions, regime, scenario, currentCycleSummary)
	raw, err = callAIWithCaching(mcpClient, sys, user, "")
	if err != nil {
		return nil, raw, err
	}
	adjustments = ParsePositionSLTPAdjustmentsFromRawResponse(raw)
	return adjustments, raw, nil
}

func extractCoTTrace(response string) string {
	if match := reReasoningTag.FindStringSubmatch(response); match != nil && len(match) > 1 {
		logger.Infof("✓ Extracted reasoning chain using <reasoning> tag")
		return strings.TrimSpace(match[1])
	}

	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		logger.Infof("✓ Extracted content before <decision> tag as reasoning chain")
		return strings.TrimSpace(response[:decisionIdx])
	}

	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		logger.Infof("⚠️  Extracted reasoning chain using old format ([ character separator)")
		return strings.TrimSpace(response[:jsonStart])
	}

	return strings.TrimSpace(response)
}

func extractDecisions(response string) ([]Decision, error) {
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)
	s = fixMissingQuotes(s)

	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); match != nil && len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		logger.Infof("✓ Extracted JSON using <decision> tag")
	} else {
		jsonPart = s
		logger.Infof("⚠️  <decision> tag not found, searching JSON in full text")
	}

	jsonPart = fixMissingQuotes(jsonPart)

	if m := reJSONFence.FindStringSubmatch(jsonPart); m != nil && len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent)
		jsonContent = fixMissingQuotes(jsonContent)
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
		}
		return decisions, nil
	}

	// 兼容无 "json" 的代码块（如 deepseek-reasoner 只输出 ``` ... ```）
	for _, sub := range reJSONFencePlain.FindAllStringSubmatch(jsonPart, -1) {
		if len(sub) < 2 {
			continue
		}
		block := strings.TrimSpace(sub[1])
		if !reArrayHead.MatchString(block) {
			continue
		}
		jsonContent := compactArrayOpen(block)
		jsonContent = fixMissingQuotes(jsonContent)
		if err := validateJSONFormat(jsonContent); err != nil {
			continue
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			continue
		}
		logger.Infof("✓ Extracted JSON from plain ``` code block")
		return decisions, nil
	}

	// 从全文末尾按括号匹配提取数组（模型先输出推理再输出 JSON 且无 tag 时）
	if extracted := extractJSONArrayByBracketMatch(jsonPart); extracted != "" {
		jsonContent := compactArrayOpen(extracted)
		jsonContent = fixMissingQuotes(jsonContent)
		if err := validateJSONFormat(jsonContent); err == nil {
			var decisions []Decision
			if err := json.Unmarshal([]byte(jsonContent), &decisions); err == nil {
				logger.Infof("✓ Extracted JSON by bracket match from end of response")
				return decisions, nil
			}
		}
	}

	jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
	if jsonContent == "" {
		// 云上常见原因：响应被截断（max_tokens 不足）、超时、或网络导致未返回完整 JSON；可提高 AI_MAX_TOKENS、AI_TIMEOUT_SECONDS 并查看日志
		logger.Infof("⚠️  [SafeFallback] AI didn't output JSON decision, entering safe wait mode (response len=%d)", len(response))
		if len(jsonPart) > 0 && len(jsonPart) <= 600 {
			logger.Infof("⚠️  [SafeFallback] jsonPart snippet: %s", jsonPart)
		} else if len(jsonPart) > 600 {
			logger.Infof("⚠️  [SafeFallback] jsonPart tail(500): ...%s", jsonPart[len(jsonPart)-500:])
		}

		cotSummary := jsonPart
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("Model didn't output structured JSON decision, entering safe wait; summary: %s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent)

	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
	}

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
	}

	return decisions, nil
}

func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")

	jsonStr = strings.ReplaceAll(jsonStr, "［", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{")
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}")
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":")
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "【", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "　", " ")

	return jsonStr
}

// extractJSONArrayByBracketMatch 从文本中查找 [{ 并从该处括号匹配提取完整 JSON 数组（应对无 tag、先推理后 JSON 的输出）
func extractJSONArrayByBracketMatch(s string) string {
	idx := strings.LastIndex(s, "[{")
	if idx < 0 {
		return ""
	}
	depth := 0
	inString := false
	escape := false
	var quote byte
	for i := idx; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			if c == '\\' {
				escape = true
				continue
			}
			if c == quote {
				inString = false
			}
			continue
		}
		if c == '"' || c == '\'' {
			inString = true
			quote = c
			continue
		}
		if c == '[' || c == '{' {
			depth++
			continue
		}
		if c == ']' || c == '}' {
			depth--
			if depth == 0 {
				return s[idx : i+1]
			}
		}
	}
	return ""
}

func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	if !reArrayHead.MatchString(trimmed) {
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	// 允许 ~ 出现在字符串值内（如 reasoning 中的「止损价~221.78」）；仅当数字字段含 ~ 时 json.Unmarshal 会报错
	// Allow ~ inside string values (e.g. "止损价~221.78" in reasoning); numeric fields with ~ will fail at decode

	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON numbers cannot contain thousand separator comma, found: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// ============================================================================
// Decision Validation
// ============================================================================

func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) error {
	for i := range decisions {
		if err := validateDecision(&decisions[i], accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio); err != nil {
			return fmt.Errorf("decision #%d validation failed: %w", i+1, err)
		}
	}
	return nil
}

func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) error {
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("invalid action: %s", d.Action)
	}

	if d.Action == "open_long" || d.Action == "open_short" {
		maxLeverage := altcoinLeverage
		posRatio := altcoinPosRatio
		maxPositionValue := accountEquity * posRatio
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage
			posRatio = btcEthPosRatio
			maxPositionValue = accountEquity * posRatio
		}

		if d.Leverage <= 0 {
			return fmt.Errorf("leverage must be greater than 0: %d", d.Leverage)
		}
		if d.Leverage > maxLeverage {
			logger.Infof("⚠️  [Leverage Fallback] %s leverage exceeded (%dx > %dx), auto-adjusting to limit %dx",
				d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("position size must be greater than 0: %.2f", d.PositionSizeUSD)
		}

		const minPositionSizeGeneral = 12.0
		const minPositionSizeBTCETH = 60.0

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		tolerance := maxPositionValue * 0.01
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			// 自动封顶到允许上限，避免因 AI 输出超限而整条决策失败
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				logger.Infof("⚠️ [Position Cap] %s position size %.0f USDT exceeds max %.0f (%.1fx equity), auto-capping to %.0f",
					d.Symbol, d.PositionSizeUSD, maxPositionValue, posRatio, maxPositionValue)
			} else {
				logger.Infof("⚠️ [Position Cap] %s altcoin position size %.0f USDT exceeds max %.0f (%.1fx equity), auto-capping to %.0f",
					d.Symbol, d.PositionSizeUSD, maxPositionValue, posRatio, maxPositionValue)
			}
			d.PositionSizeUSD = maxPositionValue
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("stop loss and take profit must be greater than 0")
		}

		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("for long positions, stop loss price must be less than take profit price")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("for short positions, stop loss price must be greater than take profit price")
			}
		}

		var entryPrice float64
		if d.Action == "open_long" {
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2
		} else {
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		if riskRewardRatio < 3.0 {
			return fmt.Errorf("risk/reward ratio too low (%.2f:1), must be ≥3.0:1 [risk: %.2f%% reward: %.2f%%] [stop loss: %.2f take profit: %.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// detectLanguage detects language from text content
// Returns LangChinese if text contains Chinese characters, otherwise LangEnglish
func detectLanguage(text string) Language {
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			return LangChinese
		}
	}
	return LangEnglish
}
