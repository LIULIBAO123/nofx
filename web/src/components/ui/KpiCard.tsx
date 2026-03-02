import React from 'react';
import { motion } from 'framer-motion';
import { LucideIcon } from 'lucide-react';

interface KpiCardProps {
  label: string;
  value: string | number;
  change?: number;
  changeLabel?: string;
  icon?: LucideIcon;
  trend?: 'up' | 'down' | 'neutral';
  loading?: boolean;
  className?: string;
}

export function KpiCard({
  label,
  value,
  change,
  changeLabel,
  icon: Icon,
  trend = 'neutral',
  loading = false,
  className = '',
}: KpiCardProps) {
  const trendColor = {
    up: 'text-fin-gain',
    down: 'text-fin-loss',
    neutral: 'text-slate-400',
  }[trend];

  const trendBg = {
    up: 'bg-fin-gain/10',
    down: 'bg-fin-loss/10',
    neutral: 'bg-slate-500/10',
  }[trend];

  if (loading) {
    return (
      <div className={`modern-card p-6 ${className}`}>
        <div className="space-y-3">
          <div className="h-4 w-24 skeleton-modern" />
          <div className="h-8 w-32 skeleton-modern" />
          <div className="h-3 w-20 skeleton-modern" />
        </div>
      </div>
    );
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
      className={`modern-card rounded-2xl p-4 lg:p-5 group ${className}`}
    >
      <div className="flex items-start justify-between mb-4">
        <div className="text-sm font-bold font-display text-slate-400 uppercase tracking-[0.08em]">
          {label}
        </div>
        {Icon && (
          <div className="p-2.5 rounded-xl bg-primary/10 text-primary group-hover:bg-primary/20 transition-colors duration-200">
            <Icon className="w-4 h-4" />
          </div>
        )}
      </div>

      <div className="space-y-2">
        <div className="text-2xl lg:text-3xl font-bold font-mono text-slate-50 tracking-tighter">
          {value}
        </div>

        {change !== undefined && (
          <div className="flex items-center gap-2">
            <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-semibold ${trendBg} ${trendColor}`}>
              {trend === 'up' && '↑'}
              {trend === 'down' && '↓'}
              {Math.abs(change)}%
            </span>
            {changeLabel && (
              <span className="text-xs text-slate-500">{changeLabel}</span>
            )}
          </div>
        )}
      </div>
    </motion.div>
  );
}







