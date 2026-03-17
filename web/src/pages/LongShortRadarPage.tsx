import React, { useState, useEffect } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type {
  TraderInfo,
  SystemStatus,
  RadarConfig,
  OrderFlowInfo,
  DirectionPoolResponse,
  LatestAnalysisResponse,
  DirectionPoolItem,
} from '../types'
import { DeepVoidBackground } from '../components/DeepVoidBackground'
import { GrainOverlay } from '../components/ui/GrainOverlay'
import { RefreshCw, AlertCircle, Settings2, Loader2, ChevronDown, ChevronRight } from 'lucide-react'

const RELAX_LAYER1_KEY = 'radar_relax_layer1'

interface LongShortRadarPageProps {
  liveTraders: TraderInfo[]
  simulationTraders: TraderInfo[]
  isRadarDataLoading?: boolean
  onRefreshRadar?: () => void
  selectedTraderId: string | undefined
  onTraderSelect: (traderId: string) => void
  status: SystemStatus | undefined
}

export function LongShortRadarPage({
  liveTraders,
  simulationTraders,
  isRadarDataLoading = false,
  onRefreshRadar,
  selectedTraderId,
  onTraderSelect,
  status,
}: LongShortRadarPageProps) {
  const allTraders = [...liveTraders, ...simulationTraders]
  const traders = allTraders
  const [mainTab, setMainTab] = useState<'filter' | 'radar'>('radar')
  const [radarTab, setRadarTab] = useState<'preset' | 'manual' | 'close_auto'>('close_auto')
  const [relaxLayer1, setRelaxLayer1] = useState(false)

  const { data: radarConfig, mutate: mutateRadarConfig } = useSWR<RadarConfig>(
    selectedTraderId ? `radar-config-${selectedTraderId}` : null,
    () => api.getRadarConfig(selectedTraderId!),
    { refreshInterval: 30000 }
  )
  const config = radarConfig ?? defaultRadarConfig()
  useEffect(() => {
    if (config.mode === 'preset' || config.mode === 'manual' || config.mode === 'close_auto') {
      setRadarTab(config.mode)
    }
  }, [config.mode])

  const { data: orderFlow, mutate: mutateOrderFlow } = useSWR<OrderFlowInfo>(
    selectedTraderId ? `order-flow-${selectedTraderId}` : null,
    () => api.getOrderFlowInfo(selectedTraderId!),
    { refreshInterval: 15000 }
  )
  const { data: directionPool, mutate: mutateDirectionPool } = useSWR<DirectionPoolResponse>(
    selectedTraderId ? `direction-pool-${selectedTraderId}` : null,
    () => api.getDirectionPool(selectedTraderId!),
    { refreshInterval: 15000 }
  )
  const { data: latestAnalysis, mutate: mutateLatestAnalysis } = useSWR<LatestAnalysisResponse>(
    selectedTraderId ? `latest-analysis-${selectedTraderId}` : null,
    () => api.getLatestAnalysis(selectedTraderId!),
    { refreshInterval: 20000 }
  )

  useEffect(() => {
    if (!selectedTraderId) return
    const raw = localStorage.getItem(`${RELAX_LAYER1_KEY}_${selectedTraderId}`)
    setRelaxLayer1(raw === '1')
  }, [selectedTraderId])

  const handleRelaxLayer1 = () => {
    if (!selectedTraderId) return
    const next = !relaxLayer1
    setRelaxLayer1(next)
    localStorage.setItem(`${RELAX_LAYER1_KEY}_${selectedTraderId}`, next ? '1' : '0')
  }

  const hasCandidates =
    (directionPool?.long?.length ?? 0) > 0 || (directionPool?.short?.length ?? 0) > 0
  const allowLong = config.allow_long
  const allowShort = config.allow_short
  const noCandidateHint =
    !status?.is_running ||
    !allowLong && !allowShort
      ? '当前交易员未启动或未勾选允许做多/做空'
      : '暂无候选币种，请检查：当前交易员是否已启动；币种源是否有配置；是否勾选「允许做多」「允许做空」'

  const updateRadarConfig = async (patch: Partial<RadarConfig>) => {
    if (!selectedTraderId) return
    const next = { ...config, ...patch }
    await api.putRadarConfig(selectedTraderId, next)
    await mutateRadarConfig()
  }

  const refreshAll = () => {
    if (!selectedTraderId) return
    mutateRadarConfig()
    mutateOrderFlow()
    mutateDirectionPool()
    mutateLatestAnalysis()
  }

  if (isRadarDataLoading) {
    return (
      <DeepVoidBackground className="min-h-screen" disableAnimation>
        <GrainOverlay />
        <div className="relative z-10 flex flex-col items-center justify-center min-h-[50vh] text-[#848E9C] gap-3">
          <Loader2 className="w-8 h-8 animate-spin" aria-hidden />
          <p>正在加载交易员列表…</p>
        </div>
      </DeepVoidBackground>
    )
  }

  if (allTraders.length === 0) {
    return (
      <DeepVoidBackground className="min-h-screen" disableAnimation>
        <GrainOverlay />
        <div className="relative z-10 flex flex-col items-center justify-center min-h-[50vh] text-[#848E9C] gap-4 px-4 text-center">
          <p>暂无交易员，请先在「交易员」或「实盘模拟」中创建并选择交易员后再使用多空雷达。</p>
          <p className="text-sm text-[#5E6673]">若你已创建实盘模拟交易员，请点击下方刷新或稍候重试。</p>
          {onRefreshRadar && (
            <button
              type="button"
              onClick={onRefreshRadar}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[#2B3139] border border-[#3B82F6] text-[#93C5FD] hover:bg-[#3B82F6]/20 transition-colors"
            >
              <RefreshCw className="w-4 h-4" />
              刷新列表
            </button>
          )}
        </div>
      </DeepVoidBackground>
    )
  }

  const selectedTrader = allTraders.find((t) => t.trader_id === selectedTraderId) ?? allTraders[0]
  const effectiveTraderId = selectedTraderId ?? selectedTrader?.trader_id
  if (!effectiveTraderId && allTraders[0]) {
    onTraderSelect(allTraders[0].trader_id)
  }

  return (
    <DeepVoidBackground className="min-h-screen pb-12" disableAnimation>
      <GrainOverlay />
      <div className="relative z-10 w-full max-w-[1400px] mx-auto px-4 py-6">
        {/* 交易员选择：实盘 + 实盘模拟 */}
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <label className="text-sm text-[#848E9C]">交易员</label>
            <select
              className="bg-[#1E2329] border border-[#2B3139] rounded-lg px-3 py-2 text-[#EAECEF] min-w-[260px]"
              value={effectiveTraderId ?? ''}
              onChange={(e) => onTraderSelect(e.target.value)}
            >
              {liveTraders.length > 0 && (
                <optgroup label="实盘">
                  {liveTraders.map((t) => (
                    <option key={t.trader_id} value={t.trader_id}>
                      {t.trader_name}
                    </option>
                  ))}
                </optgroup>
              )}
              {simulationTraders.length > 0 && (
                <optgroup label="实盘模拟">
                  {simulationTraders.map((t) => (
                    <option key={t.trader_id} value={t.trader_id}>
                      {t.trader_name}
                    </option>
                  ))}
                </optgroup>
              )}
            </select>
          </div>
          <button
            type="button"
            onClick={refreshAll}
            className="flex items-center gap-2 px-3 py-2 rounded-lg bg-[#1E2329] border border-[#2B3139] text-[#EAECEF] hover:border-[#F0B90B] transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            刷新
          </button>
        </div>

        {/* 提示条 */}
        {!hasCandidates && (
          <div
            className="flex items-center gap-2 mb-4 px-4 py-3 rounded-lg border border-[#3B82F6]/50 bg-[#3B82F6]/10 text-[#93C5FD]"
            role="alert"
          >
            <AlertCircle className="w-5 h-5 flex-shrink-0" />
            <span>{noCandidateHint}</span>
          </div>
        )}

        {/* 主区域：过滤机制 | 多空雷达 */}
        <div className="rounded-xl border border-[#2B3139] bg-[#181A20]/80 overflow-hidden">
          <div className="flex border-b border-[#2B3139]">
            {[
              { key: 'filter' as const, label: '过滤机制' },
              { key: 'radar' as const, label: '多空雷达' },
            ].map(({ key, label }) => (
              <button
                key={key}
                type="button"
                onClick={() => setMainTab(key)}
                className={`px-6 py-3 text-sm font-medium transition-colors ${
                  mainTab === key
                    ? 'text-[#F0B90B] border-b-2 border-[#F0B90B] bg-[#1E2329]/50'
                    : 'text-[#848E9C] hover:text-[#EAECEF]'
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          {mainTab === 'filter' && (
            <div className="p-4 sm:p-6 space-y-4">
              <div className="flex flex-wrap items-center gap-6">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" checked={allowLong} onChange={(e) => updateRadarConfig({ allow_long: e.target.checked })} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
                  <span className="text-[#EAECEF]">允许做多</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" checked={allowShort} onChange={(e) => updateRadarConfig({ allow_short: e.target.checked })} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
                  <span className="text-[#EAECEF]">允许做空</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" checked={config.ai_auto_analysis} onChange={(e) => updateRadarConfig({ ai_auto_analysis: e.target.checked })} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
                  <span className="text-[#EAECEF]">AI自动分析</span>
                </label>
                <button type="button" onClick={() => updateRadarConfig(defaultRadarConfig())} className="text-sm text-[#848E9C] hover:text-[#F0B90B]" title="一键恢复多空雷达预设">一键恢复预设</button>
              </div>
              <div className="flex border-b border-[#2B3139] pb-2 mb-2">
                {[{ key: 'preset' as const, label: '预设' }, { key: 'manual' as const, label: '手动' }, { key: 'close_auto' as const, label: '平仓(自动)' }].map(({ key, label }) => (
                  <button key={key} type="button" onClick={() => { setRadarTab(key); updateRadarConfig({ mode: key }) }} className={`px-4 py-2 text-xs font-medium rounded-t ${radarTab === key ? 'text-[#F0B90B] bg-[#2B3139]' : 'text-[#848E9C] hover:text-[#EAECEF]'}`}>{label}</button>
                ))}
              </div>
              <div className="flex flex-wrap items-center gap-6">
                <MetricChip label="HeatScore" value={config.heat_score} highlight onChange={(v) => updateRadarConfig({ heat_score: v })} />
                <MetricChip label="ATR%" value={config.atr_pct} suffix="%" onChange={(v) => updateRadarConfig({ atr_pct: v })} />
                <MetricChip label="资金临界%" value={config.capital_critical_pct} suffix="%" onChange={(v) => updateRadarConfig({ capital_critical_pct: v })} />
                <MetricChip label="可靠度门" value={config.reliability_gate_pct} suffix="%" onChange={(v) => updateRadarConfig({ reliability_gate_pct: v })} />
                <MetricChip label="方向分位" value={config.direction_quantile} onChange={(v) => updateRadarConfig({ direction_quantile: v })} />
                <div className="flex items-center gap-2">
                  <span className="text-[#848E9C] text-sm">方向问</span>
                  <input type="range" min={1} max={5} value={config.direction_query} onChange={(e) => updateRadarConfig({ direction_query: +e.target.value })} className="w-24 h-2 rounded bg-[#2B3139] accent-[#F0B90B]" />
                  <span className="text-[#EAECEF] w-6">{config.direction_query}</span>
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="rounded-lg border border-[#2B3139] bg-[#1E2329]/50 p-4">
                  <div className="flex items-center gap-2 text-[#848E9C] mb-2"><Settings2 className="w-4 h-4" /><span className="text-sm font-medium">AI参数快照</span></div>
                  <p className="text-sm text-[#5E6673]">已进化 · 全自动进化 · 暂无快照</p>
                </div>
                <div className="rounded-lg border border-[#2B3139] bg-[#1E2329]/50 p-4">
                  <div className="text-sm font-medium text-[#848E9C] mb-2">挂单候选</div>
                  <p className="text-sm text-[#5E6673]">排除已持仓 · 排除已挂未触发 · 挂单上限% · 暂无建议</p>
                </div>
              </div>
              {orderFlow && <OrderFlowCard orderFlow={orderFlow} relaxLayer1={relaxLayer1} onRelaxLayer1={handleRelaxLayer1} />}
            </div>
          )}

          {mainTab === 'radar' && (
            <div className="p-4 sm:p-6 space-y-4">
              <div className="flex flex-wrap items-center gap-4 mb-2">
                <span className="text-[#848E9C] text-sm">方向池</span>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" checked={allowLong} onChange={(e) => updateRadarConfig({ allow_long: e.target.checked })} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
                  <span className="text-[#EAECEF] text-sm">允许做多</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" checked={allowShort} onChange={(e) => updateRadarConfig({ allow_short: e.target.checked })} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
                  <span className="text-[#EAECEF] text-sm">允许做空</span>
                </label>
              </div>
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <DirectionPoolTableWithDetail title="方向池(多)" items={directionPool?.long ?? []} allowAutoIssue={config.long_pool_auto_issue} allowAutoCancel={config.long_pool_auto_cancel} onAutoIssueChange={(v) => updateRadarConfig({ long_pool_auto_issue: v })} onAutoCancelChange={(v) => updateRadarConfig({ long_pool_auto_cancel: v })} />
                <DirectionPoolTableWithDetail title="方向池(空)" items={directionPool?.short ?? []} allowAutoIssue={config.short_pool_auto_issue} allowAutoCancel={config.short_pool_auto_cancel} onAutoIssueChange={(v) => updateRadarConfig({ short_pool_auto_issue: v })} onAutoCancelChange={(v) => updateRadarConfig({ short_pool_auto_cancel: v })} />
              </div>
              <RealtimeAndAIPanel directionPool={directionPool} latestAnalysis={latestAnalysis} />
            </div>
          )}
        </div>
      </div>
    </DeepVoidBackground>
  )
}

function defaultRadarConfig(): RadarConfig {
  return {
    allow_long: true,
    allow_short: true,
    ai_auto_analysis: true,
    mode: 'close_auto',
    heat_score: 0.7,
    atr_pct: 0.6,
    capital_critical_pct: 0,
    reliability_gate_pct: 40,
    direction_quantile: 0.72,
    direction_query: 3,
    exclude_held: true,
    exclude_pending: true,
    pending_order_cap_pct: 5,
    long_pool_auto_issue: true,
    long_pool_auto_cancel: true,
    short_pool_auto_issue: true,
    short_pool_auto_cancel: true,
    pool_quantile_pct: 40,
  }
}

function MetricChip({
  label,
  value,
  suffix = '',
  highlight,
  onChange,
}: {
  label: string
  value: number
  suffix?: string
  highlight?: boolean
  onChange?: (v: number) => void
}) {
  const display = suffix === '%' && value <= 1 && value > 0 ? `${(value * 100).toFixed(0)}${suffix}` : `${value}${suffix}`
  return (
    <div className="flex items-center gap-2">
      <span className="text-[#848E9C] text-sm">{label}</span>
      <span className={`px-2 py-1 rounded text-sm font-medium ${highlight ? 'text-[#F0B90B] bg-[#F0B90B]/10' : 'text-[#EAECEF] bg-[#2B3139]'}`}>
        {display}
      </span>
      {onChange && (
        <input
          type="number"
          step={0.01}
          value={value}
          onChange={(e) => onChange(parseFloat(e.target.value) || 0)}
          className="w-16 bg-[#1E2329] border border-[#2B3139] rounded px-2 py-1 text-sm text-[#EAECEF]"
        />
      )}
    </div>
  )
}

/** 第一层条件简短说明（与 kernel layer1ConditionNames 对应，供前端展示） */
const LAYER1_CONDITION_DESC: Record<string, string> = {
  '入场时机非now/soon': '入场时机需为 now 或 soon',
  '量能未达标': '量能条件不满足',
  '持仓量/OI未达标': 'OI/持仓量条件不满足',
  '长周期未对齐': '长周期趋势未与方向一致',
  '流向未同向': '资金流向与方向不一致',
  '涨跌榜未同向': '涨跌榜与方向不一致',
  '可靠度不足': '可靠度低于阈值',
  '短周期未对齐': '短周期趋势未与方向一致',
  '大户多空未同向': '大户多空与方向不一致',
  '全市场多空未同向': '全市场多空与方向不一致',
  '趋势强度不足': '趋势强度低于阈值',
  'RSI区间不符': 'RSI 不在允许区间',
  'MACD信号不符': 'MACD 信号与方向不一致',
  '量价趋势未同向': '量价趋势与方向不一致',
  'OI趋势未同向': 'OI 趋势与方向不一致',
  '资金费率不符': '资金费率条件不满足',
}

function OrderFlowCard({
  orderFlow,
  relaxLayer1,
  onRelaxLayer1,
}: {
  orderFlow: OrderFlowInfo
  relaxLayer1: boolean
  onRelaxLayer1: () => void
}) {
  const [showFilterDetail, setShowFilterDetail] = useState(true)
  const [showSystemFilterDetail, setShowSystemFilterDetail] = useState(true)
  const [expandedCoins, setExpandedCoins] = useState<Set<number>>(new Set())
  const { pipeline, layer1_failure_stats, per_coin_failures, pipeline_description, process_stage, updated_at } = orderFlow
  const toggleCoin = (i: number) => {
    setExpandedCoins((prev) => {
      const next = new Set(prev)
      if (next.has(i)) next.delete(i)
      else next.add(i)
      return next
    })
  }
  const totalPool = pipeline.pool_long + pipeline.pool_short
  return (
    <div className="rounded-lg border border-[#2B3139] bg-[#1E2329]/50 p-4 space-y-4">
      <div className="flex items-center justify-between flex-wrap gap-2">
        <h3 className="text-sm font-medium text-[#EAECEF]">挂单流程信息</h3>
        <span className="text-xs text-[#5E6673]">更新于 {updated_at}</span>
      </div>
      <p className="text-sm text-[#848E9C]">流程阶段：{process_stage}</p>

      {/* 详细信息：过滤流程详情 */}
      <div className="rounded-lg border border-[#2B3139] bg-[#181A20] overflow-hidden">
        <button
          type="button"
          onClick={() => setShowFilterDetail((v) => !v)}
          className="w-full flex items-center justify-between px-4 py-3 text-left text-sm font-medium text-[#EAECEF] hover:bg-[#2B3139]/50"
        >
          <span>过滤流程详情</span>
          {showFilterDetail ? <ChevronDown className="w-4 h-4 text-[#848E9C]" /> : <ChevronRight className="w-4 h-4 text-[#848E9C]" />}
        </button>
        {showFilterDetail && (
          <div className="px-4 pb-4 space-y-3">
            <div className="flex flex-wrap items-center gap-2 sm:gap-4 text-sm">
              <span className="text-[#848E9C]">方向池(多+空)</span>
              <span className="text-[#EAECEF] font-medium">{totalPool}</span>
              <span className="text-[#5E6673]">（多 {pipeline.pool_long} + 空 {pipeline.pool_short}）</span>
              <span className="text-[#5E6673]">→</span>
              <span className="text-[#848E9C]">第一层后</span>
              <span className="text-[#EAECEF] font-medium">{pipeline.after_layer1}</span>
              <span className="text-[#5E6673]">→</span>
              <span className="text-[#848E9C]">第二层后</span>
              <span className="text-[#EAECEF] font-medium">{pipeline.after_layer2}</span>
              <span className="text-[#5E6673]">→</span>
              <span className="text-[#848E9C]">待提交</span>
              <span className="text-[#F0B90B] font-medium">{pipeline.to_submit}</span>
            </div>
            <p className="text-xs text-[#5E6673]">管线标签：{pipeline.flow_label}</p>
            <p className="text-xs text-[#5E6673]">{pipeline_description}</p>
          </div>
        )}
      </div>

      {/* 系统过滤的详细信息 */}
      <div className="rounded-lg border border-[#2B3139] bg-[#181A20] overflow-hidden">
        <button
          type="button"
          onClick={() => setShowSystemFilterDetail((v) => !v)}
          className="w-full flex items-center justify-between px-4 py-3 text-left text-sm font-medium text-[#EAECEF] hover:bg-[#2B3139]/50"
        >
          <span>系统过滤的详细信息</span>
          {showSystemFilterDetail ? <ChevronDown className="w-4 h-4 text-[#848E9C]" /> : <ChevronRight className="w-4 h-4 text-[#848E9C]" />}
        </button>
        {showSystemFilterDetail && (
          <div className="px-4 pb-4 space-y-4">
            <p className="text-xs text-[#5E6673]">
              第一层：16 项条件逐项检查，未通过则计入统计；第二层：至少 6 因子 + 可靠度≥0.67 + 入场信心≥31%；第三层：OI 对齐 + 信号年龄&lt;5 分钟。
            </p>
            {layer1_failure_stats.length > 0 && (
              <div>
                <p className="text-xs font-medium text-[#848E9C] mb-2">第一层(16项) 不达标统计</p>
                <div className="overflow-x-auto rounded border border-[#2B3139]">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-left text-[#848E9C] border-b border-[#2B3139] bg-[#1E2329]">
                        <th className="px-3 py-2">条件</th>
                        <th className="px-3 py-2 w-24">未通过次数</th>
                        <th className="px-3 py-2 hidden sm:table-cell">说明</th>
                      </tr>
                    </thead>
                    <tbody>
                      {layer1_failure_stats.map((s, i) => (
                        <tr key={i} className="border-b border-[#2B3139]/50">
                          <td className="px-3 py-2 text-[#EAECEF]">{s.condition}</td>
                          <td className="px-3 py-2 text-[#EAECEF]">{s.count}</td>
                          <td className="px-3 py-2 text-[#5E6673] text-xs hidden sm:table-cell">{LAYER1_CONDITION_DESC[s.condition] ?? '—'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
            {per_coin_failures.length > 0 && (
              <div>
                <p className="text-xs font-medium text-[#848E9C] mb-2">按币种未通过原因（第一层）</p>
                <div className="overflow-x-auto rounded border border-[#2B3139]">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-left text-[#848E9C] border-b border-[#2B3139] bg-[#1E2329]">
                        <th className="px-3 py-2 w-8" aria-label="展开" />
                        <th className="px-3 py-2">标的</th>
                        <th className="px-3 py-2">未通过原因</th>
                      </tr>
                    </thead>
                    <tbody>
                      {per_coin_failures.map((r, i) => (
                        <React.Fragment key={i}>
                          <tr className="border-b border-[#2B3139]/50 hover:bg-[#2B3139]/30">
                            <td className="px-2 py-2">
                              {r.reasons.length > 1 ? (
                                <button type="button" onClick={() => toggleCoin(i)} className="p-0.5 text-[#848E9C] hover:text-[#F0B90B]">
                                  {expandedCoins.has(i) ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                                </button>
                              ) : <span className="w-4 inline-block" />}
                            </td>
                            <td className="px-3 py-2 text-[#EAECEF] font-medium">{r.symbol}</td>
                            <td className="px-3 py-2 text-[#848E9C]">
                              {expandedCoins.has(i) ? r.reasons.map((reason, j) => (
                                <span key={j} className="inline-block mr-1 mb-1 px-2 py-0.5 rounded bg-[#2B3139] text-xs">{reason}</span>
                              )) : r.reasons.join('；')}
                            </td>
                          </tr>
                        </React.Fragment>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
            {layer1_failure_stats.length === 0 && per_coin_failures.length === 0 && (
              <p className="text-xs text-[#5E6673]">本周期暂无第一层不达标记录（全部通过或方向池为空）</p>
            )}
          </div>
        )}
      </div>

      <div className="flex items-center gap-2 pt-2 flex-wrap">
        <button
          type="button"
          onClick={onRelaxLayer1}
          className={`px-3 py-1.5 rounded text-sm border transition-colors ${
            relaxLayer1
              ? 'bg-[#F0B90B]/20 border-[#F0B90B] text-[#F0B90B]'
              : 'bg-[#1E2329] border-[#2B3139] text-[#848E9C] hover:border-[#F0B90B] hover:text-[#EAECEF]'
          }`}
        >
          临时放宽第一层16项
        </button>
        <span className="text-xs text-[#5E6673]">仅对自动下发第一层16项生效，写入 localStorage，不修改策略</span>
      </div>
    </div>
  )
}

function DirectionPoolTable({
  title,
  items,
  allowAutoIssue,
  allowAutoCancel,
  onAutoIssueChange,
  onAutoCancelChange,
}: {
  title: string
  items: import('../types').DirectionPoolItem[]
  allowAutoIssue: boolean
  allowAutoCancel: boolean
  onAutoIssueChange: (v: boolean) => void
  onAutoCancelChange: (v: boolean) => void
}) {
  return (
    <div className="rounded-lg border border-[#2B3139] bg-[#1E2329]/50 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#2B3139]">
        <h3 className="text-sm font-medium text-[#EAECEF]">{title}</h3>
        <div className="flex items-center gap-4">
          <label className="flex items-center gap-1.5 cursor-pointer text-xs text-[#848E9C]">
            <input type="checkbox" checked={allowAutoIssue} onChange={(e) => onAutoIssueChange(e.target.checked)} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
            自动下发
          </label>
          <label className="flex items-center gap-1.5 cursor-pointer text-xs text-[#848E9C]">
            <input type="checkbox" checked={allowAutoCancel} onChange={(e) => onAutoCancelChange(e.target.checked)} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
            自动撤单
          </label>
        </div>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-[#848E9C] border-b border-[#2B3139]">
              <th className="px-4 py-2">标的</th>
              <th className="px-4 py-2">强度</th>
              <th className="px-4 py-2">评分</th>
              <th className="px-4 py-2">市况</th>
              <th className="px-4 py-2">可靠度</th>
              <th className="px-4 py-2">时机</th>
              <th className="px-4 py-2">量价</th>
            </tr>
          </thead>
          <tbody>
            {items.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-4 py-6 text-center text-[#5E6673]">当前无{title.includes('多') ? '做多' : '做空'}候选</td>
              </tr>
            ) : (
              items.map((row, i) => (
                <tr key={`${row.symbol}-${i}`} className="border-b border-[#2B3139]/50 hover:bg-[#2B3139]/30">
                  <td className="px-4 py-2 text-[#EAECEF] font-medium">{row.symbol}</td>
                  <td className="px-4 py-2 text-[#EAECEF]">{row.strength_pct}%</td>
                  <td className="px-4 py-2 text-[#EAECEF]">{row.score.toFixed(2)}</td>
                  <td className="px-4 py-2 text-[#848E9C]">{row.market_condition}</td>
                  <td className="px-4 py-2 text-[#EAECEF]">{row.reliability_pct}%</td>
                  <td className="px-4 py-2 text-[#848E9C]">{row.timing}</td>
                  <td className="px-4 py-2 text-[#EAECEF]">{row.volume_price_pct}%</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

/** 方向池表格 + 可展开的币种详细信息 */
function DirectionPoolTableWithDetail({
  title,
  items,
  allowAutoIssue,
  allowAutoCancel,
  onAutoIssueChange,
  onAutoCancelChange,
}: {
  title: string
  items: DirectionPoolItem[]
  allowAutoIssue: boolean
  allowAutoCancel: boolean
  onAutoIssueChange: (v: boolean) => void
  onAutoCancelChange: (v: boolean) => void
}) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const toggle = (symbol: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(symbol)) next.delete(symbol)
      else next.add(symbol)
      return next
    })
  }
  return (
    <div className="rounded-lg border border-[#2B3139] bg-[#1E2329]/50 overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#2B3139]">
        <h3 className="text-sm font-medium text-[#EAECEF]">{title}</h3>
        <div className="flex items-center gap-4">
          <label className="flex items-center gap-1.5 cursor-pointer text-xs text-[#848E9C]">
            <input type="checkbox" checked={allowAutoIssue} onChange={(e) => onAutoIssueChange(e.target.checked)} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
            自动下发
          </label>
          <label className="flex items-center gap-1.5 cursor-pointer text-xs text-[#848E9C]">
            <input type="checkbox" checked={allowAutoCancel} onChange={(e) => onAutoCancelChange(e.target.checked)} className="rounded border-[#2B3139] bg-[#1E2329] text-[#F0B90B]" />
            自动撤单
          </label>
        </div>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-[#848E9C] border-b border-[#2B3139]">
              <th className="px-2 py-2 w-8" aria-label="展开" />
              <th className="px-4 py-2">标的</th>
              <th className="px-4 py-2">强度</th>
              <th className="px-4 py-2">评分</th>
              <th className="px-4 py-2">市况</th>
              <th className="px-4 py-2">可靠度</th>
              <th className="px-4 py-2">时机</th>
              <th className="px-4 py-2">量价</th>
            </tr>
          </thead>
          <tbody>
            {items.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-6 text-center text-[#5E6673]">当前无{title.includes('多') ? '做多' : '做空'}候选</td>
              </tr>
            ) : (
              items.map((row, i) => (
                <React.Fragment key={`${row.symbol}-${i}`}>
                  <tr className="border-b border-[#2B3139]/50 hover:bg-[#2B3139]/30">
                    <td className="px-2 py-2">
                      <button type="button" onClick={() => toggle(row.symbol)} className="p-0.5 text-[#848E9C] hover:text-[#F0B90B]">
                        {expanded.has(row.symbol) ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                      </button>
                    </td>
                    <td className="px-4 py-2 text-[#EAECEF] font-medium">{row.symbol}</td>
                    <td className="px-4 py-2 text-[#EAECEF]">{row.strength_pct}%</td>
                    <td className="px-4 py-2 text-[#EAECEF]">{row.score.toFixed(2)}</td>
                    <td className="px-4 py-2 text-[#848E9C]">{row.market_condition}</td>
                    <td className="px-4 py-2 text-[#EAECEF]">{row.reliability_pct}%</td>
                    <td className="px-4 py-2 text-[#848E9C]">{row.timing}</td>
                    <td className="px-4 py-2 text-[#EAECEF]">{row.volume_price_pct}%</td>
                  </tr>
                  {expanded.has(row.symbol) && (
                    <tr className="border-b border-[#2B3139]/50 bg-[#181A20]">
                      <td colSpan={8} className="px-4 py-3">
                        <div className="rounded-lg border border-[#2B3139] bg-[#1E2329] p-3 grid grid-cols-2 sm:grid-cols-3 gap-2 text-xs">
                          <div><span className="text-[#5E6673]">强度</span><span className="ml-2 text-[#EAECEF]">{row.strength_pct}%</span></div>
                          <div><span className="text-[#5E6673]">评分</span><span className="ml-2 text-[#EAECEF]">{row.score.toFixed(2)}</span></div>
                          <div><span className="text-[#5E6673]">市况</span><span className="ml-2 text-[#848E9C]">{row.market_condition}</span></div>
                          <div><span className="text-[#5E6673]">可靠度</span><span className="ml-2 text-[#EAECEF]">{row.reliability_pct}%</span></div>
                          <div><span className="text-[#5E6673]">时机</span><span className="ml-2 text-[#848E9C]">{row.timing}</span></div>
                          <div><span className="text-[#5E6673]">量价</span><span className="ml-2 text-[#EAECEF]">{row.volume_price_pct}%</span></div>
                          {row.from_ai && <div className="col-span-2 text-[#F0B90B]/80">来源：AI 预测</div>}
                        </div>
                      </td>
                    </tr>
                  )}
                </React.Fragment>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

// 统一用大写作为 key，避免方向池与 AI 返回的 symbol 大小写不一致导致查不到实时数据
function normSymbol(s: string) {
  return (s || '').toUpperCase()
}

/**
 * 实时数据 + AI 预测 合并展示
 * 开仓逻辑：① 过滤机制后系统按实时数据判定进方向池（多/空）② 实时数据传 AI，AI 输出建议开仓 + 预测方向
 * ③ 系统再按实时数据判定方向 ④ 三条件共振（实时方向、AI 预测方向、AI 建议开仓）才开仓，缺一不可
 * 若某币显示「仅AI」且无实时数据：多为该币未进入当前周期的方向池，或方向池被后续周期覆盖（见「综合」列提示）。
 */
function RealtimeAndAIPanel({
  directionPool,
  latestAnalysis,
}: {
  directionPool: DirectionPoolResponse | undefined
  latestAnalysis: LatestAnalysisResponse | undefined
}) {
  const formatUpdatedAt = (s: string) => {
    const raw = String(s || '').trim()
    if (!raw) return ''
    // backend may return RFC3339, or legacy "YYYY-MM-DD HH:mm:ss UTC"
    const isoLike = raw.includes('T') ? raw : raw.replace(' UTC', 'Z').replace(' ', 'T')
    const d = new Date(isoLike)
    if (Number.isNaN(d.getTime())) return raw
    const local = d.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
    const diffMs = Date.now() - d.getTime()
    const diffMin = Math.max(0, Math.floor(diffMs / 60000))
    const ago = diffMin <= 1 ? '刚刚' : `${diffMin} 分钟前`
    return `${local}（${ago}）`
  }
  const longMap = new Map((directionPool?.long ?? []).map((i) => [normSymbol(i.symbol), { ...i, side: '多' as const }]))
  const shortMap = new Map((directionPool?.short ?? []).map((i) => [normSymbol(i.symbol), { ...i, side: '空' as const }]))
  const aiMap = new Map((latestAnalysis?.symbol_predictions ?? []).map((p) => [normSymbol(p.symbol), p]))
  const symbols = Array.from(new Set([...longMap.keys(), ...shortMap.keys(), ...aiMap.keys()])).sort()
  return (
    <div className="rounded-lg border border-[#2B3139] bg-[#1E2329]/50 overflow-hidden">
      <div className="px-4 py-3 border-b border-[#2B3139] flex items-center justify-between flex-wrap gap-2">
        <div>
          <h3 className="text-sm font-medium text-[#EAECEF]">实时数据 + AI 预测</h3>
          <p className="text-[11px] text-[#5E6673] mt-0.5">开仓须三条件共振：实时方向、AI 预测方向、AI 建议开仓；缺一不可</p>
        </div>
        <div className="flex items-center gap-3 text-xs text-[#848E9C]">
          {latestAnalysis?.market_regime && <span>市况: {latestAnalysis.market_regime}</span>}
          {latestAnalysis?.scenario && <span>场景: {latestAnalysis.scenario}</span>}
          {latestAnalysis?.updated_at && <span title={latestAnalysis.updated_at}>更新: {formatUpdatedAt(latestAnalysis.updated_at)}</span>}
          {latestAnalysis?.risk_alert && <span className="text-amber-400">风险提示</span>}
        </div>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-[#848E9C] border-b border-[#2B3139]">
              <th className="px-4 py-2">币种</th>
              <th className="px-4 py-2">实时方向</th>
              <th className="px-4 py-2">实时强度</th>
              <th className="px-4 py-2">可靠度</th>
              <th className="px-4 py-2">AI 预测方向</th>
              <th className="px-4 py-2">AI 置信度</th>
              <th className="px-4 py-2">建议开仓</th>
              <th className="px-4 py-2">综合</th>
            </tr>
          </thead>
          <tbody>
            {symbols.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-4 py-6 text-center text-[#5E6673]">暂无数据（需启动交易员并等待周期更新）</td>
              </tr>
            ) : (
              symbols.map((symbol) => {
                const realLong = longMap.get(symbol)
                const realShort = shortMap.get(symbol)
                const real = realLong ?? realShort
                const ai = aiMap.get(symbol)
                const displaySymbol = real?.symbol ?? ai?.symbol ?? symbol
                const realDir = real ? real.side : '—'
                const aiDir = ai?.predicted_direction === 'up' ? '多' : ai?.predicted_direction === 'down' ? '空' : ai?.predicted_direction === 'neutral' ? '中性' : '—'
                const directionAlign = real && ai && ((real.side === '多' && ai.predicted_direction === 'up') || (real.side === '空' && ai.predicted_direction === 'down'))
                const suggestOpen = ai?.suggest_open === true
                const resonance = directionAlign && suggestOpen
                const summary = resonance ? '共振' : real && ai ? (directionAlign ? '缺建议开仓' : '观望') : real ? '仅实时' : '仅AI'
                const summaryTitle = summary === '仅AI' ? '该币种在 AI 预测中但未进入本周期方向池（未通过实时过滤），不参与开仓' : summary === '仅实时' ? '无本周期 AI 预测，不参与开仓' : summary === '共振' ? '实时方向、AI 预测方向、AI 建议开仓 三条件共振，可参与开仓' : summary === '缺建议开仓' ? '方向一致但 AI 未建议开仓，不参与开仓' : '实时与 AI 均有，方向待定'
                return (
                  <tr key={symbol} className="border-b border-[#2B3139]/50 hover:bg-[#2B3139]/30">
                    <td className="px-4 py-2 text-[#EAECEF] font-medium">{displaySymbol}</td>
                    <td className="px-4 py-2 text-[#EAECEF]">{realDir}</td>
                    <td className="px-4 py-2 text-[#EAECEF]">{real ? `${real.strength_pct}%` : '—'}</td>
                    <td className="px-4 py-2 text-[#EAECEF]">{real ? `${real.reliability_pct}%` : '—'}</td>
                    <td className="px-4 py-2 text-[#848E9C]">{aiDir}</td>
                    <td className="px-4 py-2 text-[#EAECEF]">{ai?.confidence != null ? ai.confidence : '—'}</td>
                    <td className="px-4 py-2">{suggestOpen ? <span className="text-emerald-400">是</span> : '—'}</td>
                    <td className="px-4 py-2" title={summaryTitle}>
                      <span className={summary === '共振' ? 'text-[#F0B90B]' : summary === '仅实时' ? 'text-[#848E9C]' : summary === '缺建议开仓' ? 'text-amber-500' : 'text-[#5E6673]'}>{summary}</span>
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
      {latestAnalysis?.market_summary && (
        <div className="px-4 py-2 border-t border-[#2B3139] text-xs text-[#5E6673]">
          {latestAnalysis.market_summary}
        </div>
      )}
    </div>
  )
}
