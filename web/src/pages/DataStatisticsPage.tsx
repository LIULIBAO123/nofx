import React, { useMemo, useState } from 'react'
import useSWR from 'swr'
import { api } from '../lib/api'
import type { TraderInfo } from '../types'
import type {
  DataStatsResponse,
  CatalogByMechanism,
  ByMechanismSource,
  DataCallRecord,
} from '../types'

// 与后端 flowToMechanisms 一致：流程 → 所属机制列表，用于将最近调用归属到「二」的机制+数据源
const FLOW_TO_MECHANISMS: Record<string, string[]> = {
  '系统周期': ['过滤机制', '方向池', '系统周期'],
  '主周期': ['过滤机制', '方向池', 'AI实时+预测'],
  '止盈止损调整': ['止盈止损调整'],
}

interface DataStatisticsPageProps {
  traders?: TraderInfo[]
  selectedTraderId?: string
  onTraderSelect?: (id: string) => void
}

function formatTime(ms: number): string {
  if (!ms) return '—'
  const d = new Date(ms)
  return d.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export function DataStatisticsPage({
  traders = [],
  selectedTraderId,
  onTraderSelect,
}: DataStatisticsPageProps) {
  const [traderFilter, setTraderFilter] = useState<string>(selectedTraderId ?? '')
  const key = traderFilter ? `data-statistics-${traderFilter}` : 'data-statistics'
  const { data, error, isLoading, mutate } = useSWR<DataStatsResponse>(key, () =>
    api.getDataStatistics(traderFilter || undefined)
  )

  // 按机制分组的「机制-数据源」统计，便于表格按机制展示
  const statsByMechanism = useMemo(() => {
    if (!data?.by_mechanism_source?.length) return new Map<string, ByMechanismSource[]>()
    const m = new Map<string, ByMechanismSource[]>()
    for (const row of data.by_mechanism_source) {
      const list = m.get(row.mechanism) ?? []
      list.push(row)
      m.set(row.mechanism, list)
    }
    return m
  }, [data?.by_mechanism_source])

  // 按 (机制, 数据源) 筛选出的最近调用，用于在「二」中每个统计行后展示
  const recentCallsByMechanismSource = useMemo(() => {
    const key = (m: string, s: string) => `${m}\0${s}`
    const map = new Map<string, DataCallRecord[]>()
    const recent = data?.recent_calls ?? []
    for (const r of recent) {
      const mechs = FLOW_TO_MECHANISMS[r.flow] ?? [r.flow]
      for (const m of mechs) {
        const k = key(m, r.source)
        const list = map.get(k) ?? []
        list.push(r)
        map.set(k, list)
      }
    }
    const maxPerGroup = 15
    map.forEach((list, k) => {
      list.sort((a, b) => b.at - a.at)
      map.set(k, list.slice(0, maxPerGroup))
    })
    return map
  }, [data?.recent_calls])

  // 按 (机制, 数据源, 数据项) 聚合的最近调用，用于「一」中每项折叠与详情
  const recentCallsByItem = useMemo(() => {
    const key = (m: string, s: string, d: string) => `${m}\0${s}\0${d}`
    const map = new Map<string, DataCallRecord[]>()
    const recent = data?.recent_calls ?? []
    for (const r of recent) {
      const mechs = FLOW_TO_MECHANISMS[r.flow] ?? [r.flow]
      for (const m of mechs) {
        const k = key(m, r.source, r.data_type)
        const list = map.get(k) ?? []
        list.push(r)
        map.set(k, list)
      }
    }
    map.forEach((list) => list.sort((a, b) => b.at - a.at))
    return map
  }, [data?.recent_calls])

  // 「一」中展开的机制 key；默认全部折叠（空 Set = 无展开）
  const [expandedMechanismKeys, setExpandedMechanismKeys] = useState<Set<string>>(new Set())
  const catalog = data?.catalog_by_mechanism ?? []
  const isMechanismExpanded = (mechName: string) => expandedMechanismKeys.has(mechName)
  const toggleMechanism = (mech: string) => {
    setExpandedMechanismKeys((prev) => {
      const next = new Set(prev)
      if (next.has(mech)) next.delete(mech)
      else next.add(mech)
      return next
    })
  }
  const expandAllMechanisms = () => {
    const all = new Set<string>()
    catalog.forEach((m: CatalogByMechanism) => all.add(m.mechanism))
    setExpandedMechanismKeys(all)
  }
  const collapseAllMechanisms = () => setExpandedMechanismKeys(new Set())
  // 「二」中展开的机制 key，默认全部折叠
  const [expandedSection2Keys, setExpandedSection2Keys] = useState<Set<string>>(new Set())
  const section2Mechanisms = Array.from(statsByMechanism.keys())
  const isSection2Expanded = (mech: string) => expandedSection2Keys.has(mech)
  const toggleSection2Mechanism = (mech: string) => {
    setExpandedSection2Keys((prev) => {
      const next = new Set(prev)
      if (next.has(mech)) next.delete(mech)
      else next.add(mech)
      return next
    })
  }
  const expandAllSection2 = () => setExpandedSection2Keys(new Set(section2Mechanisms))
  const collapseAllSection2 = () => setExpandedSection2Keys(new Set())

  // 「一」中展开的数据项 key 集合（机制\0数据源\0参数接口）
  const [expandedItemKeys, setExpandedItemKeys] = useState<Set<string>>(new Set())
  const toggleItem = (itemKey: string) => {
    setExpandedItemKeys((prev) => {
      const next = new Set(prev)
      if (next.has(itemKey)) next.delete(itemKey)
      else next.add(itemKey)
      return next
    })
  }

  return (
    <div className="max-w-[1600px] mx-auto px-4 py-6 space-y-8">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-xl font-bold text-white">数据统计</h1>
        <div className="flex items-center gap-3">
          {traders.length > 0 && (
            <select
              value={traderFilter}
              onChange={(e) => {
                const v = e.target.value
                setTraderFilter(v)
                onTraderSelect?.(v)
              }}
              className="bg-[#1E2329] border border-[#2B3139] rounded-lg px-3 py-2 text-sm text-[#EAECEF] focus:outline-none focus:border-[#F0B90B]"
            >
              <option value="">全部交易员</option>
              {traders.map((t) => (
                <option key={t.trader_id} value={t.trader_id}>
                  {t.trader_name}
                </option>
              ))}
            </select>
          )}
          <button
            type="button"
            onClick={() => mutate()}
            className="px-4 py-2 rounded-lg bg-[#2B3139] text-[#EAECEF] text-sm font-medium hover:bg-[#3a4149] transition-colors"
          >
            刷新
          </button>
        </div>
      </div>

      {error && (
        <div className="rounded-lg bg-red-500/10 border border-red-500/30 text-red-400 px-4 py-3 text-sm">
          {error.message}
        </div>
      )}
      {isLoading && (
        <div className="text-[#848E9C] text-sm">加载中…</div>
      )}
      {!isLoading && data && (
        <>
          {/* 一、按系统机制分类：默认折叠，可全部展开/收起 */}
          <section className="space-y-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="text-xl font-semibold text-white tracking-tight">
                  一、按系统机制分类
                </h2>
                <p className="text-sm text-[#848E9C] mt-1 leading-relaxed">
                  机制 → 数据源 → 数据项。默认折叠，点击机制标题展开/收起；每项可展开查看全部调用详情。
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={expandAllMechanisms}
                  className="text-xs px-3 py-1.5 rounded-lg bg-[#2B3139] text-[#B7BDC6] hover:bg-[#3a4149] hover:text-white transition-colors"
                >
                  全部展开
                </button>
                <button
                  type="button"
                  onClick={collapseAllMechanisms}
                  className="text-xs px-3 py-1.5 rounded-lg bg-[#2B3139] text-[#B7BDC6] hover:bg-[#3a4149] hover:text-white transition-colors"
                >
                  全部收起
                </button>
              </div>
            </div>
            <div className="space-y-3">
              {(data.catalog_by_mechanism ?? []).map((mech: CatalogByMechanism) => {
                const isMechExpanded = isMechanismExpanded(mech.mechanism)
                const sourceCount = (mech.sources ?? []).length
                const itemCount = (mech.sources ?? []).reduce((n, s) => n + (s.items?.length ?? 0), 0)
                const sourceNames = (mech.sources ?? []).map((s) => s.source).join(' · ')
                return (
                  <div
                    key={mech.mechanism}
                    className="rounded-xl border border-[#2B3139] bg-[#1E2329] overflow-hidden shadow-sm"
                  >
                    {/* 机制标题：可点击折叠 */}
                    <button
                      type="button"
                      onClick={() => toggleMechanism(mech.mechanism)}
                      className="w-full flex items-center gap-3 px-5 py-3.5 bg-[#252A32] hover:bg-[#2B3139] transition-colors text-left border-b border-[#2B3139]"
                    >
                      <span
                        className={`inline-flex shrink-0 w-5 h-5 items-center justify-center text-[#848E9C] transition-transform ${isMechExpanded ? 'rotate-90' : ''}`}
                        aria-hidden
                      >
                        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
                          <path d="M4 2l4 4-4 4" />
                        </svg>
                      </span>
                      <span className="text-base font-semibold text-[#F0B90B] tracking-tight">
                        {mech.mechanism}
                      </span>
                      <span className="text-sm text-[#5E6673] font-normal">
                        {sourceCount} 个数据源 · {itemCount} 项
                      </span>
                      {!isMechExpanded && sourceNames && (
                        <span className="text-xs text-[#5E6673] truncate max-w-[280px]" title={sourceNames}>
                          （{sourceNames}）
                        </span>
                      )}
                    </button>
                    {/* 机制内容：数据源 → 数据项 */}
                    {isMechExpanded && (
                      <div className="px-4 pb-4 pt-3">
                        {(mech.sources ?? []).map((sg, sgIndex) => (
                          <div
                            key={`${mech.mechanism}-${sg.source}`}
                            className={sgIndex > 0 ? 'mt-6' : ''}
                          >
                            <div className="text-sm font-medium text-[#B7BDC6] mb-2 border-l-2 border-[#3a4149] pl-2.5">
                              {sg.source}
                            </div>
                            <div className="rounded-lg border border-[#2B3139]/80 bg-[#0B0E11]/40 overflow-hidden">
                              <table className="w-full text-sm table-fixed">
                                <colgroup>
                                  <col className="w-[260px]" />
                                  <col className="w-[130px]" />
                                  <col />
                                  <col className="w-[72px]" />
                                </colgroup>
                                <thead>
                                  <tr className="text-left text-[#5E6673] text-xs font-medium border-b border-[#2B3139]/60 bg-[#1E2329]/60">
                                    <th className="py-2.5 pr-3 pl-3">参数/接口</th>
                                    <th className="py-2.5 pr-3">说明</th>
                                    <th className="py-2.5 pr-3">最近一次（时间 · 流程 · 结果）</th>
                                    <th className="py-2.5 text-center">操作</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {(sg.items ?? []).map((item, i) => {
                                    const itemKey = `${mech.mechanism}\0${sg.source}\0${item.data_type}`
                                    const calls = recentCallsByItem.get(itemKey) ?? []
                                    const last = calls[0]
                                    const isExpanded = expandedItemKeys.has(itemKey)
                                    return (
                                      <React.Fragment key={`${sg.source}-${item.data_type}-${i}`}>
                                        <tr className="border-b border-[#2B3139]/30 text-[#C7CDD6] align-top hover:bg-[#1E2329]/30 transition-colors">
                                          <td className="py-2.5 pr-3 pl-3 font-mono text-xs text-[#B7BDC6] break-all">
                                            {item.data_type}
                                          </td>
                                          <td className="py-2.5 pr-3 text-[#A0A8B3] text-xs">
                                            {item.desc}
                                          </td>
                                          <td className="py-2.5 pr-3 text-xs">
                                            {last ? (
                                              <span className="text-[#848E9C]">
                                                {formatTime(last.at)} · {last.flow} ·{' '}
                                                {last.success ? (
                                                  <span className="text-green-400 font-medium">成功</span>
                                                ) : (
                                                  <span className="text-red-400 font-medium">失败</span>
                                                )}
                                                {last.err_msg && (
                                                  <span className="text-red-400/90">
                                                    {' '}
                                                    · {last.err_msg.slice(0, 40)}
                                                    {last.err_msg.length > 40 ? '…' : ''}
                                                  </span>
                                                )}
                                              </span>
                                            ) : (
                                              <span className="text-[#5E6673]">暂无调用记录</span>
                                            )}
                                          </td>
                                          <td className="py-2.5 text-center">
                                            <button
                                              type="button"
                                              onClick={() => toggleItem(itemKey)}
                                              className="text-xs px-2.5 py-1 rounded text-[#848E9C] hover:text-[#F0B90B] hover:bg-[#2B3139]/50 transition-colors"
                                            >
                                              {isExpanded ? '收起' : '展开'}
                                            </button>
                                          </td>
                                        </tr>
                                        {isExpanded && (
                                          <tr>
                                            <td colSpan={4} className="py-0 pb-3 align-top bg-[#0B0E11]/50">
                                              <div className="rounded-lg overflow-x-auto mx-2 mt-1 border border-[#2B3139]/50">
                                                <table className="w-full text-xs">
                                                  <thead>
                                                    <tr className="text-left text-[#5E6673] border-b border-[#2B3139]/50">
                                                      <th className="px-3 py-2">调用时间</th>
                                                      <th className="px-3 py-2">流程</th>
                                                      <th className="px-3 py-2">结果</th>
                                                      <th className="px-3 py-2">耗时(ms)</th>
                                                      <th className="px-3 py-2">错误信息</th>
                                                    </tr>
                                                  </thead>
                                                  <tbody>
                                                    {calls.length ? (
                                                      calls.map((r, j) => (
                                                        <tr key={j} className="border-b border-[#2B3139]/30 text-[#C7CDD6]">
                                                          <td className="px-3 py-2 whitespace-nowrap">{formatTime(r.at)}</td>
                                                          <td className="px-3 py-2">{r.flow}</td>
                                                          <td className="px-3 py-2">
                                                            {r.success ? (
                                                              <span className="text-green-400">成功</span>
                                                            ) : (
                                                              <span className="text-red-400">失败</span>
                                                            )}
                                                          </td>
                                                          <td className="px-3 py-2">{r.duration_ms ?? '—'}</td>
                                                          <td className="px-3 py-2 max-w-[320px] text-red-400 break-words">
                                                            {r.err_msg ? (
                                                              <span className="inline-flex items-start gap-1">
                                                                <span className="flex-1">{r.err_msg}</span>
                                                                <button
                                                                  type="button"
                                                                  onClick={(e) => {
                                                                    e.stopPropagation()
                                                                    navigator.clipboard.writeText(r.err_msg)
                                                                    const btn = e.currentTarget
                                                                    const orig = btn.textContent
                                                                    btn.textContent = '已复制'
                                                                    setTimeout(() => { btn.textContent = orig }, 1200)
                                                                  }}
                                                                  className="shrink-0 p-0.5 rounded text-[#848E9C] hover:text-[#F0B90B]"
                                                                  title="复制"
                                                                >
                                                                  复制
                                                                </button>
                                                              </span>
                                                            ) : (
                                                              '—'
                                                            )}
                                                          </td>
                                                        </tr>
                                                      ))
                                                    ) : (
                                                      <tr>
                                                        <td colSpan={5} className="px-3 py-3 text-center text-[#5E6673]">
                                                          暂无调用记录
                                                        </td>
                                                      </tr>
                                                    )}
                                                  </tbody>
                                                </table>
                                              </div>
                                            </td>
                                          </tr>
                                        )}
                                      </React.Fragment>
                                    )
                                  })}
                                </tbody>
                              </table>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </section>

          {/* 二、各机制下各数据源调用统计，默认折叠 */}
          <section className="space-y-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="text-xl font-semibold text-white tracking-tight">
                  二、各机制下各数据源调用统计
                </h2>
                <p className="text-sm text-[#848E9C] mt-1 leading-relaxed">
                  默认折叠。未成功时一眼可见；每行下方可展开该机制+数据源的最近调用详情；失败时可点击「复制」错误信息。
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={expandAllSection2}
                  className="text-xs px-3 py-1.5 rounded-lg bg-[#2B3139] text-[#B7BDC6] hover:bg-[#3a4149] hover:text-white transition-colors"
                >
                  全部展开
                </button>
                <button
                  type="button"
                  onClick={collapseAllSection2}
                  className="text-xs px-3 py-1.5 rounded-lg bg-[#2B3139] text-[#B7BDC6] hover:bg-[#3a4149] hover:text-white transition-colors"
                >
                  全部收起
                </button>
              </div>
            </div>
            <div className="rounded-xl border border-[#2B3139] bg-[#1E2329] overflow-hidden shadow-sm">
              {Array.from(statsByMechanism.entries()).map(([mechanism, rows]) => {
                const isExpanded = isSection2Expanded(mechanism)
                const hasFailure = rows.some((s) => s.total_calls > 0 && !s.last_success)
                return (
                  <div key={mechanism} className="border-b border-[#2B3139] last:border-b-0">
                    <button
                      type="button"
                      onClick={() => toggleSection2Mechanism(mechanism)}
                      className="w-full flex items-center gap-3 px-5 py-3 bg-[#252A32] hover:bg-[#2B3139] transition-colors text-left border-b border-[#2B3139]/60"
                    >
                      <span
                        className={`inline-flex shrink-0 w-5 h-5 items-center justify-center text-[#848E9C] transition-transform ${isExpanded ? 'rotate-90' : ''}`}
                        aria-hidden
                      >
                        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="2">
                          <path d="M4 2l4 4-4 4" />
                        </svg>
                      </span>
                      <span className="text-sm font-semibold text-[#EAECEF] tracking-tight">{mechanism}</span>
                      <span className="text-xs text-[#5E6673]">{rows.length} 个数据源</span>
                      {hasFailure && (
                        <span className="text-xs text-red-400 font-medium">存在失败</span>
                      )}
                    </button>
                    {isExpanded && (
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-left text-[#848E9C] border-b border-[#2B3139]">
                        <th className="px-4 py-2">数据源</th>
                        <th className="px-4 py-2">总调用</th>
                        <th className="px-4 py-2">成功</th>
                        <th className="px-4 py-2">成功率</th>
                        <th className="px-4 py-2">最后调用时间</th>
                        <th className="px-4 py-2">最后是否成功</th>
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((s) => {
                        const recentKey = `${mechanism}\0${s.source}`
                        const recentCalls = recentCallsByMechanismSource.get(recentKey) ?? []
                        return (
                          <React.Fragment key={`${s.mechanism}-${s.source}`}>
                            <tr
                              className={`border-b border-[#2B3139]/50 text-[#EAECEF] ${
                                s.total_calls > 0 && !s.last_success ? 'bg-red-500/5' : ''
                              }`}
                            >
                              <td className="px-4 py-2 font-medium">{s.source}</td>
                              <td className="px-4 py-2">{s.total_calls}</td>
                              <td className="px-4 py-2">{s.success_calls}</td>
                              <td className="px-4 py-2">
                                {s.total_calls > 0
                                  ? `${((s.success_calls / s.total_calls) * 100).toFixed(1)}%`
                                  : '—'}
                              </td>
                              <td className="px-4 py-2">{formatTime(s.last_call_at)}</td>
                              <td className="px-4 py-2">
                                {s.total_calls > 0 ? (
                                  s.last_success ? (
                                    <span className="text-green-400">成功</span>
                                  ) : (
                                    <span className="text-red-400 font-medium">失败</span>
                                  )
                                ) : (
                                  <span className="text-[#5E6673]">暂无调用</span>
                                )}
                              </td>
                            </tr>
                            {recentCalls.length > 0 && (
                              <tr>
                                <td colSpan={6} className="px-4 py-0 pb-3 pt-0 align-top">
                                  <div className="bg-[#0B0E11]/60 rounded-lg mt-1 overflow-x-auto">
                                    <div className="text-[#848E9C] text-xs px-3 py-1.5 border-b border-[#2B3139]/50">
                                      最近调用详情（该机制+数据源）
                                    </div>
                                    <table className="w-full text-xs">
                                      <thead>
                                        <tr className="text-left text-[#5E6673] border-b border-[#2B3139]/50">
                                          <th className="px-3 py-1.5">时间</th>
                                          <th className="px-3 py-1.5">流程</th>
                                          <th className="px-3 py-1.5">参数/接口</th>
                                          <th className="px-3 py-1.5">结果</th>
                                          <th className="px-3 py-1.5">耗时(ms)</th>
                                          <th className="px-3 py-1.5">错误信息</th>
                                        </tr>
                                      </thead>
                                      <tbody>
                                        {recentCalls.map((r, i) => (
                                          <tr key={i} className="border-b border-[#2B3139]/30 text-[#EAECEF]">
                                            <td className="px-3 py-1.5 whitespace-nowrap">{formatTime(r.at)}</td>
                                            <td className="px-3 py-1.5">{r.flow}</td>
                                            <td className="px-3 py-1.5 font-mono text-[#B7BDC6]">{r.data_type}</td>
                                            <td className="px-3 py-1.5">
                                              {r.success ? (
                                                <span className="text-green-400">成功</span>
                                              ) : (
                                                <span className="text-red-400">失败</span>
                                              )}
                                            </td>
                                            <td className="px-3 py-1.5">{r.duration_ms ?? '—'}</td>
                                            <td className="px-3 py-1.5 max-w-[320px] text-red-400 break-words align-top">
                                              {r.err_msg ? (
                                                <span className="inline-flex items-start gap-1">
                                                  <span className="flex-1">{r.err_msg}</span>
                                                  <button
                                                    type="button"
                                                    onClick={(e) => {
                                                      navigator.clipboard.writeText(r.err_msg)
                                                      const btn = e.currentTarget
                                                      const orig = btn.textContent
                                                      btn.textContent = '已复制'
                                                      setTimeout(() => { btn.textContent = orig }, 1200)
                                                    }}
                                                    className="shrink-0 p-0.5 rounded text-[#848E9C] hover:text-[#F0B90B] hover:bg-[#2B3139]"
                                                    title="复制完整错误信息"
                                                  >
                                                    复制
                                                  </button>
                                                </span>
                                              ) : (
                                                '—'
                                              )}
                                            </td>
                                          </tr>
                                        ))}
                                      </tbody>
                                    </table>
                                  </div>
                                </td>
                              </tr>
                            )}
                          </React.Fragment>
                        )
                      })}
                    </tbody>
                  </table>
                    )}
                  </div>
                )
              })}
              {statsByMechanism.size === 0 && (
                <div className="px-4 py-8 text-center text-[#848E9C] text-sm">
                  暂无统计（请启动交易员并运行若干周期后查看）
                </div>
              )}
            </div>
          </section>
        </>
      )}
    </div>
  )
}
