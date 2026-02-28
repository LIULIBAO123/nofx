package backtest

import "time"

// RunState represents the current state of a backtest run.
type RunState string

const (
	RunStateCreated    RunState = "created"
	RunStateRunning    RunState = "running"
	RunStatePaused     RunState = "paused"
	RunStateStopped    RunState = "stopped"
	RunStateCompleted  RunState = "completed"
	RunStateFailed     RunState = "failed"
	RunStateLiquidated RunState = "liquidated"
)

// PositionSnapshot represents core position data for backtest state and persistence.
type PositionSnapshot struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"`
	Quantity         float64 `json:"quantity"`
	AvgPrice         float64 `json:"avg_price"`
	Leverage         int     `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	OpenTime         int64   `json:"open_time"`
	AccumulatedFee   float64 `json:"accumulated_fee,omitempty"` // Opening fees accumulated
	StopLoss         float64 `json:"stop_loss,omitempty"`
	TakeProfit       float64 `json:"take_profit,omitempty"`
	ATRAtOpen        float64 `json:"atr_at_open,omitempty"`
}

// BacktestState represents the real-time state during execution (in-memory state).
type BacktestState struct {
	BarIndex      int
	BarTimestamp  int64
	DecisionCycle int

	Cash            float64
	Equity          float64
	UnrealizedPnL   float64
	RealizedPnL     float64
	MaxEquity       float64
	MinEquity       float64
	MaxDrawdownPct  float64
	Positions       map[string]PositionSnapshot
	LastUpdate      time.Time
	Liquidated      bool
	LiquidationNote string
}

// EquityPoint represents a single point on the equity curve.
type EquityPoint struct {
	Timestamp   int64   `json:"ts"`
	Equity      float64 `json:"equity"`
	Available   float64 `json:"available"`
	PnL         float64 `json:"pnl"`
	PnLPct      float64 `json:"pnl_pct"`
	DrawdownPct float64 `json:"dd_pct"`
	Cycle       int     `json:"cycle"`
}

// TradeEvent records a trade execution result or special event (such as liquidation).
type TradeEvent struct {
	Timestamp       int64          `json:"ts"`
	Symbol          string         `json:"symbol"`
	Action          string         `json:"action"`
	Side            string         `json:"side,omitempty"`
	Quantity        float64        `json:"qty"`
	Price           float64        `json:"price"`
	Fee             float64        `json:"fee"`
	Slippage        float64        `json:"slippage"`
	OrderValue      float64        `json:"order_value"`
	RealizedPnL     float64        `json:"realized_pnl"`
	Leverage        int            `json:"leverage,omitempty"`
	Cycle           int            `json:"cycle"`
	PositionAfter   float64        `json:"position_after"`
	LiquidationFlag bool           `json:"liquidation"`
	Note            string         `json:"note,omitempty"`
	AIAnalysis      *TradeAnalysis `json:"ai_analysis,omitempty"` // AI analysis of this trade
	// OpenTime is set for close events (close_long/close_short/liquidated); used to display 持仓时间
	OpenTime int64 `json:"open_time,omitempty"`
	// CloseReason set when closed by strategy: "initial_stop", "trailing_stop", "fixed_tp", "scaled_tp", "liquidated", etc.
	CloseReason string `json:"close_reason,omitempty"`
	// Dynamic opening params (for open_* and close_*; displayed in positions/trades UI)
	StopLoss   float64 `json:"stop_loss,omitempty"`
	TakeProfit float64 `json:"take_profit,omitempty"`
	ATRAtOpen  float64 `json:"atr_at_open,omitempty"`
	// Final-at-close snapshot (for close_* only; real-time values become fixed at close)
	ATRMultipleSL         float64 `json:"atr_multiple_sl,omitempty"`
	ATRMultipleTP         float64 `json:"atr_multiple_tp,omitempty"`
	ATRPeriod             int     `json:"atr_period,omitempty"`
	ScaledTPLevel         int     `json:"scaled_tp_level,omitempty"`
	ScaledTPClosedPct      float64 `json:"scaled_tp_closed_pct,omitempty"`
	TrailingTierActivated  int     `json:"trailing_tier_activated,omitempty"`
	TrailingAllowedDrawdown float64 `json:"trailing_allowed_drawdown,omitempty"`
}

// TradeAnalysis contains AI-generated analysis of a trade
type TradeAnalysis struct {
	EntryQuality      string  `json:"entry_quality,omitempty"`       // Quality assessment of entry timing
	ExitQuality       string  `json:"exit_quality,omitempty"`        // Quality assessment of exit timing
	RiskManagement    string  `json:"risk_management,omitempty"`     // Risk management evaluation
	MarketCondition   string  `json:"market_condition,omitempty"`    // Market environment match
	ProfitLossReason  string  `json:"profit_loss_reason,omitempty"`  // Why profit/loss occurred
	Improvement       string  `json:"improvement,omitempty"`         // Suggestions for improvement
	OverallScore      float64 `json:"overall_score,omitempty"`       // Overall trade quality score (0-10)
	GeneratedAt       int64   `json:"generated_at,omitempty"`        // Timestamp of analysis
	AnalysisError     string  `json:"analysis_error,omitempty"`      // Error if analysis failed
}

// Metrics summarizes backtest performance metrics.
type Metrics struct {
	TotalReturnPct float64                  `json:"total_return_pct"`
	MaxDrawdownPct float64                  `json:"max_drawdown_pct"`
	SharpeRatio    float64                  `json:"sharpe_ratio"`
	ProfitFactor   float64                  `json:"profit_factor"`
	WinRate        float64                  `json:"win_rate"`
	Trades         int                      `json:"trades"`
	AvgWin         float64                  `json:"avg_win"`
	AvgLoss        float64                  `json:"avg_loss"`
	BestSymbol     string                   `json:"best_symbol"`
	WorstSymbol    string                   `json:"worst_symbol"`
	SymbolStats    map[string]SymbolMetrics `json:"symbol_stats"`
	Liquidated     bool                     `json:"liquidated"`
}

// SymbolMetrics records performance for a single symbol.
type SymbolMetrics struct {
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	TotalPnL      float64 `json:"total_pnl"`
	AvgPnL        float64 `json:"avg_pnl"`
	WinRate       float64 `json:"win_rate"`
}

// Checkpoint represents checkpoint information saved to disk for pause, resume, and crash recovery.
type Checkpoint struct {
	BarIndex        int                       `json:"bar_index"`
	BarTimestamp    int64                     `json:"bar_ts"`
	Cash            float64                   `json:"cash"`
	Equity          float64                   `json:"equity"`
	MaxEquity       float64                   `json:"max_equity"`
	MinEquity       float64                   `json:"min_equity"`
	MaxDrawdownPct  float64                   `json:"max_drawdown_pct"`
	UnrealizedPnL   float64                   `json:"unrealized_pnl"`
	RealizedPnL     float64                   `json:"realized_pnl"`
	Positions       []PositionSnapshot        `json:"positions"`
	DecisionCycle   int                       `json:"decision_cycle"`
	IndicatorsState map[string]map[string]any `json:"indicators_state,omitempty"`
	RNGSeed         int64                     `json:"rng_seed,omitempty"`
	AICacheRef      string                    `json:"ai_cache_ref,omitempty"`
	Liquidated      bool                      `json:"liquidated"`
	LiquidationNote string                    `json:"liquidation_note,omitempty"`
}

// RunMetadata records the summary required for run.json.
type RunMetadata struct {
	RunID     string     `json:"run_id"`
	Label     string     `json:"label,omitempty"`
	UserID    string     `json:"user_id,omitempty"`
	LastError string     `json:"last_error,omitempty"`
	Version   int        `json:"version"`
	State     RunState   `json:"state"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Summary   RunSummary `json:"summary"`
}

