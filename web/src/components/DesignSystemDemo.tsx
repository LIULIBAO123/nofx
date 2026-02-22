/**
 * Design System Demo Page
 * 展示 Meridian 风格的设计系统组件
 */

import React from 'react';
import { motion } from 'framer-motion';
import { 
  TrendingUp, 
  TrendingDown, 
  DollarSign, 
  Activity,
  BarChart3,
  PieChart,
  LineChart,
  Zap
} from 'lucide-react';
import { KpiCard, SectionPanel, SectionHeader, GrainOverlay } from '../components/ui';
import { formatCompactNumber, formatPercentage } from '../lib/designUtils';

export function DesignSystemDemo() {
  // 模拟数据
  const kpiData = [
    {
      label: '总资产',
      value: '$2,847,392',
      change: 12.5,
      trend: 'up' as const,
      icon: DollarSign,
    },
    {
      label: '今日收益',
      value: '+$24,891',
      change: 8.2,
      trend: 'up' as const,
      icon: TrendingUp,
    },
    {
      label: '活跃策略',
      value: '18',
      change: -2.1,
      trend: 'down' as const,
      icon: Activity,
    },
    {
      label: '胜率',
      value: '68.4%',
      change: 3.7,
      trend: 'up' as const,
      icon: BarChart3,
    },
  ];

  return (
    <div className="min-h-screen bg-slate-950 text-slate-50 relative overflow-hidden">
      {/* Grain Overlay */}
      <GrainOverlay />
      
      {/* Background Gradient */}
      <div className="fixed inset-0 bg-gradient-meridian opacity-50 pointer-events-none" />
      
      {/* Content */}
      <div className="relative z-10 container mx-auto px-4 py-8 max-w-7xl">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          className="mb-12"
        >
          <h1 className="text-5xl font-display font-bold mb-3 text-gradient-teal">
            Meridian Design System
          </h1>
          <p className="text-lg text-slate-400">
            现代化的 Cyan/Teal 主题设计系统，灵感来自 Meridian Financial Dashboard
          </p>
        </motion.div>

        {/* KPI Cards Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-12">
          {kpiData.map((kpi, index) => (
            <KpiCard
              key={index}
              label={kpi.label}
              value={kpi.value}
              change={kpi.change}
              changeLabel="vs 昨日"
              icon={kpi.icon}
              trend={kpi.trend}
            />
          ))}
        </div>

        {/* Section Panels */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-12">
          <SectionPanel>
            <SectionHeader
              title="组合表现"
              subtitle="过去 30 天的收益趋势"
            />
            <div className="space-y-4">
              <div className="flex items-center justify-between p-4 rounded-lg bg-slate-900/50 border border-slate-800">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400">
                    <TrendingUp className="w-5 h-5" />
                  </div>
                  <div>
                    <div className="text-sm text-slate-400">月度收益</div>
                    <div className="text-xl font-bold font-mono">+18.7%</div>
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-sm text-slate-400">排名</div>
                  <div className="text-xl font-bold text-teal-400">#12</div>
                </div>
              </div>

              <div className="flex items-center justify-between p-4 rounded-lg bg-slate-900/50 border border-slate-800">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg bg-rose-500/10 text-rose-400">
                    <TrendingDown className="w-5 h-5" />
                  </div>
                  <div>
                    <div className="text-sm text-slate-400">最大回撤</div>
                    <div className="text-xl font-bold font-mono">-8.2%</div>
                  </div>
                </div>
                <div className="text-right">
                  <div className="text-sm text-slate-400">恢复时间</div>
                  <div className="text-xl font-bold text-slate-300">3天</div>
                </div>
              </div>
            </div>
          </SectionPanel>

          <SectionPanel>
            <SectionHeader
              title="风险指标"
              subtitle="实时风险监控"
            />
            <div className="space-y-4">
              {[
                { label: 'Sharpe Ratio', value: '2.34', status: 'good' },
                { label: 'Sortino Ratio', value: '3.12', status: 'good' },
                { label: 'Beta', value: '0.87', status: 'neutral' },
                { label: 'VaR (95%)', value: '-$12,450', status: 'warning' },
              ].map((metric, index) => (
                <div
                  key={index}
                  className="flex items-center justify-between p-3 rounded-lg bg-slate-900/30 border border-slate-800/50 hover:border-teal-500/30 transition-colors"
                >
                  <span className="text-sm text-slate-400">{metric.label}</span>
                  <span className={`text-lg font-mono font-semibold ${
                    metric.status === 'good' ? 'text-emerald-400' :
                    metric.status === 'warning' ? 'text-amber-400' :
                    'text-slate-300'
                  }`}>
                    {metric.value}
                  </span>
                </div>
              ))}
            </div>
          </SectionPanel>
        </div>

        {/* Button Showcase */}
        <SectionPanel className="mb-12">
          <SectionHeader
            title="按钮样式"
            subtitle="不同状态和变体的按钮"
          />
          <div className="flex flex-wrap gap-4">
            <button className="px-6 py-3 rounded-lg btn-teal-gradient font-semibold transition-all">
              Teal Primary
            </button>
            <button className="px-6 py-3 rounded-lg btn-gold-gradient font-semibold transition-all">
              Gold Legacy
            </button>
            <button className="px-6 py-3 rounded-lg bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 font-semibold hover:bg-emerald-500/20 transition-all">
              Success
            </button>
            <button className="px-6 py-3 rounded-lg bg-rose-500/10 text-rose-400 border border-rose-500/30 font-semibold hover:bg-rose-500/20 transition-all">
              Danger
            </button>
            <button className="px-6 py-3 rounded-lg bg-slate-800/50 text-slate-300 border border-slate-700 font-semibold hover:border-teal-500/50 hover:text-teal-400 transition-all">
              Outline
            </button>
          </div>
        </SectionPanel>

        {/* Badge Showcase */}
        <SectionPanel className="mb-12">
          <SectionHeader
            title="徽章样式"
            subtitle="状态标签和标记"
          />
          <div className="flex flex-wrap gap-3">
            <span className="badge-modern badge-teal">
              <Zap className="w-3 h-3" />
              Teal Badge
            </span>
            <span className="badge-modern badge-gold">
              <Zap className="w-3 h-3" />
              Gold Badge
            </span>
            <span className="badge-modern" style={{
              background: 'oklch(0.65 0.18 145 / 0.15)',
              borderColor: 'oklch(0.65 0.18 145 / 0.4)',
              color: 'oklch(0.65 0.18 145)',
            }}>
              Success
            </span>
            <span className="badge-modern" style={{
              background: 'oklch(0.60 0.22 12 / 0.15)',
              borderColor: 'oklch(0.60 0.22 12 / 0.4)',
              color: 'oklch(0.60 0.22 12)',
            }}>
              Danger
            </span>
          </div>
        </SectionPanel>

        {/* Typography Showcase */}
        <SectionPanel>
          <SectionHeader
            title="排版系统"
            subtitle="字体和文本样式"
          />
          <div className="space-y-6">
            <div>
              <h1 className="text-4xl font-display font-bold mb-2">
                Display Font - Outfit
              </h1>
              <p className="text-slate-400">用于标题和重要信息</p>
            </div>
            <div>
              <p className="text-lg font-body mb-2">
                Body Font - DM Sans
              </p>
              <p className="text-slate-400">用于正文和描述文本</p>
            </div>
            <div>
              <p className="text-lg font-mono mb-2">
                Mono Font - JetBrains Mono
              </p>
              <p className="text-slate-400">用于数字、代码和数据</p>
            </div>
            <div className="pt-4 border-t border-slate-800">
              <p className="text-3xl font-bold text-gradient-teal mb-2">
                渐变文字效果
              </p>
              <p className="text-slate-400">使用 OKLCH 颜色空间的流畅渐变</p>
            </div>
          </div>
        </SectionPanel>
      </div>
    </div>
  );
}







