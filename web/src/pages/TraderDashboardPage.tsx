import React, { useEffect, useState, useRef } from 'react'
import { mutate } from 'swr'
import { api } from '../lib/api'
import { ChartTabs } from '../components/ChartTabs'
import { DecisionCard } from '../components/DecisionCard'
import { AIUsageCard } from '../components/AIUsageCard'
import { PositionHistory } from '../components/PositionHistory'
import { PunkAvatar, getTraderAvatar } from '../components/PunkAvatar'
import { confirmToast, notify } from '../lib/notify'
import { formatPrice, formatQuantity, formatFull } from '../utils/format'
import { t, type Language } from '../i18n/translations'
import { LogOut, Loader2, Eye, EyeOff, Copy, Check, ArrowLeft, Settings, RefreshCw } from 'lucide-react'
import { DeepVoidBackground } from '../components/DeepVoidBackground'
import { GrainOverlay } from '../components/ui/GrainOverlay'
import { GridRiskPanel } from '../components/strategy/GridRiskPanel'
import type {
    SystemStatus,
    AccountInfo,
    Position,
    DecisionRecord,
    Statistics,
    TraderInfo,
    Exchange,
    LatestAnalysisResponse,
    DirectionPoolResponse,
    PositionSLTPAdjustmentItem,
} from '../types'

// --- Helper Functions ---

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
    switch (modelId.toLowerCase()) {
        case 'deepseek':
            return 'DeepSeek'
        case 'qwen':
            return 'Qwen'
        case 'claude':
            return 'Claude'
        default:
            return modelId.toUpperCase()
    }
}

// Helper function to get exchange display name from exchange ID (UUID)
function getExchangeDisplayNameFromList(
    exchangeId: string | undefined,
    exchanges: Exchange[] | undefined
): string {
    if (!exchangeId) return 'Unknown'
    const exchange = exchanges?.find((e) => e.id === exchangeId)
    if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...'
    const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
    return exchange.account_name
        ? `${typeName} - ${exchange.account_name}`
        : typeName
}

// Helper function to get exchange type from exchange ID (UUID) - for kline charts
function getExchangeTypeFromList(
    exchangeId: string | undefined,
    exchanges: Exchange[] | undefined
): string {
    if (!exchangeId) return 'binance'
    const exchange = exchanges?.find((e) => e.id === exchangeId)
    if (!exchange) return 'binance' // Default to binance for charts
    return exchange.exchange_type?.toLowerCase() || 'binance'
}

// Helper function to check if exchange is a perp-dex type (wallet-based)
function isPerpDexExchange(exchangeType: string | undefined): boolean {
    if (!exchangeType) return false
    const perpDexTypes = ['hyperliquid', 'lighter', 'aster']
    return perpDexTypes.includes(exchangeType.toLowerCase())
}

// Helper function to get wallet address for perp-dex exchanges
function getWalletAddress(exchange: Exchange | undefined): string | undefined {
    if (!exchange) return undefined
    const type = exchange.exchange_type?.toLowerCase()
    switch (type) {
        case 'hyperliquid':
            return exchange.hyperliquidWalletAddr
        case 'lighter':
            return exchange.lighterWalletAddr
        case 'aster':
            return exchange.asterSigner
        default:
            return undefined
    }
}

// Helper function to truncate wallet address for display
function truncateAddress(address: string, startLen = 6, endLen = 4): string {
    if (address.length <= startLen + endLen + 3) return address
    return `${address.slice(0, startLen)}...${address.slice(-endLen)}`
}

// --- Components ---

/** 实盘与实盘模拟共用同一看板：同一组件、同一 API、同一展示逻辑；仅 isSimulation 控制标签/入口差异。 */
interface TraderDashboardPageProps {
    selectedTrader?: TraderInfo
    traders?: TraderInfo[]
    tradersError?: Error
    selectedTraderId?: string
    onTraderSelect: (traderId: string) => void
    onNavigateToTraders: () => void
    status?: SystemStatus
    account?: AccountInfo
    positions?: Position[]
    decisions?: DecisionRecord[]
    decisionsLimit: number
    onDecisionsLimitChange: (limit: number) => void
    stats?: Statistics
    lastUpdate: string
    language: Language
    exchanges?: Exchange[]
    /** 实盘模拟模式（虚拟资金，不发出真实订单） */
    isSimulation?: boolean
    /** 手动刷新看板数据（状态/账户/持仓/决策等） */
    onRefresh?: () => void
    /** 最新 AI 分析（市况、场景、预测、止盈止损调整） */
    latestAnalysis?: LatestAnalysisResponse
    /** 方向池（实时多空方向） */
    directionPool?: DirectionPoolResponse
}

