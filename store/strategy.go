package store

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// StrategyStore strategy storage
type StrategyStore struct {
	db *gorm.DB
}

// Strategy strategy configuration
type Strategy struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	UserID        string    `gorm:"column:user_id;not null;default:'';index" json:"user_id"`
	Name          string    `gorm:"not null" json:"name"`
	Description   string    `gorm:"default:''" json:"description"`
	IsActive      bool      `gorm:"column:is_active;default:false;index" json:"is_active"`
	IsDefault     bool      `gorm:"column:is_default;default:false" json:"is_default"`
	IsPublic      bool      `gorm:"column:is_public;default:false;index" json:"is_public"`       // whether visible in strategy market
	ConfigVisible bool      `gorm:"column:config_visible;default:true" json:"config_visible"`    // whether config details are visible
	Config        string    `gorm:"not null;default:'{}'" json:"config"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Strategy) TableName() string { return "strategies" }

// 预设策略用到的指针辅助函数（与 GetOptimizedStrategyConfig 内联实现一致）
func boolPtr(b bool) *bool       { return &b }
func float64Ptr(f float64) *float64 { return &f }
func intPtr(i int) *int         { return &i }

// StrategyConfig strategy configuration details (JSON structure)
type StrategyConfig struct {
	// Strategy type: "ai_trading" (default) or "grid_trading"
	StrategyType string `json:"strategy_type,omitempty"`
	// Preset version (e.g. "3.0") for default/optimized strategy
	Version string `json:"version,omitempty"`

	// language setting: "zh" for Chinese, "en" for English
	// This determines the language used for data formatting and prompt generation
	Language string `json:"language,omitempty"`
	// coin source configuration
	CoinSource CoinSourceConfig `json:"coin_source"`
	// quantitative data configuration
	Indicators IndicatorConfig `json:"indicators"`
	// custom prompt (appended at the end)
	CustomPrompt string `json:"custom_prompt,omitempty"`
	// risk control configuration
	RiskControl RiskControlConfig `json:"risk_control"`
	// editable sections of System Prompt
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`

	// Grid trading configuration (only used when StrategyType == "grid_trading")
	GridConfig *GridStrategyConfig `json:"grid_config,omitempty"`

	// Strategy mode: "classic" (default) or "multilayer_filter" (see docs/多层过滤与方向池策略-设计与部署.md)
	StrategyMode string `json:"strategy_mode,omitempty"`
	// Multilayer filter config (first/second/third layer + entry timing); used when StrategyMode == "multilayer_filter"
	MultilayerFilter *MultilayerFilterConfig `json:"multilayer_filter,omitempty"`
}

// MultilayerFilterConfig config for multi-layer candidate filtering and entry timing (design: docs/多层过滤与方向池策略-设计与部署.md)
type MultilayerFilterConfig struct {
	Enabled           bool                `json:"enabled"`
	DirectionPoolMode bool                `json:"direction_pool_mode,omitempty"` // optional: AI fills long/short pool only
	DirectionPool     *DirectionPoolConfig `json:"direction_pool,omitempty"`       // 方向池可选：最低强度、单侧归属、按强度排序
	Layer1            *Layer1Config       `json:"layer1,omitempty"`
	Layer2            *Layer2Config       `json:"layer2,omitempty"`
	Layer3            *Layer3Config       `json:"layer3,omitempty"`
}

// DirectionPoolConfig 方向池可选配置
type DirectionPoolConfig struct {
	// MinStrengthPct 仅将 StrengthPct >= 此值的标的放入方向池；0 表示不过滤
	MinStrengthPct float64 `json:"min_strength_pct,omitempty"`
	// SingleSideOnly 为 true 时每标的只归入多空池中强度更高的一侧（按 StrengthPct 比较），避免同标的出现在两侧
	SingleSideOnly bool `json:"single_side_only,omitempty"`
	// SortByStrength 为 true 时多/空池按 StrengthPct 降序排列，开仓与 API 展示时优先高强度
	SortByStrength bool `json:"sort_by_strength,omitempty"`
	// FilterPoolByLayer2 为 true 时仅 Layer2 达标（因子数、可靠度、入场信心）的标的进入方向池
	FilterPoolByLayer2 bool `json:"filter_pool_by_layer2,omitempty"`
	// EnableStrengthSmoothing 为 true 时对 StrengthPct 做短时平滑，减少抖动
	EnableStrengthSmoothing bool `json:"enable_strength_smoothing,omitempty"`
	// StrengthSmoothingWeight 平滑权重：smoothed = weight*last + (1-weight)*current，建议 0.6~0.8
	StrengthSmoothingWeight float64 `json:"strength_smoothing_weight,omitempty"`
	// StrengthBonusCap 补强加成对 StrengthPct 的上限（0 表示不设限）
	StrengthBonusCap float64 `json:"strength_bonus_cap,omitempty"`
	// ReliBonusCap 补强加成对 ReliabilityPct 的上限（0 表示不设限）
	ReliBonusCap float64 `json:"reli_bonus_cap,omitempty"`
	// MinStrengthPctToOpen 开仓最低强度：三条件共振后仅当方向池中该标的 StrengthPct >= 此值才允许开仓；0 表示不额外过滤（与进池门槛一致时可用 MinStrengthPct）
	MinStrengthPctToOpen float64 `json:"min_strength_pct_to_open,omitempty"`
}

// Layer1Config first layer: N items all pass (or configurable) to enter candidate list
type Layer1Config struct {
	RequiredAll               bool              `json:"required_all"`   // if true, all enabled items must pass (when MinItemsToPass==0)
	MinItemsToPass            int               `json:"min_items_to_pass,omitempty"` // 0=全部通过才过；>0 时至少通过 N 项即过（放宽过滤）
	MinPeriodsAligned         int               `json:"min_periods_aligned,omitempty"` // 多周期至少一致数：2=三周期(4h/1h/短)至少两周期同向；0=不启用，沿用 long_tf/short_tf 单项
	Items                     []Layer1Item     `json:"items,omitempty"`
	MaxSignalAgeMinutes       int               `json:"max_signal_age_minutes,omitempty"` // e.g. 5 for "entry within 5 min"
	FailClosedWhenDataMissing bool              `json:"fail_closed_when_data_missing,omitempty"` // 依赖数据缺失时视为不通过（默认 false 为原行为）
}

// Layer1Item single condition in layer1 (e.g. entry_timing, volume_ok, oi_ok, long_tf_aligned, ...)
type Layer1Item struct {
	ID      string   `json:"id"`      // e.g. "entry_timing", "volume_ok", "oi_ok"
	Enabled bool     `json:"enabled"`
	Allowed []string `json:"allowed,omitempty"` // for entry_timing: ["now","soon"]
	Value   float64  `json:"value,omitempty"`   // for reliability_min: 0.4
}

// Layer2Config second layer: min factors + reliability + entry confidence thresholds
type Layer2Config struct {
	MinFactors                  int     `json:"min_factors"`                      // e.g. 6
	ReliabilityThreshold        float64 `json:"reliability_threshold"`            // e.g. 0.67
	EntryConfidenceThresholdPct float64 `json:"entry_confidence_threshold_pct"`   // e.g. 31
}

// Layer3Config third layer: final check before order submit (OI aligned, entry timing strength, custom factors, signal age)
type Layer3Config struct {
	OIAlignedRequired      bool    `json:"oi_aligned_required"`
	EntryTimingStrengthMin float64 `json:"entry_timing_strength_min"`
	CustomFactorsRequired  bool    `json:"custom_factors_required"`
	MaxSignalAgeMinutes    int     `json:"max_signal_age_minutes"` // e.g. 5
}

// RealtimePriceConfig use latest price for SL/TP check to better grasp P&L (see 外部交易策略分析)
type RealtimePriceConfig struct {
	Enabled               bool   `json:"enabled"`
	FetchBeforeSLTPCheck bool   `json:"fetch_before_sltp_check"`
	Source               string `json:"source,omitempty"` // "exchange_mark" or leave empty
}

// GridStrategyConfig grid trading specific configuration
type GridStrategyConfig struct {
	// Trading pair (e.g., "BTCUSDT")
	Symbol string `json:"symbol"`
	// Number of grid levels (5-50)
	GridCount int `json:"grid_count"`
	// Total investment in USDT
	TotalInvestment float64 `json:"total_investment"`
	// Leverage (1-20)
	Leverage int `json:"leverage"`
	// Upper price boundary (0 = auto-calculate from ATR)
	UpperPrice float64 `json:"upper_price"`
	// Lower price boundary (0 = auto-calculate from ATR)
	LowerPrice float64 `json:"lower_price"`
	// Use ATR to auto-calculate bounds
	UseATRBounds bool `json:"use_atr_bounds"`
	// ATR multiplier for bound calculation (default 2.0)
	ATRMultiplier float64 `json:"atr_multiplier"`
	// Position distribution: "uniform" | "gaussian" | "pyramid"
	Distribution string `json:"distribution"`
	// Maximum drawdown percentage before emergency exit
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	// Stop loss percentage per position
	StopLossPct float64 `json:"stop_loss_pct"`
	// Daily loss limit percentage
	DailyLossLimitPct float64 `json:"daily_loss_limit_pct"`
	// Use maker-only orders for lower fees
	UseMakerOnly bool `json:"use_maker_only"`
	// Enable automatic grid direction adjustment based on box breakouts
	EnableDirectionAdjust bool `json:"enable_direction_adjust"`
	// Direction bias ratio for long_bias/short_bias modes (default 0.7 = 70%/30%)
	DirectionBiasRatio float64 `json:"direction_bias_ratio"`
}

// PromptSectionsConfig editable sections of System Prompt
type PromptSectionsConfig struct {
	// role definition (title + description)
	RoleDefinition string `json:"role_definition,omitempty"`
	// trading frequency awareness
	TradingFrequency string `json:"trading_frequency,omitempty"`
	// entry standards
	EntryStandards string `json:"entry_standards,omitempty"`
	// decision process
	DecisionProcess string `json:"decision_process,omitempty"`
}

// CoinSourceConfig coin source configuration
type CoinSourceConfig struct {
	// source type: "static" | "ai500" | "oi_top" | "oi_low" | "mixed"
	SourceType string `json:"source_type"`
	// static coin list (used when source_type = "static")
	StaticCoins []string `json:"static_coins,omitempty"`
	// excluded coins list (filtered out from all sources)
	ExcludedCoins []string `json:"excluded_coins,omitempty"`
	// whether to use AI500 coin pool
	UseAI500 bool `json:"use_ai500"`
	// AI500 coin pool maximum count
	AI500Limit int `json:"ai500_limit,omitempty"`
	// whether to use OI Top (持仓增加榜，适合做多)
	UseOITop bool `json:"use_oi_top"`
	// OI Top maximum count
	OITopLimit int `json:"oi_top_limit,omitempty"`
	// whether to use OI Low (持仓减少榜，适合做空)
	UseOILow bool `json:"use_oi_low"`
	// OI Low maximum count
	OILowLimit int `json:"oi_low_limit,omitempty"`
	// Note: API URLs are now built automatically using NofxOSAPIKey from IndicatorConfig
}

// IndicatorConfig indicator configuration
type IndicatorConfig struct {
	// K-line configuration
	Klines KlineConfig `json:"klines"`
	// raw kline data (OHLCV) - always enabled, required for AI analysis
	EnableRawKlines bool `json:"enable_raw_klines"`
	// technical indicator switches
	EnableEMA         bool `json:"enable_ema"`
	EnableMACD        bool `json:"enable_macd"`
	EnableRSI         bool `json:"enable_rsi"`
	EnableATR         bool `json:"enable_atr"`
	EnableBOLL        bool `json:"enable_boll"`         // Bollinger Bands
	EnableVolume      bool `json:"enable_volume"`
	EnableOI          bool `json:"enable_oi"`           // open interest
	EnableFundingRate bool `json:"enable_funding_rate"` // funding rate
	// EMA period configuration
	EMAPeriods []int `json:"ema_periods,omitempty"` // default [20, 50]
	// RSI period configuration
	RSIPeriods []int `json:"rsi_periods,omitempty"` // default [7, 14]
	// ATR period configuration
	ATRPeriods []int `json:"atr_periods,omitempty"` // default [14]
	// BOLL period configuration (period, standard deviation multiplier is fixed at 2)
	BOLLPeriods []int `json:"boll_periods,omitempty"` // default [20] - can select multiple timeframes
	// when true, non-primary timeframes in prompt are output as one-line summary (latest close, ema20, ema50, atr14) to save tokens
	CompactNonPrimaryTimeframe bool `json:"compact_non_primary_timeframe,omitempty"`
	// external data sources
	ExternalDataSources []ExternalDataSource `json:"external_data_sources,omitempty"`

	// ========== NofxOS Unified API Configuration ==========
	// Unified API Key for all NofxOS data sources
	NofxOSAPIKey string `json:"nofxos_api_key,omitempty"`

	// quantitative data sources (capital flow, position changes, price changes)
	EnableQuantData    bool `json:"enable_quant_data"`    // whether to enable quantitative data
	EnableQuantOI      bool `json:"enable_quant_oi"`      // whether to show OI data
	EnableQuantNetflow bool `json:"enable_quant_netflow"` // whether to show Netflow data

	// OI ranking data (market-wide open interest increase/decrease rankings)
	EnableOIRanking   bool   `json:"enable_oi_ranking"`             // whether to enable OI ranking data
	OIRankingDuration string `json:"oi_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	OIRankingLimit    int    `json:"oi_ranking_limit,omitempty"`    // number of entries (default 10)

	// NetFlow ranking data (market-wide fund flow rankings - institution/personal)
	EnableNetFlowRanking   bool   `json:"enable_netflow_ranking"`             // whether to enable NetFlow ranking data
	NetFlowRankingDuration string `json:"netflow_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	NetFlowRankingLimit    int    `json:"netflow_ranking_limit,omitempty"`    // number of entries (default 10)

	// Price ranking data (market-wide gainers/losers)
	EnablePriceRanking   bool   `json:"enable_price_ranking"`             // whether to enable price ranking data
	PriceRankingDuration string `json:"price_ranking_duration,omitempty"` // durations: "1h" or "1h,4h,24h"
	PriceRankingLimit    int    `json:"price_ranking_limit,omitempty"`    // number of entries per ranking (default 10)

	// Binance derivatives data (long/short ratio, funding, taker) — 以币安为主增强市场判断
	EnableBinanceLongShortRatio bool   `json:"enable_binance_long_short_ratio,omitempty"` // 启用币安多空比（全账户+大户）
	BinanceLongShortPeriod      string `json:"binance_long_short_period,omitempty"`      // 5m, 15m, 1h, 4h（默认 15m）
	EnableBinanceFundingHistory bool   `json:"enable_binance_funding_history,omitempty"` // 启用币安资金费率历史/当前
	EnableBinanceTakerVolume    bool   `json:"enable_binance_taker_volume,omitempty"`    // 启用币安 Taker 买卖比
	BinanceTakerPeriod          string `json:"binance_taker_period,omitempty"`           // 5m, 15m, 1h（默认 15m）

	// 数据补强：资金费率近 8h 均值、永续-现货价差、BTC 占比、爆仓聚合、WS
	EnableBinanceFundingRateHistory bool `json:"enable_binance_funding_rate_history,omitempty"` // 资金费率历史，算近 8h 均值
	EnableBasis                     bool `json:"enable_basis,omitempty"`                         // 永续-现货价差 Basis（自算）
	EnableBTCDominance              bool `json:"enable_btc_dominance,omitempty"`               // BTC 市值占比（CoinGecko）
	EnableBinanceWSForceOrder       bool `json:"enable_binance_ws_force_order,omitempty"`       // 币安 WS 强平流，本地 1h/4h 聚合
	// CoinAnk 清算（套餐1 含交易所清算统计 allExchange/intervals；爆仓排行榜需套餐2）
	EnableCoinAnkLiquidation bool   `json:"enable_coinank_liquidation,omitempty"` // 启用 CoinAnk 清算统计（需 CoinAnk API Key）
	CoinAnkAPIKey           string `json:"coinank_api_key,omitempty"`           // CoinAnk OpenAPI Key（与 NofxOS 独立）
	CoinAnkURL              string `json:"coinank_url,omitempty"`               // 如 https://open-api.coinank.com，空则用默认

	// Coinglass 中转站（KeyStore 代理）：通过 KeyStore 获取 Coinglass 市场数据（OI/资金费率/强平/多空比等）
	EnableCoinglassData      bool   `json:"enable_coinglass_data,omitempty"`       // 是否启用 Coinglass 数据（经 KeyStore 代理）
	CoinglassProxyURL        string `json:"coinglass_proxy_url,omitempty"`         // KeyStore 代理 Base URL，如 https://www.keystore.com.cn/api/v1/proxy/coinglass/v4
	CoinglassAPIKey          string `json:"coinglass_api_key,omitempty"`           // KeyStore API Key（请求头 X-Api-Key，由网关消费并注入上游凭证）
	CoinglassRateLimitPerMin int    `json:"coinglass_rate_limit_per_min,omitempty"` // 中转站每分钟请求上限，如 10；0 表示使用默认 10
	EnableCoinglassWSS       bool   `json:"enable_coinglass_wss,omitempty"`        // 是否启用 Coinglass WSS 实时推送（融资率/清算/OI/价格），补强 AI 实时与预测
}

