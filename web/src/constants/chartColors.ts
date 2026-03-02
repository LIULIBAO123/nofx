/**
 * Recharts / 图表专用颜色常量（模板 Meridian 规范，OKLCH）
 * 用于净值曲线、K 线、柱状图等，与 tailwind 语义色一致
 */
export const CHART_COLORS = {
  /** 主色 - 折线/面积 */
  teal: 'oklch(0.78 0.16 182)',
  tealMuted: 'oklch(0.78 0.16 182 / 0.3)',
  azure: 'oklch(0.68 0.14 245)',
  amber: 'oklch(0.76 0.14 75)',
  rose: 'oklch(0.62 0.22 18)',
  /** 收益/亏损 */
  gain: 'oklch(0.76 0.16 162)',
  loss: 'oklch(0.62 0.22 18)',
  /** 网格、刻度、背景 */
  grid: 'oklch(0.24 0.01 260)',
  tick: 'oklch(0.50 0.015 260)',
  surface: 'oklch(0.175 0.01 260)',
} as const

/** 用于 Recharts 的 stroke/fill（部分库需要 hex） */
export const CHART_COLORS_HEX = {
  teal: '#14b8a6',
  gain: '#34d399',
  loss: '#f43f5e',
  grid: '#2B3139',
  tick: '#848E9C',
}
