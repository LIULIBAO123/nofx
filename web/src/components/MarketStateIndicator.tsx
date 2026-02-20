import { TrendingUp, TrendingDown, Activity, Volume2, Zap } from 'lucide-react'

type TrendState = 'strong_uptrend' | 'uptrend' | 'sideways' | 'downtrend' | 'strong_downtrend'
type VolatilityState = 'extreme' | 'high' | 'normal' | 'low'
type VolumeState = 'surge' | 'high' | 'normal' | 'low'

interface MarketStateIndicatorProps {
    trend: TrendState
    volatility: VolatilityState
    volume: VolumeState
    symbol?: string
    compact?: boolean
}

export function MarketStateIndicator({
    trend,
    volatility,
    volume,
    symbol,
    compact = false,
}: MarketStateIndicatorProps) {
    const getTrendConfig = (trend: TrendState) => {
        switch (trend) {
            case 'strong_uptrend':
                return {
                    label: '强上升趋势',
                    icon: <TrendingUp className="w-5 h-5" />,
                    color: 'text-nofx-success',
                    bgColor: 'bg-nofx-success/10',
                    borderColor: 'border-nofx-success/30',
                    emoji: '🚀',
                }
            case 'uptrend':
                return {
                    label: '上升趋势',
                    icon: <TrendingUp className="w-5 h-5" />,
                    color: 'text-green-400',
                    bgColor: 'bg-green-400/10',
                    borderColor: 'border-green-400/30',
                    emoji: '📈',
                }
            case 'sideways':
                return {
                    label: '横盘震荡',
                    icon: <Activity className="w-5 h-5" />,
                    color: 'text-yellow-400',
                    bgColor: 'bg-yellow-400/10',
                    borderColor: 'border-yellow-400/30',
                    emoji: '↔️',
                }
            case 'downtrend':
                return {
                    label: '下降趋势',
                    icon: <TrendingDown className="w-5 h-5" />,
                    color: 'text-orange-400',
                    bgColor: 'bg-orange-400/10',
                    borderColor: 'border-orange-400/30',
                    emoji: '📉',
                }
            case 'strong_downtrend':
                return {
                    label: '强下降趋势',
                    icon: <TrendingDown className="w-5 h-5" />,
                    color: 'text-nofx-danger',
                    bgColor: 'bg-nofx-danger/10',
                    borderColor: 'border-nofx-danger/30',
                    emoji: '💥',
                }
        }
    }

    const getVolatilityConfig = (volatility: VolatilityState) => {
        switch (volatility) {
            case 'extreme':
                return {
                    label: '极端波动',
                    color: 'text-purple-400',
                    bgColor: 'bg-purple-400/10',
                    borderColor: 'border-purple-400/30',
                    emoji: '⚡',
                }
            case 'high':
                return {
                    label: '高波动',
                    color: 'text-orange-400',
                    bgColor: 'bg-orange-400/10',
                    borderColor: 'border-orange-400/30',
                    emoji: '🔥',
                }
            case 'normal':
                return {
                    label: '正常波动',
                    color: 'text-blue-400',
                    bgColor: 'bg-blue-400/10',
                    borderColor: 'border-blue-400/30',
                    emoji: '🌊',
                }
            case 'low':
                return {
                    label: '低波动',
                    color: 'text-gray-400',
                    bgColor: 'bg-gray-400/10',
                    borderColor: 'border-gray-400/30',
                    emoji: '😴',
                }
        }
    }

    const getVolumeConfig = (volume: VolumeState) => {
        switch (volume) {
            case 'surge':
                return {
                    label: '成交激增',
                    color: 'text-nofx-gold',
                    bgColor: 'bg-nofx-gold/10',
                    borderColor: 'border-nofx-gold/30',
                    emoji: '💰',
                }
            case 'high':
                return {
                    label: '高成交',
                    color: 'text-green-400',
                    bgColor: 'bg-green-400/10',
                    borderColor: 'border-green-400/30',
                    emoji: '📊',
                }
            case 'normal':
                return {
                    label: '正常成交',
                    color: 'text-blue-400',
                    bgColor: 'bg-blue-400/10',
                    borderColor: 'border-blue-400/30',
                    emoji: '📈',
                }
            case 'low':
                return {
                    label: '低成交',
                    color: 'text-gray-400',
                    bgColor: 'bg-gray-400/10',
                    borderColor: 'border-gray-400/30',
                    emoji: '💤',
                }
        }
    }

    const trendConfig = getTrendConfig(trend)
    const volatilityConfig = getVolatilityConfig(volatility)
    const volumeConfig = getVolumeConfig(volume)

    if (compact) {
        return (
            <div className="flex items-center gap-2">
                <div
                    className={`
                        flex items-center gap-1.5 px-2 py-1 rounded-md border text-xs font-bold
                        ${trendConfig.bgColor} ${trendConfig.borderColor} ${trendConfig.color}
                    `}
                >
                    <span>{trendConfig.emoji}</span>
                    <span>{trendConfig.label}</span>
                </div>
                <div
                    className={`
                        flex items-center gap-1.5 px-2 py-1 rounded-md border text-xs font-bold
                        ${volatilityConfig.bgColor} ${volatilityConfig.borderColor} ${volatilityConfig.color}
                    `}
                >
                    <span>{volatilityConfig.emoji}</span>
                    <span>{volatilityConfig.label}</span>
                </div>
                <div
                    className={`
                        flex items-center gap-1.5 px-2 py-1 rounded-md border text-xs font-bold
                        ${volumeConfig.bgColor} ${volumeConfig.borderColor} ${volumeConfig.color}
                    `}
                >
                    <span>{volumeConfig.emoji}</span>
                    <span>{volumeConfig.label}</span>
                </div>
            </div>
        )
    }

    return (
        <div className="space-y-4">
            {/* Header */}
            {symbol && (
                <div className="flex items-center gap-2 mb-4">
                    <Zap className="w-5 h-5 text-nofx-gold" />
                    <h3 className="text-lg font-bold text-nofx-text-main">
                        市场状态
                        <span className="ml-2 text-nofx-gold font-mono">{symbol}</span>
                    </h3>
                </div>
            )}

            {/* Indicators Grid */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                {/* Trend */}
                <div
                    className={`
                        relative overflow-hidden rounded-lg border p-4
                        transition-all duration-300 hover:scale-[1.02]
                        ${trendConfig.bgColor} ${trendConfig.borderColor}
                    `}
                >
                    <div className="absolute inset-0 opacity-5">
                        <div className="absolute inset-0 bg-gradient-radial from-current to-transparent" />
                    </div>

                    <div className="relative z-10">
                        <div className="flex items-center gap-2 mb-2">
                            <div className={trendConfig.color}>{trendConfig.icon}</div>
                            <span className="text-xs font-medium text-nofx-text-muted uppercase tracking-wider">
                                趋势
                            </span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-2xl">{trendConfig.emoji}</span>
                            <span className={`text-lg font-bold ${trendConfig.color}`}>
                                {trendConfig.label}
                            </span>
                        </div>
                    </div>
                </div>

                {/* Volatility */}
                <div
                    className={`
                        relative overflow-hidden rounded-lg border p-4
                        transition-all duration-300 hover:scale-[1.02]
                        ${volatilityConfig.bgColor} ${volatilityConfig.borderColor}
                    `}
                >
                    <div className="absolute inset-0 opacity-5">
                        <div className="absolute inset-0 bg-gradient-radial from-current to-transparent" />
                    </div>

                    <div className="relative z-10">
                        <div className="flex items-center gap-2 mb-2">
                            <Activity className={`w-5 h-5 ${volatilityConfig.color}`} />
                            <span className="text-xs font-medium text-nofx-text-muted uppercase tracking-wider">
                                波动率
                            </span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-2xl">{volatilityConfig.emoji}</span>
                            <span className={`text-lg font-bold ${volatilityConfig.color}`}>
                                {volatilityConfig.label}
                            </span>
                        </div>
                    </div>
                </div>

                {/* Volume */}
                <div
                    className={`
                        relative overflow-hidden rounded-lg border p-4
                        transition-all duration-300 hover:scale-[1.02]
                        ${volumeConfig.bgColor} ${volumeConfig.borderColor}
                    `}
                >
                    <div className="absolute inset-0 opacity-5">
                        <div className="absolute inset-0 bg-gradient-radial from-current to-transparent" />
                    </div>

                    <div className="relative z-10">
                        <div className="flex items-center gap-2 mb-2">
                            <Volume2 className={`w-5 h-5 ${volumeConfig.color}`} />
                            <span className="text-xs font-medium text-nofx-text-muted uppercase tracking-wider">
                                成交量
                            </span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-2xl">{volumeConfig.emoji}</span>
                            <span className={`text-lg font-bold ${volumeConfig.color}`}>
                                {volumeConfig.label}
                            </span>
                        </div>
                    </div>
                </div>
            </div>

            {/* Market Summary */}
            <div className="rounded-lg border border-nofx-gold/20 bg-nofx-gold/5 p-4">
                <div className="text-sm text-nofx-text-muted mb-2">市场综合评估</div>
                <div className="text-base text-nofx-text-main">
                    {getMarketSummary(trend, volatility, volume)}
                </div>
            </div>
        </div>
    )
}

// Generate market summary based on states
function getMarketSummary(
    trend: TrendState,
    volatility: VolatilityState,
    volume: VolumeState
): string {
    // Strong uptrend scenarios
    if (trend === 'strong_uptrend') {
        if (volume === 'surge' && volatility === 'high') {
            return '🚀 强势突破行情，成交量激增，建议顺势做多'
        }
        if (volume === 'low') {
            return '⚠️ 上涨但成交量不足，警惕假突破'
        }
        return '📈 强势上涨趋势，可考虑做多'
    }

    // Strong downtrend scenarios
    if (trend === 'strong_downtrend') {
        if (volume === 'surge') {
            return '💥 恐慌性下跌，成交量激增，建议观望或做空'
        }
        return '📉 强势下跌趋势，建议观望'
    }

    // Sideways scenarios
    if (trend === 'sideways') {
        if (volatility === 'high') {
            return '⚡ 横盘震荡但波动剧烈，适合短线交易'
        }
        if (volume === 'low') {
            return '😴 横盘整理，成交清淡，建议观望'
        }
        return '↔️ 横盘震荡，等待方向明确'
    }

    // Default
    return '📊 市场处于观察期，等待更明确信号'
}

