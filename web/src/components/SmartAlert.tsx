import { TrendingUp, TrendingDown, AlertCircle, CheckCircle, Zap } from 'lucide-react'
import { useState } from 'react'

type AlertType = 'opportunity' | 'warning' | 'info' | 'success'

interface SmartAlertProps {
    type: AlertType
    symbol: string
    title: string
    signals: Array<{
        text: string
        status: 'positive' | 'negative' | 'neutral'
    }>
    confidence?: number
    action?: {
        label: string
        onClick: () => void
    }
    onDismiss?: () => void
}

export function SmartAlert({
    type,
    symbol,
    title,
    signals,
    confidence,
    action,
    onDismiss,
}: SmartAlertProps) {
    const [isExpanded, setIsExpanded] = useState(true)

    const getTypeConfig = (type: AlertType) => {
        switch (type) {
            case 'opportunity':
                return {
                    icon: <TrendingUp className="w-5 h-5" />,
                    color: 'text-nofx-success',
                    bgColor: 'bg-nofx-success/10',
                    borderColor: 'border-nofx-success/30',
                    glowColor: 'shadow-[0_0_20px_rgba(14,203,129,0.2)]',
                }
            case 'warning':
                return {
                    icon: <AlertCircle className="w-5 h-5" />,
                    color: 'text-yellow-400',
                    bgColor: 'bg-yellow-400/10',
                    borderColor: 'border-yellow-400/30',
                    glowColor: 'shadow-[0_0_20px_rgba(250,204,21,0.2)]',
                }
            case 'info':
                return {
                    icon: <Zap className="w-5 h-5" />,
                    color: 'text-nofx-accent',
                    bgColor: 'bg-nofx-accent/10',
                    borderColor: 'border-nofx-accent/30',
                    glowColor: 'shadow-[0_0_20px_rgba(0,240,255,0.2)]',
                }
            case 'success':
                return {
                    icon: <CheckCircle className="w-5 h-5" />,
                    color: 'text-nofx-gold',
                    bgColor: 'bg-nofx-gold/10',
                    borderColor: 'border-nofx-gold/30',
                    glowColor: 'shadow-[0_0_20px_rgba(240,185,11,0.2)]',
                }
        }
    }

    const config = getTypeConfig(type)

    const getSignalIcon = (status: 'positive' | 'negative' | 'neutral') => {
        switch (status) {
            case 'positive':
                return '✓'
            case 'negative':
                return '⚠️'
            case 'neutral':
                return '•'
        }
    }

    const getSignalColor = (status: 'positive' | 'negative' | 'neutral') => {
        switch (status) {
            case 'positive':
                return 'text-nofx-success'
            case 'negative':
                return 'text-nofx-danger'
            case 'neutral':
                return 'text-nofx-text-muted'
        }
    }

    if (!isExpanded) {
        return (
            <div
                className={`
                    flex items-center gap-3 px-4 py-2 rounded-lg border cursor-pointer
                    transition-all duration-300 hover:scale-[1.02]
                    ${config.bgColor} ${config.borderColor}
                `}
                onClick={() => setIsExpanded(true)}
            >
                <div className={config.color}>{config.icon}</div>
                <div className="flex-1">
                    <span className="text-sm font-bold text-nofx-text-main">{symbol}</span>
                    <span className="text-sm text-nofx-text-muted ml-2">{title}</span>
                </div>
                <button className="text-xs text-nofx-text-muted hover:text-nofx-text-main">
                    展开
                </button>
            </div>
        )
    }

    return (
        <div
            className={`
                relative overflow-hidden rounded-xl border p-5
                transition-all duration-300 hover:scale-[1.01]
                ${config.bgColor} ${config.borderColor} ${config.glowColor}
                animate-fade-in
            `}
        >
            {/* Animated Background Gradient */}
            <div className="absolute inset-0 opacity-5">
                <div className="absolute inset-0 bg-gradient-radial from-current to-transparent animate-pulse-slow" />
            </div>

            {/* Content */}
            <div className="relative z-10">
                {/* Header */}
                <div className="flex items-start justify-between mb-4">
                    <div className="flex items-center gap-3">
                        <div
                            className={`
                                w-10 h-10 rounded-full flex items-center justify-center
                                ${config.bgColor} ${config.borderColor} border-2
                                ${config.color}
                            `}
                        >
                            {config.icon}
                        </div>
                        <div>
                            <div className="flex items-center gap-2">
                                <span className="text-lg font-bold text-nofx-text-main font-mono">
                                    {symbol}
                                </span>
                                {confidence && (
                                    <span
                                        className={`
                                            text-xs px-2 py-0.5 rounded-full font-bold
                                            ${config.bgColor} ${config.borderColor} border ${config.color}
                                        `}
                                    >
                                        信心度 {confidence}%
                                    </span>
                                )}
                            </div>
                            <div className={`text-sm font-medium ${config.color}`}>{title}</div>
                        </div>
                    </div>

                    {onDismiss && (
                        <button
                            onClick={onDismiss}
                            className="text-nofx-text-muted hover:text-nofx-text-main transition-colors"
                        >
                            <svg
                                className="w-5 h-5"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                            >
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth={2}
                                    d="M6 18L18 6M6 6l12 12"
                                />
                            </svg>
                        </button>
                    )}
                </div>

                {/* Signals */}
                <div className="space-y-2 mb-4">
                    {signals.map((signal, index) => (
                        <div
                            key={index}
                            className="flex items-start gap-2 text-sm animate-slide-in"
                            style={{ animationDelay: `${index * 0.1}s` }}
                        >
                            <span className={`font-bold ${getSignalColor(signal.status)}`}>
                                {getSignalIcon(signal.status)}
                            </span>
                            <span className="text-nofx-text-main">{signal.text}</span>
                        </div>
                    ))}
                </div>

                {/* Action Button */}
                {action && (
                    <div className="flex items-center gap-3">
                        <button
                            onClick={action.onClick}
                            className={`
                                flex-1 px-4 py-2 rounded-lg font-bold text-sm
                                transition-all duration-200
                                ${config.bgColor} ${config.borderColor} border-2 ${config.color}
                                hover:scale-[1.02] hover:${config.glowColor}
                            `}
                        >
                            {action.label}
                        </button>
                        <button
                            onClick={() => setIsExpanded(false)}
                            className="px-4 py-2 rounded-lg text-sm text-nofx-text-muted hover:text-nofx-text-main border border-nofx-text-muted/20 hover:border-nofx-text-muted/40 transition-all"
                        >
                            收起
                        </button>
                    </div>
                )}

                {/* Confidence Bar */}
                {confidence && (
                    <div className="mt-4 pt-4 border-t border-current/20">
                        <div className="flex items-center justify-between text-xs text-nofx-text-muted mb-1">
                            <span>AI 信心度</span>
                            <span className="font-mono font-bold">{confidence}%</span>
                        </div>
                        <div className="h-2 bg-black/30 rounded-full overflow-hidden">
                            <div
                                className={`h-full ${config.bgColor} transition-all duration-1000 ease-out`}
                                style={{ width: `${confidence}%` }}
                            >
                                <div className="h-full bg-gradient-to-r from-transparent via-white/20 to-transparent animate-shimmer" />
                            </div>
                        </div>
                    </div>
                )}
            </div>

            {/* Pulse Effect */}
            <div className="absolute inset-0 rounded-xl border-2 border-current/0 animate-pulse-glow pointer-events-none" />
        </div>
    )
}

// Smart Alert Container for multiple alerts
interface SmartAlertContainerProps {
    alerts: Array<SmartAlertProps & { id: string }>
    onDismissAll?: () => void
}

export function SmartAlertContainer({ alerts, onDismissAll }: SmartAlertContainerProps) {
    if (alerts.length === 0) return null

    return (
        <div className="space-y-3">
            <div className="flex items-center justify-between mb-2">
                <h3 className="text-sm font-bold text-nofx-text-muted uppercase tracking-wider">
                    智能提示 ({alerts.length})
                </h3>
                {onDismissAll && alerts.length > 1 && (
                    <button
                        onClick={onDismissAll}
                        className="text-xs text-nofx-text-muted hover:text-nofx-text-main transition-colors"
                    >
                        全部清除
                    </button>
                )}
            </div>
            <div className="space-y-3">
                {alerts.map((alert) => (
                    <SmartAlert key={alert.id} {...alert} />
                ))}
            </div>
        </div>
    )
}

