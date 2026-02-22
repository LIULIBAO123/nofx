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

// StrategyConfig strategy configuration details (JSON structure)
type StrategyConfig struct {
	// Strategy type: "ai_trading" (default) or "grid_trading"
	StrategyType string `json:"strategy_type,omitempty"`

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

	// Dynamic Stop Loss & Take Profit
	DynamicStopLoss   *DynamicStopLossConfig   `json:"dynamic_stop_loss,omitempty"`
	DynamicTakeProfit *DynamicTakeProfitConfig `json:"dynamic_take_profit,omitempty"`
}

// DynamicStopLossConfig dynamic stop loss configuration
type DynamicStopLossConfig struct {
	Enabled      bool   `json:"enabled"`
	TriggerLogic string `json:"trigger_logic"` // "any" = 任一条件触发即平仓, "all" = 所有启用的条件都触发才平仓

	// Initial fixed stop loss (required, acts as safety net)
	InitialStopPercent float64 `json:"initial_stop_percent"` // initial fixed stop %

	// Trailing Stop - Tiered Mode
	TrailingEnabled *bool                `json:"trailing_enabled,omitempty"` // enable trailing stop
	TrailingLevels  []TrailingStopLevel  `json:"trailing_levels,omitempty"`  // trailing stop levels

	// ATR Stop - Dynamic Range Mode
	ATREnabled        *bool    `json:"atr_enabled,omitempty"`          // enable ATR stop
	ATRMultiplierMin  *float64 `json:"atr_multiplier_min,omitempty"`   // ATR multiplier min (AI range)
	ATRMultiplierMax  *float64 `json:"atr_multiplier_max,omitempty"`   // ATR multiplier max (AI range)
	ATRPeriodBTCETH   *int     `json:"atr_period_btc_eth,omitempty"`   // ATR period for BTC/ETH
	ATRPeriodAltcoin  *int     `json:"atr_period_altcoin,omitempty"`   // ATR period for altcoins

	// Support/Resistance Stop
	SupportResistanceEnabled *bool    `json:"support_resistance_enabled,omitempty"` // enable S/R stop
	SupportResistanceBuffer  *float64 `json:"support_resistance_buffer,omitempty"`  // buffer %
}

// TrailingStopLevel trailing stop level configuration
type TrailingStopLevel struct {
	ProfitThreshold float64 `json:"profit_threshold"` // profit % threshold to activate this level
	TrailingPercent float64 `json:"trailing_percent"` // trailing stop % at this level
}

// DynamicTakeProfitConfig dynamic take profit configuration
type DynamicTakeProfitConfig struct {
	Enabled bool `json:"enabled"`

	// Fixed Take Profit
	FixedEnabled *bool    `json:"fixed_enabled,omitempty"` // enable fixed take profit
	FixedPercent *float64 `json:"fixed_percent,omitempty"` // fixed take profit %

	// Scaled Take Profit
	ScaledEnabled *bool                   `json:"scaled_enabled,omitempty"` // enable scaled take profit
	ScaledLevels  []ScaledTakeProfitLevel `json:"scaled_levels,omitempty"`

	// ATR Take Profit - Dynamic Range Mode
	ATREnabled        *bool    `json:"atr_enabled,omitempty"`          // enable ATR take profit
	ATRMultiplierMin  *float64 `json:"atr_multiplier_min,omitempty"`   // ATR multiplier min (AI range)
	ATRMultiplierMax  *float64 `json:"atr_multiplier_max,omitempty"`   // ATR multiplier max (AI range)
	ATRPeriodBTCETH   *int     `json:"atr_period_btc_eth,omitempty"`   // ATR period for BTC/ETH
	ATRPeriodAltcoin  *int     `json:"atr_period_altcoin,omitempty"`   // ATR period for altcoins

	// Resistance Take Profit
	ResistanceEnabled *bool    `json:"resistance_enabled,omitempty"` // enable resistance take profit
	ResistanceBuffer  *float64 `json:"resistance_buffer,omitempty"`  // buffer %

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
	// No longer pre-populate strategies - create on demand when user configures
	return nil
}

