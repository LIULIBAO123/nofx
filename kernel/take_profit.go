package kernel

import (
	"fmt"
	"strings"

	"nofx/logger"
	"nofx/store"
)

// TakeProfitChecker checks if take profit conditions are met
type TakeProfitChecker struct {
	config *store.DynamicTakeProfitConfig
}

// NewTakeProfitChecker creates a new take profit checker
func NewTakeProfitChecker(config *store.DynamicTakeProfitConfig) *TakeProfitChecker {
	if config == nil || !config.Enabled {
		return nil
	}
	return &TakeProfitChecker{config: config}
}

// TakeProfitSignal represents a take profit trigger signal
type TakeProfitSignal struct {
	Triggered      bool
	Reason         string
	Price          float64
	Type           string  // "fixed", "partial", "atr", "resistance"
	PartialPercent float64 // percentage to close (0-100), 100 means full close
	// ScaledProfitPercentUsed records the profit% threshold unit used for "scaled" dedup.
	// When scaled_profit_percent_mode="roe", this should store ROE% (level.ProfitPercent).
	// When mode="price", this should store price% (level.ProfitPercent).
	ScaledProfitPercentUsed float64
}

// CheckTakeProfit checks if any take profit condition is triggered for a position.
// highestPrice: for long = highest price since entry; for short = lowest price since entry (peak profit level).
// atrLong: longer-period ATR (e.g. 28) for high-vol detection; when ATRUseMaxInHighVolatility and atr > atrLong*threshold, use max multiplier for ATR TP.
func (c *TakeProfitChecker) CheckTakeProfit(
	position *PositionInfo,
	currentPrice float64,
	highestPrice float64, // peak price since entry (long=high, short=low)
	atr float64, // current ATR value
	atrLong float64, // longer ATR for high-vol (0 = disable)
	resistanceLevel float64, // resistance level (0 if not available)
) *TakeProfitSignal {
	if c == nil || c.config == nil {
		return &TakeProfitSignal{Triggered: false}
	}

	// Min profit filter: do not trigger any TP if current price-based profit is below threshold
	if c.config.MinProfitPercentToAllowTP != nil && *c.config.MinProfitPercentToAllowTP > 0 {
		var profitPct float64
		if position.Side == "long" && position.EntryPrice > 0 {
			profitPct = (currentPrice - position.EntryPrice) / position.EntryPrice * 100
		} else if position.Side == "short" && position.EntryPrice > 0 {
			profitPct = (position.EntryPrice - currentPrice) / position.EntryPrice * 100
		}
		if profitPct < *c.config.MinProfitPercentToAllowTP {
			return &TakeProfitSignal{Triggered: false}
		}
	}

	signals := make([]*TakeProfitSignal, 0)

	// 1. Check scaled take profit first so partial closes can trigger before fixed (when both would fire at same level)
	scaledEnabled := (c.config.ScaledEnabled != nil && *c.config.ScaledEnabled) || (len(c.config.ScaledLevels) > 0 && (c.config.ScaledEnabled == nil || *c.config.ScaledEnabled))
	if scaledEnabled {
		if signal := c.checkScaledTakeProfit(position, currentPrice); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 2. Trailing / pullback take profit: lock profit before retrace; resist oscillation via ATR
	if c.config.TrailingTPEnabled != nil && *c.config.TrailingTPEnabled {
		if signal := c.checkTrailingTakeProfit(position, currentPrice, highestPrice, atr); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 3. Check fixed take profit levels
	if c.config.FixedEnabled != nil && *c.config.FixedEnabled && c.config.FixedPercent != nil {
		if signal := c.checkFixedTakeProfit(position, currentPrice); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 4. Check ATR-based take profit
	if c.config.ATREnabled != nil && *c.config.ATREnabled && atr > 0 {
		if signal := c.checkATRTakeProfit(position, currentPrice, atr, atrLong); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 5. Check resistance-based take profit
	if c.config.ResistanceEnabled != nil && *c.config.ResistanceEnabled && resistanceLevel > 0 {
		if signal := c.checkResistanceTakeProfit(position, currentPrice, resistanceLevel); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// Apply trigger logic
	if len(signals) == 0 {
		return &TakeProfitSignal{Triggered: false}
	}

	// Default: "any" logic - any condition triggers = take profit
	// Return first triggered signal
	return signals[0]
}

// checkTrailingTakeProfit triggers when profit has reached activate threshold and price retraces from peak.
// Uses max(fixed retrace%, ATR-based retrace%) to resist oscillation. Altcoin can use different activate/retrace %.
func (c *TakeProfitChecker) checkTrailingTakeProfit(position *PositionInfo, currentPrice, highestPrice, atr float64) *TakeProfitSignal {
	activatePct := getFloat64Value(c.config.TrailingTPActivateProfitPct, 2.0)
	retracePct := getFloat64Value(c.config.TrailingTPRetracePct, 1.5)
	if !isBtcEth(position.Symbol) {
		if c.config.TrailingTPActivateProfitPctAltcoin != nil && *c.config.TrailingTPActivateProfitPctAltcoin > 0 {
			activatePct = *c.config.TrailingTPActivateProfitPctAltcoin
		}
		if c.config.TrailingTPRetracePctAltcoin != nil && *c.config.TrailingTPRetracePctAltcoin > 0 {
			retracePct = *c.config.TrailingTPRetracePctAltcoin
		}
	}
	atrMult := getFloat64Value(c.config.TrailingTPRetraceATRMult, 0.5)
	closePct := getFloat64Value(c.config.TrailingTPClosePercent, 100)
	if closePct <= 0 {
		closePct = 100
	}

	entryPrice := position.EntryPrice
	var peakProfitPct float64
	var retraceFromPeakPct float64
	if position.Side == "long" {
		if highestPrice <= entryPrice {
			return &TakeProfitSignal{Triggered: false}
		}
		peakProfitPct = ((highestPrice - entryPrice) / entryPrice) * 100
		if peakProfitPct < activatePct {
			return &TakeProfitSignal{Triggered: false}
		}
		retraceFromPeakPct = ((highestPrice - currentPrice) / highestPrice) * 100
		if currentPrice >= highestPrice {
			return &TakeProfitSignal{Triggered: false}
		}
	} else {
		if highestPrice >= entryPrice {
			return &TakeProfitSignal{Triggered: false}
		}
		peakProfitPct = ((entryPrice - highestPrice) / entryPrice) * 100
		if peakProfitPct < activatePct {
			return &TakeProfitSignal{Triggered: false}
		}
		retraceFromPeakPct = ((currentPrice - highestPrice) / highestPrice) * 100
		if currentPrice <= highestPrice {
			return &TakeProfitSignal{Triggered: false}
		}
	}

	// Required retrace: max(fixed%, ATR-based%) to resist oscillation
	requiredRetrace := retracePct
	if atr > 0 && currentPrice > 0 {
		atrRetracePct := (atr / currentPrice) * 100 * atrMult
		if atrRetracePct > requiredRetrace {
			requiredRetrace = atrRetracePct
		}
	}
	if retraceFromPeakPct < requiredRetrace {
		return &TakeProfitSignal{Triggered: false}
	}

	return &TakeProfitSignal{
		Triggered:      true,
		Reason:         fmt.Sprintf("Trailing TP: peak profit %.2f%%, retrace %.2f%% (required %.2f%%)", peakProfitPct, retraceFromPeakPct, requiredRetrace),
		Price:          currentPrice,
		Type:           "trailing_tp",
		PartialPercent: closePct,
	}
}

// checkFixedTakeProfit checks fixed take profit level
func (c *TakeProfitChecker) checkFixedTakeProfit(position *PositionInfo, currentPrice float64) *TakeProfitSignal {
	if c.config.FixedPercent == nil {
		return &TakeProfitSignal{Triggered: false}
	}

	entryPrice := position.EntryPrice
	profitPercent := *c.config.FixedPercent

	var targetPrice float64
	if position.Side == "long" {
		targetPrice = entryPrice * (1 + profitPercent/100)
		if currentPrice >= targetPrice {
			return &TakeProfitSignal{
				Triggered:      true,
				Reason:         fmt.Sprintf("Fixed take profit reached: %.2f%% profit", profitPercent),
				Price:          targetPrice,
				Type:           "fixed",
				PartialPercent: 100,
			}
		}
	} else { // short
		targetPrice = entryPrice * (1 - profitPercent/100)
		if currentPrice <= targetPrice {
			return &TakeProfitSignal{
				Triggered:      true,
				Reason:         fmt.Sprintf("Fixed take profit reached: %.2f%% profit", profitPercent),
				Price:          targetPrice,
				Type:           "fixed",
				PartialPercent: 100,
			}
		}
	}

	return &TakeProfitSignal{Triggered: false}
}

// checkScaledTakeProfit checks scaled take profit levels
func (c *TakeProfitChecker) checkScaledTakeProfit(position *PositionInfo, currentPrice float64) *TakeProfitSignal {
	if len(c.config.ScaledLevels) == 0 {
		return &TakeProfitSignal{Triggered: false}
	}

	entryPrice := position.EntryPrice
	firstLevel := c.config.ScaledLevels[0]
	mode := strings.TrimSpace(strings.ToLower(c.config.ScaledProfitPercentMode))
	if mode == "" {
		mode = "price"
	}

	// Check each level in order
	for _, level := range c.config.ScaledLevels {
		// Skip if already taken
		if c.isLevelTaken(position, level.ProfitPercent, "scaled") {
			logger.Infof("📋 Scaled TP %s: level %.2f%% already taken, skip", position.Symbol, level.ProfitPercent)
			continue
		}

		var targetPrice float64

		if position.Side == "long" {
			targetPrice = entryPrice * (1 + level.ProfitPercent/100)
			priceProfitPct := 0.0
			if entryPrice > 0 {
				priceProfitPct = (currentPrice - entryPrice) / entryPrice * 100
			}
			roeProfitPct := priceProfitPct
			lev := position.Leverage
			if lev <= 0 {
				lev = 1
			}
			roeProfitPct = priceProfitPct * float64(lev)
			ok := false
			if mode == "roe" {
				ok = roeProfitPct >= level.ProfitPercent
			} else {
				ok = currentPrice >= targetPrice
			}
			if ok {
				return &TakeProfitSignal{
					Triggered:      true,
					Reason:         fmt.Sprintf("Scaled take profit reached (%s): %.2f%% threshold, closing %.2f%% (price%%=%.2f, roe%%=%.2f, lev=%d)", mode, level.ProfitPercent, level.ClosePercent, priceProfitPct, roeProfitPct, lev),
					Price:          currentPrice,
					Type:           "scaled",
					PartialPercent: level.ClosePercent,
					ScaledProfitPercentUsed: level.ProfitPercent,
				}
			}
		} else { // short
			targetPrice = entryPrice * (1 - level.ProfitPercent/100)
			priceProfitPct := 0.0
			if entryPrice > 0 {
				priceProfitPct = (entryPrice - currentPrice) / entryPrice * 100
			}
			roeProfitPct := priceProfitPct
			lev := position.Leverage
			if lev <= 0 {
				lev = 1
			}
			roeProfitPct = priceProfitPct * float64(lev)
			ok := false
			if mode == "roe" {
				ok = roeProfitPct >= level.ProfitPercent
			} else {
				ok = currentPrice <= targetPrice
			}
			if ok {
				return &TakeProfitSignal{
					Triggered:      true,
					Reason:         fmt.Sprintf("Scaled take profit reached (%s): %.2f%% threshold, closing %.2f%% (price%%=%.2f, roe%%=%.2f, lev=%d)", mode, level.ProfitPercent, level.ClosePercent, priceProfitPct, roeProfitPct, lev),
					Price:          currentPrice,
					Type:           "scaled",
					PartialPercent: level.ClosePercent,
					ScaledProfitPercentUsed: level.ProfitPercent,
				}
			}
		}
	}

	// 盈利已超过第一档却未触发时打日志，便于排查分层止盈未激活
	var currentProfitPct float64
	if entryPrice > 0 {
		if position.Side == "long" {
			currentProfitPct = (currentPrice - entryPrice) / entryPrice * 100
		} else {
			currentProfitPct = (entryPrice - currentPrice) / entryPrice * 100
		}
	}
	lev := position.Leverage
	if lev <= 0 {
		lev = 1
	}
	currentROE := currentProfitPct * float64(lev)
	compareVal := currentProfitPct
	if mode == "roe" {
		compareVal = currentROE
	}
	if compareVal >= firstLevel.ProfitPercent {
		targetFirst := entryPrice * (1 + firstLevel.ProfitPercent/100)
		if position.Side == "short" {
			targetFirst = entryPrice * (1 - firstLevel.ProfitPercent/100)
		}
		logger.Infof("📋 Scaled TP %s: profit %s %.2f%% >= first level %.2f%% but no trigger (price%%=%.2f roe%%=%.2f lev=%d target=%.4f current=%.4f side=%s)",
			position.Symbol, mode, compareVal, firstLevel.ProfitPercent, currentProfitPct, currentROE, lev, targetFirst, currentPrice, position.Side)
	}

	return &TakeProfitSignal{Triggered: false}
}

// checkATRTakeProfit checks ATR-based dynamic take profit.
// atrLong: when ATRUseMaxInHighVolatility is set and atr >= atrLong*threshold, use max multiplier to tolerate volatility.
func (c *TakeProfitChecker) checkATRTakeProfit(position *PositionInfo, currentPrice float64, atr, atrLong float64) *TakeProfitSignal {
	entryPrice := position.EntryPrice

	multiplierMin := getFloat64Value(c.config.ATRMultiplierMin, 2.0)
	multiplierMax := getFloat64Value(c.config.ATRMultiplierMax, 4.0)
	multiplier := (multiplierMin + multiplierMax) / 2
	if c.config.ATRUseMaxInHighVolatility != nil && *c.config.ATRUseMaxInHighVolatility && atrLong > 0 {
		th := getFloat64Value(c.config.ATRHighVolatilityThreshold, 1.2)
		if atr >= atrLong*th {
			multiplier = multiplierMax
		}
	}

	var targetPrice float64
	if position.Side == "long" {
		targetPrice = entryPrice + (atr * multiplier)
		if currentPrice >= targetPrice {
			profitPct := ((currentPrice - entryPrice) / entryPrice) * 100
			return &TakeProfitSignal{
				Triggered:      true,
				Reason:         fmt.Sprintf("ATR take profit triggered: %.2fx ATR above entry (%.2f%% profit)", multiplier, profitPct),
				Price:          targetPrice,
				Type:           "atr",
				PartialPercent: 100,
			}
		}
	} else { // short
		targetPrice = entryPrice - (atr * multiplier)
		if currentPrice <= targetPrice {
			profitPct := ((entryPrice - currentPrice) / entryPrice) * 100
			return &TakeProfitSignal{
				Triggered:      true,
				Reason:         fmt.Sprintf("ATR take profit triggered: %.2fx ATR below entry (%.2f%% profit)", multiplier, profitPct),
				Price:          targetPrice,
				Type:           "atr",
				PartialPercent: 100,
			}
		}
	}

	return &TakeProfitSignal{Triggered: false}
}

// checkResistanceTakeProfit checks resistance/support-based take profit
// Caller should pass: resistanceLevel for long positions, supportLevel for short positions
func (c *TakeProfitChecker) checkResistanceTakeProfit(position *PositionInfo, currentPrice float64, srLevel float64) *TakeProfitSignal {
	buffer := getFloat64Value(c.config.ResistanceBuffer, 0.5)

	var targetPrice float64
	if position.Side == "long" {
		// For long positions, take profit near resistance level
		// IMPORTANT: resistance must be ABOVE entry price, otherwise it's not a valid take profit target
		if srLevel <= position.EntryPrice {
			return &TakeProfitSignal{Triggered: false}
		}
		targetPrice = srLevel * (1 - buffer/100)
		// Also verify we're actually in profit
		if currentPrice >= targetPrice && currentPrice > position.EntryPrice {
			profitPct := ((currentPrice - position.EntryPrice) / position.EntryPrice) * 100
			return &TakeProfitSignal{
				Triggered:      true,
				Reason:         fmt.Sprintf("Resistance level reached: price at %.2f (resistance: %.2f, %.2f%% profit)", currentPrice, srLevel, profitPct),
				Price:          targetPrice,
				Type:           "resistance",
				PartialPercent: 100,
			}
		}
	} else { // short
		// For short positions, take profit near support level
		// IMPORTANT: support must be BELOW entry price, otherwise it's not a valid take profit target
		if srLevel >= position.EntryPrice {
			return &TakeProfitSignal{Triggered: false}
		}
		targetPrice = srLevel * (1 + buffer/100)
		// Also verify we're actually in profit
		if currentPrice <= targetPrice && currentPrice < position.EntryPrice {
			profitPct := ((position.EntryPrice - currentPrice) / position.EntryPrice) * 100
			return &TakeProfitSignal{
				Triggered:      true,
				Reason:         fmt.Sprintf("Support level reached: price at %.2f (support: %.2f, %.2f%% profit)", currentPrice, srLevel, profitPct),
				Price:          targetPrice,
				Type:           "resistance",
				PartialPercent: 100,
			}
		}
	}

	return &TakeProfitSignal{Triggered: false}
}

// isLevelTaken checks if a specific profit level has already been taken
// This prevents triggering the same level multiple times (e.g. backtest passes ScaledLevelsTaken).
func (c *TakeProfitChecker) isLevelTaken(position *PositionInfo, profitPercent float64, levelType string) bool {
	if position == nil || len(position.ScaledLevelsTaken) == 0 {
		return false
	}
	const tol = 0.15
	for _, taken := range position.ScaledLevelsTaken {
		if taken >= profitPercent-tol && taken <= profitPercent+tol {
			return true
		}
	}
	return false
}

// countEnabledConditions counts how many take profit conditions are enabled
func (c *TakeProfitChecker) countEnabledConditions() int {
	count := 0

	if c.config.FixedEnabled != nil && *c.config.FixedEnabled {
		count++
	}
	if c.config.ScaledEnabled != nil && *c.config.ScaledEnabled {
		count++
	}
	if c.config.ATREnabled != nil && *c.config.ATREnabled {
		count++
	}
	if c.config.ResistanceEnabled != nil && *c.config.ResistanceEnabled {
		count++
	}

	return count
}

func isBtcEth(symbol string) bool {
	sym := strings.ToUpper(symbol)
	return strings.Contains(sym, "BTC") || strings.Contains(sym, "ETH")
}

// LogTakeProfitCheck logs take profit check result
func LogTakeProfitCheck(symbol string, signal *TakeProfitSignal) {
	if signal.Triggered {
		if signal.PartialPercent < 100 {
			logger.Infof("💰 Partial Take Profit Triggered: %s - %s (price: %.4f, closing %.1f%%)", 
				symbol, signal.Reason, signal.Price, signal.PartialPercent)
		} else {
			logger.Infof("💰 Take Profit Triggered: %s - %s (price: %.4f)", 
				symbol, signal.Reason, signal.Price)
		}
	}
}