// KlineConfig K-line configuration
type KlineConfig struct {
	// primary timeframe: "1m", "3m", "5m", "15m", "1h", "4h"
	PrimaryTimeframe string `json:"primary_timeframe"`
	// primary timeframe K-line count
	PrimaryCount int `json:"primary_count"`
	// longer timeframe
	LongerTimeframe string `json:"longer_timeframe,omitempty"`
	// longer timeframe K-line count
	LongerCount int `json:"longer_count,omitempty"`
	// whether to enable multi-timeframe analysis
	EnableMultiTimeframe bool `json:"enable_multi_timeframe"`
	// selected timeframe list (new: supports multi-timeframe selection)
	SelectedTimeframes []string `json:"selected_timeframes,omitempty"`
	// max number of candidate coins (excluding positions) to include in prompt; 0 = use default 8. Reduce to save tokens.
	MaxCoinsInPrompt int `json:"max_coins_in_prompt,omitempty"`
}

// ExternalDataSource external data source configuration
type ExternalDataSource struct {
	Name        string            `json:"name"`         // data source name
	Type        string            `json:"type"`         // type: "api" | "webhook"
	URL         string            `json:"url"`          // API URL
	Method      string            `json:"method"`       // HTTP method
	Headers     map[string]string `json:"headers,omitempty"`
	DataPath    string            `json:"data_path,omitempty"`    // JSON data path
	RefreshSecs int               `json:"refresh_secs,omitempty"` // refresh interval (seconds)
}

// RiskControlConfig risk control configuration
type RiskControlConfig struct {
	// Max number of coins held simultaneously (CODE ENFORCED)
	MaxPositions int `json:"max_positions"`

	// BTC/ETH exchange leverage for opening positions (AI guided)
	BTCETHMaxLeverage int `json:"btc_eth_max_leverage"`
	// Altcoin exchange leverage for opening positions (AI guided)
	AltcoinMaxLeverage int `json:"altcoin_max_leverage"`

	// BTC/ETH single position max value = equity × this ratio (CODE ENFORCED, default: 5)
	BTCETHMaxPositionValueRatio float64 `json:"btc_eth_max_position_value_ratio"`
	// Altcoin single position max value = equity × this ratio (CODE ENFORCED, default: 1)
	AltcoinMaxPositionValueRatio float64 `json:"altcoin_max_position_value_ratio"`

	// Max margin utilization (e.g. 0.9 = 90%) (CODE ENFORCED)
	MaxMarginUsage float64 `json:"max_margin_usage"`
	// Min position size in USDT (CODE ENFORCED)
	MinPositionSize float64 `json:"min_position_size"`

	// Min take_profit / stop_loss ratio (AI guided)
	MinRiskRewardRatio float64 `json:"min_risk_reward_ratio"`
	// Min AI confidence to open position (AI guided)
	MinConfidence int `json:"min_confidence"`

	// AI 仅开仓模式：true 时不执行 AI 的 close_long/close_short，平仓完全由策略动态 SL/TP 执行（适应震荡市拿住仓、盈利后平仓）
	AIOnlyEntry bool `json:"ai_only_entry,omitempty"`

	// 系统执行开仓：true 时 AI 仅作辅助、分析量化数据，不输出 open_long/open_short；开仓动作由系统根据多层过滤/方向池结果执行（需启用 strategy_mode=multilayer_filter）
	SystemExecutesEntry bool `json:"system_executes_entry,omitempty"`

	// AIPredictOnly: 为 true 时禁止 AI 输出开平仓动作，AI 只输出预测信息（market_regime、scenario、symbol_predictions 等）；开平仓完全由系统根据预测 + 多层过滤/方向池/动态止盈止损 判断执行。与 SystemExecutesEntry 同时生效时，开仓由 pipeline + AI 预测过滤后执行，平仓仅由系统 TP/SL 执行。
	AIPredictOnly bool `json:"ai_predict_only,omitempty"`

	// AllowAIClose: 当为 true 且 AIOnlyEntry=false 且 AIPredictOnly=false 时，允许执行 AI 的 close_long/close_short 建议（仍保留连续周期确认与置信度门槛）；false 或空 = 仅由动态 SL/TP 负责平仓
	AllowAIClose bool `json:"allow_ai_close,omitempty"`
	// MinConfidenceForAIClose: AI 建议平仓/止盈/止损时，仅当 confidence ≥ 此值才执行；0 = 不额外要求
	MinConfidenceForAIClose int `json:"min_confidence_for_ai_close,omitempty"`
	// RequireExitReasonForAIClose: 为 true 时，仅当 AI 输出 exit_reason 为 take_profit | stop_loss | prediction_mismatch 之一时才执行平仓（确保基于预测与交易不符的退出）
	RequireExitReasonForAIClose bool `json:"require_exit_reason_for_ai_close,omitempty"`

	// 额外补强：按 market_regime 提高开仓门槛（震荡/高波/反转时要求更高置信度）
	RegimeAdjustEnabled    *bool          `json:"regime_adjust_enabled,omitempty"`    // 是否启用 regime 调节 MinConfidence
	RegimeMinConfidenceMap map[string]int `json:"regime_min_confidence_map,omitempty"` // 如 "ranging"->75, "high_volatility"->78, "reversal"->80；未列出的 regime 用基础 MinConfidence

	// 额外补强：极端资金费率/多空比时限制开仓或提高置信度
	ExtremeFundingRule *ExtremeFundingRule `json:"extreme_funding_rule,omitempty"`

	// Dynamic Stop Loss & Take Profit
	DynamicStopLoss   *DynamicStopLossConfig   `json:"dynamic_stop_loss,omitempty"`
	DynamicTakeProfit *DynamicTakeProfitConfig `json:"dynamic_take_profit,omitempty"`

	// PartialCloseCooldownSeconds: 部分平仓冷却时间（秒），用于避免短时间重复部分平仓（如信号减仓/分层止盈等）。
	// 0 表示使用系统默认值（45 秒）。
	PartialCloseCooldownSeconds int `json:"partial_close_cooldown_seconds,omitempty"`

	// PartialCloseMinPercent: 部分平仓最小比例（% of position），低于该比例将跳过（避免噪声/最小下单量问题）。0=默认 2。
	PartialCloseMinPercent float64 `json:"partial_close_min_percent,omitempty"`
	// SLTPExitStateTTLMinutes: 结构化退场状态机 TTL（分钟）；超过该时间未更新则视为过期并忽略。0=默认 60。
	SLTPExitStateTTLMinutes int `json:"sltp_exit_state_ttl_minutes,omitempty"`
	// SignalExitMinHoldSeconds: 结构化信号 exit 绕过 MinHold 的最小持仓秒数（0=立刻允许）。
	SignalExitMinHoldSeconds int `json:"signal_exit_min_hold_seconds,omitempty"`
	// SLTPPreferStopLossOverTakeProfit: true 时先检查止损再止盈（默认 false=先止盈再止损）。
	SLTPPreferStopLossOverTakeProfit bool `json:"sltp_prefer_stop_loss_over_take_profit,omitempty"`
	// ScaleOutBlocksScaledTPSeconds: scale_out 执行后阻断 scaled TP 的窗口秒数（0=默认 600）。
	ScaleOutBlocksScaledTPSeconds int `json:"scale_out_blocks_scaled_tp_seconds,omitempty"`
	// ScaledTPBlocksScaleOutSeconds: scaled TP 执行后阻断 scale_out 的窗口秒数（0=默认 600）。
	ScaledTPBlocksScaleOutSeconds int `json:"scaled_tp_blocks_scale_out_seconds,omitempty"`

	// StructuralExitEscalation: 当 AI 给出 tighten/hold 但 phase=late_trend/reversal_risk 且强度持续较高时，系统可升级到 scale_out/exit。
	// 0 值表示使用默认：scale_out_strength=70, exit_strength=85, scale_out_confirm=3, exit_confirm=2。
	StructExitScaleOutStrength int `json:"struct_exit_scale_out_strength,omitempty"`
	StructExitExitStrength     int `json:"struct_exit_exit_strength,omitempty"`
	StructExitScaleOutConfirm  int `json:"struct_exit_scale_out_confirm,omitempty"`
	StructExitExitConfirm      int `json:"struct_exit_exit_confirm,omitempty"`

	// Realtime price: fetch latest mark before SL/TP check to better grasp P&L (optional)
	RealtimePrice *RealtimePriceConfig `json:"realtime_price,omitempty"`

	// AI参与仓位与分层止盈止损（系统兜底裁剪）
	PositionSizeBuckets *PositionSizeBucketsConfig `json:"position_size_buckets,omitempty"` // equity 比例档位
	TPProfiles          map[string]DynamicTakeProfitConfig `json:"tp_profiles,omitempty"`  // 预设分层止盈模板（按名称选择）
	SLProfiles          map[string]DynamicStopLossConfig   `json:"sl_profiles,omitempty"`  // 预设止损模板（按名称选择）
}

// PositionSizeBucketsConfig controls discrete equity-ratio sizing buckets for AI to choose.
// AI must choose bucket name; system maps it to ratio and enforces hard caps (position value ratio, margin, min size).
type PositionSizeBucketsConfig struct {
	Enabled             bool               `json:"enabled"`
	DefaultBucket       string             `json:"default_bucket,omitempty"`        // fallback bucket when AI missing/invalid
	MinBucketConfidence int                `json:"min_bucket_confidence,omitempty"` // when opening: if AI confidence < this, force DefaultBucket
	Buckets             map[string]float64 `json:"buckets,omitempty"`               // e.g. {"low":0.003,"medium":0.007,"high":0.012}
	MaxBucket           string             `json:"max_bucket,omitempty"`            // optional: cap AI bucket at this (e.g. during drawdown)
}

// ExtremeFundingRule 极端资金费率与多空比时的开仓约束（补强）
type ExtremeFundingRule struct {
	Enabled bool `json:"enabled"`

	// 资金费率绝对值超过此阈值（小数，如 0.001 = 0.1%）视为极端
	FundingThresholdPct float64 `json:"funding_threshold_pct,omitempty"`
	// 多空比 > LongShortRatioHigh 视为多头过热；< LongShortRatioLow 视为空头过热（如 1.4 与 0.714）
	LongShortRatioHigh float64 `json:"long_short_ratio_high,omitempty"`
	LongShortRatioLow  float64 `json:"long_short_ratio_low,omitempty"`

	// 多头过热时是否禁止开多（否则仅提高置信度）
	BlockOpenLongWhenExcessiveLongs bool `json:"block_open_long_when_excessive_longs,omitempty"`
	// 空头过热时是否禁止开空
	BlockOpenShortWhenExcessiveShorts bool `json:"block_open_short_when_excessive_shorts,omitempty"`
	// 不禁止时：在基础/regime 置信度上再提高的数值（如 10）
	RaiseConfidenceBy int `json:"raise_confidence_by,omitempty"`
	// 极端时的最低置信度（如 80），与 RaiseConfidenceBy 取更严
	MinConfidenceWhenExtreme int `json:"min_confidence_when_extreme,omitempty"`
}

