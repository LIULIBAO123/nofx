import useSWR from 'swr'
import { Database, Zap } from 'lucide-react'
import { api } from '../lib/api'
import type { AIUsage } from '../types'

export function AIUsageCard({
  language,
  runId,
  traderId,
  context,
}: {
  language: string
  runId?: string
  /** 实盘/实盘模拟：传入当前交易员 ID，仅显示该交易员的 token 用量 */
  traderId?: string
  /** 策略 AI 测试：传入 'strategy_studio'，仅显示该场景的 token 用量 */
  context?: string
}) {
  const swrKey = ['ai-usage', runId ?? '', traderId ?? '', context ?? '']
  const fetcher = () => api.getAIUsage(runId, traderId, context)
  const { data: usage } = useSWR<AIUsage | null>(swrKey, fetcher, {
    refreshInterval: 10000,
    revalidateOnFocus: true,
  })

  const zh = language === 'zh'
  const labelToken = zh ? 'Token 用量' : 'Token usage'
  const labelCache = zh ? 'Prompt 缓存' : 'Prompt cache'
  const labelRead = zh ? '读取' : 'read'
  const labelCreated = zh ? '创建' : 'created'
  const labelNone = zh ? '暂无数据（完成一次 AI 调用后显示）' : 'No data yet (shown after an AI call)'

  if (!usage) {
    return (
      <div className="rounded-lg border border-white/10 bg-white/5 p-3 text-xs text-nofx-text-muted">
        <div className="flex items-center gap-2">
          <Zap className="w-3.5 h-3.5" />
          <span>{labelToken}</span>
        </div>
        <p className="mt-1.5">{labelNone}</p>
      </div>
    )
  }

  const hasCache = usage.cache_read_input_tokens > 0 || usage.cache_creation_input_tokens > 0
  const labelCacheHint = zh
    ? '（仅部分模型/场景返回；首轮或 prompt 变化时常为 0）'
    : '(only some models; often 0 on first run or when prompt changes)'

  return (
    <div className="rounded-lg border border-white/10 bg-white/5 p-3 text-xs">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-nofx-text-muted">
          <Zap className="w-3.5 h-3.5" />
          <span>{labelToken}</span>
        </div>
        <span className="font-mono text-nofx-text">
          {usage.provider} / {usage.model}
        </span>
      </div>
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-nofx-text-muted">
        <span>in: {usage.prompt_tokens}</span>
        <span>out: {usage.completion_tokens}</span>
        <span>total: {usage.total_tokens}</span>
      </div>
      <div className={`mt-2 flex flex-wrap items-center gap-2 rounded border px-2 py-1.5 ${hasCache ? 'border-nofx-gold/20 bg-nofx-gold/5' : 'border-white/10 bg-white/5'}`}>
        <Database className={`w-3.5 h-3.5 flex-shrink-0 ${hasCache ? 'text-nofx-gold' : 'text-nofx-text-muted'}`} />
        <span className={hasCache ? 'text-nofx-gold font-medium' : 'text-nofx-text-muted'}>{labelCache}:</span>
        <span className="font-mono text-nofx-text-muted">
          {labelRead} {usage.cache_read_input_tokens} · {labelCreated} {usage.cache_creation_input_tokens}
        </span>
        {!hasCache && <span className="text-nofx-text-muted text-[10px]">{labelCacheHint}</span>}
      </div>
    </div>
  )
}