export function TraderDashboardPage({
    selectedTrader,
    status,
    account,
    positions,
    decisions,
    decisionsLimit,
    onDecisionsLimitChange,
    lastUpdate,
    language,
    traders,
    tradersError,
    selectedTraderId,
    onTraderSelect,
    onNavigateToTraders,
    exchanges,
    isSimulation,
    onRefresh,
    latestAnalysis,
    directionPool,
}: TraderDashboardPageProps) {
    const [closingPosition, setClosingPosition] = useState<string | null>(null)
    const [selectedChartSymbol, setSelectedChartSymbol] = useState<string | undefined>(undefined)
    const [chartUpdateKey, setChartUpdateKey] = useState<number>(0)
    const chartSectionRef = useRef<HTMLDivElement>(null)
    const [showWalletAddress, setShowWalletAddress] = useState<boolean>(false)
    const [copiedAddress, setCopiedAddress] = useState<boolean>(false)

    // Current positions pagination
    const [positionsPageSize, setPositionsPageSize] = useState<number>(20)
    const [positionsCurrentPage, setPositionsCurrentPage] = useState<number>(1)

    // 系统周期/止盈止损周期「查看输出」弹层：'system' | 'sltp' | null
    const [cycleOutputView, setCycleOutputView] = useState<'system' | 'sltp' | null>(null)

    // Calculate paginated positions
    const totalPositions = positions?.length || 0
    const totalPositionPages = Math.ceil(totalPositions / positionsPageSize)
    const paginatedPositions = positions?.slice(
        (positionsCurrentPage - 1) * positionsPageSize,
        positionsCurrentPage * positionsPageSize
    ) || []

    // Reset page when positions change
    useEffect(() => {
        setPositionsCurrentPage(1)
    }, [selectedTraderId, positionsPageSize])

    // Auto-set chart symbol for grid trading
    useEffect(() => {
        if (status?.strategy_type === 'grid_trading' && status?.grid_symbol) {
            setSelectedChartSymbol(status.grid_symbol)
        }
    }, [status?.strategy_type, status?.grid_symbol])

    // Get current exchange info for perp-dex wallet display
    const currentExchange = exchanges?.find(
        (e) => e.id === selectedTrader?.exchange_id
    )
    const walletAddress = getWalletAddress(currentExchange)
    const isPerpDex = isPerpDexExchange(currentExchange?.exchange_type)

    // Copy wallet address to clipboard
    const handleCopyAddress = async () => {
        if (!walletAddress) return
        try {
            await navigator.clipboard.writeText(walletAddress)
            setCopiedAddress(true)
            setTimeout(() => setCopiedAddress(false), 2000)
        } catch (err) {
            console.error('Failed to copy address:', err)
        }
    }

    // Handle symbol click from Decision Card
    const handleSymbolClick = (symbol: string) => {
        // Set the selected symbol
        setSelectedChartSymbol(symbol)
        // Scroll to chart section
        setTimeout(() => {
            chartSectionRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
        }, 100)
    }

    // 平仓操作
    const handleClosePosition = async (symbol: string, side: string) => {
        if (!selectedTraderId) return

        const confirmMsg =
            language === 'zh'
                ? `确定要平仓 ${symbol} ${side === 'LONG' ? '多仓' : '空仓'} 吗？`
                : `Are you sure you want to close ${symbol} ${side === 'LONG' ? 'LONG' : 'SHORT'} position?`

        const confirmed = await confirmToast(confirmMsg, {
            title: language === 'zh' ? '确认平仓' : 'Confirm Close',
            okText: language === 'zh' ? '确认' : 'Confirm',
            cancelText: language === 'zh' ? '取消' : 'Cancel',
        })

        if (!confirmed) return

        setClosingPosition(symbol)
        try {
            await api.closePosition(selectedTraderId, symbol, side)
            notify.success(
                language === 'zh' ? '平仓成功' : 'Position closed successfully'
            )
            // 使用 SWR mutate 刷新数据而非重新加载页面
            await Promise.all([
                mutate(`positions-${selectedTraderId}`),
                mutate(`account-${selectedTraderId}`),
            ])
        } catch (err: unknown) {
            const errorMsg =
                err instanceof Error
                    ? err.message
                    : language === 'zh'
                        ? '平仓失败'
                        : 'Failed to close position'
            notify.error(errorMsg)
        } finally {
            setClosingPosition(null)
        }
    }

    // If API failed with error, show empty state (likely backend not running)
    if (tradersError) {
        return (
            <div className="flex items-center justify-center min-h-[60vh] relative z-10">
                <div className="text-center max-w-md mx-auto px-6">
                    <div
                        className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center nofx-glass"
                        style={{
                            background: 'rgba(240, 185, 11, 0.1)',
                            borderColor: 'rgba(240, 185, 11, 0.3)',
                        }}
                    >
                        <svg
                            className="w-12 h-12 text-nofx-gold"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                        >
                            <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth={2}
                                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                            />
                        </svg>
                    </div>
                    <h2 className="text-2xl font-bold mb-3 text-nofx-text-main">
                        {language === 'zh' ? '无法连接到服务器' : 'Connection Failed'}
                    </h2>
                    <p className="text-base mb-6 text-nofx-text-muted">
                        {language === 'zh'
                            ? '请确认后端服务已启动。'
                            : 'Please check if the backend service is running.'}
                    </p>
                    <button
                        onClick={() => window.location.reload()}
                        className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95 nofx-glass border border-nofx-gold/30 text-nofx-gold hover:bg-nofx-gold/10"
                    >
                        {language === 'zh' ? '重试' : 'Retry'}
                    </button>
                </div>
            </div>
        )
    }

    // If traders is loaded and empty, show empty state
    if (traders && traders.length === 0) {
        return (
            <div className="flex items-center justify-center min-h-[60vh] relative z-10">
                <div className="text-center max-w-md mx-auto px-6">
                    <div
                        className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center nofx-glass"
                        style={{
                            background: 'rgba(240, 185, 11, 0.1)',
                            borderColor: 'rgba(240, 185, 11, 0.3)',
                        }}
                    >
                        <svg
                            className="w-12 h-12 text-nofx-gold"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                        >
                            <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth={2}
                                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                            />
                        </svg>
                    </div>
                    <h2 className="text-2xl font-bold mb-3 text-nofx-text-main">
                        {t('dashboardEmptyTitle', language)}
                    </h2>
                    <p className="text-base mb-6 text-nofx-text-muted">
                        {t('dashboardEmptyDescription', language)}
                    </p>
                    <button
                        onClick={onNavigateToTraders}
                        className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95 nofx-glass border border-nofx-gold/30 text-nofx-gold hover:bg-nofx-gold/10"
                    >
                        {t('goToTradersPage', language)}
                    </button>
                </div>
            </div>
        )
    }

    // If traders is still loading or selectedTrader is not ready, show skeleton
    if (!selectedTrader) {
        return (
            <div className="space-y-6 relative z-10">
                <div className="nofx-glass p-6 animate-pulse">
                    <div className="h-8 w-48 mb-3 bg-nofx-bg/50 rounded"></div>
                    <div className="flex gap-4">
                        <div className="h-4 w-32 bg-nofx-bg/50 rounded"></div>
                        <div className="h-4 w-24 bg-nofx-bg/50 rounded"></div>
                        <div className="h-4 w-28 bg-nofx-bg/50 rounded"></div>
                    </div>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                    {[1, 2, 3, 4].map((i) => (
                        <div key={i} className="nofx-glass p-5 animate-pulse">
                            <div className="h-4 w-24 mb-3 bg-nofx-bg/50 rounded"></div>
                            <div className="h-8 w-32 bg-nofx-bg/50 rounded"></div>
                        </div>
                    ))}
                </div>
                <div className="nofx-glass p-6 animate-pulse">
                    <div className="h-6 w-40 mb-4 bg-nofx-bg/50 rounded"></div>
                    <div className="h-64 w-full bg-nofx-bg/50 rounded"></div>
                </div>
            </div>
        )
    }

    return (
        <DeepVoidBackground className="min-h-screen pb-12" disableAnimation>
            <GrainOverlay />
            <main className="w-full px-5 lg:px-10 xl:px-14 py-6 lg:py-8 relative z-10 flex flex-col gap-5">
                {/* Trader Header */}
                <div className="trader-header-block">
                <div className="rounded-2xl p-5 lg:p-6 animate-scale-in modern-card group" style={{ background: 'linear-gradient(135deg, rgba(15, 23, 42, 0.6) 0%, rgba(15, 23, 42, 0.4) 100%)' }}>
                    <div className="flex items-start justify-between mb-4">
                        <h2 className="text-2xl font-bold flex items-center gap-4 text-white">
                            <div className="relative">
                                <PunkAvatar
                                    seed={getTraderAvatar(
                                        selectedTrader.trader_id,
                                        selectedTrader.trader_name
                                    )}
                                    size={56}
                                    className="rounded-xl border-2 border-teal-500/30 shadow-glow-teal"
                                />
                                <div className="absolute -bottom-1 -right-1 w-4 h-4 bg-teal-500 rounded-full border-2 border-zinc-950 shadow-glow-teal animate-pulse" />
                            </div>
                            <div className="flex flex-col">
                                <span className="text-3xl tracking-tight text-white font-semibold">
                                    {selectedTrader.trader_name}
                                </span>
                                {isSimulation && (
                                    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-500/20 text-amber-400 border border-amber-500/30 mt-1 w-fit">
                                        {language === 'zh' ? '实盘模拟' : 'Paper'}
                                    </span>
                                )}
                                <span className="text-xs font-mono text-zinc-400 opacity-60 flex items-center gap-2">
                                    <div className="w-1.5 h-1.5 bg-teal-400 rounded-full" />
                                    ID: {selectedTrader.trader_id.slice(0, 8)}...
                                </span>
                            </div>
                        </h2>

                        <div className="flex items-center gap-4 flex-wrap">
                            {/* 实盘模拟：返回配置/列表页入口，便于找到「创建模拟交易员、模拟交易所」 */}
                            {isSimulation && (
                                <button
                                    type="button"
                                    onClick={onNavigateToTraders}
                                    className="flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium border border-amber-500/30 bg-amber-500/10 text-amber-400 hover:bg-amber-500/20 transition-colors"
                                    title={language === 'zh' ? '返回实盘模拟配置（交易员/交易所）' : 'Back to paper config (traders & exchanges)'}
                                >
                                    <ArrowLeft className="w-4 h-4 shrink-0" />
                                    <Settings className="w-3.5 h-3.5 shrink-0" />
                                    {language === 'zh' ? '实盘模拟配置' : 'Paper config'}
                                </button>
                            )}
                            {/* Trader Selector */}
                            {traders && traders.length > 0 && (
                                <div className="flex items-center gap-2 modern-card px-1 py-1 rounded-lg border border-white/5">
                                    <select
                                        value={selectedTraderId}
                                        onChange={(e) => onTraderSelect(e.target.value)}
                                        className="bg-transparent text-sm font-medium cursor-pointer transition-colors text-white focus:outline-none px-2 py-1"
                                    >
                                        {traders.map((trader) => (
                                            <option key={trader.trader_id} value={trader.trader_id} className="bg-zinc-950">
                                                {trader.trader_name}
                                            </option>
                                        ))}
                                    </select>
                                </div>
                            )}

                            {/* Wallet Address Display for Perp-DEX */}
                            {exchanges && isPerpDex && (
                                <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg modern-card border border-teal-500/20">
                                    {walletAddress ? (
                                        <>
                                            <span className="text-xs font-mono text-teal-400">
                                                {showWalletAddress
                                                    ? walletAddress
                                                    : truncateAddress(walletAddress)}
                                            </span>
                                            <button
                                                type="button"
                                                onClick={() => setShowWalletAddress(!showWalletAddress)}
                                                className="p-1 rounded hover:bg-white/10 transition-colors"
                                                title={
                                                    showWalletAddress
                                                        ? language === 'zh'
                                                            ? '隐藏地址'
                                                            : 'Hide address'
                                                        : language === 'zh'
                                                            ? '显示完整地址'
                                                            : 'Show full address'
                                                }
                                            >
                                                {showWalletAddress ? (
                                                    <EyeOff className="w-3.5 h-3.5 text-zinc-400" />
                                                ) : (
                                                    <Eye className="w-3.5 h-3.5 text-zinc-400" />
                                                )}
                                            </button>
                                            <button
                                                type="button"
                                                onClick={handleCopyAddress}
                                                className="p-1 rounded hover:bg-white/10 transition-colors"
                                                title={language === 'zh' ? '复制地址' : 'Copy address'}
                                            >
                                                {copiedAddress ? (
                                                    <Check className="w-3.5 h-3.5 text-teal-500" />
                                                ) : (
                                                    <Copy className="w-3.5 h-3.5 text-zinc-400" />
                                                )}
                                            </button>
                                        </>
                                    ) : (
                                        <span className="text-xs text-zinc-400">
                                            {language === 'zh' ? '未配置地址' : 'No address configured'}
                                        </span>
                                    )}
                                </div>
                            )}
                        </div>
                    </div>
                    <div className="flex items-center gap-6 text-sm flex-wrap text-zinc-400 font-mono pl-2">
                        <span className="flex items-center gap-2">
                            <span className="opacity-60">AI Model:</span>
                            <span
                                className="font-bold px-2 py-0.5 rounded text-xs tracking-wide"
                                style={{
                                    background: selectedTrader.ai_model.includes('qwen') ? 'rgba(192, 132, 252, 0.15)' : 'rgba(96, 165, 250, 0.15)',
                                    color: selectedTrader.ai_model.includes('qwen') ? '#c084fc' : '#60a5fa',
                                    border: `1px solid ${selectedTrader.ai_model.includes('qwen') ? '#c084fc' : '#60a5fa'}40`
                                }}
                            >
                                {getModelDisplayName(
                                    selectedTrader.ai_model.split('_').pop() ||
                                    selectedTrader.ai_model
                                )}
                            </span>
                        </span>
                        <span className="w-px h-3 bg-white/10 hidden md:block" />
                        <span className="flex items-center gap-2">
                            <span className="opacity-60">Exchange:</span>
                            <span className="text-white font-semibold">
                                {getExchangeDisplayNameFromList(
                                    selectedTrader.exchange_id,
                                    exchanges
                                )}
                            </span>
                        </span>
                        <span className="w-px h-3 bg-white/10 hidden md:block" />
                        <span className="flex items-center gap-2">
                            <span className="opacity-60">Strategy:</span>
                            <span className="text-teal-400 font-semibold tracking-wide">
                                {selectedTrader.strategy_name || 'No Strategy'}
                            </span>
                        </span>
                        {status && (
                            <div className="hidden md:contents">
                                <span className="w-px h-3 bg-white/10" />
                                <span>Cycles: <span className="text-white">{status.call_count}</span></span>
                                <span className="w-px h-3 bg-white/10" />
                                <span>Runtime: <span className="text-white">{status.runtime_minutes} min</span></span>
                            </div>
                        )}
                    </div>
                    </div>
                </div>
                {/* 模拟交易隔离标识：明确标注当前为模拟交易看板 */}
                {isSimulation && (
                    <div className="mb-4 px-4 py-2 rounded-lg border border-amber-500/30 bg-amber-500/10 flex items-center gap-2">
                        <span className="text-amber-400 font-semibold text-sm">
                            {language === 'zh' ? '模拟交易' : 'Paper Trading'}
                        </span>
                        <span className="text-zinc-400 text-xs">
                            {language === 'zh' ? '· 虚拟资金，不发出真实订单' : '· Virtual funds, no real orders'}
                        </span>
                    </div>
                )}

                {/* Debug Info + 手动刷新 */}
                {(account || lastUpdate !== '--:--:--') && (
                    <div className="mb-4 px-3 py-1.5 rounded bg-black/40 border border-white/5 text-[10px] font-mono text-nofx-text-muted flex justify-between items-center opacity-60 hover:opacity-100 transition-opacity">
                        <span>SYSTEM_STATUS::ONLINE</span>
                        <div className="flex items-center gap-4">
                            <span>LAST_UPDATE::{lastUpdate}</span>
                            {account && (
                                <>
                                    <span>EQ::{account?.total_equity?.toFixed(2)}</span>
                                    <span>PNL::{account?.total_pnl?.toFixed(2)}</span>
                                </>
                            )}
                            {onRefresh && (
                                <button
                                    type="button"
                                    onClick={onRefresh}
                                    className="p-1 rounded hover:bg-white/10 text-nofx-text-muted hover:text-white transition-colors"
                                    title={language === 'zh' ? '刷新数据' : 'Refresh data'}
                                >
                                    <RefreshCw className="w-3.5 h-3.5" />
                                </button>
                            )}
                        </div>
                    </div>
                )}

                {/* Account Overview - 模板: KPI 卡片行 gap-3 lg:gap-4 */}
                <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 lg:gap-4">
                    <StatCard
                        title={t('totalEquity', language)}
                        value={`${account?.total_equity?.toFixed(2) || '0.00'}`}
                        unit="USDT"
                        change={account?.total_pnl_pct || 0}
                        positive={(account?.total_pnl ?? 0) > 0}
                        icon="💰"
                    />
                    <StatCard
                        title={t('availableBalance', language)}
                        value={`${account?.available_balance?.toFixed(2) || '0.00'}`}
                        unit="USDT"
                        subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% ${t('free', language)}`}
                        icon="💳"
                    />
                    <StatCard
                        title={t('totalPnL', language)}
                        value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'}`}
                        unit="USDT"
                        change={account?.total_pnl_pct || 0}
                        positive={(account?.total_pnl ?? 0) >= 0}
                        subtitle={language === 'zh' ? `含持仓浮盈/浮亏${account?.realized_pnl !== undefined ? ` · 已实现: ${account.realized_pnl >= 0 ? '+' : ''}${account.realized_pnl.toFixed(2)}` : ''}` : `Includes unrealized${account?.realized_pnl !== undefined ? ` · Realized: ${account.realized_pnl >= 0 ? '+' : ''}${account.realized_pnl.toFixed(2)}` : ''}`}
                        icon="📈"
                    />
                    <StatCard
                        title={t('positions', language)}
                        value={`${account?.position_count || 0}`}
                        unit="ACTIVE"
                        subtitle={`${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`}
                        icon="📊"
                    />
                </div>

                {/* Grid Risk Panel - Only show for grid trading strategy */}
                {status?.strategy_type === 'grid_trading' && selectedTraderId && (
                    <div className="mb-8 animate-slide-in" style={{ animationDelay: '0.05s' }}>
                        <GridRiskPanel
                            traderId={selectedTraderId}
                            language={language}
                            refreshInterval={5000}
                        />
                    </div>
                )}

                {/* Main Content Area - 账户净值与 AI 决策 50-50 等宽；当前持仓/历史仓位全宽同宽 */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                    {/* Left Cell: 账户净值曲线 */}
                    <div className="min-w-0">
                        {/* Chart Tabs (Equity / K-line) */}
                        <div
                            ref={chartSectionRef}
                            className="chart-container rounded-2xl p-5 lg:p-6 animate-slide-in scroll-mt-32 backdrop-blur-sm"
                            style={{ animationDelay: '0.1s' }}
                        >
                            <ChartTabs
                                traderId={selectedTrader.trader_id}
                                selectedSymbol={selectedChartSymbol}
                                updateKey={chartUpdateKey}
                                exchangeId={getExchangeTypeFromList(
                                    selectedTrader.exchange_id,
                                    exchanges
                                )}
                                isSimulation={isSimulation}
                            />
                        </div>
                    </div>

                    {/* Right Column: AI 决策/分析 */}
                    <div
                        className="nofx-glass p-6 animate-slide-in h-fit lg:sticky lg:top-24 lg:max-h-[calc(100vh-120px)] flex flex-col min-w-0"
                        style={{ animationDelay: '0.2s' }}
                    >
                        {/* Header */}
                        <div className="flex items-center gap-3 mb-5 pb-4 border-b border-white/5 shrink-0">
                            <div
                                className="w-10 h-10 rounded-xl flex items-center justify-center text-xl shadow-[0_4px_14px_rgba(99,102,241,0.4)]"
                                style={{
                                    background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
                                }}
                            >
                                🧠
                            </div>
                            <div className="flex-1">
                                <h2 className="text-xl font-bold text-nofx-text-main">
                                    {language === 'zh' ? 'AI 决策 / 分析' : 'AI Decisions / Analysis'}
                                </h2>
                                <div className="text-xs text-nofx-text-muted mt-0.5">
                                    {language === 'zh' ? '与回测实验室一致的思维链与决策记录' : 'Same as Backtest Lab: chain-of-thought and decision log'}
                                </div>
                                {decisions && decisions.length > 0 && (
                                    <>
                                        <div className="text-xs text-nofx-text-muted mt-0.5">
                                            {t('lastCycles', language, { count: decisions.length })}
                                        </div>
                                        <div className="text-[11px] text-nofx-text-muted mt-1 opacity-90">
                                            {t('decisionListHint', language)}
                                        </div>
                                        <div className="text-[11px] text-nofx-text-muted mt-0.5 opacity-75">
                                            {t('decisionCycleNote', language)}
                                        </div>
                                    </>
                                )}
                            </div>
                            {/* Limit Selector */}
                            <select
                                value={decisionsLimit}
                                onChange={(e) => onDecisionsLimitChange(Number(e.target.value))}
                                className="px-3 py-1.5 rounded-lg text-sm font-medium cursor-pointer transition-all bg-black/40 text-nofx-text-main border border-white/10 hover:border-nofx-accent focus:outline-none"
                            >
                                <option value={5}>5</option>
                                <option value={10}>10</option>
                                <option value={20}>20</option>
                                <option value={50}>50</option>
                                <option value={100}>100</option>
                            </select>
                        </div>

                        {/* 系统周期 / 止盈止损周期：独立板块，视觉隔离 */}
                        {status && (
                            <div className="mb-4 p-4 rounded-xl border border-white/15 bg-white/[0.03]">
                                <div className="text-[11px] text-nofx-text-muted uppercase tracking-wide mb-3">
                                    {language === 'zh' ? '本会话周期统计' : 'Session cycle stats'}
                                </div>
                                <div className="flex flex-wrap gap-3">
                                    <div
                                        role="button"
                                        tabIndex={0}
                                        onClick={() => setCycleOutputView('system')}
                                        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') setCycleOutputView('system') }}
                                        className="px-3 py-2 rounded-lg bg-white/[0.06] border border-white/10 min-w-0 flex-1 basis-24 cursor-pointer hover:border-white/20 hover:bg-white/[0.08] transition-colors"
                                        title={language === 'zh' ? '本会话系统周期执行次数（点击查看思维链历史）' : 'System cycles this session (click to view chain history)'}
                                    >
                                        <div className="text-[11px] text-nofx-text-muted uppercase tracking-wide">
                                            {t('systemCycleLabel', language)}
                                        </div>
                                        <div className="text-sm font-semibold text-nofx-text-main tabular-nums">
                                            {typeof status.system_cycle_count === 'number' ? status.system_cycle_count : '–'}
                                        </div>
                                        <div className="text-[10px] text-nofx-text-muted mt-0.5 opacity-80">
                                            {language === 'zh' ? '点击查看思维链' : 'View chain'}
                                        </div>
                                    </div>
                                    <div
                                        role="button"
                                        tabIndex={0}
                                        onClick={() => setCycleOutputView('sltp')}
                                        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') setCycleOutputView('sltp') }}
                                        className="px-3 py-2 rounded-lg bg-white/[0.06] border border-white/10 min-w-0 flex-1 basis-24 cursor-pointer hover:border-white/20 hover:bg-white/[0.08] transition-colors"
                                        title={language === 'zh' ? '本会话止盈止损分析周期执行次数（点击查看思维链历史）' : 'SL/TP analysis cycles (click to view chain history)'}
                                    >
                                        <div className="text-[11px] text-nofx-text-muted uppercase tracking-wide">
                                            {t('sltpAnalysisCycleLabel', language)}
                                        </div>
                                        <div className="text-sm font-semibold text-nofx-text-main tabular-nums">
                                            {typeof status.sltp_analysis_cycle_count === 'number' ? status.sltp_analysis_cycle_count : '–'}
                                        </div>
                                        <div className="text-[10px] text-nofx-text-muted mt-0.5 opacity-80">
                                            {language === 'zh' ? '点击查看思维链' : 'View chain'}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        )}

                        {/* Token 用量 */}
                        <div className="mb-4 shrink-0">
                            <AIUsageCard language={language} traderId={selectedTraderId} />
                        </div>

                        {/* Decisions List - Scrollable */}
                        <div
                            className="space-y-4 overflow-y-auto pr-2 custom-scrollbar"
                            style={{ maxHeight: 'calc(100vh - 380px)' }}
                        >
                            {decisions && decisions.length > 0 ? (
                                decisions.map((decision, i) => {
                                    // AI 思维链只展示 wait/hold；开平仓由系统执行，见「系统周期」
                                    const onlyHoldWait = {
                                        ...decision,
                                        decisions: (decision.decisions || []).filter(
                                            (a) => a.action === 'hold' || a.action === 'wait'
                                        )
                                    }
                                    return (
                                        <DecisionCard
                                            key={i}
                                            decision={onlyHoldWait}
                                            language={language}
                                            onSymbolClick={handleSymbolClick}
                                            systemExecutionHint
                                        />
                                    )
                                })
                            ) : (
                                <div className="py-16 text-center text-nofx-text-muted opacity-60">
                                    <div className="text-6xl mb-4 opacity-30 grayscale">🧠</div>
                                    <div className="text-lg font-semibold mb-2 text-nofx-text-main">
                                        {t('noDecisionsYet', language)}
                                    </div>
                                    <div className="text-sm">
                                        {t('aiDecisionsWillAppear', language)}
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                </div>

                {/* Full-width: 当前持仓（与历史仓位同宽） */}
                <div className="full-width-positions-section">
                <div className="space-y-6 mt-5">
                    <div
                        className="nofx-glass rounded-2xl p-5 lg:p-6 animate-slide-in relative overflow-hidden group"
                        style={{ animationDelay: '0.15s' }}
                    >
                        <div className="absolute top-0 right-0 p-3 opacity-10 group-hover:opacity-20 transition-opacity">
                            <div className="w-24 h-24 rounded-full bg-blue-500 blur-3xl" />
                        </div>
                        <div className="flex items-center justify-between mb-5 relative z-10 flex-wrap gap-2">
                            <h2 className="text-lg font-bold flex items-center gap-2 text-nofx-text-main uppercase tracking-wide">
                                <span className="text-blue-500">◈</span> {t('currentPositions', language)}
                            </h2>
                            {positions && positions.length > 0 && (
                                <div className="flex items-center gap-3 flex-wrap">
                                    <div className="text-xs px-2 py-1 rounded bg-nofx-gold/10 text-nofx-gold border border-nofx-gold/20 font-mono shadow-[0_0_10px_rgba(240,185,11,0.1)]">
                                        {positions.length} {t('active', language)}
                                    </div>
                                    <div className="flex items-center gap-3 text-xs font-mono">
                                        <span className="text-nofx-text-muted">
                                            {language === 'zh' ? '保证金' : 'Margin'}: ${(positions.reduce((s, p) => s + (p.margin_used ?? 0), 0)).toFixed(2)}
                                        </span>
                                        <span className={`font-semibold ${positions.reduce((s, p) => s + p.unrealized_pnl, 0) >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>
                                            {language === 'zh' ? '浮盈' : 'uPnL'}: {positions.reduce((s, p) => s + p.unrealized_pnl, 0) >= 0 ? '+' : ''}{positions.reduce((s, p) => s + p.unrealized_pnl, 0).toFixed(2)}
                                        </span>
                                    </div>
                                </div>
                            )}
                        </div>
                        {positions && positions.length > 0 ? (
                            <div>
                                <div className="overflow-x-auto">
                                    <table className="w-full text-xs">
                                            <thead className="text-left border-b border-white/5">
                                                <tr>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-left uppercase tracking-[0.08em]">{t('symbol', language)}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-center uppercase tracking-[0.08em]">{t('side', language)}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-center uppercase tracking-[0.08em]">{language === 'zh' ? '操作' : 'Action'}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell uppercase tracking-[0.08em]" title={t('entryPrice', language)}>{language === 'zh' ? '入场价' : 'Entry'}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell uppercase tracking-[0.08em]" title={t('markPrice', language)}>{language === 'zh' ? '标记价' : 'Mark'}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-right uppercase tracking-[0.08em]" title={t('quantity', language)}>{language === 'zh' ? '数量' : 'Qty'}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell uppercase tracking-[0.08em]" title={t('positionValue', language)}>{language === 'zh' ? '价值' : 'Value'}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-center hidden md:table-cell uppercase tracking-[0.08em]" title={t('leverage', language)}>{language === 'zh' ? '杠杆' : 'Lev.'}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-right uppercase tracking-[0.08em]" title={t('unrealizedPnL', language)}>{language === 'zh' ? '未实现盈亏' : 'uPnL'}</th>
                                                    <th className="p-4 text-[11px] font-semibold text-nofx-text-muted whitespace-nowrap text-right hidden md:table-cell uppercase tracking-[0.08em]" title={t('liqPrice', language)}>{language === 'zh' ? '强平价' : 'Liq.'}</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {paginatedPositions.map((pos, i) => (
                                                    <React.Fragment key={i}>
                                                    <tr
                                                        className="border-b border-white/5 last:border-0 transition-all duration-200 hover:bg-accent/20 cursor-pointer group/row"
                                                        onClick={() => {
                                                            setSelectedChartSymbol(pos.symbol)
                                                            setChartUpdateKey(Date.now())
                                                            if (chartSectionRef.current) {
                                                                chartSectionRef.current.scrollIntoView({
                                                                    behavior: 'smooth',
                                                                    block: 'start',
                                                                })
                                                            }
                                                        }}
                                                    >
                                                        <td className="p-4 font-mono font-semibold whitespace-nowrap text-left text-nofx-text-main group-hover/row:text-white transition-colors">
                                                            {pos.symbol}
                                                        </td>
                                                        <td className="p-4 whitespace-nowrap text-center">
                                                            <span
                                                                className={`px-2.5 py-1 rounded-lg text-[11px] font-bold uppercase tracking-[0.08em] ${pos.side === 'long' ? 'bg-fin-gain/10 text-fin-gain' : 'bg-fin-loss/10 text-fin-loss'}`}
                                                            >
                                                                {t(pos.side === 'long' ? 'long' : 'short', language)}
                                                            </span>
                                                        </td>
                                                        <td className="p-4 whitespace-nowrap text-center">
                                                            <button
                                                                type="button"
                                                                onClick={(e) => {
                                                                    e.stopPropagation()
                                                                    handleClosePosition(pos.symbol, pos.side.toUpperCase())
                                                                }}
                                                                disabled={closingPosition === pos.symbol}
                                                                className="inline-flex items-center gap-1 px-2.5 py-1 rounded-xl text-[11px] font-semibold transition-all duration-200 hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed mx-auto bg-fin-loss/10 text-fin-loss hover:bg-fin-loss/20"
                                                                title={language === 'zh' ? '平仓' : 'Close Position'}
                                                            >
                                                                {closingPosition === pos.symbol ? (
                                                                    <Loader2 className="w-3 h-3 animate-spin" />
                                                                ) : (
                                                                    <LogOut className="w-3 h-3" />
                                                                )}
                                                                {language === 'zh' ? '平仓' : 'Close'}
                                                            </button>
                                                        </td>
                                                        <td className="p-4 font-mono whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">{formatPrice(pos.entry_price)}</td>
                                                        <td className="p-4 font-mono whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">{formatPrice(pos.mark_price)}</td>
                                                        <td className="p-4 font-mono whitespace-nowrap text-right text-nofx-text-main">{formatQuantity(pos.quantity)}</td>
                                                        <td className="p-4 font-mono font-bold whitespace-nowrap text-right text-nofx-text-main hidden md:table-cell">{(pos.quantity * pos.mark_price).toFixed(2)}</td>
                                                        <td className="p-4 font-mono whitespace-nowrap text-center text-primary hidden md:table-cell">{pos.leverage}x</td>
                                                        <td className="p-4 font-mono whitespace-nowrap text-right">
                                                            <span
                                                                className={`font-bold ${pos.unrealized_pnl >= 0 ? 'text-fin-gain' : 'text-fin-loss'}`}
                                                            >
                                                                {pos.unrealized_pnl >= 0 ? '+' : ''}
                                                                {pos.unrealized_pnl.toFixed(2)}
                                                            </span>
                                                        </td>
                                                        <td className="p-4 font-mono whitespace-nowrap text-right text-nofx-text-muted hidden md:table-cell">{formatPrice(pos.liquidation_price)}</td>
                                                    </tr>
                                                    {(() => {
                                                        const hasLive = pos.distance_to_sl_pct != null || pos.distance_to_tp_pct != null || pos.trailing_enabled || pos.scaled_tp_enabled || pos.support_resistance_enabled || pos.resistance_enabled
                                                        const adj = latestAnalysis?.position_sl_tp_adjustments?.find(
                                                            (a) => (a.symbol || '').toUpperCase() === (pos.symbol || '').toUpperCase() && (a.side || '').toLowerCase() === (pos.side || '').toLowerCase()
                                                        )
                                                        return (
                                                        <tr className="border-b border-white/5 bg-white/[0.02]">
                                                            <td colSpan={10} className="px-2 py-2 text-[11px]">
                                                                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 font-mono">
                                                                    <div className="space-y-2">
                                                                        {/* 固定实时参数：始终显示，有值显示数值否则显示 — */}
                                                                        <div className="rounded-lg px-2.5 py-2 grid grid-cols-2 sm:grid-cols-3 gap-x-4 gap-y-0.5 text-[11px] font-mono" style={{ background: 'rgba(43,49,57,0.4)', border: '1px solid rgba(43,49,57,0.6)' }}>
                                                                            <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '止损价' : 'SL'}</span><span className={(pos.stop_loss != null && pos.stop_loss > 0) ? 'text-red-400 font-medium' : 'text-nofx-text-muted'}>{(pos.stop_loss != null && pos.stop_loss > 0) ? '$' + formatFull(pos.stop_loss) : '—'}</span></div>
                                                                            <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '止盈价' : 'TP'}</span><span className={(pos.take_profit != null && pos.take_profit > 0) ? 'text-emerald-400 font-medium' : 'text-nofx-text-muted'}>{(pos.take_profit != null && pos.take_profit > 0) ? '$' + formatFull(pos.take_profit) : '—'}</span></div>
                                                                            <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">ATR ×</span><span className="text-nofx-text-main">{((pos.atr_multiple_sl != null && pos.atr_multiple_sl > 0) || (pos.atr_multiple_tp != null && pos.atr_multiple_tp > 0)) ? (pos.atr_multiple_sl != null && pos.atr_multiple_sl > 0 ? formatFull(pos.atr_multiple_sl, 2) + '×' : '—') + ((pos.atr_multiple_sl != null && pos.atr_multiple_sl > 0) && (pos.atr_multiple_tp != null && pos.atr_multiple_tp > 0) ? ' / ' : '') + (pos.atr_multiple_tp != null && pos.atr_multiple_tp > 0 ? formatFull(pos.atr_multiple_tp, 2) + '×' : '') : '—'}</span></div>
                                                                            <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">ATR {language === 'zh' ? '数值' : 'value'}</span><span className="text-nofx-text-main">{(pos.atr_at_open != null && pos.atr_at_open > 0) ? '$' + formatFull(pos.atr_at_open, 6) : '—'}</span></div>
                                                                            <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">ATR {language === 'zh' ? '周期' : 'period'}</span><span className="text-nofx-text-muted">{(pos.atr_period != null && pos.atr_period > 0) ? String(pos.atr_period) : '—'}</span></div>
                                                                            <div className="col-span-2 sm:col-span-3 text-[10px] mt-0.5" style={{ color: '#848E9C' }}>{language === 'zh' ? '参数在开仓时写入并持久保存，随刷新实时显示；距止损/距止盈随行情更新。无记录时显示 —' : 'Params saved at open and shown on each refresh; distance to SL/TP updates with price. No record = —'}</div>
                                                                        </div>
                                                                        {/* AI 调节（当前生效）：与下方「止盈止损调整」同源，便于在当前持仓直接看到该仓位的调整后参数 */}
                                                                        {adj && (
                                                                            <div className="rounded-lg px-2.5 py-2 grid grid-cols-2 sm:grid-cols-3 gap-x-4 gap-y-0.5 text-[11px] font-mono" style={{ background: 'rgba(59,130,246,0.08)', border: '1px solid rgba(59,130,246,0.25)' }} title={language === 'zh' ? 'AI 止盈止损调节，系统在边界内应用' : 'AI SL/TP adjustment applied within bounds'}>
                                                                                <div className="col-span-2 sm:col-span-3 text-[10px] font-semibold text-blue-400/90 mb-0.5">{language === 'zh' ? 'AI 调节（当前生效）' : 'AI adjustment (active)'}</div>
                                                                                <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '建议' : 'Advice'}</span><span className="text-nofx-text-main">{adj.advice ?? '—'}</span></div>
                                                                                <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '追踪' : 'Trail'}</span><span className="text-nofx-text-main">{adj.trail_aggressiveness ?? '—'}</span></div>
                                                                                <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">ATR×SL</span><span className="text-nofx-text-main">{adj.atr_mult_sl != null ? String(adj.atr_mult_sl) : '—'}</span></div>
                                                                                <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '锁利%' : 'Lock%'}</span><span className="text-nofx-text-main">{adj.lock_profit_pct != null ? String(adj.lock_profit_pct) : '—'}</span></div>
                                                                                <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">ΔCycles</span><span className="text-nofx-text-main">{adj.confirm_cycles_delta != null ? String(adj.confirm_cycles_delta) : '—'}</span></div>
                                                                            </div>
                                                                        )}
                                                                    </div>
                                                                    {hasLive && (
                                                                        <div className="rounded-lg px-2.5 py-2 grid grid-cols-2 sm:grid-cols-3 gap-x-4 gap-y-0.5 text-[11px] font-mono" style={{ background: 'rgba(30,35,41,0.6)', border: '1px solid #2B3139' }}>
                                                                            {pos.distance_to_sl_pct != null && (<div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '距止损' : 'To SL'}</span><span className={pos.distance_to_sl_pct >= 0 ? 'text-emerald-400 font-medium' : 'text-red-400 font-medium'}>{pos.distance_to_sl_pct >= 0 ? '+' : ''}{formatFull(pos.distance_to_sl_pct, 3)}%</span></div>)}
                                                                            {pos.distance_to_tp_pct != null && (<div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '距止盈' : 'To TP'}</span><span className={pos.distance_to_tp_pct >= 0 ? 'text-emerald-400 font-medium' : 'text-nofx-text-muted'}>{pos.distance_to_tp_pct >= 0 ? '+' : ''}{formatFull(pos.distance_to_tp_pct, 3)}%</span></div>)}
                                                                            {pos.trailing_enabled && (<div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '追踪止损' : 'Trailing'}</span><span className="text-nofx-gold">{(pos.trailing_tier_activated ?? 0) > 0 ? (language === 'zh' ? `L${pos.trailing_tier_activated} 激活` : `L${pos.trailing_tier_activated} on`) : (language === 'zh' ? '未激活' : 'off')}</span></div>)}
                                                                            {pos.trailing_enabled && pos.trailing_allowed_drawdown != null && pos.trailing_allowed_drawdown > 0 && (pos.trailing_tier_activated ?? 0) > 0 && (<div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '允许回撤' : 'Allowed DD'}</span><span className="text-nofx-gold">{formatFull(pos.trailing_allowed_drawdown, 2)}%</span></div>)}
                                                                            {pos.scaled_tp_enabled && (<div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '分层止盈' : 'Scaled TP'}</span><span className="text-nofx-gold">{(pos.scaled_tp_level ?? 0) > 0 ? (language === 'zh' ? `L${pos.scaled_tp_level} 激活` : `L${pos.scaled_tp_level} on`) : (language === 'zh' ? '未激活' : 'off')}</span></div>)}
                                                                            {pos.scaled_tp_enabled && pos.scaled_tp_closed_pct != null && pos.scaled_tp_closed_pct > 0 && (
                                                                              <>
                                                                                <div className="flex items-center gap-1.5">
                                                                                  <span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '已平仓' : 'Closed'}</span>
                                                                                  <span className="text-emerald-400">{formatFull(pos.scaled_tp_closed_pct, 2)}%</span>
                                                                                </div>
                                                                                <div className="flex items-center gap-1.5">
                                                                                  <span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '剩余' : 'Remaining'}</span>
                                                                                  <span className="text-nofx-text-main">
                                                                                    {formatFull(Math.max(0, 100 - (pos.scaled_tp_closed_pct ?? 0)), 2)}%
                                                                                  </span>
                                                                                </div>
                                                                              </>
                                                                            )}
                                                                            {pos.support_resistance_enabled && (<div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '支撑阻力止损' : 'S/R Stop'}</span><span className="text-nofx-gold">{language === 'zh' ? '已开' : 'on'}{pos.support_resistance_buffer != null && pos.support_resistance_buffer > 0 ? ` (${formatFull(pos.support_resistance_buffer, 2)}%)` : ''}</span></div>)}
                                                                            {pos.resistance_enabled && (<div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '阻力位止盈' : 'Resistance TP'}</span><span className="text-nofx-gold">{language === 'zh' ? '已开' : 'on'}{pos.resistance_buffer != null && pos.resistance_buffer > 0 ? ` (${formatFull(pos.resistance_buffer, 2)}%)` : ''}</span></div>)}
                                                                            <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '当前盈亏' : 'PnL'}</span><span className={`font-medium ${pos.unrealized_pnl >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>{pos.unrealized_pnl_pct >= 0 ? '+' : ''}{formatFull(pos.unrealized_pnl_pct, 3)}%</span></div>
                                                                            {pos.price_change_pct != null && (
                                                                              <div className="flex items-center gap-1.5"><span className="text-nofx-text-muted shrink-0">{language === 'zh' ? '价格变动' : 'Price chg'}</span><span className={pos.price_change_pct >= 0 ? 'text-emerald-400' : 'text-red-400'}>{pos.price_change_pct >= 0 ? '+' : ''}{formatFull(pos.price_change_pct, 3)}%</span></div>
                                                                            )}
                                                                        </div>
                                                                    )}
                                                                </div>
                                                            </td>
                                                        </tr>
                                                        )
                                                    })()}
                                                    </React.Fragment>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                    {/* Pagination footer */}
                                    {totalPositions > 10 && (
                                        <div className="flex flex-wrap items-center justify-between gap-3 pt-4 mt-4 text-xs border-t border-white/5 text-nofx-text-muted">
                                            <span>
                                                {language === 'zh'
                                                    ? `显示 ${paginatedPositions.length} / ${totalPositions} 个持仓`
                                                    : `Showing ${paginatedPositions.length} of ${totalPositions} positions`}
                                            </span>
                                            <div className="flex items-center gap-3">
                                                <div className="flex items-center gap-2">
                                                    <span>{language === 'zh' ? '每页' : 'Per page'}:</span>
                                                    <select
                                                        value={positionsPageSize}
                                                        onChange={(e) => setPositionsPageSize(Number(e.target.value))}
                                                        className="bg-black/40 border border-white/10 rounded px-2 py-1 text-xs text-nofx-text-main focus:outline-none focus:border-nofx-gold/50 transition-colors"
                                                    >
                                                        <option value={20}>20</option>
                                                        <option value={50}>50</option>
                                                        <option value={100}>100</option>
                                                    </select>
                                                </div>
                                                {totalPositionPages > 1 && (
                                                    <div className="flex items-center gap-1">
                                                        {['«', '‹', `${positionsCurrentPage} / ${totalPositionPages}`, '›', '»'].map((label, idx) => {
                                                            const isText = idx === 2;
                                                            const isFirst = idx === 0;
                                                            const isPrev = idx === 1;
                                                            const isNext = idx === 3;
                                                            const isLast = idx === 4;
                                                            if (isText) return <span key={idx} className="px-3 text-nofx-text-main">{label}</span>;

                                                            let onClick = () => { };
                                                            let disabled = false;

                                                            if (isFirst) { onClick = () => setPositionsCurrentPage(1); disabled = positionsCurrentPage === 1; }
                                                            if (isPrev) { onClick = () => setPositionsCurrentPage(p => Math.max(1, p - 1)); disabled = positionsCurrentPage === 1; }
                                                            if (isNext) { onClick = () => setPositionsCurrentPage(p => Math.min(totalPositionPages, p + 1)); disabled = positionsCurrentPage === totalPositionPages; }
                                                            if (isLast) { onClick = () => setPositionsCurrentPage(totalPositionPages); disabled = positionsCurrentPage === totalPositionPages; }

                                                            return (
                                                                <button
                                                                    key={idx}
                                                                    onClick={onClick}
                                                                    disabled={disabled}
                                                                    className={`px-2 py-1 rounded transition-colors ${disabled ? 'opacity-30 cursor-not-allowed' : 'hover:bg-white/10 text-nofx-text-main bg-white/5'}`}
                                                                >
                                                                    {label}
                                                                </button>
                                                            )
                                                        })}
                                                    </div>
                                                )}
                                            </div>
                                        </div>
                                    )}
                                </div>
                            ) : (
                                <div className="text-center py-16 text-nofx-text-muted opacity-60">
                                    <div className="text-6xl mb-4 opacity-50 grayscale">📊</div>
                                    <div className="text-lg font-semibold mb-2">{t('noPositions', language)}</div>
                                    <div className="text-sm">{t('noActivePositions', language)}</div>
                                </div>
                            )}
                        </div>
                    </div>
                </div>

                {/* 持仓 · AI 实时+预测 · 止盈止损调整 汇总板块 */}
                <div className="mt-6 nofx-glass rounded-2xl p-5 lg:p-6 border border-white/10 overflow-hidden">
                    <h2 className="text-lg font-bold flex items-center gap-2 text-nofx-text-main mb-4 uppercase tracking-wide">
                        <span className="text-amber-500/90">◇</span>
                        {language === 'zh' ? '持仓 · AI 实时+预测 · 止盈止损' : 'Positions · AI real-time+prediction · SL/TP'}
                    </h2>
                    <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
                        {/* 1. 持仓信息 */}
                        <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
                            <h3 className="text-sm font-semibold text-nofx-text-main mb-3 flex items-center gap-2">
                                <span className="text-[10px] uppercase tracking-wider text-nofx-text-muted">{language === 'zh' ? '持仓信息' : 'Positions'}</span>
                            </h3>
                            {positions && positions.length > 0 ? (
                                <div className="overflow-x-auto -mx-1">
                                    <table className="w-full text-xs">
                                        <thead>
                                            <tr className="text-left text-nofx-text-muted border-b border-white/10">
                                                <th className="py-1.5 pr-2">{t('symbol', language)}</th>
                                                <th className="py-1.5 pr-2">{t('side', language)}</th>
                                                <th className="py-1.5 pr-2 text-right">{language === 'zh' ? '入场' : 'Entry'}</th>
                                                <th className="py-1.5 pr-2 text-right">{language === 'zh' ? '标记' : 'Mark'}</th>
                                                <th className="py-1.5 pr-2 text-right">{language === 'zh' ? 'uPnL' : 'uPnL'}</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {positions.slice(0, 8).map((pos, i) => (
                                                <tr key={i} className="border-b border-white/5 text-nofx-text-main">
                                                    <td className="py-1.5 pr-2 font-mono">{pos.symbol}</td>
                                                    <td className="py-1.5 pr-2">
                                                        <span className={pos.side === 'long' ? 'text-fin-gain' : 'text-fin-loss'}>
                                                            {pos.side === 'long' ? 'L' : 'S'}
                                                        </span>
                                                    </td>
                                                    <td className="py-1.5 pr-2 text-right font-mono">{formatPrice(pos.entry_price)}</td>
                                                    <td className="py-1.5 pr-2 text-right font-mono">{formatPrice(pos.mark_price)}</td>
                                                    <td className={`py-1.5 pr-2 text-right font-medium ${pos.unrealized_pnl >= 0 ? 'text-fin-gain' : 'text-fin-loss'}`}>
                                                        {pos.unrealized_pnl >= 0 ? '+' : ''}{pos.unrealized_pnl.toFixed(2)}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                    {positions.length > 8 && (
                                        <div className="text-[10px] text-nofx-text-muted mt-1">
                                            {language === 'zh' ? `共 ${positions.length} 个，仅显示前 8` : `Showing 8 of ${positions.length}`}
                                        </div>
                                    )}
                                </div>
                            ) : (
                                <p className="text-xs text-nofx-text-muted py-4">{t('noPositions', language)}</p>
                            )}
                        </div>

                        {/* 2. AI 实时+预测 */}
                        <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
                            <h3 className="text-sm font-semibold text-nofx-text-main mb-3 flex items-center gap-2">
                                <span className="text-[10px] uppercase tracking-wider text-nofx-text-muted">{language === 'zh' ? 'AI 实时+预测' : 'AI real-time + prediction'}</span>
                            </h3>
                            <div className="space-y-2 text-xs">
                                {latestAnalysis?.market_regime && (
                                    <div><span className="text-nofx-text-muted">{language === 'zh' ? '市况' : 'Regime'}:</span> <span className="text-nofx-text-main">{latestAnalysis.market_regime}</span></div>
                                )}
                                {latestAnalysis?.scenario && (
                                    <div><span className="text-nofx-text-muted">{language === 'zh' ? '场景' : 'Scenario'}:</span> <span className="text-nofx-text-main">{latestAnalysis.scenario}</span></div>
                                )}
                                {latestAnalysis?.risk_alert && (
                                    <div className="text-amber-400 font-medium">{language === 'zh' ? '风险提示' : 'Risk alert'}</div>
                                )}
                                {latestAnalysis?.updated_at && (
                                    <div className="text-nofx-text-muted">{language === 'zh' ? '更新' : 'Updated'}: {latestAnalysis.updated_at}</div>
                                )}
                                {(directionPool?.long?.length || directionPool?.short?.length) ? (
                                    <div className="pt-1 border-t border-white/10">
                                        <div className="text-[10px] text-nofx-text-muted mb-0.5 uppercase">{language === 'zh' ? '实时方向池' : 'Realtime direction'}</div>
                                        <div className="flex flex-wrap gap-x-2 gap-y-0.5 text-[10px]">
                                            {directionPool?.long?.length ? (
                                                <span><span className="text-fin-gain">L</span>: {(directionPool.long.slice(0, 5).map((i) => i.symbol).join(', '))}{directionPool.long.length > 5 ? ` +${directionPool.long.length - 5}` : ''}</span>
                                            ) : null}
                                            {directionPool?.short?.length ? (
                                                <span><span className="text-fin-loss">S</span>: {(directionPool.short.slice(0, 5).map((i) => i.symbol).join(', '))}{directionPool.short.length > 5 ? ` +${directionPool.short.length - 5}` : ''}</span>
                                            ) : null}
                                        </div>
                                    </div>
                                ) : null}
                                {latestAnalysis?.symbol_predictions && latestAnalysis.symbol_predictions.length > 0 && (
                                    <div className="mt-2 pt-2 border-t border-white/10">
                                        <div className="text-[10px] text-nofx-text-muted mb-1 uppercase">{language === 'zh' ? '预测' : 'Predictions'}</div>
                                        <div className="flex flex-wrap gap-1.5">
                                            {latestAnalysis.symbol_predictions.slice(0, 12).map((p, i) => (
                                                <span key={i} className="px-1.5 py-0.5 rounded bg-white/5 text-nofx-text-main font-mono text-[10px]">
                                                    {p.symbol} {p.predicted_direction === 'up' ? '↑' : p.predicted_direction === 'down' ? '↓' : '−'} {p.confidence != null ? p.confidence + '%' : ''}
                                                </span>
                                            ))}
                                            {latestAnalysis.symbol_predictions.length > 12 && (
                                                <span className="text-nofx-text-muted text-[10px]">+{latestAnalysis.symbol_predictions.length - 12}</span>
                                            )}
                                        </div>
                                    </div>
                                )}
                                {latestAnalysis?.market_summary && (
                                    <div className="mt-2 pt-2 border-t border-white/10 text-nofx-text-muted text-[11px] line-clamp-3">
                                        {latestAnalysis.market_summary}
                                    </div>
                                )}
                                {!latestAnalysis?.market_regime && !latestAnalysis?.scenario && !latestAnalysis?.symbol_predictions?.length && (
                                    <p className="text-nofx-text-muted py-2">{language === 'zh' ? '暂无 AI 分析数据' : 'No AI analysis yet'}</p>
                                )}
                            </div>
                        </div>

                        {/* 3. 止盈止损调整（展示原始→调整后）；无 latest-analysis 调节时用思维链最近一条补足，与思维链一致 */}
                        <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
                            <h3 className="text-sm font-semibold text-nofx-text-main mb-3 flex items-center gap-2">
                                <span className="text-[10px] uppercase tracking-wider text-nofx-text-muted">{language === 'zh' ? '止盈止损调整' : 'SL/TP adjustments'}</span>
                            </h3>
                            {(() => {
                                const fromLatest = latestAnalysis?.position_sl_tp_adjustments?.length ? latestAnalysis.position_sl_tp_adjustments : null
                                const fromChain = status?.sltp_cycle_output_history?.length
                                    ? status.sltp_cycle_output_history[status.sltp_cycle_output_history.length - 1].adjustments
                                    : (status?.last_sltp_cycle_output?.length ? status.last_sltp_cycle_output : null)
                                const effectiveAdjustments = fromLatest ?? fromChain ?? []
                                const hasAdjustments = Array.isArray(effectiveAdjustments) && effectiveAdjustments.length > 0
                                const fromChainHint = hasAdjustments && !fromLatest && fromChain ? (language === 'zh' ? '（来自思维链最近周期）' : ' (from latest cycle in chain)') : ''
                                return hasAdjustments ? (
                                    <div className="overflow-x-auto -mx-1">
                                        {(() => {
                                            const baselines = latestAnalysis?.position_sl_tp_baselines ?? []
                                            const getBaseline = (symbol: string, side: string) =>
                                                baselines.find((b) => (b.symbol || '').toLowerCase() === (symbol || '').toLowerCase() && (b.side || '').toLowerCase() === (side || '').toLowerCase())
                                            const fmt = (orig: string | number | undefined | null, adj: string | number | undefined | null) => {
                                                const o = orig != null && orig !== '' ? String(orig) : '–'
                                                const a = adj != null && adj !== '' ? String(adj) : '–'
                                                return o === a ? a : `${o} → ${a}`
                                            }
                                            return (
                                                <table className="w-full text-xs">
                                                    <thead>
                                                        <tr className="text-left text-nofx-text-muted border-b border-white/10">
                                                            <th className="py-1.5 pr-2">Symbol</th>
                                                            <th className="py-1.5 pr-2">Side</th>
                                                            <th className="py-1.5 pr-2">{language === 'zh' ? '建议' : 'Advice'}</th>
                                                            <th className="py-1.5 pr-2">{language === 'zh' ? '追踪' : 'Trail'}</th>
                                                            <th className="py-1.5 pr-2">ATR×SL</th>
                                                            <th className="py-1.5 pr-2">{language === 'zh' ? '锁利%' : 'Lock%'}</th>
                                                            <th className="py-1.5 pr-2">ΔCycles</th>
                                                            <th className="py-1.5 pr-2">{language === 'zh' ? '阶段/退场' : 'Phase/Exit'}</th>
                                                        </tr>
                                                    </thead>
                                                    <tbody>
                                                        {effectiveAdjustments.map((row: PositionSLTPAdjustmentItem, i: number) => {
                                                            const base = getBaseline(row.symbol, row.side)
                                                            const origTrail = base?.trail_aggressiveness
                                                            const origAtr = base?.atr_mult_sl
                                                            const origLock = base?.lock_profit_pct
                                                            const origCycles = base?.confirm_cycles
                                                            const adjCycles = origCycles != null && row.confirm_cycles_delta != null ? origCycles + row.confirm_cycles_delta : row.confirm_cycles_delta
                                                            const phaseExit = [row.phase_label, row.exit_bias].filter(Boolean).join(' / ') || '–'
                                                            return (
                                                                <tr key={i} className="border-b border-white/5 text-nofx-text-main">
                                                                    <td className="py-1.5 pr-2 font-mono">{row.symbol}</td>
                                                                    <td className="py-1.5 pr-2">{row.side}</td>
                                                                    <td className="py-1.5 pr-2">{row.advice ?? '–'}</td>
                                                                    <td className="py-1.5 pr-2" title={language === 'zh' ? '原始 → 调整后' : 'Original → Adjusted'}>{fmt(origTrail, row.trail_aggressiveness)}</td>
                                                                    <td className="py-1.5 pr-2" title={language === 'zh' ? '原始 → 调整后' : 'Original → Adjusted'}>{fmt(origAtr, row.atr_mult_sl)}</td>
                                                                    <td className="py-1.5 pr-2" title={language === 'zh' ? '原始 → 调整后' : 'Original → Adjusted'}>{fmt(origLock, row.lock_profit_pct)}</td>
                                                                    <td className="py-1.5 pr-2" title={language === 'zh' ? '确认周期：原始 → 调整后' : 'Confirm cycles: original → adjusted'}>{fmt(origCycles, adjCycles)}</td>
                                                                    <td className="py-1.5 pr-2 text-[10px] text-nofx-text-muted" title={[row.invalidation_level, row.rationale].filter(Boolean).join(' | ') || undefined}>{phaseExit}</td>
                                                                </tr>
                                                            )
                                                        })}
                                                    </tbody>
                                                </table>
                                            )
                                        })()}
                                        <p className="text-[10px] text-nofx-text-muted mt-1.5">
                                            {language === 'zh' ? '策略边界已传给 AI，在边界内调整；表中为「原始 → 调整后」。' : 'Strategy bounds passed to AI; values shown as original → adjusted.'}
                                            {fromChainHint}
                                        </p>
                                    </div>
                                ) : (
                                    <p className="text-xs text-nofx-text-muted py-4">
                                        {language === 'zh' ? '无持仓或尚未有 AI 止盈止损调节' : 'No positions or no AI SL/TP adjustments yet'}
                                    </p>
                                )
                            })()}
                        </div>
                    </div>
                </div>

                {/* Position History Section */}
                {selectedTraderId && (
                    <div
                        className="nofx-glass p-6 animate-slide-in"
                        style={{ animationDelay: '0.25s' }}
                    >
                        <div className="flex items-center justify-between mb-5">
                            <h2 className="text-xl font-bold flex items-center gap-2 text-nofx-text-main">
                                <span className="text-2xl">📜</span>
                                {t('positionHistory.title', language)}
                            </h2>
                        </div>
                        <PositionHistory traderId={selectedTraderId} />
                    </div>
                )}

            {/* 系统周期/止盈止损周期 · 思维链式历史输出弹层 */}
            {cycleOutputView && (
                <div
                    className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm"
                    onClick={() => setCycleOutputView(null)}
                    role="dialog"
                    aria-modal="true"
                    aria-label={language === 'zh' ? '周期输出思维链' : 'Cycle output chain'}
                >
                    <div
                        className="nofx-glass rounded-2xl border border-white/15 max-w-2xl w-full max-h-[85vh] overflow-hidden flex flex-col shadow-xl"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <div className="flex items-center justify-between p-4 border-b border-white/10">
                            <h3 className="text-lg font-bold text-nofx-text-main">
                                {cycleOutputView === 'system'
                                    ? (language === 'zh' ? '系统周期 · 思维链' : 'System cycle · Chain')
                                    : (language === 'zh' ? '止盈止损周期 · 思维链' : 'SL/TP cycle · Chain')}
                                {(cycleOutputView === 'system' ? (status?.system_cycle_output_history?.length ?? 0) : (status?.sltp_cycle_output_history?.length ?? 0)) > 0 && (
                                    <span className="ml-2 text-sm font-normal text-nofx-text-muted">
                                        {language === 'zh'
                                            ? `最近 ${cycleOutputView === 'system' ? (status?.system_cycle_output_history?.length ?? 0) : (status?.sltp_cycle_output_history?.length ?? 0)} 条`
                                            : `Last ${cycleOutputView === 'system' ? (status?.system_cycle_output_history?.length ?? 0) : (status?.sltp_cycle_output_history?.length ?? 0)}`}
                                    </span>
                                )}
                            </h3>
                            <button
                                type="button"
                                onClick={() => setCycleOutputView(null)}
                                className="p-2 rounded-lg hover:bg-white/10 text-nofx-text-muted hover:text-white transition-colors"
                                aria-label={language === 'zh' ? '关闭' : 'Close'}
                            >
                                ×
                            </button>
                        </div>
                        <div className="p-4 overflow-y-auto flex-1 text-sm space-y-4">
                            {cycleOutputView === 'system' && (() => {
                                const history = status?.system_cycle_output_history && status.system_cycle_output_history.length > 0
                                    ? status.system_cycle_output_history
                                    : (status?.last_system_cycle_summary
                                        ? [{ at: '', cycle_number: status.system_cycle_count ?? 0, summary: status.last_system_cycle_summary }]
                                        : [])
                                if (history.length === 0) {
                                    return <p className="text-nofx-text-muted">{language === 'zh' ? '暂无输出' : 'No output yet'}</p>
                                }
                                return (
                                    <ul className="space-y-3 list-none">
                                        {history.map((entry, idx) => (
                                            <li key={idx} className="rounded-lg border border-white/10 bg-white/[0.04] p-3">
                                                <div className="flex items-center gap-2 mb-2 text-nofx-text-muted text-xs">
                                                    {entry.at && <span>{new Date(entry.at).toLocaleString()}</span>}
                                                    <span>{language === 'zh' ? `周期 #${entry.cycle_number}` : `Cycle #${entry.cycle_number}`}</span>
                                                </div>
                                                <pre className="whitespace-pre-wrap font-sans text-nofx-text-main text-sm">{entry.summary}</pre>
                                            </li>
                                        ))}
                                    </ul>
                                )
                            })()}
                            {cycleOutputView === 'sltp' && (() => {
                                const history = status?.sltp_cycle_output_history && status.sltp_cycle_output_history.length > 0
                                    ? status.sltp_cycle_output_history
                                    : (status?.last_sltp_cycle_output && status.last_sltp_cycle_output.length > 0
                                        ? [{ at: '', cycle_number: status.sltp_analysis_cycle_count ?? 0, adjustments: status.last_sltp_cycle_output, raw_output: '' }]
                                        : [])
                                if (history.length === 0) {
                                    return <p className="text-nofx-text-muted">{language === 'zh' ? '暂无输出（无持仓或尚未执行止盈止损分析周期）' : 'No output (no positions or SL/TP cycle not run yet)'}</p>
                                }
                                return (
                                    <ul className="space-y-4 list-none">
                                        {history.map((entry, idx) => (
                                            <li key={idx} className="rounded-lg border border-white/10 bg-white/[0.04] p-3">
                                                <details className="group">
                                                    <summary className="flex items-center justify-between cursor-pointer select-none list-none">
                                                        <div className="flex items-center gap-2 text-nofx-text-muted text-xs">
                                                            {entry.at && (
                                                                <span title={entry.scheduled_at ? `${language === 'zh' ? '计划:' : 'Scheduled:'} ${new Date(entry.scheduled_at).toLocaleString()}` : undefined}>
                                                                    {new Date(entry.at).toLocaleString()}
                                                                </span>
                                                            )}
                                                            <span>{language === 'zh' ? `周期 #${entry.cycle_number}` : `Cycle #${entry.cycle_number}`}</span>
                                                            {entry.adjustments && entry.adjustments.length > 0 ? (
                                                                <span className="ml-2 text-nofx-text-main/80">
                                                                    {language === 'zh' ? `调节 ${entry.adjustments.length} 项` : `${entry.adjustments.length} adjustments`}
                                                                </span>
                                                            ) : (
                                                                <span className="ml-2 text-nofx-text-muted/80">
                                                                    {language === 'zh' ? '无调节' : 'No adjustments'}
                                                                </span>
                                                            )}
                                                        </div>
                                                        <span className="text-nofx-text-muted group-open:rotate-180 transition-transform">▾</span>
                                                    </summary>
                                                    <div className="mt-3 space-y-3">
                                                        {entry.raw_output ? (
                                                            <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                                                                <div className="text-xs text-nofx-text-muted mb-2">
                                                                    {language === 'zh' ? 'AI 原始输出' : 'AI raw output'}
                                                                </div>
                                                                <pre className="whitespace-pre-wrap font-mono text-xs text-nofx-text-main max-h-[260px] overflow-auto">
                                                                    {entry.raw_output}
                                                                </pre>
                                                            </div>
                                                        ) : (
                                                            <p className="text-nofx-text-muted text-xs">
                                                                {language === 'zh' ? '本周期未记录 AI 原始输出' : 'AI raw output not recorded for this cycle'}
                                                            </p>
                                                        )}
                                                        {entry.adjustments && entry.adjustments.length > 0 ? (
                                                            <div className="overflow-x-auto">
                                                                <table className="w-full text-left border-collapse text-xs">
                                                                    <thead>
                                                                        <tr className="border-b border-white/10 text-nofx-text-muted uppercase">
                                                                            <th className="py-1.5 pr-2">Symbol</th>
                                                                            <th className="py-1.5 pr-2">Side</th>
                                                                            <th className="py-1.5 pr-2">{language === 'zh' ? '建议' : 'Advice'}</th>
                                                                            <th className="py-1.5 pr-2">{language === 'zh' ? '追踪' : 'Trail'}</th>
                                                                            <th className="py-1.5 pr-2">ATR×SL</th>
                                                                            <th className="py-1.5 pr-2">{language === 'zh' ? '锁利%' : 'Lock%'}</th>
                                                                            <th className="py-1.5 pr-2">ΔCycles</th>
                                                                        </tr>
                                                                    </thead>
                                                                    <tbody>
                                                                        {entry.adjustments.map((row, i) => (
                                                                            <tr key={i} className="border-b border-white/5 text-nofx-text-main">
                                                                                <td className="py-1 pr-2 font-mono">{row.symbol}</td>
                                                                                <td className="py-1 pr-2">{row.side}</td>
                                                                                <td className="py-1 pr-2">{row.advice ?? '–'}</td>
                                                                                <td className="py-1 pr-2">{row.trail_aggressiveness ?? '–'}</td>
                                                                                <td className="py-1 pr-2">{row.atr_mult_sl != null ? row.atr_mult_sl : '–'}</td>
                                                                                <td className="py-1 pr-2">{row.lock_profit_pct != null ? row.lock_profit_pct : '–'}</td>
                                                                                <td className="py-1 pr-2">{row.confirm_cycles_delta != null ? row.confirm_cycles_delta : '–'}</td>
                                                                            </tr>
                                                                        ))}
                                                                    </tbody>
                                                                </table>
                                                            </div>
                                                        ) : null}
                                                    </div>
                                                </details>
                                            </li>
                                        ))}
                                    </ul>
                                )
                            })()}
                        </div>
                    </div>
                </div>
            )}
            </main>
        </DeepVoidBackground>
    )
}

// Stat Card - 模板: KPI 卡片 rounded-2xl, p-4 lg:p-5, 区块标题 text-sm font-bold font-display, 数值 tracking-tighter, fin-gain/fin-loss
function StatCard({
    title,
    value,
    unit,
    change,
    positive,
    subtitle,
    icon,
}: {
    title: string
    value: string
    unit?: string
    change?: number
    positive?: boolean
    subtitle?: string
    icon?: string
}) {
    return (
        <div className="group nofx-glass rounded-2xl p-4 lg:p-5 transition-all duration-300 hover:border-primary/30 relative overflow-hidden">
            <div className="absolute top-0 right-0 p-4 opacity-5 group-hover:opacity-10 transition-opacity text-4xl grayscale group-hover:grayscale-0">
                {icon}
            </div>
            <div className="text-sm mb-2 font-bold font-display text-nofx-text-muted flex items-center gap-2">
                {title}
            </div>
            <div className="flex items-baseline gap-1 mb-1">
                <div className="text-2xl lg:text-3xl font-bold font-mono text-nofx-text-main tracking-tighter group-hover:text-white transition-colors">
                    {value}
                </div>
                {unit && <span className="text-xs font-mono text-nofx-text-muted opacity-60">{unit}</span>}
            </div>

            {change !== undefined && (
                <div className="flex items-center gap-1">
                    <div
                        className={`text-xs font-bold font-mono flex items-center gap-1 ${positive ? 'text-fin-gain' : 'text-fin-loss'}`}
                    >
                        <span>{positive ? '▲' : '▼'}</span>
                        <span>{positive ? '+' : ''}{change.toFixed(2)}%</span>
                    </div>
                </div>
            )}
            {subtitle && (
                <div className="text-[11px] mt-2 font-sans text-nofx-text-muted opacity-80">
                    {subtitle}
                </div>
            )}
        </div>
    )
}
