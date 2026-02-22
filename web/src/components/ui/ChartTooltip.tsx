import React from 'react';

interface ChartTooltipProps {
  active?: boolean;
  payload?: any[];
  label?: string;
  formatter?: (value: any, name: string) => [string, string];
}

export function ChartTooltipContent({ active, payload, label, formatter }: ChartTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  return (
    <div className="modern-tooltip bg-slate-900/95 backdrop-blur-xl border border-slate-700/50 rounded-lg p-3 shadow-xl">
      {label && (
        <div className="text-xs font-semibold text-slate-300 mb-2 pb-2 border-b border-slate-700/50">
          {label}
        </div>
      )}
      <div className="space-y-1.5">
        {payload.map((entry, index) => {
          const [displayValue, displayName] = formatter
            ? formatter(entry.value, entry.name)
            : [entry.value, entry.name];

          return (
            <div key={index} className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-2">
                <div
                  className="w-2 h-2 rounded-full"
                  style={{ backgroundColor: entry.color }}
                />
                <span className="text-xs text-slate-400">{displayName}</span>
              </div>
              <span className="text-xs font-mono font-semibold text-slate-50">
                {displayValue}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}