// DynamicStopLossConfig dynamic stop loss configuration
type DynamicStopLossConfig struct {
	Enabled      bool   `json:"enabled"`
	TriggerLogic string `json:"trigger_logic"` // "any" = 任一条件触发即平仓, "all" = 所有启用的条件都触发才平仓

	// MinHoldMinutes: 最小持仓分钟数，未满不触发动态止损，避免开仓即止损（策略设置过紧）。0=不限制
	MinHoldMinutes float64 `json:"min_hold_minutes,omitempty"`

	// Initial fixed stop loss (required, acts as safety net)
	InitialStopPercent float64 `json:"initial_stop_percent"` // initial fixed stop %

	// Trailing Stop - Tiered Mode
	TrailingEnabled *bool                `json:"trailing_enabled,omitempty"` // enable trailing stop
	TrailingLevels  []TrailingStopLevel  `json:"trailing_levels,omitempty"`
	// TrailingStopOnlyAfterFirstScaledTP 为 true 时，仅当该仓位已触发过至少一档分层止盈后才启用追踪止损，避免尚未兑现止盈就被追踪平仓
	TrailingStopOnlyAfterFirstScaledTP *bool `json:"trailing_stop_only_after_first_scaled_tp,omitempty"`  // trailing stop levels

	// ATR Stop - Dynamic Range Mode
	ATREnabled        *bool    `json:"atr_enabled,omitempty"`          // enable ATR stop
	ATRMultiplierMin  *float64 `json:"atr_multiplier_min,omitempty"`   // ATR multiplier min (AI range)
	ATRMultiplierMax  *float64 `json:"atr_multiplier_max,omitempty"`   // ATR multiplier max (AI range)
	ATRPeriodBTCETH   *int     `json:"atr_period_btc_eth,omitempty"`   // ATR period for BTC/ETH
	ATRPeriodAltcoin  *int     `json:"atr_period_altcoin,omitempty"`   // ATR period for altcoins

	// Support/Resistance Stop
	SupportResistanceEnabled *bool    `json:"support_resistance_enabled,omitempty"` // enable S/R stop
	SupportResistanceBuffer  *float64 `json:"support_resistance_buffer,omitempty"`  // buffer %

	// Confirm before execute: require N consecutive cycles with SL condition met (reduces premature stop on one-candle dip)
	ConfirmCycles int `json:"confirm_cycles,omitempty"` // 1=immediate; 2+ = delay execute until condition holds N cycles
	// ConfirmMinutes: when > 0, require SL condition to hold for this many minutes (real time) instead of ConfirmCycles; 0 = use ConfirmCycles
	ConfirmMinutes float64 `json:"confirm_minutes,omitempty"`
	// ConfirmMode: "auto"(default) | "minutes" | "cycles" | "minutes_then_samples"
	ConfirmMode string `json:"confirm_mode,omitempty"`
	// ConfirmMinSamples: when ConfirmMode="minutes_then_samples", require at least N triggered samples (AI main cycles) after minutes condition met. 0=default 2.
	ConfirmMinSamples int `json:"confirm_min_samples,omitempty"`

	// ATR tolerance: in high volatility use wider stop / extra confirm cycle so we don't stop on noise
	ATRToleranceEnabled *bool    `json:"atr_tolerance_enabled,omitempty"` // when true, high vol => more tolerant
	ATRHighMultiplier   *float64 `json:"atr_high_multiplier,omitempty"`   // current ATR > long-term ATR * this = high vol (default 1.2)

	// ScenarioAdjustEnabled: 当 AI 的 scenario= \"reversal\" 时，是否在 ConfirmCycles 基础上额外减少一次确认（加快真反转止损）；false 或空 = 不使用 scenario 影响确认次数
	ScenarioAdjustEnabled *bool `json:"scenario_adjust_enabled,omitempty"`

	// KlinesTimeframe: timeframe for klines used in SL (ATR, S/R, trailing extremes, adverse exit). "15m" (default) or "1h". 1h reduces 15m noise.
	KlinesTimeframe string `json:"klines_timeframe,omitempty"` // "15m", "1h"

	// SupportResistanceUseEMA20: when true, use EMA20 from same klines as support (long) / resistance (short) instead of local extrema only
	SupportResistanceUseEMA20 *bool `json:"support_resistance_use_ema20,omitempty"`

	// Adverse exit when never in profit: if position has never been in profit and price moved against by >= this many ATRs, trigger stop (0 or nil = off). Reduces loss when trend is opposite without tightening normal ATR stop.
	AdverseExitWhenNeverProfitATR *float64 `json:"adverse_exit_when_never_profit_atr,omitempty"`
	// AdverseExitWhenNeverProfitATRAltcoin: when set, use this multiplier for non-BTC/ETH symbols instead of AdverseExitWhenNeverProfitATR (allows different sensitivity per group)
	AdverseExitWhenNeverProfitATRAltcoin *float64 `json:"adverse_exit_when_never_profit_atr_altcoin,omitempty"`
	// AdverseExitRequireATRSpike: when true, only trigger adverse exit when current ATR >= long ATR * threshold (avoid exit in mild chop)
	AdverseExitRequireATRSpike    *bool    `json:"adverse_exit_require_atr_spike,omitempty"`
	AdverseExitATRSpikeThreshold  *float64 `json:"adverse_exit_atr_spike_threshold,omitempty"` // default 1.2
}

// TrailingStopLevel trailing stop level configuration
type TrailingStopLevel struct {
	ProfitThreshold float64 `json:"profit_threshold"` // profit % threshold to activate this level
	TrailingPercent float64 `json:"trailing_percent"` // trailing stop % at this level
}

// DynamicTakeProfitConfig dynamic take profit configuration
type DynamicTakeProfitConfig struct {
	Enabled bool `json:"enabled"`

	// MinHoldMinutes: 最小持仓分钟数，未满不触发动态止盈，避免开仓即止盈（策略过紧）。0=不限制
	MinHoldMinutes float64 `json:"min_hold_minutes,omitempty"`

	// MinProfitPercentToAllowTP: 止盈侧最低盈利过滤（价格%）。当前浮盈（价格相对入场）低于此值时不触发任何止盈；0=不限制
	MinProfitPercentToAllowTP *float64 `json:"min_profit_percent_to_allow_tp,omitempty"`

	// Fixed Take Profit
	FixedEnabled *bool    `json:"fixed_enabled,omitempty"` // enable fixed take profit
	FixedPercent *float64 `json:"fixed_percent,omitempty"` // fixed take profit %

	// Scaled Take Profit
	ScaledEnabled *bool                   `json:"scaled_enabled,omitempty"` // enable scaled take profit
	ScaledLevels  []ScaledTakeProfitLevel `json:"scaled_levels,omitempty"`
	// ScaledProfitPercentMode: 分层止盈盈利阈值口径。
	// - "price" (default): 以价格相对入场价的涨跌幅%触发（不含杠杆）
	// - "roe": 以保证金收益率 ROE% 触发（近似 = 价格涨跌幅% × leverage）
	ScaledProfitPercentMode string `json:"scaled_profit_percent_mode,omitempty"`

	// ATR Take Profit - Dynamic Range Mode
	ATREnabled                 *bool    `json:"atr_enabled,omitempty"`                   // enable ATR take profit
	ATRMultiplierMin           *float64 `json:"atr_multiplier_min,omitempty"`           // ATR multiplier min (AI range)
	ATRMultiplierMax           *float64 `json:"atr_multiplier_max,omitempty"`           // ATR multiplier max (AI range)
	ATRUseMaxInHighVolatility  *bool    `json:"atr_use_max_in_high_volatility,omitempty"` // when true, use max multiplier when atr > atrLong*threshold (e.g. 1.2)
	ATRHighVolatilityThreshold *float64 `json:"atr_high_volatility_threshold,omitempty"`  // atr/atrLong >= this => high vol (default 1.2)
	ATRPeriodBTCETH            *int     `json:"atr_period_btc_eth,omitempty"`           // ATR period for BTC/ETH
	ATRPeriodAltcoin           *int     `json:"atr_period_altcoin,omitempty"`            // ATR period for altcoins

	// Resistance Take Profit
	ResistanceEnabled *bool    `json:"resistance_enabled,omitempty"` // enable resistance take profit
	ResistanceBuffer  *float64 `json:"resistance_buffer,omitempty"`  // buffer %

	// Trailing / Pullback Take Profit: lock profit before retrace; resist oscillation via ATR or confirm
	TrailingTPEnabled          *bool    `json:"trailing_tp_enabled,omitempty"`           // enable take profit on pullback from peak
	TrailingTPActivateProfitPct *float64 `json:"trailing_tp_activate_profit_pct,omitempty"` // min profit % (price) to activate (e.g. 2)
	TrailingTPRetracePct        *float64 `json:"trailing_tp_retrace_pct,omitempty"`       // retrace % from peak to trigger (e.g. 1.5)
	TrailingTPRetraceATRMult    *float64 `json:"trailing_tp_retrace_atr_mult,omitempty"`   // retrace >= this * ATR/price (e.g. 0.5); use max(fixed%, ATR%) to resist chop
	TrailingTPConfirmMinutes    float64  `json:"trailing_tp_confirm_minutes,omitempty"`    // 0 = no confirm; >0 = condition must hold this many minutes
	TrailingTPClosePercent      *float64 `json:"trailing_tp_close_percent,omitempty"`      // % of position to close (e.g. 50 or 100)
	TrailingTPActivateProfitPctAltcoin *float64 `json:"trailing_tp_activate_profit_pct_altcoin,omitempty"` // for non-BTC/ETH, use this activate % if set
	TrailingTPRetracePctAltcoin *float64 `json:"trailing_tp_retrace_pct_altcoin,omitempty"` // for non-BTC/ETH, use this retrace % if set
	// Common Settings
	LockProfitPercent *float64 `json:"lock_profit_percent,omitempty"` // move stop to breakeven after this profit
}

// ScaledTakeProfitLevel scaled take profit level
type ScaledTakeProfitLevel struct {
	ProfitPercent        float64 `json:"profit_percent"`                    // profit % trigger
	ClosePercent         float64 `json:"close_percent"`                     // position % to close
	MoveStopToBreakeven  *bool   `json:"move_stop_to_breakeven,omitempty"`  // move stop to breakeven
}

// NewStrategyStore creates a new StrategyStore
func NewStrategyStore(db *gorm.DB) *StrategyStore {
	return &StrategyStore{db: db}
}

func (s *StrategyStore) initTables() error {
	// AutoMigrate will add missing columns without dropping existing data
	return s.db.AutoMigrate(&Strategy{})
}

func (s *StrategyStore) initDefaultData() error {
	// 不再自动创建/刷新「默认策略」：用户可删除任意策略，通过「一键生成」用当前代码预设创建新策略（全部参数为预设值）
	return nil
}