// GetDefaultStrategyConfig returns the default strategy configuration for the given language
func GetDefaultStrategyConfig(lang string) StrategyConfig {
	// Normalize language to "zh" or "en"
	normalizedLang := "en"
	if lang == "zh" {
		normalizedLang = "zh"
	}

	config := StrategyConfig{
		Language: normalizedLang,
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
				PrimaryTimeframe:     "5m",
				PrimaryCount:         30,
				LongerTimeframe:      "4h",
				LongerCount:          10,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"5m", "15m", "1h", "4h"},
			},
			EnableRawKlines:   true, // Required - raw OHLCV data for AI analysis
			EnableEMA:         false,
			EnableMACD:        false,
			EnableRSI:         false,
			EnableATR:         false,
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
		},
		RiskControl: RiskControlConfig{
			MaxPositions:                    3,   // Max 3 coins simultaneously (CODE ENFORCED)
			BTCETHMaxLeverage:               5,   // BTC/ETH exchange leverage (AI guided)
			AltcoinMaxLeverage:              5,   // Altcoin exchange leverage (AI guided)
			BTCETHMaxPositionValueRatio:     5.0, // BTC/ETH: max position = 5x equity (CODE ENFORCED)
			AltcoinMaxPositionValueRatio:    1.0, // Altcoin: max position = 1x equity (CODE ENFORCED)
			MaxMarginUsage:                  0.9, // Max 90% margin usage (CODE ENFORCED)
			MinPositionSize:                 12,  // Min 12 USDT per position (CODE ENFORCED)
			MinRiskRewardRatio:              3.0, // Min 3:1 profit/loss ratio (AI guided)
			MinConfidence:                   75,  // Min 75% confidence (AI guided)
		},
	}

	// Enhanced prompt sections for optimized strategy
	if lang == "zh" {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# 你是一个专业的加密货币交易AI（优化版 v2.0）

你的任务是根据提供的市场数据做出交易决策。你是一个经验丰富的量化交易员，擅长：
- 多时间框架技术分析（15m/1h/4h）
- OI（持仓量）变化解读
- 机构vs散户资金流分析
- 动态风险管理`,
			TradingFrequency: `# ⏱️ 交易频率意识

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时超过2笔 = 过度交易
- 单笔持仓时间 ≥ 30-60分钟（系统会自动管理）
- 系统已启用分批止盈：3%/5%/8%自动平仓
- 系统已启用追踪止损：保护利润`,
			EntryStandards: `# 🎯 入场标准（严格 - 多周期共振）

**必须满足以下条件才开仓**：
1. **多时间框架共振**：4h定方向 + 1h确认 + 15m入场
2. **OI变化支持**：
   - 做多：OI增加 + 价格上涨（新多单开仓）
   - 做空：OI增加 + 价格下跌（新空单开仓）
3. **资金流确认**：机构资金流向与方向一致
4. **技术指标共振**：EMA、MACD、RSI多个指标确认
5. **信心度 ≥ 60**，盈亏比 ≥ 1:3

**避免以下情况**：
- 单一指标开仓
- 周期不一致（如4h下跌但15m做多）
- OI减少时的突破（可能是假突破）
- 散户接盘 + 机构流出
- 横盘震荡市场`,
			DecisionProcess: `# 📋 决策流程

1. **检查持仓**
   - 系统会自动处理止损/止盈
   - 你只需判断是否有更好的机会

2. **扫描候选币种**
   - 优先分析AI500池中的币种
   - 查看OI排行榜和资金流排行榜

3. **多时间框架分析**
   - 4h：判断大趋势（做多/做空/观望）
   - 1h：确认短期趋势
   - 15m：寻找精确入场点

4. **OI和资金流确认**
   - OI增加 + 价格同向 = 强趋势
   - 机构流入 + 散户流出 = 强烈信号

5. **输出决策**
   - 先写思维链（分析过程）
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

1. **分批止盈**：盈利3%/5%/8%自动平仓33%/50%/100%
2. **追踪止损**：盈利2%后启动，距离1.5%
3. **ATR动态止损**：根据波动率自动调整
4. **持仓时间管理**：最小30分钟，最大4小时
5. **回撤控制**：回撤10%/15%/20%自动响应

你只需专注于：
- 识别高质量的入场机会
- 确保多周期共振
- 验证OI和资金流支持
`
	} else {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# You are a professional cryptocurrency trading AI (Optimized v2.0)

Your task is to make trading decisions based on the provided market data. You are an experienced quantitative trader skilled in:
- Multi-timeframe technical analysis (15m/1h/4h)
- Open Interest (OI) change interpretation
- Institutional vs retail money flow analysis
- Dynamic risk management`,
			TradingFrequency: `# ⏱️ Trading Frequency Awareness

- Excellent trader: 2-4 trades per day ≈ 0.1-0.2 trades per hour
- >2 trades per hour = overtrading
- Single position holding time ≥ 30-60 minutes (system managed)
- System has scaled take-profit: 3%/5%/8% auto-close
- System has trailing stop-loss: protect profits`,
			EntryStandards: `# 🎯 Entry Standards (Strict - Multi-timeframe Resonance)

**Must meet all conditions to open position**:
1. **Multi-timeframe resonance**: 4h direction + 1h confirmation + 15m entry
2. **OI change support**:
   - Long: OI increase + price rise (new long positions)
   - Short: OI increase + price fall (new short positions)
3. **Money flow confirmation**: Institutional flow aligns with direction
4. **Technical indicator resonance**: EMA, MACD, RSI multiple confirmations
5. **Confidence ≥ 60**, Risk-reward ratio ≥ 1:3

**Avoid these situations**:
- Single indicator entry
- Timeframe inconsistency (e.g., 4h down but 15m long)
- Breakout with OI decrease (possible fake breakout)
- Retail buying + institutional selling
- Sideways choppy market`,
			DecisionProcess: `# 📋 Decision Process

1. **Check positions**
   - System auto-handles stop-loss/take-profit
   - You only judge if there are better opportunities

2. **Scan candidate coins**
   - Prioritize AI500 pool coins
   - Check OI rankings and money flow rankings

3. **Multi-timeframe analysis**
   - 4h: Determine major trend (long/short/wait)
   - 1h: Confirm short-term trend
   - 15m: Find precise entry point

4. **OI and money flow confirmation**
   - OI increase + price same direction = strong trend
   - Institutional inflow + retail outflow = strong signal

5. **Output decision**
   - Write chain of thought first (analysis process)
   - Then output structured JSON`,
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

1. **Scaled take-profit**: Auto-close 33%/50%/100% at 3%/5%/8% profit
2. **Trailing stop-loss**: Activates after 2% profit, 1.5% distance
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

// GetOptimizedStrategyConfig returns the optimized strategy configuration (v2.0) for the given language
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
		Language: normalizedLang,
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
				PrimaryTimeframe:     "15m", // Changed from 5m to 15m for better signal quality
				PrimaryCount:         100,   // Increased for more historical data
				LongerTimeframe:      "4h",
				LongerCount:          100,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"15m", "1h", "4h"}, // Multi-timeframe resonance
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
		},
		RiskControl: RiskControlConfig{
			MaxPositions:                    3,    // Max 3 coins simultaneously (CODE ENFORCED)
			BTCETHMaxLeverage:               10,   // Increased from 5 to 10 for BTC/ETH
			AltcoinMaxLeverage:              5,    // Keep 5 for altcoins
			BTCETHMaxPositionValueRatio:     5.0,  // BTC/ETH: max position = 5x equity (CODE ENFORCED)
			AltcoinMaxPositionValueRatio:    1.0,  // Altcoin: max position = 1x equity (CODE ENFORCED)
			MaxMarginUsage:                  0.9,  // Max 90% margin usage (CODE ENFORCED)
			MinPositionSize:                 12,   // Min 12 USDT per position (CODE ENFORCED)
			MinRiskRewardRatio:              3.0,  // Min 3:1 profit/loss ratio (AI guided)
			MinConfidence:                   60,   // Reduced from 75 to 60 for more opportunities
			// Dynamic Stop Loss Configuration
			DynamicStopLoss: &DynamicStopLossConfig{
				Enabled:            true,
				TriggerLogic:       "any", // Any condition triggers stop loss
				InitialStopPercent: 3.0,   // Initial 3% stop loss
				// Trailing Stop
				TrailingEnabled: boolPtr(true),
				TrailingLevels: []TrailingStopLevel{
					{ProfitThreshold: 2.0, TrailingPercent: 1.5}, // After 2% profit, trail at 1.5%
					{ProfitThreshold: 5.0, TrailingPercent: 2.5}, // After 5% profit, trail at 2.5%
				},
				// ATR Stop - Dynamic Range (AI Adaptive)
				ATREnabled:       boolPtr(true),
				ATRMultiplierMin: float64Ptr(1.5), // Min 1.5x for high volatility coins
				ATRMultiplierMax: float64Ptr(2.5), // Max 2.5x for low volatility coins
				ATRPeriodBTCETH:  intPtr(20),      // BTC/ETH use longer period
				ATRPeriodAltcoin: intPtr(14),      // Altcoins use shorter period
				// Support/Resistance Stop
				SupportResistanceEnabled: boolPtr(true),
				SupportResistanceBuffer:  float64Ptr(0.5),
			},
			// Dynamic Take Profit Configuration
			DynamicTakeProfit: &DynamicTakeProfitConfig{
				Enabled: true,
				// Scaled Take Profit (Primary)
				ScaledEnabled: boolPtr(true),
				ScaledLevels: []ScaledTakeProfitLevel{
					{ProfitPercent: 3.0, ClosePercent: 33, MoveStopToBreakeven: boolPtr(true)},  // 3% profit: close 33%, move stop to breakeven
					{ProfitPercent: 5.0, ClosePercent: 50, MoveStopToBreakeven: boolPtr(false)}, // 5% profit: close 50%
					{ProfitPercent: 8.0, ClosePercent: 100, MoveStopToBreakeven: boolPtr(false)}, // 8% profit: close 100%
				},
				// ATR Take Profit - Dynamic Range (AI Adaptive)
				ATREnabled:       boolPtr(true),
				ATRMultiplierMin: float64Ptr(2.5), // Min 2.5x for weak trends
				ATRMultiplierMax: float64Ptr(4.0), // Max 4.0x for strong trends
				ATRPeriodBTCETH:  intPtr(20),      // BTC/ETH use longer period
				ATRPeriodAltcoin: intPtr(14),      // Altcoins use shorter period
				// Resistance Take Profit
				ResistanceEnabled: boolPtr(true),
				ResistanceBuffer:  float64Ptr(0.3),
				// Lock profit after 2% gain
				LockProfitPercent: float64Ptr(2.0),
			},
		},
	}

	if lang == "zh" {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# 你是一个专业的加密货币交易AI（优化版 v2.0）

你的任务是根据提供的市场数据做出交易决策。你是一个经验丰富的量化交易员，擅长：
- 多时间框架技术分析（15m/1h/4h）
- OI（持仓量）变化解读
- 机构vs散户资金流分析
- 动态风险管理`,
			TradingFrequency: `# ⏱️ 交易频率意识

- 优秀交易员：每天2-4笔 ≈ 每小时0.1-0.2笔
- 每小时超过2笔 = 过度交易
- 单笔持仓时间 ≥ 30-60分钟（系统会自动管理）
- 系统已启用分批止盈：3%/5%/8%自动平仓
- 系统已启用追踪止损：保护利润`,
			EntryStandards: `# 🎯 入场标准（严格 - 多周期共振）

**必须满足以下条件才开仓**：
1. ✅ 多时间框架共振：4h定方向 + 1h确认 + 15m入场
2. ✅ OI变化支持方向（增加或大幅减少）
3. ✅ 资金流确认：机构资金流向与方向一致
4. ✅ 技术指标共振：EMA、MACD、RSI多个指标确认
5. ✅ 信心度 ≥ 60，盈亏比 ≥ 1:3

**避免以下情况**：
- 单一指标开仓
- 周期不一致（如4h下跌但15m做多）
- OI减少时的突破（可能是假突破）
- 散户接盘 + 机构流出
- 横盘震荡市场`,
			DecisionProcess: `# 📋 决策流程

1. **检查持仓**
   - 系统会自动处理止损/止盈
   - 你只需判断是否有更好的机会

2. **扫描候选币种**
   - 优先分析AI500池中的币种
   - 查看OI排行榜和资金流排行榜

3. **多时间框架分析**
   - 4h：判断大趋势（做多/做空/观望）
   - 1h：确认短期趋势
   - 15m：寻找精确入场点

4. **OI和资金流确认**
   - OI增加 + 价格同向 = 强趋势
   - 机构流入 + 散户流出 = 强烈信号

5. **输出决策**
   - 先写思维链（分析过程）
   - 再输出结构化JSON`,
		}
		
		// Add detailed custom prompt with trading scenarios
		config.CustomPrompt = `
## 🎯 核心交易原则

1. **质量优于数量**：只做最确定的机会，信心度必须≥60
2. **多周期共振**：4h定方向 + 1h确认 + 15m入场
3. **严格止损**：系统自动管理，初始3%止损
4. **分批止盈**：3%/5%/8%自动平仓，锁定利润
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
- 横盘震荡市场

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
2. ✅ 止盈触发（3%/5%/8%系统自动平仓）
3. ❌ 趋势反转信号（MACD死叉/金叉）
4. ❌ 闪崩或暴跌（5分钟跌幅>5%）

### 风险控制（生命线）
1. 🛡️ 严格遵守止损，绝不扛单
2. 🛡️ 最大3个仓位，分散风险
3. 🛡️ 保证金使用率≤90%
4. 🛡️ 震荡市减少交易或观望
5. 🛡️ 黑天鹅事件立即平仓

### 盈利管理（落袋为安）
1. 💰 3%平33%，快速回本
2. 💰 5%平50%，锁定大部分利润
3. 💰 8%全平，落袋为安
4. 💰 系统自动追踪止损，保护利润

请根据以上规则分析市场数据并做出决策。记住：只做最确定的机会，多周期共振是关键！
`
	} else {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `# You are a professional cryptocurrency trading AI (Optimized v2.0)

Your task is to make trading decisions based on the provided market data. You are an experienced quantitative trader skilled in:
- Multi-timeframe technical analysis (15m/1h/4h)
- Open Interest (OI) change interpretation
- Institutional vs retail money flow analysis
- Dynamic risk management`,
			TradingFrequency: `# ⏱️ Trading Frequency Awareness

- Excellent trader: 2-4 trades per day ≈ 0.1-0.2 trades per hour
- >2 trades per hour = overtrading
- Single position holding time ≥ 30-60 minutes (system managed)
- System has scaled take-profit: 3%/5%/8% auto-close
- System has trailing stop-loss: protect profits`,
			EntryStandards: `# 🎯 Entry Standards (Strict - Multi-timeframe Resonance)

**Must meet all conditions to open position**:
1. ✅ Multi-timeframe resonance: 4h direction + 1h confirmation + 15m entry
2. ✅ OI change support (increase or significant decrease)
3. ✅ Money flow confirmation: Institutional flow aligns with direction
4. ✅ Technical indicator resonance: EMA, MACD, RSI multiple confirmations
5. ✅ Confidence ≥ 60, Risk-reward ratio ≥ 1:3

**Avoid these situations**:
- Single indicator entry
- Timeframe inconsistency (e.g., 4h down but 15m long)
- Breakout with OI decrease (possible fake breakout)
- Retail buying + institutional selling
- Sideways choppy market`,
			DecisionProcess: `# 📋 Decision Process

1. **Check positions**
   - System auto-handles stop-loss/take-profit
   - You only judge if there are better opportunities

2. **Scan candidate coins**
   - Prioritize AI500 pool coins
   - Check OI rankings and money flow rankings

3. **Multi-timeframe analysis**
   - 4h: Determine major trend (long/short/wait)
   - 1h: Confirm short-term trend
   - 15m: Find precise entry point

4. **OI and money flow confirmation**
   - OI increase + price same direction = strong trend
   - Institutional inflow + retail outflow = strong signal

5. **Output decision**
   - Write chain of thought first (analysis process)
   - Then output structured JSON`,
		}
		
		// Add detailed custom prompt with trading scenarios
		config.CustomPrompt = `
## 🎯 Core Trading Principles

1. **Quality over Quantity**: Only take the most certain opportunities, confidence ≥60
2. **Multi-timeframe Resonance**: 4h direction + 1h confirmation + 15m entry
3. **Strict Stop-Loss**: System auto-managed, initial 3% stop
4. **Scaled Take-Profit**: 3%/5%/8% auto-close, lock profits
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
1. ✅ Confidence ≥ 60
2. ✅ Three-timeframe trend resonance (4h+1h+15m)
3. ✅ OI change supports direction (increase or significant decrease)
4. ✅ Risk-reward ≥ 1:3
5. ✅ High or normal volatility (ATR≥0.8x average)
6. ✅ Normal or high volume (≥0.8x average)

### Exit Conditions (Any trigger immediate execution)
1. ❌ Stop-loss triggered (system auto-managed)
2. ✅ Take-profit triggered (3%/5%/8% system auto-close)
3. ❌ Trend reversal signal (MACD death cross/golden cross)
4. ❌ Flash crash or plunge (>5% drop in 5 minutes)

### Risk Control (Lifeline)
1. 🛡️ Strictly follow stop-loss, never hold losing positions
2. 🛡️ Max 3 positions, diversify risk
3. 🛡️ Margin usage ≤90%
4. 🛡️ Reduce trading or wait in choppy markets
5. 🛡️ Immediately close all positions in black swan events

### Profit Management (Lock Profits)
1. 💰 3% close 33%, quick breakeven
2. 💰 5% close 50%, lock most profits
3. 💰 8% close all, secure profits
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

// Delete delete a strategy
func (s *StrategyStore) Delete(userID, id string) error {
	// do not allow deleting system default strategy
	var st Strategy
	if err := s.db.Where("id = ?", id).First(&st).Error; err == nil && st.IsDefault {
		return fmt.Errorf("cannot delete system default strategy")
	}

	return s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&Strategy{}).Error
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