// RunSummary represents the summary field in run.json.
type RunSummary struct {
	SymbolCount     int     `json:"symbol_count"`
	DecisionTF      string  `json:"decision_tf"`
	ProcessedBars   int     `json:"processed_bars"`
	ProgressPct     float64 `json:"progress_pct"`
	EquityLast      float64 `json:"equity_last"`
	MaxDrawdownPct  float64 `json:"max_drawdown_pct"`
	Liquidated      bool    `json:"liquidated"`
	LiquidationNote string  `json:"liquidation_note,omitempty"`
}

// StatusPayload is used for /status API responses.
type StatusPayload struct {
	RunID          string            `json:"run_id"`
	State          RunState          `json:"state"`
	ProgressPct    float64           `json:"progress_pct"`
	ProcessedBars  int               `json:"processed_bars"`
	CurrentTime    int64             `json:"current_time"`
	DecisionCycle  int               `json:"decision_cycle"`
	Equity         float64           `json:"equity"`
	UnrealizedPnL  float64           `json:"unrealized_pnl"`
	RealizedPnL    float64           `json:"realized_pnl"`
	Positions      []PositionStatus  `json:"positions,omitempty"`
	Note           string            `json:"note,omitempty"`
	LastError      string            `json:"last_error,omitempty"`
	LastUpdatedIso string            `json:"last_updated_iso"`
}

// PositionStatus represents a position with unrealized P&L for status display.
type PositionStatus struct {
	Symbol            string  `json:"symbol"`
	Side              string  `json:"side"`
	Quantity          float64 `json:"quantity"`
	EntryPrice        float64 `json:"entry_price"`
	MarkPrice         float64 `json:"mark_price"`
	Leverage          int     `json:"leverage"`
	UnrealizedPnL     float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct  float64 `json:"unrealized_pnl_pct"`
	MarginUsed        float64 `json:"margin_used"`
	StopLoss          float64 `json:"stop_loss,omitempty"`
	TakeProfit        float64 `json:"take_profit,omitempty"`
	ATRAtOpen         float64 `json:"atr_at_open,omitempty"`
	// ATR as multiples (e.g. 1.6 = 1.6× ATR) for display instead of raw ATR value
	ATRMultipleSL float64 `json:"atr_multiple_sl,omitempty"`
	ATRMultipleTP float64 `json:"atr_multiple_tp,omitempty"`
	// Real-time: distance from current price to SL/TP (as % of mark price)
	DistanceToSLPct float64 `json:"distance_to_sl_pct,omitempty"`
	DistanceToTPPct float64 `json:"distance_to_tp_pct,omitempty"`
	// Strategy flags (same for all positions when strategy is loaded)
	TrailingEnabled  bool `json:"trailing_enabled,omitempty"`
	ScaledTPEnabled  bool `json:"scaled_tp_enabled,omitempty"`
	ScaledTPLevel    int  `json:"scaled_tp_level,omitempty"` // 0-based: how many scaled TP levels already taken (0 = none)
	ScaledTPClosedPct float64 `json:"scaled_tp_closed_pct,omitempty"` // cumulative % of position closed by scaled TP (0–100)
	// Real-time: trailing stop activation (by profit threshold, not checkbox)
	TrailingTierActivated   int     `json:"trailing_tier_activated,omitempty"`   // 0=none, 1=first tier, 2=second...
	TrailingAllowedDrawdown float64 `json:"trailing_allowed_drawdown,omitempty"` // allowed drawback % at current tier
	ATRPeriod               int     `json:"atr_period,omitempty"`               // ATR period used (e.g. 14) for display
}