// GetDefaultStrategyConfig returns the default strategy configuration (preset v3.0) for the given language.
// 用于「一键生成」与「应用预设」：所有参数与勾选项的默认值。
func GetDefaultStrategyConfig(lang string) StrategyConfig {
	// Normalize language to "zh" or "en"
	normalizedLang := "en"
	if lang == "zh" {
		normalizedLang = "zh"
	}

	config := StrategyConfig{
		Version:      "3.0",
		StrategyType: "ai_trading",
		Language:     normalizedLang,
		CoinSource: CoinSourceConfig{
			SourceType: "ai500",
			UseAI500:   true,
			AI500Limit: 10,
			UseOITop:   false,
			OITopLimit: 10,
			UseOILow:   false,
			OILowLimit: 10,
		},
		Indicators: IndicatorConfig{
			Klines: KlineConfig{
				PrimaryTimeframe:     "15m",                      // 与「先 4h/1h 定方向再 15m 找入场」一致；建议扫描间隔 5m 或 15m
				PrimaryCount:         60, // 中短期波段：加厚窗口，减少被短时抖动误导
				LongerTimeframe:      "4h",
				LongerCount:          10,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"15m", "1h", "4h"},
				MaxCoinsInPrompt:     8, // 候选写入 prompt 上限，0 时 kernel 用 8
			},
			// 多周期非主周期（1h/4h）默认用 compact 输出，避免加厚窗口后 prompt 体量暴涨
			CompactNonPrimaryTimeframe: true,
			EnableRawKlines:   true, // Required - raw OHLCV data for AI analysis
			EnableEMA:         true, // 趋势/结构（EMA20）供 4h/1h 方向与入场判断
			EnableMACD:        true,
			EnableRSI:         true,
			EnableATR:         true,
			EnableBOLL:        false,
			EnableVolume:      true,
			EnableOI:          true,
			EnableFundingRate: true,
			EMAPeriods:        []int{20, 50},
			RSIPeriods:        []int{7, 14},
			ATRPeriods:        []int{14},
			BOLLPeriods:       []int{20},
			// NofxOS unified API key
			NofxOSAPIKey: "cm_568c67eae410d912c54c",
			// Quant data
			EnableQuantData:    true,
			EnableQuantOI:      true,
			EnableQuantNetflow: true,
			// OI ranking data
			EnableOIRanking:   true,
			OIRankingDuration: "1h",
			OIRankingLimit:    10,
			// NetFlow ranking data
			EnableNetFlowRanking:   true,
			NetFlowRankingDuration: "1h",
			NetFlowRankingLimit:    10,
			// Price ranking data
			EnablePriceRanking:   true,
			PriceRankingDuration: "1h,4h,24h",
			PriceRankingLimit:    10,
			// 币安衍生数据（可选，以币安为主增强市场判断）
			EnableBinanceLongShortRatio: false,
			BinanceLongShortPeriod:      "15m",
			EnableBinanceFundingHistory: false,
			EnableBinanceTakerVolume:    false,
			BinanceTakerPeriod:          "15m",
			EnableBinanceFundingRateHistory: false,
			EnableBasis:                     false,
			EnableBTCDominance:              false,
			EnableBinanceWSForceOrder:       false,
			EnableCoinAnkLiquidation:        false,
			EnableCoinglassData:             false,
			CoinglassProxyURL:               "https://www.keystore.com.cn/api/v1/proxy/coinglass/v4",
			CoinglassAPIKey:                 "",
			CoinglassRateLimitPerMin:        10,  // 中转站常见限制 10/分钟；0 表示使用默认 10
			EnableCoinglassWSS:              false,
		},
		RiskControl: RiskControlConfig{
			MaxPositions:                    3,   // Max 3 coins simultaneously (CODE ENFORCED)
			BTCETHMaxLeverage:               5,   // BTC/ETH exchange leverage (AI guided)
			AltcoinMaxLeverage:              5,   // Altcoin exchange leverage (AI guided)
			BTCETHMaxPositionValueRatio:     5.0, // BTC/ETH: max position = 5x equity (CODE ENFORCED)
			AltcoinMaxPositionValueRatio:    1.0, // Altcoin: max position = 1x equity (CODE ENFORCED)
			MaxMarginUsage:                  0.9, // Max 90% margin usage (CODE ENFORCED)
			MinPositionSize:                 12,  // Min 12 USDT per position (CODE ENFORCED)
			MinRiskRewardRatio:              3.0, // Min 3:1 profit/loss ratio (execution enforced)
			MinConfidence:                   70,  // Min 70% confidence (AI guided)，平衡机会与质量
			AIOnlyEntry:                     true, // 平仓由策略 SL/TP 执行；AI 仅开仓 + 持仓 trend_view 区分
			SystemExecutesEntry:             false, // 默认：由 AI 建议开仓（或用户改为 true 则由方向池系统开仓）
			AIPredictOnly:                   false, // 默认：关闭；为 true 时 AI 只输出预测，开平仓由系统根据预测+pipeline 执行
			AllowAIClose:                    false, // 默认：不执行 AI 平仓建议，仅 SL/TP 平仓
			MinConfidenceForAIClose:          70,    // 启用 AI 平仓时，仅当 confidence≥此值才执行
			RequireExitReasonForAIClose:     true,  // 启用 AI 平仓时，要求 exit_reason 为 take_profit|stop_loss|prediction_mismatch
			// 结构化退场升级默认值（也可在前端改）：phase=late_trend/reversal_risk 且强度持续满足时，允许升级到 scale_out/exit
			StructExitScaleOutStrength: 70,
			StructExitExitStrength:     85,
			StructExitScaleOutConfirm:  3,
			StructExitExitConfirm:      2,
			// 额外补强：regime 调节与极端资金费率/多空比（预设开启：震荡/高波/反转时提高开仓置信度）
			RegimeAdjustEnabled:    boolPtr(true),
			RegimeMinConfidenceMap: map[string]int{"ranging": 75, "high_volatility": 78, "reversal": 80},
			ExtremeFundingRule: &ExtremeFundingRule{
				Enabled: false, FundingThresholdPct: 0.001, LongShortRatioHigh: 1.4, LongShortRatioLow: 0.714,
				BlockOpenLongWhenExcessiveLongs: false, BlockOpenShortWhenExcessiveShorts: false,
				RaiseConfidenceBy: 10, MinConfidenceWhenExtreme: 80,
			},
			RealtimePrice: nil, // 默认不启用；启用时设 FetchBeforeSLTPCheck 等
			PositionSizeBuckets: &PositionSizeBucketsConfig{
				Enabled:             false, // 默认关闭：由 AI 通过 position_size_usd 设置仓位
				DefaultBucket:       "medium",
				MinBucketConfidence: 70,
				Buckets:             map[string]float64{"low": 0.003, "medium": 0.007, "high": 0.012},
				MaxBucket:           "high",
			},
			TPProfiles: map[string]DynamicTakeProfitConfig{
				"tp_conservative": {Enabled: true, MinHoldMinutes: 10, ScaledEnabled: boolPtr(true), ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 3.0, ClosePercent: 50, MoveStopToBreakeven: boolPtr(true)},
					{ProfitPercent: 6.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)},
				}, ScaledProfitPercentMode: "roe"},
				"tp_balanced": {Enabled: true, MinHoldMinutes: 10, ScaledEnabled: boolPtr(true), ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 2.5, ClosePercent: 25, MoveStopToBreakeven: boolPtr(false)},
					{ProfitPercent: 6.0, ClosePercent: 25, MoveStopToBreakeven: boolPtr(true)},
					{ProfitPercent: 10.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)},
				}, ScaledProfitPercentMode: "roe"},
				"tp_aggressive": {Enabled: true, MinHoldMinutes: 10, ScaledEnabled: boolPtr(true), ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 1.5, ClosePercent: 30, MoveStopToBreakeven: boolPtr(false)},
					{ProfitPercent: 3.5, ClosePercent: 30, MoveStopToBreakeven: boolPtr(true)},
					{ProfitPercent: 10.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)},
				}, ScaledProfitPercentMode: "roe"},
			},
			SLProfiles: map[string]DynamicStopLossConfig{
				"sl_tight":  {Enabled: true, TriggerLogic: "any", MinHoldMinutes: 10, InitialStopPercent: 0, TrailingEnabled: boolPtr(true), TrailingLevels: []TrailingStopLevel{{ProfitThreshold: 2.0, TrailingPercent: 1.2}, {ProfitThreshold: 5.0, TrailingPercent: 2.0}}, ATREnabled: boolPtr(true), ATRMultiplierMin: float64Ptr(1.2), ATRMultiplierMax: float64Ptr(2.0), ConfirmCycles: 1, ATRToleranceEnabled: boolPtr(true), ATRHighMultiplier: float64Ptr(1.2), KlinesTimeframe: "15m", TrailingStopOnlyAfterFirstScaledTP: boolPtr(false), AdverseExitWhenNeverProfitATR: float64Ptr(1.2)},
				"sl_normal": {Enabled: true, TriggerLogic: "any", MinHoldMinutes: 10, InitialStopPercent: 0, TrailingEnabled: boolPtr(true), TrailingLevels: []TrailingStopLevel{{ProfitThreshold: 2.5, TrailingPercent: 1.5}, {ProfitThreshold: 6.0, TrailingPercent: 2.5}}, ATREnabled: boolPtr(true), ATRMultiplierMin: float64Ptr(1.5), ATRMultiplierMax: float64Ptr(2.5), ConfirmCycles: 2, ATRToleranceEnabled: boolPtr(true), ATRHighMultiplier: float64Ptr(1.2), KlinesTimeframe: "15m", TrailingStopOnlyAfterFirstScaledTP: boolPtr(false), AdverseExitWhenNeverProfitATR: float64Ptr(1.5)},
				"sl_loose":  {Enabled: true, TriggerLogic: "any", MinHoldMinutes: 10, InitialStopPercent: 0, TrailingEnabled: boolPtr(true), TrailingLevels: []TrailingStopLevel{{ProfitThreshold: 3.0, TrailingPercent: 2.0}, {ProfitThreshold: 7.0, TrailingPercent: 3.0}}, ATREnabled: boolPtr(true), ATRMultiplierMin: float64Ptr(2.0), ATRMultiplierMax: float64Ptr(3.2), ConfirmCycles: 2, ATRToleranceEnabled: boolPtr(true), ATRHighMultiplier: float64Ptr(1.25), KlinesTimeframe: "15m", TrailingStopOnlyAfterFirstScaledTP: boolPtr(false), AdverseExitWhenNeverProfitATR: float64Ptr(1.8)},
			},
			// 预设：动态止损/止盈（略放宽），最小持仓时间，与回测一致
			DynamicStopLoss: &DynamicStopLossConfig{
				Enabled:              true,
				TriggerLogic:         "any",
				MinHoldMinutes:       10,                 // 10min：主周期 15m 下更稳，减少开仓即触发
				InitialStopPercent:   0,                  // 0=无固定初始止损，与前端「已移除固定初始止损」一致；仅用动态/ATR/追踪
				TrailingEnabled:      boolPtr(true),
				TrailingLevels: []TrailingStopLevel{
					{ProfitThreshold: 2.5, TrailingPercent: 1.5},
					{ProfitThreshold: 6.0, TrailingPercent: 2.5},
				},
				ATREnabled:                   boolPtr(true),
				ATRMultiplierMin:             float64Ptr(1.5),
				ATRMultiplierMax:             float64Ptr(2.5),
				ATRPeriodBTCETH:              intPtr(20),
				ATRPeriodAltcoin:             intPtr(14),
				SupportResistanceEnabled:     boolPtr(false), // 默认关闭
				ConfirmCycles:                2,             // 连续2周期满足才执行，减少单K线假跌破
				ConfirmMinutes:              0,             // 0=按周期数确认；>0 按真实分钟数
				ATRToleranceEnabled:          boolPtr(true), // 高波动时更宽容
				ATRHighMultiplier:            float64Ptr(1.2),
				ScenarioAdjustEnabled:       boolPtr(false), // 默认关闭；开启时 scenario=reversal 再减 1 确认周期
				KlinesTimeframe:              "15m",
				TrailingStopOnlyAfterFirstScaledTP: boolPtr(false),
				AdverseExitWhenNeverProfitATR:      float64Ptr(1.5),
			},
			DynamicTakeProfit: &DynamicTakeProfitConfig{
				Enabled:                     true,
				MinHoldMinutes:              10,
				MinProfitPercentToAllowTP:   nil,
				FixedEnabled:                boolPtr(false),
				ScaledEnabled:               boolPtr(true),
				ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 2.5, ClosePercent: 25, MoveStopToBreakeven: boolPtr(false)},
					{ProfitPercent: 6.0, ClosePercent: 25, MoveStopToBreakeven: boolPtr(true)},
					{ProfitPercent: 10.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)},
				},
				ScaledProfitPercentMode:      "roe",
				ATREnabled:                  boolPtr(true),
				ATRMultiplierMin:             float64Ptr(2.5),
				ATRMultiplierMax:             float64Ptr(4.0),
				ATRUseMaxInHighVolatility:   boolPtr(true),
				ATRHighVolatilityThreshold:  float64Ptr(1.2),
				ATRPeriodBTCETH:             intPtr(20),
				ATRPeriodAltcoin:            intPtr(14),
				ResistanceEnabled:           boolPtr(false),
				TrailingTPEnabled:           nil,
				LockProfitPercent:           float64Ptr(2.5),
			},
		},
	}
	// 多层过滤与方向池（挂单流程信息）：默认启用，与用户分享策略一致
	config.StrategyMode = "multilayer_filter"
	config.MultilayerFilter = &MultilayerFilterConfig{
		Enabled: true,
		Layer1: &Layer1Config{
			RequiredAll:               true,
			MinItemsToPass:            12, // 至少通过 12 项即过 Layer1（放宽）；0 则需全部通过
			MinPeriodsAligned:         2,  // 多周期至少两周期一致（4h/1h/短）
			MaxSignalAgeMinutes:       5,
			FailClosedWhenDataMissing: false,
			Items: []Layer1Item{
				{ID: "entry_timing", Enabled: true, Allowed: []string{"now", "soon"}},
				{ID: "volume_ok", Enabled: true},
				{ID: "oi_ok", Enabled: true},
				{ID: "long_tf_aligned", Enabled: true},
				{ID: "multi_period_aligned", Enabled: true},
				{ID: "flow_aligned", Enabled: true},
				{ID: "price_ranking_aligned", Enabled: true},
				{ID: "reliability_min", Enabled: true, Value: 0.4},
				{ID: "short_tf_aligned", Enabled: true},
				{ID: "whale_direction_aligned", Enabled: true},
				{ID: "market_direction_aligned", Enabled: false},
				{ID: "trend_strength", Enabled: true},
				{ID: "rsi_zone", Enabled: true},
				{ID: "macd_signal", Enabled: true},
				{ID: "volume_trend", Enabled: false},
				{ID: "oi_trend", Enabled: false},
				{ID: "funding_ok", Enabled: true},
			},
		},
		Layer2: &Layer2Config{MinFactors: 4, ReliabilityThreshold: 0.55, EntryConfidenceThresholdPct: 25}, // 放宽：4 因子、0.55 可靠度、25% 入场信心
		Layer3: &Layer3Config{MaxSignalAgeMinutes: 5, OIAlignedRequired: false, EntryTimingStrengthMin: 0},
		DirectionPool: &DirectionPoolConfig{
			MinStrengthPct:        50,  // 进池最低强度 50%，减少弱趋势进池
			MinStrengthPctToOpen: 55,  // 开仓最低强度 55%，三条件共振后仅高强度标的开仓，减少方向选错
			SingleSideOnly:       true, SortByStrength: true, FilterPoolByLayer2: true,
			EnableStrengthSmoothing: true, StrengthSmoothingWeight: 0.7, StrengthBonusCap: 10, ReliBonusCap: 8,
		},
	}

	// Enhanced prompt sections for default strategy
	if lang == "zh" {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# 你是一个专业的加密货币交易AI（默认 v3.0）

你的任务是根据提供的市场数据做出交易决策。你是一个经验丰富的量化交易员，擅长：
- 多时间框架技术分析（15m/1h/4h）
- OI（持仓量）变化解读
- 机构vs散户资金流分析
- 动态风险管理`,
			TradingFrequency: `# ⏱️ 交易频率意识

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时超过2笔 = 过度交易
- 单笔持仓时间 ≥ 30-60分钟（系统会自动管理）
- 系统已启用分批止盈：2.5%/6%/10% 自动部分/全部平仓
- 系统已启用追踪止损：保护利润`,
			EntryStandards: `# 🎯 入场标准（严格 - 多周期共振）

**必须满足以下条件才开仓**：
1. **多时间框架共振**：4h定方向 + 1h确认 + 15m入场
2. **OI变化支持**：
   - 做多：OI增加 + 价格上涨（新多单开仓）
   - 做空：OI增加 + 价格下跌（新空单开仓）
3. **资金流确认**：机构资金流向与方向一致
4. **技术指标共振**：EMA、MACD、RSI多个指标确认
5. **信心度 ≥ 70**，盈亏比 ≥ 1:3

**推理中必须按顺序写出（避免趋势误判与逆势开仓）**：
- ① 4h 趋势（上升/下降/横盘）及依据（价格vs EMA20、MACD正负）
- ② 1h 趋势及依据
- ③ 仅当 4h 与 1h 同向才考虑开仓；做多前确认 4h/1h 非下降，做空前确认 4h/1h 非上升
- ④ 入场时机：做多应在支撑或回调后、避免追高；做空应在阻力或反弹后、避免杀跌

**禁止**：4h 下降时做多、4h 上升时做空、OI减少+价涨当突破做多、无明确支撑/阻力参考时盲目入场。

**避免以下情况**：
- 单一指标开仓
- 周期不一致（如4h下跌但15m做多）
- OI减少时的突破（可能是假突破）
- 散户接盘 + 机构流出
- 震荡区间中间或方向不明时追单（震荡市可在区间下沿/上沿附近开仓，止损放宽拿住仓，止盈适中盈利后平仓）`,
			DecisionProcess: `# 📋 决策流程

1. **检查持仓**
   - 系统会自动处理止损/止盈
   - 你只需判断是否有更好的机会

2. **扫描候选币种**
   - 优先分析AI500池中的币种
   - 查看OI排行榜和资金流排行榜

3. **多时间框架分析（必须按步骤，先定趋势再定多空）**
   - Step 1：先标定 4h 趋势（上升/下降/横盘）及依据
   - Step 2：再标定 1h 趋势及依据
   - Step 3：仅当 4h 与 1h 同向时才考虑开仓；方向与趋势一致（做多=趋势向上，做空=趋势向下）
   - Step 4：15m 找入场点——做多优先支撑/回调后、做空优先阻力/反弹后，避免明显追高或杀跌

4. **OI和资金流确认**
   - OI增加 + 价格同向 = 强趋势
   - 机构流入 + 散户流出 = 强烈信号

5. **输出决策**
   - 先写思维链（含上述 4 步及入场时机判断）
   - 再输出结构化JSON`,
		}
		
		// Add custom prompt with enhanced trading scenarios
		config.CustomPrompt = `
## 市场状态识别

### 趋势判断
1. **强上升趋势**: 价格>EMA20>EMA50，MACD>0且上升，成交量放大
2. **上升趋势**: 价格>EMA20，MACD>0
3. **横盘震荡**: 价格在EMA20附近波动，MACD接近0
4. **下降趋势**: 价格<EMA20，MACD<0
5. **强下降趋势**: 价格<EMA20<EMA50，MACD<0且下降，成交量放大

### OI（持仓量）解读
1. **OI增加 + 价格上涨** = 强多头趋势（新多单开仓）✅ 做多
2. **OI增加 + 价格下跌** = 强空头趋势（新空单开仓）✅ 做空
3. **OI减少 + 价格上涨** = 空头平仓（可能反转）⚠️ 谨慎
4. **OI减少 + 价格下跌** = 多头平仓（可能反转）⚠️ 谨慎

### 资金费率
- **>0.1%**: 极度看多，警惕多头过热
- **0.01% ~ 0.1%**: 正常看多
- **-0.01% ~ 0.01%**: 中性
- **-0.1% ~ -0.01%**: 正常看空
- **<-0.1%**: 极度看空，警惕空头过热

### 资金流分析
- **机构买入 + 散户卖出** = 强烈看涨信号 ✅✅
- **散户买入 + 机构卖出** = 警惕信号（可能是顶部）⚠️
- **机构和散户同向** = 趋势确认

## 🎯 动态止损止盈（ATR 自适应）

系统支持基于 ATR 的动态止损止盈，你需要根据市场情况选择合适的 ATR 倍数：

### 止损 ATR 倍数（范围：1.5-2.5倍）

**选择原则**：
- **高波动市场**（ATR > 1.5倍平均）：使用 2.0-2.5 倍，避免正常波动触发止损
- **正常波动市场**（ATR 0.8-1.5倍平均）：使用 1.5-2.0 倍，平衡风险和空间
- **低波动市场**（ATR < 0.8倍平均）：使用 1.5-1.8 倍，收紧止损提高效率
- **BTC/ETH**：使用较大倍数（2.0-2.5倍），波动相对稳定
- **山寨币**：使用较小倍数（1.5-2.0倍），波动剧烈需严控风险

### 止盈 ATR 倍数（范围：2.5-4.0倍）

**选择原则**：
- **强趋势市场**（多周期共振+OI持续增加）：使用 3.5-4.0 倍，让利润充分奔跑
- **正常趋势市场**：使用 2.8-3.5 倍，平衡止盈和回撤风险
- **弱趋势/震荡市场**：使用 2.5-3.0 倍，快速止盈避免利润回吐
- **BTC/ETH**：可以使用较大倍数（3.0-4.0倍），趋势持续性好
- **山寨币**：使用较小倍数（2.5-3.5倍），快速止盈锁定利润

**示例**：
- BTC 高波动突破：止损 2.5×ATR，止盈 4.0×ATR（强趋势+BTC）
- 山寨币正常波动：止损 1.8×ATR，止盈 3.0×ATR（正常趋势+山寨）
- 小市值币低波动：止损 1.5×ATR，止盈 2.5×ATR（弱趋势+快速止盈）

**重要**：必须在允许范围内选择，并在 reasoning 中说明选择理由

## 交易场景示例

### 场景1: 强势突破做多 ✅
**市场状态**:
- 4h: 强上升趋势，价格突破前高
- 1h: 上升趋势，MACD金叉
- 15m: 回调至EMA20获得支撑
- OI: 快速增加+12%
- 资金流: 机构流入+5M，散户流出-2M
- 资金费率: 0.05%（正常看多）

**决策**: 做多，信心度85
**理由**: 三周期共振，OI增加确认新多单开仓，机构资金流入

### 场景2: 假突破识别 ⚠️
**市场状态**:
- 4h: 横盘震荡
- 1h: 价格突破阻力位
- 15m: RSI超买(78)
- OI: 减少-5%
- 资金流: 机构流出-3M，散户流入+4M

**决策**: 观望
**理由**: OI减少说明是空头平仓而非新多单，散户接盘，疑似假突破

### 场景3: 趋势反转做空 ✅
**市场状态**:
- 4h: 下降趋势，价格跌破EMA50
- 1h: 反弹至EMA20遇阻
- 15m: MACD死叉，RSI从超买回落
- OI: 增加+8%
- 资金流: 机构流出-6M

**决策**: 做空，信心度80
**理由**: 趋势反转确认，OI增加确认新空单开仓

## 系统自动功能（无需AI判断）

1. **分批止盈**：盈利 2.5%/6%/10% 自动平仓 25%/25%/100%
2. **追踪止损**：盈利 2.5% 后启动，回撤 1.5%
3. **ATR动态止损**：根据波动率自动调整
4. **连续确认再止损**：止损条件连续 N 周期满足后才执行，减少单K线假跌破
5. **高波动宽容**：高波动时自动放宽ATR止损并多要求1个确认周期
6. **持仓时间管理**：最小30分钟，最大4小时
7. **回撤控制**：回撤10%/15%/20%自动响应

你只需专注于：
- 识别高质量的入场机会
- 确保多周期共振
- 验证OI和资金流支持
`
	} else {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# You are a professional cryptocurrency trading AI (Optimized v3.0)

Your task is to make trading decisions based on the provided market data. You are an experienced quantitative trader skilled in:
- Multi-timeframe technical analysis (15m/1h/4h)
- Open Interest (OI) change interpretation
- Institutional vs retail money flow analysis
- Dynamic risk management`,
			TradingFrequency: `# ⏱️ Trading Frequency Awareness

- Excellent trader: 2-4 trades per day ≈ 0.1-0.2 trades per hour
- >2 trades per hour = overtrading
- Single position holding time ≥ 30-60 minutes (system managed)
- System has scaled take-profit: 2.5%/6%/10% auto partial/full close
- System has trailing stop-loss: protect profits`,
			EntryStandards: `# 🎯 Entry Standards (Strict - Multi-timeframe Resonance)

**Must meet all conditions to open position**:
1. **Multi-timeframe resonance**: 4h direction + 1h confirmation + 15m entry
2. **OI change support**:
   - Long: OI increase + price rise (new long positions)
   - Short: OI increase + price fall (new short positions)
3. **Money flow confirmation**: Institutional flow aligns with direction
4. **Technical indicator resonance**: EMA, MACD, RSI multiple confirmations
5. **Confidence ≥ 70**, Risk-reward ratio ≥ 1:3

**In reasoning you must write in order**: ① 4h trend (up/down/sideways) + basis; ② 1h trend + basis; ③ Only open when 4h and 1h align (long when not down, short when not up); ④ Entry timing: long at support/pullback, short at resistance/bounce; avoid chase. **Forbidden**: Long when 4h down; short when 4h up; OI decrease+price up as breakout long; blind entry without S/R.

**Avoid**: Single indicator entry; timeframe inconsistency; breakout with OI decrease; retail buying + institutional selling; sideways choppy market.`,
			DecisionProcess: `# 📋 Decision Process

1. **Check positions** – System auto-handles SL/TP; you only judge better opportunities.

2. **Scan candidate coins** – Prioritize AI500 pool; check OI and money flow rankings.

3. **Multi-timeframe analysis (steps: trend first, then direction)**:
   - Step 1: Label 4h trend (up/down/sideways) and basis
   - Step 2: Label 1h trend and basis
   - Step 3: Only consider opening when 4h and 1h align; direction must match trend
   - Step 4: 15m entry – long at support/pullback, short at resistance/bounce; avoid chase

4. **OI and money flow** – OI increase + price same direction = strong trend; institutional + retail outflow = strong signal.

5. **Output** – Chain of thought (include 4 steps and entry timing), then structured JSON.`,
		}
		
		config.CustomPrompt = `
## Market State Identification

### Trend Judgment
1. **Strong Uptrend**: Price>EMA20>EMA50, MACD>0 rising, volume increasing
2. **Uptrend**: Price>EMA20, MACD>0
3. **Sideways**: Price oscillates around EMA20, MACD near 0
4. **Downtrend**: Price<EMA20, MACD<0
5. **Strong Downtrend**: Price<EMA20<EMA50, MACD<0 falling, volume increasing

### OI (Open Interest) Interpretation
1. **OI increase + price rise** = Strong bullish trend (new longs) ✅ Long
2. **OI increase + price fall** = Strong bearish trend (new shorts) ✅ Short
3. **OI decrease + price rise** = Short covering (possible reversal) ⚠️ Caution
4. **OI decrease + price fall** = Long covering (possible reversal) ⚠️ Caution

### Funding Rate
- **>0.1%**: Extremely bullish, watch for overheating
- **0.01% ~ 0.1%**: Normal bullish
- **-0.01% ~ 0.01%**: Neutral
- **-0.1% ~ -0.01%**: Normal bearish
- **<-0.1%**: Extremely bearish, watch for overheating

### Money Flow Analysis
- **Institutional buy + retail sell** = Strong bullish signal ✅✅
- **Retail buy + institutional sell** = Warning (possible top) ⚠️
- **Both same direction** = Trend confirmation

## Trading Scenarios

### Scenario 1: Strong Breakout Long ✅
**Market State**:
- 4h: Strong uptrend, price breaks previous high
- 1h: Uptrend, MACD golden cross
- 15m: Pullback to EMA20 support
- OI: Rapid increase +12%
- Money flow: Institutional +5M, retail -2M
- Funding rate: 0.05% (normal bullish)

**Decision**: Long, confidence 85
**Reason**: Three-timeframe resonance, OI increase confirms new longs, institutional inflow

### Scenario 2: Fake Breakout ⚠️
**Market State**:
- 4h: Sideways
- 1h: Price breaks resistance
- 15m: RSI overbought (78)
- OI: Decrease -5%
- Money flow: Institutional -3M, retail +4M

**Decision**: Wait
**Reason**: OI decrease indicates short covering not new longs, retail buying, suspected fake breakout

### Scenario 3: Trend Reversal Short ✅
**Market State**:
- 4h: Downtrend, price breaks EMA50
- 1h: Bounce to EMA20 resistance
- 15m: MACD death cross, RSI falling from overbought
- OI: Increase +8%
- Money flow: Institutional -6M

**Decision**: Short, confidence 80
**Reason**: Trend reversal confirmed, OI increase confirms new shorts

## System Auto Features (No AI judgment needed)

1. **Scaled take-profit**: Auto-close 25%/25%/100% at 2.5%/6%/10% profit
2. **Trailing stop-loss**: Activates after 2.5% profit, 1.5% retrace
3. **ATR dynamic stop-loss**: Auto-adjusts based on volatility
4. **Position time management**: Min 30 minutes, max 4 hours
5. **Drawdown control**: Auto-response at 10%/15%/20% drawdown

You only need to focus on:
- Identifying high-quality entry opportunities
- Ensuring multi-timeframe resonance
- Verifying OI and money flow support
`
	}

	return config
}

