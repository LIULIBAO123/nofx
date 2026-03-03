package kernel

import (
	"fmt"
	"math"
	"strings"

	"nofx/logger"
	"nofx/market"
	"nofx/store"
)

// StopLossChecker checks if stop loss conditions are met
type StopLossChecker struct {
	config *store.DynamicStopLossConfig
}

// NewStopLossChecker creates a new stop loss checker
func NewStopLossChecker(config *store.DynamicStopLossConfig) *StopLossChecker {
	if config == nil || !config.Enabled {
		return nil
	}
	return &StopLossChecker{config: config}
}

// StopLossSignal represents a stop loss trigger signal
type StopLossSignal struct {
	Triggered bool
	Reason    string
	Price     float64
	Type      string // "initial", "trailing", "atr", "support_resistance"

	// Trailing only: which tier triggered and key percentages (for close_reason and UI)
	TrailingTier         int     // 1-based level index (0 = not trailing)
	TrailingPeakPct      float64 // peak profit % at trigger (e.g. 3.5)
	TrailingRetracePct   float64 // allowed retracement % for this tier (e.g. 2.5)
}

// CheckStopLoss checks if any stop loss condition is triggered for a position.
// atrLong: longer-period ATR (e.g. 28) for high-vol detection; when atr > atrLong*threshold we use wider ATR stop.
func (c *StopLossChecker) CheckStopLoss(
	position *PositionInfo,
	currentPrice float64,
	highestPrice float64, // highest price since entry (for trailing stop)
	atr float64, // current ATR value (e.g. 14)
	atrLong float64, // longer-period ATR for volatility regime (0 = disable tolerance)
	supportLevel float64, // support level (0 if not available)
) *StopLossSignal {
	if c == nil || c.config == nil {
		return &StopLossSignal{Triggered: false}
	}

	signals := make([]*StopLossSignal, 0)

	// 1. Check initial fixed stop loss only when configured (InitialStopPercent > 0). Removed as default; use dynamic/ATR/trailing only when 0.
	if c.config.InitialStopPercent > 0 {
		if signal := c.checkInitialStop(position, currentPrice); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 2. Check trailing stop (if enabled)
	if c.config.TrailingEnabled != nil && *c.config.TrailingEnabled {
		if signal := c.checkTrailingStop(position, currentPrice, highestPrice); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 3. Check ATR stop (if enabled); pass atrLong for high-vol tolerance
	if c.config.ATREnabled != nil && *c.config.ATREnabled && atr > 0 {
		if signal := c.checkATRStop(position, currentPrice, atr, atrLong); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 4. Check support/resistance stop (if enabled)
	if c.config.SupportResistanceEnabled != nil && *c.config.SupportResistanceEnabled && supportLevel > 0 {
		if signal := c.checkSupportResistanceStop(position, currentPrice, supportLevel); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 5. Adverse exit when never in profit: if never had浮盈 and price moved against by >= X×ATR, treat as wrong direction and exit early (does not tighten normal ATR stop)
	if atr > 0 {
		if signal := c.checkAdverseExitWhenNeverProfit(position, currentPrice, highestPrice, atr, atrLong); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// Apply trigger logic
	if len(signals) == 0 {
		return &StopLossSignal{Triggered: false}
	}

	// "any" logic: any condition triggers = close position
	if c.config.TriggerLogic == "any" {
		return signals[0] // return first triggered signal
	}

	// "all" logic: all enabled conditions must trigger
	enabledCount := c.countEnabledConditions()
	if len(signals) >= enabledCount {
		// All enabled conditions triggered
		return &StopLossSignal{
			Triggered: true,
			Reason:    "All stop loss conditions triggered",
			Price:     currentPrice,
			Type:      "combined",
		}
	}

	return &StopLossSignal{Triggered: false}
}

// checkInitialStop checks initial fixed stop loss
func (c *StopLossChecker) checkInitialStop(position *PositionInfo, currentPrice float64) *StopLossSignal {
	entryPrice := position.EntryPrice
	stopPercent := c.config.InitialStopPercent

	var stopPrice float64
	if position.Side == "long" {
		stopPrice = entryPrice * (1 - stopPercent/100)
		if currentPrice <= stopPrice {
			return &StopLossSignal{
				Triggered: true,
				Reason:    fmt.Sprintf("Initial stop loss triggered: %.2f%% loss", stopPercent),
				Price:     stopPrice,
				Type:      "initial",
			}
		}
	} else { // short
		stopPrice = entryPrice * (1 + stopPercent/100)
		if currentPrice >= stopPrice {
			return &StopLossSignal{
				Triggered: true,
				Reason:    fmt.Sprintf("Initial stop loss triggered: %.2f%% loss", stopPercent),
				Price:     stopPrice,
				Type:      "initial",
			}
		}
	}

	return &StopLossSignal{Triggered: false}
}

// checkTrailingStop checks trailing stop loss (tiered mode)
func (c *StopLossChecker) checkTrailingStop(position *PositionInfo, currentPrice float64, highestPrice float64) *StopLossSignal {
	if len(c.config.TrailingLevels) == 0 {
		return &StopLossSignal{Triggered: false}
	}

	entryPrice := position.EntryPrice
	
	// Calculate current profit percentage
	var profitPct float64
	if position.Side == "long" {
		if highestPrice <= entryPrice {
			return &StopLossSignal{Triggered: false} // no profit yet
		}
		profitPct = ((highestPrice - entryPrice) / entryPrice) * 100
	} else { // short
		if highestPrice >= entryPrice {
			return &StopLossSignal{Triggered: false} // no profit yet
		}
		profitPct = ((entryPrice - highestPrice) / entryPrice) * 100
	}

	// Find the active trailing level (highest profit threshold reached) and tier index
	var activeLevel *store.TrailingStopLevel
	tierIndex := 0
	for i := range c.config.TrailingLevels {
		level := &c.config.TrailingLevels[i]
		if profitPct >= level.ProfitThreshold {
			activeLevel = level
			tierIndex = i + 1 // 1-based for display (L1, L2, ...)
		}
	}

	if activeLevel == nil {
		return &StopLossSignal{Triggered: false} // no level activated yet
	}

	// Check if price has retraced beyond trailing percent
	var stopPrice float64
	if position.Side == "long" {
		stopPrice = highestPrice * (1 - activeLevel.TrailingPercent/100)
		if currentPrice <= stopPrice {
			return &StopLossSignal{
				Triggered:            true,
				Reason:               fmt.Sprintf("Trailing stop triggered: %.2f%% profit, %.2f%% retracement", profitPct, activeLevel.TrailingPercent),
				Price:                stopPrice,
				Type:                 "trailing",
				TrailingTier:         tierIndex,
				TrailingPeakPct:      profitPct,
				TrailingRetracePct:   activeLevel.TrailingPercent,
			}
		}
	} else { // short
		stopPrice = highestPrice * (1 + activeLevel.TrailingPercent/100)
		if currentPrice >= stopPrice {
			return &StopLossSignal{
				Triggered:            true,
				Reason:               fmt.Sprintf("Trailing stop triggered: %.2f%% profit, %.2f%% retracement", profitPct, activeLevel.TrailingPercent),
				Price:                stopPrice,
				Type:                 "trailing",
				TrailingTier:         tierIndex,
				TrailingPeakPct:      profitPct,
				TrailingRetracePct:   activeLevel.TrailingPercent,
			}
		}
	}

	return &StopLossSignal{Triggered: false}
}

// checkATRStop checks ATR-based dynamic stop loss.
// When atrLong > 0 and atr > atrLong*ATRHighMultiplier (high volatility), use ATRMultiplierMax for wider stop (more tolerant).
func (c *StopLossChecker) checkATRStop(position *PositionInfo, currentPrice float64, atr, atrLong float64) *StopLossSignal {
	entryPrice := position.EntryPrice

	multiplierMin := getFloat64Value(c.config.ATRMultiplierMin, 1.5)
	multiplierMax := getFloat64Value(c.config.ATRMultiplierMax, 3.5)
	multiplier := (multiplierMin + multiplierMax) / 2

	// High volatility: use max multiplier so stop is further away (more tolerant)
	if atrLong > 0 && c.config.ATRToleranceEnabled != nil && *c.config.ATRToleranceEnabled {
		highMult := getFloat64Value(c.config.ATRHighMultiplier, 1.2)
		if atr > atrLong*highMult {
			multiplier = multiplierMax
		}
	}

	var stopPrice float64
	if position.Side == "long" {
		stopPrice = entryPrice - (atr * multiplier)
		if currentPrice <= stopPrice {
			return &StopLossSignal{
				Triggered: true,
				Reason:    fmt.Sprintf("ATR stop loss triggered: %.2fx ATR below entry", multiplier),
				Price:     stopPrice,
				Type:      "atr",
			}
		}
	} else { // short
		stopPrice = entryPrice + (atr * multiplier)
		if currentPrice >= stopPrice {
			return &StopLossSignal{
				Triggered: true,
				Reason:    fmt.Sprintf("ATR stop loss triggered: %.2fx ATR above entry", multiplier),
				Price:     stopPrice,
				Type:      "atr",
			}
		}
	}

	return &StopLossSignal{Triggered: false}
}

// checkSupportResistanceStop checks support/resistance based stop loss
// Caller should pass: supportLevel for long positions, resistanceLevel for short positions
func (c *StopLossChecker) checkSupportResistanceStop(position *PositionInfo, currentPrice float64, srLevel float64) *StopLossSignal {
	buffer := getFloat64Value(c.config.SupportResistanceBuffer, 0.5)
	
	var stopPrice float64
	if position.Side == "long" {
		// For long positions, stop below support level
		stopPrice = srLevel * (1 - buffer/100)
		if currentPrice <= stopPrice {
			return &StopLossSignal{
				Triggered: true,
				Reason:    fmt.Sprintf("Support level broken: price below %.2f (support: %.2f)", stopPrice, srLevel),
				Price:     stopPrice,
				Type:      "support_resistance",
			}
		}
	} else { // short
		// For short positions, stop above resistance level
		stopPrice = srLevel * (1 + buffer/100)
		if currentPrice >= stopPrice {
			return &StopLossSignal{
				Triggered: true,
				Reason:    fmt.Sprintf("Resistance level broken: price above %.2f (resistance: %.2f)", stopPrice, srLevel),
				Price:     stopPrice,
				Type:      "support_resistance",
			}
		}
	}

	return &StopLossSignal{Triggered: false}
}

// checkAdverseExitWhenNeverProfit triggers when position has never been in profit and price moved against by >= config ATR multiple (reduces loss on wrong-direction opens without tightening normal stop).
// When AdverseExitRequireATRSpike is true, also requires atr >= atrLong*threshold so we only exit on strong moves, not mild chop.
func (c *StopLossChecker) checkAdverseExitWhenNeverProfit(position *PositionInfo, currentPrice, highestPrice, atr, atrLong float64) *StopLossSignal {
	mult := c.getAdverseExitMultiplier(position.Symbol)
	if mult <= 0 {
		return &StopLossSignal{Triggered: false}
	}
	if c.config.AdverseExitRequireATRSpike != nil && *c.config.AdverseExitRequireATRSpike && atrLong > 0 {
		th := getFloat64Value(c.config.AdverseExitATRSpikeThreshold, 1.2)
		if atr < atrLong*th {
			return &StopLossSignal{Triggered: false} // mild volatility, skip adverse exit
		}
	}
	entryPrice := position.EntryPrice
	var neverInProfit bool
	var adverseDist float64
	if position.Side == "long" {
		neverInProfit = highestPrice <= entryPrice
		adverseDist = entryPrice - currentPrice
	} else {
		neverInProfit = highestPrice >= entryPrice
		adverseDist = currentPrice - entryPrice
	}
	if !neverInProfit || adverseDist <= 0 {
		return &StopLossSignal{Triggered: false}
	}
	threshold := atr * mult
	if adverseDist >= threshold {
		return &StopLossSignal{
			Triggered: true,
			Reason:    fmt.Sprintf("Adverse exit (never in profit, reverse move %.2fx ATR >= %.2f)", adverseDist/atr, mult),
			Price:     currentPrice,
			Type:      "adverse_never_profit",
		}
	}
	return &StopLossSignal{Triggered: false}
}

func (c *StopLossChecker) getAdverseExitMultiplier(symbol string) float64 {
	sym := strings.ToUpper(symbol)
	isBtcEth := strings.Contains(sym, "BTC") || strings.Contains(sym, "ETH")
	if !isBtcEth && c.config.AdverseExitWhenNeverProfitATRAltcoin != nil && *c.config.AdverseExitWhenNeverProfitATRAltcoin > 0 {
		return *c.config.AdverseExitWhenNeverProfitATRAltcoin
	}
	return getFloat64Value(c.config.AdverseExitWhenNeverProfitATR, 0)
}

// countEnabledConditions counts how many stop loss conditions are enabled
func (c *StopLossChecker) countEnabledConditions() int {
	count := 0
	if c.config.InitialStopPercent > 0 {
		count++
	}
	if c.config.TrailingEnabled != nil && *c.config.TrailingEnabled {
		count++
	}
	if c.config.ATREnabled != nil && *c.config.ATREnabled {
		count++
	}
	if c.config.SupportResistanceEnabled != nil && *c.config.SupportResistanceEnabled {
		count++
	}
	if getFloat64Value(c.config.AdverseExitWhenNeverProfitATR, 0) > 0 {
		count++
	}
	return count
}

// CalculateATR calculates ATR for a symbol
func CalculateATR(klines []market.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}

	// Calculate True Range for each bar
	trs := make([]float64, 0, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		trs = append(trs, tr)
	}

	if len(trs) < period {
		return 0
	}

	// Calculate initial ATR (simple average of first 'period' TRs)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Calculate smoothed ATR using Wilder's smoothing
	for i := period; i < len(trs); i++ {
		atr = ((atr * float64(period-1)) + trs[i]) / float64(period)
	}

	return atr
}

// CalculateEMA returns the latest EMA(period) value from klines (closes). Returns 0 if not enough bars.
func CalculateEMA(klines []market.Kline, period int) float64 {
	if len(klines) < period || period <= 0 {
		return 0
	}
	alpha := 2.0 / float64(period+1)
	ema := float64(klines[0].Close)
	for i := 1; i < len(klines); i++ {
		ema = alpha*float64(klines[i].Close) + (1-alpha)*ema
	}
	return ema
}

// FindSupportLevel finds the nearest support level from recent price action
func FindSupportLevel(klines []market.Kline, currentPrice float64, lookback int) float64 {
	if len(klines) < lookback {
		lookback = len(klines)
	}

	if lookback < 3 {
		return 0
	}

	// Get recent klines
	recentKlines := klines[len(klines)-lookback:]
	
	// Find local lows (potential support levels)
	supports := make([]float64, 0)
	for i := 1; i < len(recentKlines)-1; i++ {
		if recentKlines[i].Low < recentKlines[i-1].Low && recentKlines[i].Low < recentKlines[i+1].Low {
			supports = append(supports, recentKlines[i].Low)
		}
	}

	if len(supports) == 0 {
		// No clear support, use lowest low
		lowest := recentKlines[0].Low
		for _, k := range recentKlines {
			if k.Low < lowest {
				lowest = k.Low
			}
		}
		return lowest
	}

	// Find the nearest support below current price
	nearestSupport := 0.0
	minDistance := math.MaxFloat64
	for _, support := range supports {
		if support < currentPrice {
			distance := currentPrice - support
			if distance < minDistance {
				minDistance = distance
				nearestSupport = support
			}
		}
	}

	if nearestSupport == 0 {
		// All supports are above current price, use the lowest one
		nearestSupport = supports[0]
		for _, s := range supports {
			if s < nearestSupport {
				nearestSupport = s
			}
		}
	}

	return nearestSupport
}

// FindResistanceLevel finds the nearest resistance level from recent price action
func FindResistanceLevel(klines []market.Kline, currentPrice float64, lookback int) float64 {
	if len(klines) < lookback {
		lookback = len(klines)
	}

	if lookback < 3 {
		return 0
	}

	// Get recent klines
	recentKlines := klines[len(klines)-lookback:]
	
	// Find local highs (potential resistance levels)
	resistances := make([]float64, 0)
	for i := 1; i < len(recentKlines)-1; i++ {
		if recentKlines[i].High > recentKlines[i-1].High && recentKlines[i].High > recentKlines[i+1].High {
			resistances = append(resistances, recentKlines[i].High)
		}
	}

	if len(resistances) == 0 {
		// No clear resistance, use highest high
		highest := recentKlines[0].High
		for _, k := range recentKlines {
			if k.High > highest {
				highest = k.High
			}
		}
		return highest
	}

	// Find the nearest resistance above current price
	nearestResistance := math.MaxFloat64
	minDistance := math.MaxFloat64
	for _, resistance := range resistances {
		if resistance > currentPrice {
			distance := resistance - currentPrice
			if distance < minDistance {
				minDistance = distance
				nearestResistance = resistance
			}
		}
	}

	if nearestResistance == math.MaxFloat64 {
		// All resistances are below current price, use the highest one
		nearestResistance = resistances[0]
		for _, r := range resistances {
			if r > nearestResistance {
				nearestResistance = r
			}
		}
	}

	return nearestResistance
}

// getFloat64Value safely gets float64 value from pointer with default
func getFloat64Value(ptr *float64, defaultValue float64) float64 {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// getIntValue safely gets int value from pointer with default
func getIntValue(ptr *int, defaultValue int) int {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

// LogStopLossCheck logs stop loss check result
func LogStopLossCheck(symbol string, signal *StopLossSignal) {
	if signal.Triggered {
		logger.Infof("🛑 Stop Loss Triggered: %s - %s (price: %.4f)", symbol, signal.Reason, signal.Price)
	}
}



