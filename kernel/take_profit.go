package kernel

import (
	"fmt"
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
}

// CheckTakeProfit checks if any take profit condition is triggered for a position
func (c *TakeProfitChecker) CheckTakeProfit(
	position *PositionInfo,
	currentPrice float64,
	atr float64, // current ATR value
	resistanceLevel float64, // resistance level (0 if not available)
) *TakeProfitSignal {
	if c == nil || c.config == nil {
		return &TakeProfitSignal{Triggered: false}
	}

	signals := make([]*TakeProfitSignal, 0)

	// 1. Check fixed take profit levels
	if c.config.FixedEnabled != nil && *c.config.FixedEnabled && c.config.FixedPercent != nil {
		if signal := c.checkFixedTakeProfit(position, currentPrice); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 2. Check scaled take profit levels
	if c.config.ScaledEnabled != nil && *c.config.ScaledEnabled {
		if signal := c.checkScaledTakeProfit(position, currentPrice); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 3. Check ATR-based take profit
	if c.config.ATREnabled != nil && *c.config.ATREnabled && atr > 0 {
		if signal := c.checkATRTakeProfit(position, currentPrice, atr); signal.Triggered {
			signals = append(signals, signal)
		}
	}

	// 4. Check resistance-based take profit
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

			if currentPrice >= targetPrice {
				return &TakeProfitSignal{
					Triggered:      true,
					Reason:         fmt.Sprintf("Scaled take profit reached: %.2f%% profit, closing %.2f%% of position", level.ProfitPercent, level.ClosePercent),
					Price:          targetPrice,
					Type:           "scaled",
					PartialPercent: level.ClosePercent,
				}
			}
		} else { // short
			targetPrice = entryPrice * (1 - level.ProfitPercent/100)

			if currentPrice <= targetPrice {
				return &TakeProfitSignal{
					Triggered:      true,
					Reason:         fmt.Sprintf("Scaled take profit reached: %.2f%% profit, closing %.2f%% of position", level.ProfitPercent, level.ClosePercent),
					Price:          targetPrice,
					Type:           "scaled",
					PartialPercent: level.ClosePercent,
				}
			}
		}
	}

	// 盈利已超过第一档却未触发时打日志，便于排查分层止盈未激活
	var currentProfitPct float64
	if position.Side == "long" && entryPrice > 0 {
		currentProfitPct = (currentPrice - entryPrice) / entryPrice * 100
	} else if position.Side == "short" && entryPrice > 0 {
		currentProfitPct = (entryPrice - currentPrice) / entryPrice * 100
	}
	if currentProfitPct >= firstLevel.ProfitPercent {
		targetFirst := entryPrice * (1 + firstLevel.ProfitPercent/100)
		if position.Side == "short" {
			targetFirst = entryPrice * (1 - firstLevel.ProfitPercent/100)
		}
		logger.Infof("📋 Scaled TP %s: profit %.2f%% >= first level %.2f%% but no trigger (target=%.4f current=%.4f, side=%s)",
			position.Symbol, currentProfitPct, firstLevel.ProfitPercent, targetFirst, currentPrice, position.Side)
	}

	return &TakeProfitSignal{Triggered: false}
}

// checkATRTakeProfit checks ATR-based dynamic take profit
func (c *TakeProfitChecker) checkATRTakeProfit(position *PositionInfo, currentPrice float64, atr float64) *TakeProfitSignal {
	entryPrice := position.EntryPrice

	// Use mid-point of ATR multiplier range as default
	multiplierMin := getFloat64Value(c.config.ATRMultiplierMin, 2.0)
	multiplierMax := getFloat64Value(c.config.ATRMultiplierMax, 4.0)
	multiplier := (multiplierMin + multiplierMax) / 2

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