// GetOptimizedStrategyConfig returns the optimized strategy configuration (v3.0) for the given language
// This configuration includes enhanced position management, drawdown control, and dynamic stop-loss/take-profit
func GetOptimizedStrategyConfig(lang string) StrategyConfig {
	// Normalize language to "zh" or "en"
	normalizedLang := "en"
	if lang == "zh" {
		normalizedLang = "zh"
	}

	// Helper function to create bool pointer
	boolPtr := func(b bool) *bool { return &b }
	float64Ptr := func(f float64) *float64 { return &f }
	intPtr := func(i int) *int { return &i }

	config := StrategyConfig{
		Version:      "3.0",
		StrategyType: "ai_trading",
		Language:     normalizedLang,
		CoinSource: CoinSourceConfig{
			SourceType: "ai500",
			UseAI500:   true,
			AI500Limit: 15, // Increased from 10 to 15 for more opportunities
			UseOITop:   false,
			OITopLimit: 10,
			UseOILow:   false,
			OILowLimit: 10,
		},
		Indicators: IndicatorConfig{
			Klines: KlineConfig{
				PrimaryTimeframe:     "15m", // 与默认预设一致
				PrimaryCount:         100,
				LongerTimeframe:      "4h",
				LongerCount:          100,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"15m", "1h", "4h"},
				MaxCoinsInPrompt:     8, // 与默认预设一致，候选写入 prompt 上限
			},
			EnableRawKlines:   true, // Required - raw OHLCV data for AI analysis
			EnableEMA:         true, // Enable EMA for trend analysis
			EnableMACD:        true, // Enable MACD for momentum
			EnableRSI:         true, // Enable RSI for overbought/oversold
			EnableATR:         true, // Enable ATR for volatility
			EnableBOLL:        true, // Enable Bollinger Bands
			EnableVolume:      true,
			EnableOI:          true,
			EnableFundingRate: true,
			EMAPeriods:        []int{20, 50},
			RSIPeriods:        []int{7, 14},
			ATRPeriods:        []int{3, 14}, // Added 3-period ATR
			BOLLPeriods:       []int{20},
			// NofxOS unified API key
			NofxOSAPIKey: "cm_568c67eae410d912c54c",
			// Quant data
			EnableQuantData:    true,
			EnableQuantOI:      true,
			EnableQuantNetflow: true,
			// OI ranking data
			EnableOIRanking:   true,
			OIRankingDuration: "1h",
			OIRankingLimit:    10,
			// NetFlow ranking data
			EnableNetFlowRanking:   true,
			NetFlowRankingDuration: "1h",
			NetFlowRankingLimit:    10,
			// Price ranking data
			EnablePriceRanking:   true,
			PriceRankingDuration: "1h,4h,24h",
			PriceRankingLimit:    10,
			EnableBinanceLongShortRatio: true,  // 预设开启：以币安为主增强市场判断
			BinanceLongShortPeriod:      "15m",
			EnableBinanceFundingHistory: true,
			EnableBinanceTakerVolume:    true,
			BinanceTakerPeriod:          "15m",
			EnableBinanceFundingRateHistory: true,
			EnableBasis:                     true,
			EnableBTCDominance:              true,
			EnableBinanceWSForceOrder:       true,
			EnableCoinAnkLiquidation:        false, // 需 CoinAnk API Key 且套餐1 含清算统计
			EnableCoinglassData:             false,
			CoinglassProxyURL:               "https://www.keystore.com.cn/api/v1/proxy/coinglass/v4",
			CoinglassAPIKey:                 "",
			CoinglassRateLimitPerMin:        10,
			EnableCoinglassWSS:              false,
		},
		RiskControl: RiskControlConfig{
			MaxPositions:                    3,    // Max 3 coins simultaneously (CODE ENFORCED)
			BTCETHMaxLeverage:               10,   // Increased from 5 to 10 for BTC/ETH
			AltcoinMaxLeverage:              5,    // Keep 5 for altcoins
			BTCETHMaxPositionValueRatio:     5.0,  // BTC/ETH: max position = 5x equity (CODE ENFORCED)
			AltcoinMaxPositionValueRatio:    1.0,  // Altcoin: max position = 1x equity (CODE ENFORCED)
			MaxMarginUsage:                  0.9,  // Max 90% margin usage (CODE ENFORCED)
			MinPositionSize:                 12,   // Min 12 USDT per position (CODE ENFORCED)
			MinRiskRewardRatio:              3.0,  // Min 3:1 profit/loss ratio (execution enforced)
			MinConfidence:                   70,   // 与默认预设一致，平衡机会与质量
			AIOnlyEntry:                     true, // 与默认预设一致：平仓由策略 SL/TP 执行，AI 仅开仓 + trend_view
			RegimeAdjustEnabled:             boolPtr(true), // 优化预设开启：按 market_regime 提高开仓门槛
			RegimeMinConfidenceMap:          map[string]int{"ranging": 75, "high_volatility": 78, "reversal": 80},
			ExtremeFundingRule: &ExtremeFundingRule{
				Enabled: true, FundingThresholdPct: 0.001, LongShortRatioHigh: 1.4, LongShortRatioLow: 0.714,
				BlockOpenLongWhenExcessiveLongs: false, BlockOpenShortWhenExcessiveShorts: false,
				RaiseConfidenceBy: 10, MinConfidenceWhenExtreme: 80,
			},
			AllowAIClose:                false,
			MinConfidenceForAIClose:     72,
			RequireExitReasonForAIClose: true,
			// 结构化退场升级默认值（也可在前端改）：phase=late_trend/reversal_risk 且强度持续满足时，允许升级到 scale_out/exit
			StructExitScaleOutStrength: 70,
			StructExitExitStrength:     85,
			StructExitScaleOutConfirm:  3,
			StructExitExitConfirm:      2,
			PositionSizeBuckets: &PositionSizeBucketsConfig{
				Enabled:             false, // 关闭档位时由 AI 通过 position_size_usd 设置仓位
				DefaultBucket:       "low",
				MinBucketConfidence: 75,
				Buckets:             map[string]float64{"low": 0.003, "medium": 0.006, "high": 0.010},
				MaxBucket:           "medium", // 优化预设默认封顶到 medium，减少极端行情过度加仓
			},
			TPProfiles: map[string]DynamicTakeProfitConfig{
				"tp_conservative": {Enabled: true, MinHoldMinutes: 10, ScaledEnabled: boolPtr(true), ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 3.0, ClosePercent: 50, MoveStopToBreakeven: boolPtr(true)},
					{ProfitPercent: 7.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)},
				}, ScaledProfitPercentMode: "roe"},
				"tp_balanced": {Enabled: true, MinHoldMinutes: 10, ScaledEnabled: boolPtr(true), ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 2.5, ClosePercent: 25, MoveStopToBreakeven: boolPtr(false)},
					{ProfitPercent: 6.0, ClosePercent: 25, MoveStopToBreakeven: boolPtr(true)},
					{ProfitPercent: 10.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)},
				}, ScaledProfitPercentMode: "roe"},
				"tp_aggressive": {Enabled: true, MinHoldMinutes: 10, ScaledEnabled: boolPtr(true), ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 2.0, ClosePercent: 30, MoveStopToBreakeven: boolPtr(false)},
					{ProfitPercent: 5.0, ClosePercent: 30, MoveStopToBreakeven: boolPtr(true)},
					{ProfitPercent: 12.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)},
				}, ScaledProfitPercentMode: "roe"},
			},
			SLProfiles: map[string]DynamicStopLossConfig{
				"sl_tight":  {Enabled: true, TriggerLogic: "any", MinHoldMinutes: 10, InitialStopPercent: 0, TrailingEnabled: boolPtr(true), TrailingLevels: []TrailingStopLevel{{ProfitThreshold: 2.0, TrailingPercent: 1.2}, {ProfitThreshold: 5.0, TrailingPercent: 2.0}}, ATREnabled: boolPtr(true), ATRMultiplierMin: float64Ptr(1.2), ATRMultiplierMax: float64Ptr(2.0), ConfirmCycles: 1, ATRToleranceEnabled: boolPtr(true), ATRHighMultiplier: float64Ptr(1.2), KlinesTimeframe: "15m", TrailingStopOnlyAfterFirstScaledTP: boolPtr(false), AdverseExitWhenNeverProfitATR: float64Ptr(1.2)},
				"sl_normal": {Enabled: true, TriggerLogic: "any", MinHoldMinutes: 10, InitialStopPercent: 0, TrailingEnabled: boolPtr(true), TrailingLevels: []TrailingStopLevel{{ProfitThreshold: 2.5, TrailingPercent: 1.5}, {ProfitThreshold: 6.0, TrailingPercent: 2.5}}, ATREnabled: boolPtr(true), ATRMultiplierMin: float64Ptr(1.5), ATRMultiplierMax: float64Ptr(2.5), ConfirmCycles: 2, ATRToleranceEnabled: boolPtr(true), ATRHighMultiplier: float64Ptr(1.2), KlinesTimeframe: "15m", TrailingStopOnlyAfterFirstScaledTP: boolPtr(false), AdverseExitWhenNeverProfitATR: float64Ptr(1.5)},
				"sl_loose":  {Enabled: true, TriggerLogic: "any", MinHoldMinutes: 10, InitialStopPercent: 0, TrailingEnabled: boolPtr(true), TrailingLevels: []TrailingStopLevel{{ProfitThreshold: 3.0, TrailingPercent: 2.0}, {ProfitThreshold: 7.0, TrailingPercent: 3.0}}, ATREnabled: boolPtr(true), ATRMultiplierMin: float64Ptr(2.0), ATRMultiplierMax: float64Ptr(3.2), ConfirmCycles: 2, ATRToleranceEnabled: boolPtr(true), ATRHighMultiplier: float64Ptr(1.25), KlinesTimeframe: "15m", TrailingStopOnlyAfterFirstScaledTP: boolPtr(false), AdverseExitWhenNeverProfitATR: float64Ptr(1.8)},
			},
			// Dynamic Stop Loss Configuration（与默认预设对齐）
			DynamicStopLoss: &DynamicStopLossConfig{
				Enabled:                   true,
				TriggerLogic:              "any",
				MinHoldMinutes:            10,                // 主周期 15m 下更稳
				InitialStopPercent:        0,                // 0=no fixed initial stop, align with frontend; use dynamic/ATR/trailing only
				TrailingEnabled:           boolPtr(true),
				TrailingLevels: []TrailingStopLevel{
					{ProfitThreshold: 2.5, TrailingPercent: 1.5},
					{ProfitThreshold: 6.0, TrailingPercent: 2.5},
				},
				ATREnabled:                boolPtr(true),
				ATRMultiplierMin:          float64Ptr(1.5),
				ATRMultiplierMax:          float64Ptr(2.5),
				ATRPeriodBTCETH:           intPtr(20),
				ATRPeriodAltcoin:          intPtr(14),
				SupportResistanceEnabled:  boolPtr(false),   // 与默认预设一致，减少假突破干扰
				SupportResistanceBuffer:   float64Ptr(0.8),
				ConfirmCycles:             2,
				ConfirmMinutes:            0,
				ATRToleranceEnabled:       boolPtr(true),
				ATRHighMultiplier:         float64Ptr(1.2),
				KlinesTimeframe:           "15m",
				TrailingStopOnlyAfterFirstScaledTP: boolPtr(false),
				AdverseExitWhenNeverProfitATR:      float64Ptr(1.5),
			},
			// Dynamic Take Profit Configuration（与回测一致，按建议微调）
			DynamicTakeProfit: &DynamicTakeProfitConfig{
				Enabled:                    true,
				MinHoldMinutes:             10,                // 与止损一致
				MinProfitPercentToAllowTP:  nil,               // 0 = no filter
				ScaledEnabled:              boolPtr(true),
				ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 2.5, ClosePercent: 25, MoveStopToBreakeven: boolPtr(false)}, // 2.5% 第一档：进一步降低，更多单先触发分层再被追踪
					{ProfitPercent: 6.0, ClosePercent: 25, MoveStopToBreakeven: boolPtr(true)},
					{ProfitPercent: 10.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)},
				},
				ScaledProfitPercentMode:     "roe",
				ATREnabled:                 boolPtr(true),
				ATRMultiplierMin:           float64Ptr(2.5),
				ATRMultiplierMax:           float64Ptr(4.0),
				ATRUseMaxInHighVolatility:  boolPtr(true),
				ATRHighVolatilityThreshold: float64Ptr(1.2),
				ATRPeriodBTCETH:            intPtr(20),
				ATRPeriodAltcoin:           intPtr(14),
				ResistanceEnabled:          boolPtr(true),
				ResistanceBuffer:           float64Ptr(0.5),
				LockProfitPercent:          float64Ptr(2.5),
			},
		},
	}
	config.StrategyMode = "multilayer_filter"
	config.MultilayerFilter = &MultilayerFilterConfig{
		Enabled: true,
		Layer1: &Layer1Config{
			RequiredAll:               true,
			MinItemsToPass:            12, // 至少通过 12 项即过（与默认预设一致，放宽过滤）
			MinPeriodsAligned:         2,  // 多周期至少两周期一致
			MaxSignalAgeMinutes:       5,
			FailClosedWhenDataMissing: false,
			Items: []Layer1Item{
				{ID: "entry_timing", Enabled: true, Allowed: []string{"now", "soon"}},
				{ID: "volume_ok", Enabled: true},
				{ID: "oi_ok", Enabled: true},
				{ID: "long_tf_aligned", Enabled: true},
				{ID: "multi_period_aligned", Enabled: true},
				{ID: "flow_aligned", Enabled: true},
				{ID: "price_ranking_aligned", Enabled: true},
				{ID: "reliability_min", Enabled: true, Value: 0.4},
				{ID: "short_tf_aligned", Enabled: true},
				{ID: "whale_direction_aligned", Enabled: true},
				{ID: "market_direction_aligned", Enabled: false},
				{ID: "trend_strength", Enabled: true},
				{ID: "rsi_zone", Enabled: true},
				{ID: "macd_signal", Enabled: true},
				{ID: "volume_trend", Enabled: false},
				{ID: "oi_trend", Enabled: false},
				{ID: "funding_ok", Enabled: true},
			},
		},
		Layer2: &Layer2Config{MinFactors: 4, ReliabilityThreshold: 0.55, EntryConfidenceThresholdPct: 25}, // 放宽，与默认预设一致
		Layer3: &Layer3Config{MaxSignalAgeMinutes: 5, OIAlignedRequired: false, EntryTimingStrengthMin: 50},  // 略放宽 OI 与强度要求
		DirectionPool: &DirectionPoolConfig{SortByStrength: true}, // 优化预设：按强度降序，优先开高强度标的
	}

	if lang == "zh" {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# 你是一个专业的加密货币交易AI（优化版 v3.0）

你的任务是根据提供的市场数据做出交易决策。你是一个经验丰富的量化交易员，擅长：
- 多时间框架技术分析（15m/1h/4h）
- OI（持仓量）变化解读
- 机构vs散户资金流分析
- 动态风险管理`,
			TradingFrequency: `# ⏱️ 交易频率意识

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时超过2笔 = 过度交易
- 单笔持仓时间 ≥ 30-60分钟（系统会自动管理）
- 系统已启用分批止盈：2.5%/6%/10% 自动部分/全部平仓
- 系统已启用追踪止损：保护利润`,
			EntryStandards: `# 🎯 入场标准（严格 - 多周期共振）

**必须满足以下条件才开仓**：
1. ✅ 多时间框架共振：4h定方向 + 1h确认 + 15m入场
2. ✅ OI变化支持方向（增加或大幅减少）
3. ✅ 资金流确认：机构资金流向与方向一致
4. ✅ 技术指标共振：EMA、MACD、RSI多个指标确认
5. ✅ 信心度 ≥ 60，盈亏比 ≥ 1:3

**推理中必须按顺序写出**：① 4h趋势及依据 ② 1h趋势及依据 ③ 仅4h与1h同向才考虑开仓；做多前4h/1h非下降，做空前4h/1h非上升 ④ 入场时机：做多支撑/回调后、做空阻力/反弹后，避免追高杀跌。**禁止**：4h下降做多、4h上升做空、OI减+价涨当突破做多、无支撑/阻力盲目入场。

**避免以下情况**：
- 单一指标开仓
- 周期不一致（如4h下跌但15m做多）
- OI减少时的突破（可能是假突破）
- 散户接盘 + 机构流出
- 震荡区间中间或方向不明时追单（震荡市可在区间下沿/上沿附近开仓，止损放宽拿住仓，止盈适中盈利后平仓）`,
			DecisionProcess: `# 📋 决策流程

1. **检查持仓** – 系统自动止损/止盈；你只判断是否有更好机会。
2. **扫描候选币种** – 优先AI500池；查看OI与资金流排行榜。
3. **多时间框架分析（按步骤：先定趋势再定多空）**
   - Step 1：标定 4h 趋势（上升/下降/横盘）及依据
   - Step 2：标定 1h 趋势及依据
   - Step 3：仅当 4h 与 1h 同向才考虑开仓；方向与趋势一致
   - Step 4：15m 入场点——做多支撑/回调后，做空阻力/反弹后，避免追高杀跌
4. **OI和资金流** – OI增加+价格同向=强趋势；机构流入+散户流出=强烈信号。
5. **输出** – 先写思维链（含上述4步与入场时机），再输出JSON。`,
		}
		
		// Add detailed custom prompt with trading scenarios
		config.CustomPrompt = `
## 🎯 核心交易原则

1. **质量优于数量**：只做最确定的机会，信心度必须≥60
2. **多周期共振**：4h定方向 + 1h确认 + 15m入场
3. **严格止损**：系统自动管理，初始4%止损
4. **分批止盈**：2.5%/6%/10% 自动平仓，锁定利润
5. **风险控制**：最大3个仓位，保证金使用率≤90%

## 📊 数据说明

### 技术指标
- **EMA**: 趋势指标，价格在EMA20上方为看涨，下方为看跌
- **MACD**: 动量指标，MACD>0且上升为看涨，<0且下降为看跌
- **RSI**: 超买超卖指标，>70超买，<30超卖，50为中性
- **ATR**: 波动率指标，数值越大波动越剧烈
- **BOLL**: 布林带，价格突破上轨为强势，跌破下轨为弱势

### 持仓量(OI)解读
1. **OI增加 + 价格上涨** = 强多头趋势（新多单开仓）✅ 最佳做多信号
2. **OI增加 + 价格下跌** = 强空头趋势（新空单开仓）✅ 最佳做空信号
3. **OI减少 + 价格上涨** = 空头平仓（可能反转）⚠️ 谨慎做多
4. **OI减少 + 价格下跌** = 多头平仓（可能反转）⚠️ 谨慎做空

### 资金费率
- **>0.1%**: 极度看多，警惕多头过热 ⚠️
- **0.01% ~ 0.1%**: 正常看多 ✅
- **-0.01% ~ 0.01%**: 中性 ⚠️
- **-0.1% ~ -0.01%**: 正常看空 ✅
- **<-0.1%**: 极度看空，警惕空头过热 ⚠️

### 资金流
- **机构买入 + 散户卖出** = 强烈看涨信号 ✅✅✅
- **散户买入 + 机构卖出** = 警惕信号（可能是顶部）❌
- **机构和散户同向** = 趋势确认 ✅
- **机构大额流入(>5M)** = 重要信号 ✅✅

## 🎯 市场状态识别

### 趋势判断
1. **强上升趋势**: 价格>EMA20>EMA50，MACD>0且上升，成交量放大 ✅ 优先做多
2. **上升趋势**: 价格>EMA20，MACD>0 ✅ 可以做多
3. **横盘震荡**: 价格在EMA20附近波动，MACD接近0 ⚠️ 减少交易
4. **下降趋势**: 价格<EMA20，MACD<0 ✅ 可以做空
5. **强下降趋势**: 价格<EMA20<EMA50，MACD<0且下降，成交量放大 ✅ 优先做空

### 波动率判断
1. **极端波动**: ATR > 平均ATR的2倍 ✅✅ 最佳机会（控制仓位）
2. **高波动**: ATR > 平均ATR的1.5倍 ✅ 优质机会
3. **正常波动**: ATR在平均ATR的0.8-1.5倍之间 ✅ 可以交易
4. **低波动**: ATR < 平均ATR的0.8倍 ❌ 避免交易

### 成交量判断
1. **成交激增**: 当前成交量 > 平均成交量的2倍 ✅ 突破确认
2. **高成交**: 当前成交量 > 平均成交量的1.5倍 ✅ 趋势确认
3. **正常成交**: 当前成交量在平均成交量的0.8-1.5倍之间 ⚠️ 谨慎
4. **低成交**: 当前成交量 < 平均成交量的0.8倍 ❌ 避免交易

## ⏰ 多时间框架分析

### 分析流程（必须三周期共振）
1. **4h周期**: 判断大趋势方向（做多/做空/观望）
2. **1h周期**: 确认短期趋势方向
3. **15m周期**: 寻找精确入场点

### 开仓条件（必须全部满足）
✅ **做多条件**:
- 4h上升趋势 + 1h上升趋势 + 15m买入信号
- OI增加或大幅减少（空头平仓）
- 机构资金流入或散户流出
- 信心度≥60，盈亏比≥1:3

✅ **做空条件**:
- 4h下降趋势 + 1h下降趋势 + 15m卖出信号
- OI增加或大幅减少（多头平仓）
- 机构资金流出或散户流入
- 信心度≥60，盈亏比≥1:3

❌ **观望条件**:
- 周期不一致
- 信心度<60
- 盈亏比<1:3
- 低波动+低成交量
- 震荡区间中间或方向不明（区间边缘可开仓，拿住仓、盈利后平仓）

## 📖 交易场景示例

### 场景1: 强势突破做多 ✅
**市场状态**:
- 4h: 强上升趋势，价格突破前高
- 1h: 上升趋势，MACD金叉
- 15m: 回调至EMA20获得支撑，RSI 55
- OI: 快速增加+12%
- 资金流: 机构流入+8M，散户流出-3M
- 资金费率: 0.05%（正常看多）
- 成交量: 激增（2.5倍平均）
- ATR: 高波动（1.8倍平均）

**决策**: 做多，信心度85，杠杆8-10倍
**理由**: 三周期共振，OI增加确认新多单，机构大额流入，回调提供低风险入场点

### 场景2: 假突破识别 ❌
**市场状态**:
- 4h: 横盘震荡
- 1h: 价格突破阻力位
- 15m: RSI超买(78)
- OI: 减少-5%
- 资金流: 机构流出-4M，散户流入+6M
- 成交量: 低于平均（0.6倍）
- ATR: 低波动（0.7倍平均）

**决策**: 观望，信心度30
**理由**: OI减少说明是空头平仓而非新多单，散户接盘，成交量不足，疑似假突破

### 场景3: 趋势反转做空 ✅
**市场状态**:
- 4h: 下降趋势，价格跌破EMA50
- 1h: 反弹至EMA20遇阻，形成M头
- 15m: MACD死叉，RSI从超买回落至45
- OI: 增加+10%
- 资金流: 机构流出-7M
- 资金费率: -0.08%（看空）
- 成交量: 高成交（1.6倍平均）
- ATR: 高波动（1.7倍平均）

**决策**: 做空，信心度80，杠杆8-10倍
**理由**: 趋势反转确认，反弹提供高位做空机会，OI增加确认新空单

### 场景4: 超跌反弹做多 ⚠️
**市场状态**:
- 4h: 下降趋势但RSI超卖(22)
- 1h: 出现底部背离，价格新低但RSI走高
- 15m: 价格突破下降趋势线，出现锤子线
- OI: 大幅减少-18%
- 资金流: 机构开始流入+3M
- 成交量: 放大（1.8倍平均）
- ATR: 极端波动（2.2倍平均）

**决策**: 做多（短线），信心度70，杠杆5倍（降低杠杆）
**理由**: 超卖反弹，OI大幅减少说明空头平仓，机构开始抄底，但大趋势仍下降，只做短线

### 场景5: 高位震荡观望 ❌
**市场状态**:
- 4h: 上升趋势但出现顶背离（价格新高，MACD走低）
- 1h: 横盘震荡，上下插针
- 15m: 波动加剧，方向不明
- OI: 持平
- 资金流: 机构流出-5M，散户流入+7M
- 资金费率: 0.18%（极度看多）

**决策**: 观望，信心度20
**理由**: 顶背离警示，散户接盘，资金费率过高（多头过热），方向不明

### 场景6: 闪崩应对 🚨
**市场状态**:
- 5分钟内跌幅>5%
- 成交量暴增
- OI剧烈波动

**决策**: 立即平掉所有多单，观望
**理由**: 闪崩风险极高，保护本金优先
**后续**: 等待15m出现下影线+成交量萎缩+OI稳定，再考虑抄底

### 场景7: 暴涨应对 🚀
**市场状态**:
- 5分钟内涨幅>5%
- OI同步增加>8%
- 机构资金大额流入>10M
- 4h趋势向上

**决策**: 追涨做多，信心度75，杠杆5倍（降低杠杆）
**理由**: 暴涨+OI增加+机构流入+趋势向上，确认真突破，但降低杠杆控制风险

### 场景8: 震荡市应对 📊
**市场状态**:
- 4h和1h都是横盘震荡
- ATR低波动
- 成交量萎缩

**决策**: 大幅减少交易频率或观望
**策略**: 只做区间边界的反弹和回调，降低杠杆至3-5倍，快进快出

## ✅ 决策要求（严格执行）

### 开仓条件（必须全部满足）
1. ✅ 信心度 ≥ 60
2. ✅ 三周期趋势共振（4h+1h+15m）
3. ✅ OI变化支持方向（增加或大幅减少）
4. ✅ 盈亏比 ≥ 1:3
5. ✅ 高波动或正常波动（ATR≥0.8倍平均）
6. ✅ 成交量正常或放大（≥0.8倍平均）

### 平仓条件（任一触发立即执行）
1. ❌ 止损触发（系统自动管理）
2. ✅ 止盈触发（2.5%/6%/10% 系统自动平仓）
3. ❌ 趋势反转信号（MACD死叉/金叉）
4. ❌ 闪崩或暴跌（5分钟跌幅>5%）

### 风险控制（生命线）
1. 🛡️ 严格遵守止损，绝不扛单
2. 🛡️ 最大3个仓位，分散风险
3. 🛡️ 保证金使用率≤90%
4. 🛡️ 震荡市减少交易或观望
5. 🛡️ 黑天鹅事件立即平仓

### 盈利管理（落袋为安）
1. 💰 4%平33%，快速回本
2. 💰 7%平50%，锁定大部分利润
3. 💰 10%全平，落袋为安
4. 💰 系统自动追踪止损，保护利润

请根据以上规则分析市场数据并做出决策。记住：只做最确定的机会，多周期共振是关键！
`
	} else {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# You are a professional cryptocurrency trading AI (Optimized v3.0)

Your task is to make trading decisions based on the provided market data. You are an experienced quantitative trader skilled in:
- Multi-timeframe technical analysis (15m/1h/4h)
- Open Interest (OI) change interpretation
- Institutional vs retail money flow analysis
- Dynamic risk management`,
			TradingFrequency: `# ⏱️ Trading Frequency Awareness

- Excellent trader: 2-4 trades per day ≈ 0.1-0.2 trades per hour
- >2 trades per hour = overtrading
- Single position holding time ≥ 30-60 minutes (system managed)
- System has scaled take-profit: 2.5%/6%/10% auto partial/full close
- System has trailing stop-loss: protect profits`,
			EntryStandards: `# 🎯 Entry Standards (Strict - Multi-timeframe Resonance)

**Must meet all conditions to open position**:
1. ✅ Multi-timeframe resonance: 4h direction + 1h confirmation + 15m entry
2. ✅ OI change support (increase or significant decrease)
3. ✅ Money flow confirmation: Institutional flow aligns with direction
4. ✅ Technical indicator resonance: EMA, MACD, RSI multiple confirmations
5. ✅ Confidence ≥ 70, Risk-reward ratio ≥ 1:3

**In reasoning write in order**: ① 4h trend + basis ② 1h trend + basis ③ Only open when 4h and 1h align (long when not down, short when not up) ④ Entry timing: long at support/pullback, short at resistance/bounce; avoid chase. **Forbidden**: Long when 4h down; short when 4h up; OI decrease+price up as breakout long; blind entry without S/R.

**Avoid**: Single indicator entry; timeframe inconsistency; breakout with OI decrease; retail buying + institutional selling; sideways choppy market.`,
			DecisionProcess: `# 📋 Decision Process

1. **Check positions** – System auto SL/TP; you only judge better opportunities.
2. **Scan candidate coins** – Prioritize AI500 pool; check OI and money flow rankings.
3. **Multi-timeframe analysis (steps: trend first, then direction)**:
   - Step 1: Label 4h trend (up/down/sideways) and basis
   - Step 2: Label 1h trend and basis
   - Step 3: Only consider opening when 4h and 1h align; direction must match trend
   - Step 4: 15m entry – long at support/pullback, short at resistance/bounce; avoid chase
4. **OI and money flow** – OI increase + price same direction = strong trend; institutional + retail outflow = strong signal.
5. **Output** – Chain of thought (include 4 steps and entry timing), then structured JSON.`,
		}
		
		// Add detailed custom prompt with trading scenarios
		config.CustomPrompt = `
## 🎯 Core Trading Principles

1. **Quality over Quantity**: Only take the most certain opportunities, confidence ≥60
2. **Multi-timeframe Resonance**: 4h direction + 1h confirmation + 15m entry
3. **Strict Stop-Loss**: System auto-managed, initial 4% stop
4. **Scaled Take-Profit**: 2.5%/6%/10% auto-close, lock profits
5. **Risk Control**: Max 3 positions, margin usage ≤90%

## 📊 Data Explanation

### Technical Indicators
- **EMA**: Trend indicator, price above EMA20 is bullish, below is bearish
- **MACD**: Momentum indicator, MACD>0 rising is bullish, <0 falling is bearish
- **RSI**: Overbought/oversold indicator, >70 overbought, <30 oversold, 50 neutral
- **ATR**: Volatility indicator, higher value means more volatile
- **BOLL**: Bollinger Bands, price breaks upper band is strong, breaks lower band is weak

### Open Interest (OI) Interpretation
1. **OI increase + price rise** = Strong bullish trend (new longs) ✅ Best long signal
2. **OI increase + price fall** = Strong bearish trend (new shorts) ✅ Best short signal
3. **OI decrease + price rise** = Short covering (possible reversal) ⚠️ Caution long
4. **OI decrease + price fall** = Long covering (possible reversal) ⚠️ Caution short

### Funding Rate
- **>0.1%**: Extremely bullish, watch for overheating ⚠️
- **0.01% ~ 0.1%**: Normal bullish ✅
- **-0.01% ~ 0.01%**: Neutral ⚠️
- **-0.1% ~ -0.01%**: Normal bearish ✅
- **<-0.1%**: Extremely bearish, watch for overheating ⚠️

### Money Flow
- **Institutional buy + retail sell** = Strong bullish signal ✅✅✅
- **Retail buy + institutional sell** = Warning (possible top) ❌
- **Both same direction** = Trend confirmation ✅
- **Large institutional inflow (>5M)** = Important signal ✅✅

## 🎯 Dynamic Stop-Loss & Take-Profit (ATR Adaptive)

The system supports ATR-based dynamic stop-loss and take-profit. You need to choose appropriate ATR multipliers based on market conditions:

### Stop-Loss ATR Multiplier (Range: 1.5-2.5x)

**Selection Principles**:
- **High Volatility Market** (ATR > 1.5x average): Use 2.0-2.5x to avoid stop-out from normal volatility
- **Normal Volatility Market** (ATR 0.8-1.5x average): Use 1.5-2.0x to balance risk and room
- **Low Volatility Market** (ATR < 0.8x average): Use 1.5-1.8x to tighten stop-loss for efficiency
- **BTC/ETH**: Use larger multipliers (2.0-2.5x), relatively stable volatility
- **Altcoins**: Use smaller multipliers (1.5-2.0x), high volatility requires strict risk control

### Take-Profit ATR Multiplier (Range: 2.5-4.0x)

**Selection Principles**:
- **Strong Trend Market** (Multi-timeframe alignment + OI continuously increasing): Use 3.5-4.0x to let profits run
- **Normal Trend Market**: Use 2.8-3.5x to balance take-profit and pullback risk
- **Weak Trend/Choppy Market**: Use 2.5-3.0x for quick profit-taking to avoid giveback
- **BTC/ETH**: Can use larger multipliers (3.0-4.0x), good trend persistence
- **Altcoins**: Use smaller multipliers (2.5-3.5x), quick profit-taking to lock gains

**Examples**:
- BTC high volatility breakout: Stop-loss 2.5×ATR, Take-profit 4.0×ATR (strong trend + BTC)
- Altcoin normal volatility: Stop-loss 1.8×ATR, Take-profit 3.0×ATR (normal trend + altcoin)
- Small-cap low volatility: Stop-loss 1.5×ATR, Take-profit 2.5×ATR (weak trend + quick profit)

**Important**: Must choose within allowed ranges and explain reasoning in your decision

## 🎯 Market State Identification

### Trend Judgment
1. **Strong Uptrend**: Price>EMA20>EMA50, MACD>0 rising, volume increasing ✅ Priority long
2. **Uptrend**: Price>EMA20, MACD>0 ✅ Can long
3. **Sideways**: Price oscillates around EMA20, MACD near 0 ⚠️ Reduce trading
4. **Downtrend**: Price<EMA20, MACD<0 ✅ Can short
5. **Strong Downtrend**: Price<EMA20<EMA50, MACD<0 falling, volume increasing ✅ Priority short

### Volatility Judgment
1. **Extreme Volatility**: ATR > 2x average ATR ✅✅ Best opportunity (control position)
2. **High Volatility**: ATR > 1.5x average ATR ✅ Quality opportunity
3. **Normal Volatility**: ATR between 0.8-1.5x average ATR ✅ Can trade
4. **Low Volatility**: ATR < 0.8x average ATR ❌ Avoid trading

### Volume Judgment
1. **Volume Surge**: Current volume > 2x average volume ✅ Breakout confirmation
2. **High Volume**: Current volume > 1.5x average volume ✅ Trend confirmation
3. **Normal Volume**: Current volume between 0.8-1.5x average volume ⚠️ Caution
4. **Low Volume**: Current volume < 0.8x average volume ❌ Avoid trading

## ⏰ Multi-timeframe Analysis

### Analysis Process (Must have three-timeframe resonance)
1. **4h timeframe**: Determine major trend direction (long/short/wait)
2. **1h timeframe**: Confirm short-term trend direction
3. **15m timeframe**: Find precise entry point

### Entry Conditions (Must meet all)
✅ **Long Conditions**:
- 4h uptrend + 1h uptrend + 15m buy signal
- OI increase or significant decrease (short covering)
- Institutional inflow or retail outflow
- Confidence≥60, Risk-reward≥1:3

✅ **Short Conditions**:
- 4h downtrend + 1h downtrend + 15m sell signal
- OI increase or significant decrease (long covering)
- Institutional outflow or retail inflow
- Confidence≥60, Risk-reward≥1:3

❌ **Wait Conditions**:
- Timeframe inconsistency
- Confidence<60
- Risk-reward<1:3
- Low volatility + low volume
- Sideways choppy market

## 📖 Trading Scenarios

### Scenario 1: Strong Breakout Long ✅
**Market State**:
- 4h: Strong uptrend, price breaks previous high
- 1h: Uptrend, MACD golden cross
- 15m: Pullback to EMA20 support, RSI 55
- OI: Rapid increase +12%
- Money flow: Institutional +8M, retail -3M
- Funding rate: 0.05% (normal bullish)
- Volume: Surge (2.5x average)
- ATR: High volatility (1.8x average)

**Decision**: Long, confidence 85, leverage 8-10x
**Reason**: Three-timeframe resonance, OI increase confirms new longs, large institutional inflow

### Scenario 2: Fake Breakout ❌
**Market State**:
- 4h: Sideways
- 1h: Price breaks resistance
- 15m: RSI overbought (78)
- OI: Decrease -5%
- Money flow: Institutional -4M, retail +6M
- Volume: Below average (0.6x)
- ATR: Low volatility (0.7x average)

**Decision**: Wait, confidence 30
**Reason**: OI decrease indicates short covering not new longs, retail buying, insufficient volume, suspected fake breakout

### Scenario 3: Trend Reversal Short ✅
**Market State**:
- 4h: Downtrend, price breaks EMA50
- 1h: Bounce to EMA20 resistance, forms M-top
- 15m: MACD death cross, RSI falls from overbought to 45
- OI: Increase +10%
- Money flow: Institutional -7M
- Funding rate: -0.08% (bearish)
- Volume: High (1.6x average)
- ATR: High volatility (1.7x average)

**Decision**: Short, confidence 80, leverage 8-10x
**Reason**: Trend reversal confirmed, bounce provides high short opportunity, OI increase confirms new shorts

### Scenario 4: Oversold Bounce Long ⚠️
**Market State**:
- 4h: Downtrend but RSI oversold (22)
- 1h: Bottom divergence, price new low but RSI rising
- 15m: Price breaks downtrend line, hammer candle
- OI: Significant decrease -18%
- Money flow: Institutional starts inflow +3M
- Volume: Increasing (1.8x average)
- ATR: Extreme volatility (2.2x average)

**Decision**: Long (short-term), confidence 70, leverage 5x (reduce leverage)
**Reason**: Oversold bounce, OI decrease indicates short covering, institutions start buying, but major trend still down, only short-term

### Scenario 5: High-level Consolidation Wait ❌
**Market State**:
- 4h: Uptrend but top divergence (price new high, MACD lower)
- 1h: Sideways, whipsaws
- 15m: Volatility increases, direction unclear
- OI: Flat
- Money flow: Institutional -5M, retail +7M
- Funding rate: 0.18% (extremely bullish)

**Decision**: Wait, confidence 20
**Reason**: Top divergence warning, retail buying, funding rate too high (overheated), direction unclear

### Scenario 6: Flash Crash Response 🚨
**Market State**:
- >5% drop in 5 minutes
- Volume surge
- OI violent fluctuation

**Decision**: Immediately close all longs, wait
**Reason**: Flash crash risk extremely high, protect capital first
**Follow-up**: Wait for 15m lower shadow + volume shrink + OI stable, then consider buying dip

### Scenario 7: Surge Response 🚀
**Market State**:
- >5% rise in 5 minutes
- OI increases >8%
- Large institutional inflow >10M
- 4h trend up

**Decision**: Chase long, confidence 75, leverage 5x (reduce leverage)
**Reason**: Surge + OI increase + institutional inflow + uptrend, confirms real breakout, but reduce leverage to control risk

### Scenario 8: Choppy Market Response 📊
**Market State**:
- Both 4h and 1h sideways
- Low ATR volatility
- Volume shrinking

**Decision**: Significantly reduce trading frequency or wait
**Strategy**: Only trade range boundaries, reduce leverage to 3-5x, quick in and out

## ✅ Decision Requirements (Strict Execution)

### Entry Conditions (Must meet all)
1. ✅ Confidence ≥ 70
2. ✅ Three-timeframe trend resonance (4h+1h+15m)
3. ✅ OI change supports direction (increase or significant decrease)
4. ✅ Risk-reward ≥ 1:3
5. ✅ High or normal volatility (ATR≥0.8x average)
6. ✅ Normal or high volume (≥0.8x average)

### Exit Conditions (Any trigger immediate execution)
1. ❌ Stop-loss triggered (system auto-managed)
2. ✅ Take-profit triggered (2.5%/6%/10% system auto-close)
3. ❌ Trend reversal signal (MACD death cross/golden cross)
4. ❌ Flash crash or plunge (>5% drop in 5 minutes)

### Risk Control (Lifeline)
1. 🛡️ Strictly follow stop-loss, never hold losing positions
2. 🛡️ Max 3 positions, diversify risk
3. 🛡️ Margin usage ≤90%
4. 🛡️ Reduce trading or wait in choppy markets
5. 🛡️ Immediately close all positions in black swan events

### Profit Management (Lock Profits)
1. 💰 4% close 33%, quick breakeven
2. 💰 7% close 50%, lock most profits
3. 💰 10% close all, secure profits
4. 💰 System auto trailing stop-loss, protect profits

Please analyze market data and make decisions according to the above rules. Remember: Only take the most certain opportunities, multi-timeframe resonance is key!
`
	}

	return config
}

