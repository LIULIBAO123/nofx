import { AlertTriangle, TrendingUp, Shield, Activity } from 'lucide-react'

interface RiskMetric {
    label: string
    value: string | number
    status: 'safe' | 'warning' | 'danger'
    threshold?: number
    icon?: React.ReactNode
    description?: string
}

interface RiskDashboardProps {
    currentDrawdown: number
    marginUsage: number
    positionCorrelation?: number
    openPositions: number
    maxPositions: number
    dailyPnL?: number
}

export function RiskDashboard({
    currentDrawdown,
    marginUsage,
    positionCorrelation = 0,
    openPositions,
    maxPositions,
    dailyPnL = 0,
}: RiskDashboardProps) {
    const getDrawdownStatus = (dd: number): 'safe' | 'warning' | 'danger' => {
        if (dd < 5) return 'safe'
        if (dd < 10) return 'warning'
        return 'danger'
    }

    const getMarginStatus = (margin: number): 'safe' | 'warning' | 'danger' => {
        if (margin < 50) return 'safe'
        if (margin < 70) return 'warning'
        return 'danger'
    }

    const getCorrelationStatus = (corr: number): 'safe' | 'warning' | 'danger' => {
        if (corr < 0.5) return 'safe'
        if (corr < 0.7) return 'warning'
        return 'danger'
    }

    const metrics: RiskMetric[] = [
        {
            label: '当前回撤',
            value: `${currentDrawdown.toFixed(2)}%`,
            status: getDrawdownStatus(currentDrawdown),
            threshold: 10,
            icon: <TrendingUp className="w-5 h-5" />,
            description: '账户从峰值的最大回撤',
        },
        {
            label: '保证金使用率',
            value: `${marginUsage.toFixed(1)}%`,
            status: getMarginStatus(marginUsage),
            threshold: 70,
            icon: <Shield className="w-5 h-5" />,
            description: '已使用保证金占总权益的比例',
        },
        {
            label: '持仓相关性',
            value: positionCorrelation.toFixed(2),
            status: getCorrelationStatus(positionCorrelation),
            threshold: 0.7,
            icon: <Activity className="w-5 h-5" />,
            description: '持仓币种之间的相关性',
        },
        {
            label: '持仓数量',
            value: `${openPositions}/${maxPositions}`,
            status: openPositions >= maxPositions ? 'warning' : 'safe',
            icon: <AlertTriangle className="w-5 h-5" />,
            description: '当前持仓数量/最大持仓限制',
        },
    ]

    const getStatusColor = (status: 'safe' | 'warning' | 'danger') => {
        switch (status) {
            case 'safe':
                return 'text-nofx-success border-nofx-success/30 bg-nofx-success/10'
            case 'warning':
                return 'text-yellow-400 border-yellow-400/30 bg-yellow-400/10'
            case 'danger':
                return 'text-nofx-danger border-nofx-danger/30 bg-nofx-danger/10'
        }
    }

    const getStatusBadge = (status: 'safe' | 'warning' | 'danger') => {
        switch (status) {
            case 'safe':
                return '✓ 安全'
            case 'warning':
                return '⚠ 警告'
            case 'danger':
                return '✕ 危险'
        }
    }

    return (
        <div className="space-y-4">
            {/* Header */}
            <div className="flex items-center justify-between">
                <h3 className="text-lg font-bold text-nofx-text-main flex items-center gap-2">
                    <Shield className="w-5 h-5 text-nofx-gold" />
                    风险仪表盘
                </h3>
                {dailyPnL !== 0 && (
                    <div
                        className={`text-sm font-mono font-bold ${
                            dailyPnL > 0 ? 'text-nofx-success' : 'text-nofx-danger'
                        }`}
                    >
                        今日盈亏: {dailyPnL > 0 ? '+' : ''}
                        {dailyPnL.toFixed(2)}%
                    </div>
                )}
            </div>

            {/* Metrics Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {metrics.map((metric, index) => (
                    <div
                        key={index}
                        className={`
                            relative overflow-hidden rounded-lg border p-4
                            transition-all duration-300 hover:scale-[1.02]
                            ${getStatusColor(metric.status)}
                        `}
                    >
                        {/* Background Glow Effect */}
                        <div className="absolute inset-0 opacity-5">
                            <div className="absolute inset-0 bg-gradient-radial from-current to-transparent" />
                        </div>

                        {/* Content */}
                        <div className="relative z-10">
                            <div className="flex items-start justify-between mb-2">
                                <div className="flex items-center gap-2">
                                    {metric.icon}
                                    <span className="text-sm font-medium opacity-80">
                                        {metric.label}
                                    </span>
                                </div>
                                <span className="text-xs px-2 py-0.5 rounded-full border border-current/30 bg-current/10">
                                    {getStatusBadge(metric.status)}
                                </span>
                            </div>

                            <div className="flex items-end justify-between">
                                <div className="text-2xl font-bold font-mono">
                                    {metric.value}
                                </div>
                                {metric.threshold && (
                                    <div className="text-xs opacity-60">
                                        阈值: {metric.threshold}
                                        {typeof metric.value === 'string' && metric.value.includes('%')
                                            ? '%'
                                            : ''}
                                    </div>
                                )}
                            </div>

                            {metric.description && (
                                <div className="mt-2 text-xs opacity-70 border-t border-current/20 pt-2">
                                    {metric.description}
                                </div>
                            )}
                        </div>

                        {/* Status Indicator Bar */}
                        <div className="absolute bottom-0 left-0 right-0 h-1 bg-current/20">
                            <div
                                className="h-full bg-current transition-all duration-500"
                                style={{
                                    width:
                                        metric.label === '当前回撤'
                                            ? `${Math.min((currentDrawdown / 20) * 100, 100)}%`
                                            : metric.label === '保证金使用率'
                                            ? `${Math.min(marginUsage, 100)}%`
                                            : metric.label === '持仓相关性'
                                            ? `${positionCorrelation * 100}%`
                                            : `${(openPositions / maxPositions) * 100}%`,
                                }}
                            />
                        </div>
                    </div>
                ))}
            </div>

            {/* Overall Risk Score */}
            <div className="rounded-lg border border-nofx-gold/20 bg-nofx-gold/5 p-4">
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="w-12 h-12 rounded-full border-2 border-nofx-gold flex items-center justify-center">
                            <Shield className="w-6 h-6 text-nofx-gold" />
                        </div>
                        <div>
                            <div className="text-sm text-nofx-text-muted">综合风险评分</div>
                            <div className="text-xl font-bold text-nofx-gold font-mono">
                                {calculateRiskScore(metrics)}/100
                            </div>
                        </div>
                    </div>
                    <div className="text-right">
                        <div className="text-sm text-nofx-text-muted">风险等级</div>
                        <div
                            className={`text-lg font-bold ${
                                calculateRiskScore(metrics) >= 80
                                    ? 'text-nofx-success'
                                    : calculateRiskScore(metrics) >= 60
                                    ? 'text-yellow-400'
                                    : 'text-nofx-danger'
                            }`}
                        >
                            {calculateRiskScore(metrics) >= 80
                                ? '低风险'
                                : calculateRiskScore(metrics) >= 60
                                ? '中风险'
                                : '高风险'}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}

// Calculate overall risk score (0-100, higher is safer)
function calculateRiskScore(metrics: RiskMetric[]): number {
    let score = 100

    metrics.forEach((metric) => {
        if (metric.status === 'warning') score -= 10
        if (metric.status === 'danger') score -= 25
    })

    return Math.max(0, score)
}

