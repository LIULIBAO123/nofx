// AI Trade Analysis API functions

import { httpClient } from './httpClient'
import type { TradeAnalysis } from '../types'

export interface AnalyzeTradeRequest {
  run_id: string
  trade_id: number
}

export interface AnalyzeTradeResponse extends TradeAnalysis {}

/**
 * Analyze a specific trade using AI
 */
export async function analyzeTrade(
  runId: string,
  tradeId: number
): Promise<TradeAnalysis> {
  const result = await httpClient.post<AnalyzeTradeResponse>('/api/backtest/analyze-trade', {
    run_id: runId,
    trade_id: tradeId,
  })
  if (!result.success || !result.data) {
    throw new Error(result.message || 'Failed to analyze trade')
  }
  return result.data
}

/**
 * Get existing analysis for a trade
 */
export async function getTradeAnalysis(
  runId: string,
  tradeId: number
): Promise<TradeAnalysis | null> {
  try {
    const result = await httpClient.get<TradeAnalysis>(
      `/api/backtest/trade-analysis?run_id=${runId}&trade_id=${tradeId}`
    )
    if (!result.success || !result.data) {
      return null
    }
    return result.data
  } catch (error) {
    return null
  }
}

/**
 * Get rating color based on rating value
 */
export function getRatingColor(rating: string): string {
  switch (rating) {
    case 'excellent':
      return '#0ECB81' // Green
    case 'good':
      return '#F0B90B' // Yellow
    case 'fair':
      return '#848E9C' // Gray
    case 'poor':
      return '#F6465D' // Red
    default:
      return '#848E9C'
  }
}

/**
 * Get rating text based on rating value
 */
export function getRatingText(rating: string, language: 'en' | 'zh' = 'en'): string {
  const texts = {
    en: {
      excellent: 'Excellent',
      good: 'Good',
      fair: 'Fair',
      poor: 'Poor',
    },
    zh: {
      excellent: '优秀',
      good: '良好',
      fair: '一般',
      poor: '较差',
    },
  }
  return texts[language][rating as keyof typeof texts.en] || rating
}

/**
 * Get rating icon based on rating value
 */
export function getRatingIcon(rating: string): string {
  switch (rating) {
    case 'excellent':
      return '⭐⭐⭐'
    case 'good':
      return '⭐⭐'
    case 'fair':
      return '⭐'
    case 'poor':
      return '⚠️'
    default:
      return '—'
  }
}