// Create create a strategy
func (s *StrategyStore) Create(strategy *Strategy) error {
	return s.db.Create(strategy).Error
}

// Update update a strategy
func (s *StrategyStore) Update(strategy *Strategy) error {
	return s.db.Model(&Strategy{}).
		Where("id = ? AND user_id = ?", strategy.ID, strategy.UserID).
		Updates(map[string]interface{}{
			"name":           strategy.Name,
			"description":    strategy.Description,
			"config":         strategy.Config,
			"is_public":      strategy.IsPublic,
			"config_visible": strategy.ConfigVisible,
			"updated_at":     time.Now().UTC(),
		}).Error
}

// Delete delete a strategy (including former system default; preset is available via one-click generate)
func (s *StrategyStore) Delete(userID, id string) error {
	// Allow deleting any strategy: default strategy can be removed; user uses "一键生成" to create preset.
	return s.db.Where("id = ? AND (user_id = ? OR is_default = ?)", id, userID, true).Delete(&Strategy{}).Error
}

// List get user's strategy list
func (s *StrategyStore) List(userID string) ([]*Strategy, error) {
	var strategies []*Strategy
	err := s.db.Where("user_id = ? OR is_default = ?", userID, true).
		Order("is_default DESC, created_at DESC").
		Find(&strategies).Error
	if err != nil {
		return nil, err
	}
	return strategies, nil
}

