import { CheckCircle, Circle, Clock, TrendingUp } from 'lucide-react'

interface DecisionStep {
    id: string
    label: string
    status: 'completed' | 'active' | 'pending' | 'error'
    detail?: string
    duration?: number
}

interface DecisionFlowProps {
    steps: DecisionStep[]
    symbol?: string
    overallStatus?: 'processing' | 'completed' | 'error'
}

export function DecisionFlow({ steps, symbol, overallStatus = 'processing' }: DecisionFlowProps) {
    const getStepIcon = (status: DecisionStep['status']) => {
        switch (status) {
            case 'completed':
                return <CheckCircle className="w-5 h-5 text-nofx-success" />
            case 'active':
                return (
                    <div className="w-5 h-5 rounded-full border-2 border-nofx-gold animate-pulse">
                        <div className="w-full h-full rounded-full bg-nofx-gold/50 animate-ping" />
                    </div>
                )
            case 'error':
                return (
                    <div className="w-5 h-5 rounded-full border-2 border-nofx-danger flex items-center justify-center">
                        <span className="text-nofx-danger text-xs">✕</span>
                    </div>
                )
            case 'pending':
                return <Circle className="w-5 h-5 text-nofx-text-muted/30" />
        }
    }

    const getStepColor = (status: DecisionStep['status']) => {
        switch (status) {
            case 'completed':
                return 'text-nofx-success border-nofx-success/30 bg-nofx-success/5'
            case 'active':
                return 'text-nofx-gold border-nofx-gold/50 bg-nofx-gold/10'
            case 'error':
                return 'text-nofx-danger border-nofx-danger/30 bg-nofx-danger/5'
            case 'pending':
                return 'text-nofx-text-muted border-nofx-text-muted/20 bg-nofx-text-muted/5'
        }
    }

    return (
        <div className="space-y-4">
            {/* Header */}
            {symbol && (
                <div className="flex items-center justify-between mb-4">
                    <div className="flex items-center gap-3">
                        <TrendingUp className="w-5 h-5 text-nofx-gold" />
                        <h3 className="text-lg font-bold text-nofx-text-main">
                            AI 决策流程
                            {symbol && (
                                <span className="ml-2 text-nofx-gold font-mono">{symbol}</span>
                            )}
                        </h3>
                    </div>
                    {overallStatus === 'processing' && (
                        <div className="flex items-center gap-2 text-sm text-nofx-text-muted">
                            <Clock className="w-4 h-4 animate-spin" />
                            <span>处理中...</span>
                        </div>
                    )}
                </div>
            )}

            {/* Steps */}
            <div className="relative">
                {/* Vertical Line */}
                <div className="absolute left-[18px] top-8 bottom-8 w-0.5 bg-gradient-to-b from-nofx-gold/50 via-nofx-gold/20 to-transparent" />

                {/* Steps List */}
                <div className="space-y-3">
                    {steps.map((step, index) => (
                        <div
                            key={step.id}
                            className={`
                                relative flex items-start gap-4 p-4 rounded-lg border
                                transition-all duration-300
                                ${getStepColor(step.status)}
                                ${step.status === 'active' ? 'scale-[1.02] shadow-lg' : ''}
                            `}
                        >
                            {/* Icon */}
                            <div className="relative z-10 flex-shrink-0 mt-0.5">
                                {getStepIcon(step.status)}
                            </div>

                            {/* Content */}
                            <div className="flex-1 min-w-0">
                                <div className="flex items-center justify-between gap-2">
                                    <div className="flex items-center gap-2">
                                        <span className="text-xs font-mono text-nofx-text-muted">
                                            {String(index + 1).padStart(2, '0')}
                                        </span>
                                        <span className="font-bold text-sm">{step.label}</span>
                                    </div>
                                    {step.duration && step.status === 'completed' && (
                                        <span className="text-xs text-nofx-text-muted font-mono">
                                            {step.duration}ms
                                        </span>
                                    )}
                                </div>

                                {step.detail && (
                                    <div className="mt-1 text-sm text-nofx-text-muted">
                                        {step.detail}
                                    </div>
                                )}

                                {/* Active Step Progress Bar */}
                                {step.status === 'active' && (
                                    <div className="mt-2 h-1 bg-black/30 rounded-full overflow-hidden">
                                        <div className="h-full bg-nofx-gold/50 animate-shimmer" />
                                    </div>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            </div>

            {/* Summary */}
            {overallStatus === 'completed' && (
                <div className="mt-4 p-4 rounded-lg border border-nofx-success/30 bg-nofx-success/5">
                    <div className="flex items-center gap-2 text-nofx-success">
                        <CheckCircle className="w-5 h-5" />
                        <span className="font-bold">决策完成</span>
                    </div>
                    <div className="mt-1 text-sm text-nofx-text-muted">
                        总耗时:{' '}
                        {steps
                            .filter((s) => s.duration)
                            .reduce((sum, s) => sum + (s.duration || 0), 0)}
                        ms
                    </div>
                </div>
            )}

            {overallStatus === 'error' && (
                <div className="mt-4 p-4 rounded-lg border border-nofx-danger/30 bg-nofx-danger/5">
                    <div className="flex items-center gap-2 text-nofx-danger">
                        <span className="text-lg">✕</span>
                        <span className="font-bold">决策失败</span>
                    </div>
                    <div className="mt-1 text-sm text-nofx-text-muted">
                        请检查错误信息并重试
                    </div>
                </div>
            )}
        </div>
    )
}

// Example usage component
export function DecisionFlowExample() {
    const exampleSteps: DecisionStep[] = [
        {
            id: '1',
            label: '数据收集',
            status: 'completed',
            detail: '获取 K线、OI、资金费率等数据',
            duration: 245,
        },
        {
            id: '2',
            label: '市场状态识别',
            status: 'completed',
            detail: '趋势: 强上升 | 波动率: 高 | 成交量: 激增',
            duration: 89,
        },
        {
            id: '3',
            label: '多时间框架分析',
            status: 'completed',
            detail: '4h/1h/15m 三周期共振确认',
            duration: 156,
        },
        {
            id: '4',
            label: 'AI 决策分析',
            status: 'active',
            detail: '正在分析技术指标和市场信号...',
        },
        {
            id: '5',
            label: '风控验证',
            status: 'pending',
        },
        {
            id: '6',
            label: '执行订单',
            status: 'pending',
        },
    ]

    return (
        <div className="max-w-2xl mx-auto p-6">
            <DecisionFlow steps={exampleSteps} symbol="BTCUSDT" overallStatus="processing" />
        </div>
    )
}

