package kernel

import (
	"math"
	"strings"

	"nofx/logger"
	"nofx/store"
)

// EffectiveOpenConstraints 根据 market_regime 与极端资金费率/多空比计算本周期开仓的有效最低置信度及是否禁止开多/开空。
// 用于 AI 开仓与系统开仓前的统一校验。
func EffectiveOpenConstraints(
	ctx *Context,
	symbol string,
	marketRegime string,
	baseMinConf int,
	regimeEnabled bool,
	regimeMap map[string]int,
	extreme *store.ExtremeFundingRule,
) (effectiveMinConf int, blockLong bool, blockShort bool) {
	effectiveMinConf = baseMinConf
	if baseMinConf <= 0 {
		effectiveMinConf = 70
	}

	// 1. Regime 调节：震荡/高波/反转时提高置信度要求
	if regimeEnabled && len(regimeMap) > 0 && marketRegime != "" {
		regimeKey := strings.TrimSpace(strings.ToLower(marketRegime))
		if override, ok := regimeMap[regimeKey]; ok && override > 0 && override > effectiveMinConf {
			effectiveMinConf = override
			logger.Infof("📋 [Constraints] regime=%s → min_confidence override %d", regimeKey, override)
		}
	}

	// 2. 极端资金费率 / 多空比
	if extreme != nil && extreme.Enabled {
		excessiveLongs := false
		excessiveShorts := false

		// 2a. 资金费率：正 = 多头付钱给空头（多头过热），负 = 空头过热
		var funding float64
		if ctx.BinanceFundingRateAvg8h != nil {
			if v, ok := ctx.BinanceFundingRateAvg8h[symbol]; ok {
				funding = v
			}
		}
		if ctx.BinanceFundingMap != nil {
			if f, ok := ctx.BinanceFundingMap[symbol]; ok {
				funding = f.LastFundingRate
			}
		}
		thresh := extreme.FundingThresholdPct
		if thresh <= 0 {
			thresh = 0.001 // 0.1%
		}
		if math.Abs(funding) >= thresh {
			if funding > 0 {
				excessiveLongs = true
			} else {
				excessiveShorts = true
			}
		}

		// 2b. 多空比：> High 多头过热，< Low 空头过热
		if ctx.BinanceLongShortMap != nil {
			if ls, ok := ctx.BinanceLongShortMap[symbol]; ok {
				high := extreme.LongShortRatioHigh
				low := extreme.LongShortRatioLow
				if high <= 0 {
					high = 1.4
				}
				if low <= 0 {
					low = 1.0 / high // 0.714
				}
				ratio := ls.LongShortRatio
				if ratio >= high {
					excessiveLongs = true
				}
				if ratio > 0 && ratio <= low {
					excessiveShorts = true
				}
			}
		}

		if excessiveLongs {
			if extreme.BlockOpenLongWhenExcessiveLongs {
				blockLong = true
				logger.Infof("📋 [Constraints] symbol=%s excessive longs → block open_long", symbol)
			} else {
				raised := baseMinConf + extreme.RaiseConfidenceBy
				if extreme.MinConfidenceWhenExtreme > 0 && extreme.MinConfidenceWhenExtreme > raised {
					raised = extreme.MinConfidenceWhenExtreme
				}
				if raised > effectiveMinConf {
					effectiveMinConf = raised
					logger.Infof("📋 [Constraints] symbol=%s excessive longs → min_confidence raised to %d", symbol, effectiveMinConf)
				}
			}
		}
		if excessiveShorts {
			if extreme.BlockOpenShortWhenExcessiveShorts {
				blockShort = true
				logger.Infof("📋 [Constraints] symbol=%s excessive shorts → block open_short", symbol)
			} else {
				raised := baseMinConf + extreme.RaiseConfidenceBy
				if extreme.MinConfidenceWhenExtreme > 0 && extreme.MinConfidenceWhenExtreme > raised {
					raised = extreme.MinConfidenceWhenExtreme
				}
				if raised > effectiveMinConf {
					effectiveMinConf = raised
					logger.Infof("📋 [Constraints] symbol=%s excessive shorts → min_confidence raised to %d", symbol, effectiveMinConf)
				}
			}
		}
	}

	return effectiveMinConf, blockLong, blockShort
}
