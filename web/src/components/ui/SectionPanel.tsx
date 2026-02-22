import React, { ReactNode } from 'react';
import { motion } from 'framer-motion';

interface SectionPanelProps {
  children: ReactNode;
  className?: string;
  noPadding?: boolean;
}

export function SectionPanel({ children, className = '', noPadding = false }: SectionPanelProps) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
      className={`modern-card ${noPadding ? '' : 'p-6'} ${className}`}
    >
      {children}
    </motion.div>
  );
}

interface SectionHeaderProps {
  title: string;
  subtitle?: string;
  action?: ReactNode;
  className?: string;
}

export function SectionHeader({ title, subtitle, action, className = '' }: SectionHeaderProps) {
  return (
    <div className={`flex items-start justify-between mb-6 ${className}`}>
      <div>
        <h2 className="text-xl font-bold text-slate-50 mb-1">{title}</h2>
        {subtitle && (
          <p className="text-sm text-slate-400">{subtitle}</p>
        )}
      </div>
      {action && <div>{action}</div>}
    </div>
  );
}