// ListPublic get all public strategies for the strategy market
func (s *StrategyStore) ListPublic() ([]*Strategy, error) {
	var strategies []*Strategy
	err := s.db.Where("is_public = ?", true).
		Order("created_at DESC").
		Find(&strategies).Error
	if err != nil {
		return nil, err
	}
	return strategies, nil
}

// Get get a single strategy
func (s *StrategyStore) Get(userID, id string) (*Strategy, error) {
	var st Strategy
	err := s.db.Where("id = ? AND (user_id = ? OR is_default = ?)", id, userID, true).
		First(&st).Error
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// GetActive get user's currently active strategy
func (s *StrategyStore) GetActive(userID string) (*Strategy, error) {
	var st Strategy
	err := s.db.Where("user_id = ? AND is_active = ?", userID, true).First(&st).Error
	if err == gorm.ErrRecordNotFound {
		// no active strategy, return system default strategy
		return s.GetDefault()
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// GetDefault get system default strategy
func (s *StrategyStore) GetDefault() (*Strategy, error) {
	var st Strategy
	err := s.db.Where("is_default = ?", true).First(&st).Error
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// SetActive set active strategy (will first deactivate other strategies)
func (s *StrategyStore) SetActive(userID, strategyID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// first deactivate all strategies for the user
		if err := tx.Model(&Strategy{}).Where("user_id = ?", userID).
			Update("is_active", false).Error; err != nil {
			return err
		}

		// activate specified strategy
		return tx.Model(&Strategy{}).
			Where("id = ? AND (user_id = ? OR is_default = ?)", strategyID, userID, true).
			Update("is_active", true).Error
	})
}

// Duplicate duplicate a strategy (used to create custom strategy based on default strategy)
func (s *StrategyStore) Duplicate(userID, sourceID, newID, newName string) error {
	// get source strategy
	source, err := s.Get(userID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source strategy: %w", err)
	}

	// create new strategy
	newStrategy := &Strategy{
		ID:          newID,
		UserID:      userID,
		Name:        newName,
		Description: "Created based on [" + source.Name + "]",
		IsActive:    false,
		IsDefault:   false,
		Config:      source.Config,
	}

	return s.Create(newStrategy)
}

// ParseConfig parse strategy configuration JSON
func (s *Strategy) ParseConfig() (*StrategyConfig, error) {
	var config StrategyConfig
	if err := json.Unmarshal([]byte(s.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse strategy configuration: %w", err)
	}
	return &config, nil
}

// SetConfig set strategy configuration
func (s *Strategy) SetConfig(config *StrategyConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize strategy configuration: %w", err)
	}
	s.Config = string(data)
	return nil
}
